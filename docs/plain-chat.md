# 对话

## 范围

入口 `/home/chat`，JWT 登录保护。使用本人配置的对话模型，可切换本次模型而不改变默认配置。支持多轮、流式回复、停止、复制、手动重试；未完成的回复不进入后续上下文，新建/切换模型清空消息需确认；页面卸载取消请求。

P1 起对话只有一条链路：**检索是常驻工具，由模型自主决定是否调用**。不再区分「普通对话」与「知识库对话」，前端也不再有「知识库检索」开关。

## 服务链路

浏览器 POST `/api/chat` → Go JWT `/chat` → Mastra `/chat` → 配置中的供应商（OpenAI 兼容协议直连）。

- Go 根据用户与 `provider_models.id` 解析模型和凭据（0 表示默认），每轮解密后放进 turn context 的 `model` 字段；浏览器与 Mastra 都不持久化供应商密钥。
- Mastra 使用唯一 Agent（id `chat`）：单份系统提示词声明身份、何时检索、如何检索（每轮最多三次）与如何回答（`[n]` 引用、部分可答、无命中如实告知）；`search_knowledge` 工具常驻，`maxSteps=8`、`maxRetries=0`。
- 工具执行：Mastra 带每轮 capability POST Go `/internal/tools/v1/search`（body `{query}`），Go 按用户设置取 TopK/阈值并只检索该用户自己的知识库。capability 为随机 256-bit 单次只读凭证，请求完成、取消或超时即撤销；工具失败由模型如实告知，不中断回答。
- 事件流为 `application/x-ndjson`：`delta`（增量文本）、`meta`（模型与会话 id）、`sources`（工具检索结果，按 `knowledge_id:section` 去重累积）、`done`（`message_id`、供应商 token 用量 `usage`）、`error`（流内错误）。Go 逐行中继并保存检索溯源；`sources` 编号与回答中的 `[n]` 引用一致。
- 多步工具调用会产生多条 assistant 消息，会话落库取最后一条作为可见回复（`persistedAssistantId`）。
- 中断的多步回答可能只留下 reasoning/工具调用、没有可见文本的 assistant 消息。历史接口不返回这类轮次；Go 在发送前也会丢弃它们并合并相邻用户消息，保证严格的 user/assistant 交替，恢复历史后继续对话不会因空内容报错。

## 安全与边界

- 供应商地址只允许 http/https，拒绝内嵌凭据、查询参数与片段；解析模型配置时拒绝私网、回环、链路本地与云元数据地址，除非部署侧 `MODEL_ALLOWED_PRIVATE_ENDPOINTS` 精确列出该地址；公网模型必须使用 HTTPS。
- DSML 防护：部分网关不解析工具调用标记，若模型把 `<｜｜DSML｜｜ calls>` 当正文输出，Mastra 在首 600 字符内识别并中止，返回可操作的流内错误，不把原始标记转给浏览器。
- 错误正文不透传第三方响应，避免凭据泄漏；Mastra 只监听本机，入站校验 `INTERNAL_API_SECRET`。

架构背景与后续阶段见 [ai-architecture.md](./ai-architecture.md)。

## 启动

本机 MySQL、Go、Vite 保持原有 `pnpm dev` 流程；另一个终端执行：

```bash
pnpm dev:ai
```

默认 AI URL 为 http://127.0.0.1:4111，内部 Go 回调为 http://127.0.0.1:9000。修改端口时在 Go 环境设置 CHAT_AI_URL、CHAT_BACKEND_URL；AI 进程也需同样的 CHAT_BACKEND_URL（该脚本不会读取根 .env.development）。Mastra 固定本机监听，不将开发 Studio 暴露公网。

本地无认证模型的例子：部署环境变量 MODEL_ALLOWED_PRIVATE_ENDPOINTS=http://127.0.0.1:11434/v1，并在用户配置中使用完全相同的地址。该变量逗号分隔；不得配置宽泛通配符。仍需先设置 MODEL_CONFIG_ENCRYPTION_KEY 才能使用加密保存的 API Key。
