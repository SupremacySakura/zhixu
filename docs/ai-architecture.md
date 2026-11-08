# AI 架构：BFF / AI 平台 / 模型直连

> 状态：已确认（2026-11-04）。P0 契约已冻结，**P1、P2、P3 已落地**（2026-11-05，见「迁移阶段」）。
> 适用：`apps/zhixu-backend`（Go BFF）与 `apps/zhixu-ai`（Mastra AI 平台）。

## 背景

（以下为 P1 之前的现状，P1 已消除。）

P1 之前 Go 扮演「改写模型请求字段的中转站」：Mastra 的对话与向量化请求都要绕回 Go 网关 `/internal/chat/v1/*`，由 Go 做字段白名单转发；Go 又按「是否 RAG」选择 `/plain-chat` 或 `/rag-chat` 两条 Mastra 路由。由此产生三类问题：

1. **契约脆弱**：每新增一个模型能力字段（`tools`、`response_format` 等）都要改 Go 白名单，漏改即静默失效（曾导致 `tools` 被剥除、模型输出 DSML 文本）。
2. **职责重叠**：AI 语义（是否检索、用哪个 agent、多步工具调用）散落在 Go 与 Mastra 两侧。
3. **无扩展点**：接入第二个 provider、引入 AI 网关或改流式策略时，Go 与 Mastra 需要同时改。

## 决策

- Go 只做 **BFF**：鉴权、凭据加密保管与逐轮下发、业务数据、流式中继、用量记账。
- Mastra 拥有 **全部 AI 能力**：对话、工具、RAG、记忆、嵌入计算、流式生成，并**直连 provider**（OpenAI 兼容协议）。
- Mastra 对外只有两个 AI 路由：`POST /chat`（唯一对话入口）与 `POST /vectorize`（文本→向量）。RAG 只是 `/chat` 的一个常驻工具，**模型自主决定是否调用**；Go 与 Mastra 都不再区分「这轮是不是 RAG」。
- 现阶段**不部署独立 AI 网关**（LiteLLM/Portkey），但契约预留插入点：`base_url` 指向网关即可切换，无需改两端代码。

## 角色与职责

| 角色    | 归属     | 负责                                                                                                   | 不负责                                                               |
| ------- | -------- | ------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------- |
| BFF     | Go       | JWT 鉴权、限流、会话管理、流式中继、凭据加密保管与逐轮解析、业务数据（知识/分块/向量任务）、token 记账 | 解析模型请求体、字段白名单、AI 语义分支                              |
| AI 平台 | Mastra   | 对话编排、工具执行、RAG 检索编排、记忆、嵌入计算、模型直连与流式输出                                   | 持久化 provider 密钥、业务鉴权、业务数据写入（除自有 `mastra_*` 表） |
| AI 网关 | 暂不部署 | 多模型路由、重试降级、硬预算、防护与审计                                                               | —（单用户单 provider 阶段不需要）                                    |

## 运行时拓扑

```text
浏览器 ──JWT──▶ Go BFF ──turn context(含模型配置)──▶ Mastra /chat ──▶ Provider
   ▲                │                                    │
   └──── NDJSON ────┘◀────────── NDJSON 事件流 ──────────┘
                    │
Mastra 工具调用 ──capability──▶ Go /internal/tools/v1/search（数据所有者）

Go 向量化 worker ──turn context(embedding 配置)──▶ Mastra /vectorize ──▶ Provider
        │◀──────────────── {embeddings} ────────────────────┘
        └──▶ Qdrant 写入、分块状态、任务与重试（编排仍归 Go）
```

## 路由面

### Mastra（内部服务，仅监听本机）

| 路由               | 职责                                                                          |
| ------------------ | ----------------------------------------------------------------------------- |
| `POST /chat`       | 唯一对话入口。NDJSON 事件流。                                                 |
| `POST /vectorize`  | 文本→向量，无状态，`{embeddings}`。                                           |
| `/conversations/*` | 会话线程/消息读写，供 Go 的会话管理接口调用（存储管理路由，不属于 AI 语义）。 |

P1 已删除 `/plain-chat`、`/rag-chat`，只保留上表三个路由面。

### Go

| 路由                                     | 职责                                                                                          |
| ---------------------------------------- | --------------------------------------------------------------------------------------------- |
| `POST /chat`（鉴权）                     | 浏览器入口：解析模型与凭据 → 调 Mastra `/chat` → 原样中继事件流。                             |
| `GET/PATCH/DELETE /chat/conversations/*` | 会话列表/详情/改名/删除（经 Mastra 存储）。                                                   |
| `POST /internal/tools/v1/search`         | 检索工具后端（capability 鉴权，读用户自己的知识库）。P1 已由 `/internal/rag/v1/search` 更名。 |

## 契约 1：Turn 请求（Go → Mastra `/chat`）

实现见 `apps/zhixu-ai/src/lib/chat.ts`（`chatTurnSchema`，strict）：

```jsonc
{
  "version": 1,
  "model": {
    "protocol": "chat", // chat | responses
    "model_id": "deepseek-chat",
    "base_url": "https://api.deepseek.com", // 每轮由 Go 从加密配置解密后下发
    "api_key": "sk-...", // 仅内存使用，不落日志
  },
  "messages": [{ "role": "user", "content": "…" }], // 1–79 条；多轮历史由 Mastra Memory 注入。Go 发送前归一化：丢弃无可见文本的 assistant 轮次并合并相邻 user 消息
  "conversation": { "id": "uuid", "resource": "42", "title": "…" }, // 可选；缺省即临时对话
  "tools": {
    // 每轮固定存在，与"是否 RAG"无关
    "search_url": "http://127.0.0.1:9000/internal/tools/v1/search",
    "capability": "<64 位十六进制短时只读凭证>",
  },
}
```

- 无 `agent` 字段、无 RAG 开关：单一 agent、单一系统提示词；`search_knowledge` 工具常驻，模型自行决定调用次数（`MAX_STEPS` 对应上限）。
- 供应商密钥由 Go 每轮解密下发；Mastra 不持久化、不写日志。

## 契约 2：事件流（Mastra → Go → 浏览器，`application/x-ndjson`）

| 事件      | 时机                                                       | 字段                                                                                                     |
| --------- | ---------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `delta`   | 模型增量文本                                               | `text`                                                                                                   |
| `sources` | 每次工具返回检索结果后（多次检索逐次发送，按来源去重累积） | `query`, `items[{knowledge_id,title,section,content,score}]`                                             |
| `done`    | 正常结束                                                   | `message_id`（临时对话无此字段）、`usage{input_tokens,output_tokens,total_tokens}`（供应商未上报时省略） |
| `error`   | 流内错误                                                   | `message`（三态优先级：工具标记泄漏 > 模型调用失败 > 超时/停止）                                         |

要求：

1. `sources` 按 `knowledge_id:section` 去重保序，编号 `[n]` 与回答引用一致；模型未检索时在 `done` 前补发一次空列表，保持事件形状稳定。
2. 200 已提交后的错误只能以流内 `error` 表达；Go 只中继并在中断时取消上游。
3. Go 响应带 `Cache-Control: no-cache`、`X-Accel-Buffering: no`，逐行 flush；记录 TTFT。
4. `error` 触发后不得再来 `delta` 或 `done`。
5. `usage` 取供应商终止事件（多步时跨 step 累计），Go 按 assistant message 记账到 `chat_turn_usage`；记账失败只记日志，不改变已生成回复的交付；删除会话时用量行随溯源一并清除。

## 契约 3：工具调用

- 工具名 `search_knowledge`，参数 `{ query: string }`（1–200 字）。
- 执行：Mastra 带 `capability` POST `search_url`，body `{query}`（topK/阈值由 Go 按用户设置取）；返回 `{sources:[KnowledgeSource]}`，失败返回 `{sources:[], error}`，由模型按提示词如实告知，不中断流。
- **capability 语义**：每轮签发的短时（≤120s）单次凭证，只允许检索当前用户自己的知识库；请求结束/取消即撤销；放请求头，不进 URL 与日志。它授权的是「数据工具访问」，不代表该轮必须检索。
- 工具失败与无命中区分：失败是服务错误（提示稍后再试），无命中是「知识库里没有相关内容」（如实告知）。

## 契约 4：向量化（Go 编排 → Mastra `/vectorize`）

> 已落地（P2）。Go 直连 Mastra，不再经 `/internal/chat/v1` 转发 embedding。

```jsonc
// 请求
{ "model": { "protocol": "chat", "model_id": "bge-m3", "base_url": "https://api.deepseek.com", "api_key": "sk-..." },
  "dimension": 1024, "texts": ["分块文本…"] }
// 响应
{ "embeddings": [[0.01, ...], ...] }
```

- Mastra 只做「文本→向量」，无状态、不写库；超时 100s，取消透传，错误不泄漏 provider 响应。
- Go 保留全部入库编排：切块、`vectorize_tasks` 状态机、失败分块重试、Qdrant 写入、`vector_state` 对前端可见、操作日志、数量/维度/NaN 校验。
- 不把入库流水线搬进 Mastra 的理由：Qdrant 写入方唯一（Go），检索接口也在 Go；双写同一数据面会让 schema 归属分裂。

## 数据归属

| 数据                                                                             | 所有者 | 存储                                                       |
| -------------------------------------------------------------------------------- | ------ | ---------------------------------------------------------- |
| 用户、文件、知识、分块、向量任务、模型配置（密文）、操作日志、检索溯源、对话用量 | Go     | MySQL（`zhixu` 业务表，用量在 `chat_turn_usage`）          |
| 会话线程/消息（agent 记忆）                                                      | Mastra | MySQL `mastra_*` 表（单一事实源，`resourceId`=Go 用户 ID） |
| 向量                                                                             | Go     | Qdrant                                                     |
| 观测数据（span/trace）                                                           | Mastra | 本地 LibSQL/DuckDB                                         |

约定：一张表一个写入方；跨服务读取走 HTTP 接口，不直连对方表。

## 安全边界

1. 供应商密钥只在 Go 加密存储（`model_providers.ciphertext`），逐轮解密经本机内网下发，Mastra 仅内存使用；两端日志不得出现明文 key。
2. Mastra 默认只监听 `127.0.0.1`（后端在容器内时显式设 `MASTRA_HOST=0.0.0.0`），入站校验 `INTERNAL_API_SECRET`；出站由（`apps/zhixu-ai/src/lib/egress-guard.ts`）双层守卫：请求派发前校验目标源（字面 IP 经 URL 规范化后判定，覆盖每一跳重定向；仅 `CHAT_BACKEND_URL` 精确 origin 与 `EGRESS_ALLOWED_HOSTS` 可豁免，其余每一跳必须 HTTPS），主机名在 `connect.lookup` 连接时解析并拒绝私网、回环、链路本地、CGNAT、组播与云元数据地址。
3. capability 单次、短时、只读、按用户隔离，请求结束即撤销。
4. 工具只能访问该用户自己的知识数据；检索 SQL/向量查询必须带 `user_id`。

## 迁移阶段

| 阶段  | 内容                                                                                                                                                   | 验收                                                                                        | 回滚          |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------- | ------------- |
| P0    | 本文档（契约、错误码、数据归属）                                                                                                                       | 评审通过，不动代码                                                                          | —             |
| P1 ✅ | Mastra 合并 `/chat`（工具常驻、单提示词）；Go `Send` 改调 `/chat` 并删除 RAG 分支与 `/internal/chat/v1/*` 对话转发；`/internal/tools/v1/search` 更名   | 真机：寒暄直接答、知识问答走工具、`sources` 正常、DSML 不复现；`go test` + `pnpm test` 全绿 | 单提交 revert |
| P2 ✅ | `/vectorize` 直连；Go `MastraEmbedder` 去 gateway/capability；删 embeddings 转发路由与 handler；运行期地址策略静态化（私网/metadata 需精确 allowlist） | 向量化全链路 + 失败分块重试回归通过；`go test` + `pnpm test` 全绿                           | 单提交 revert |
| P3 ✅ | Mastra `done` 携带跨 step 累计的 `usage`；Go 落库 `chat_turn_usage`（随会话删除清理，记账失败不影响回复）；文档收尾                                    | 全量测试 + 文档与代码一致                                                                   | 单提交 revert |

## 与现状的差异清单

- 删除（P1 已完成）：`/plain-chat`、`/rag-chat`、Go 对话网关、按 RAG 选路由的分支、`cap.Tools`/`cap.MultiStep` 的网关用途、前端「知识库检索」开关。
- 删除（P2 已完成）：`/internal/chat/v1/*` embedding 转发路由与 handler、`MastraEmbedder` 的 gateway/capability 字段、`ChatCapability.Texts`/`claimed`、`ClaimCapability`、`ModelClient`（Go 不再拨号供应商，改为解析期静态地址校验）、Mastra `allowedGateway`。
- 变更：capability 从「网关凭证」变为「工具访问凭证」；`sources` 事件由「流水线前置检索」变为「工具调用返回后」；对话不再有「普通/知识库」两种入口。
- 新增（P3 已完成）：`done.usage` 与 Go 侧 `chat_turn_usage` 记账，配额拦截与页面展示留作扩展点。
- 不变：浏览器接口与事件码、会话存储、失败分块重试、Qdrant 检索语义。

## 后续扩展点（不在本次范围）

- 插入 AI 网关（LiteLLM/Portkey）：`base_url` 改指向网关，Go/Mastra 不改代码；届时由网关承担多模型路由与硬预算。
- 工具注册表升级为 MCP server：Mastra 侧工具定义改为远程 MCP，Go 提供 server。
- 用量配额与账单：Go 已有 `chat_turn_usage` 落库基础（P3），可增加配额拦截与页面展示。

## 参考

- WSO2《AI Gateway vs API Gateway》：https://wso2.com/api-platform/learn/ai-gateway-vs-api-gateway/
- WSO2《Streaming LLM Responses》（TTFT、流内错误、缓冲）：https://wso2.com/api-platform/learn/llm-streaming-responses/
- LiteLLM Virtual Keys / RBAC：https://docs.litellm.ai/docs/proxy/virtual_keys 、https://docs.litellm.ai/docs/proxy/access_control
