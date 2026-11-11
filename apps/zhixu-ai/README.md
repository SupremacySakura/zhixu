# zhixu-ai

「知序」知识库的 AI 服务，基于 [Mastra](https://mastra.ai) 运行。Go 后端通过内部凭证调用它完成向量化与问答；它自身不对外暴露公网接口。

## 职责

- `/vectorize`：接收向量化任务，分块、调用 Embedding 模型并写入 Qdrant。
- `/chat`：处理一次问答轮次。模型自主决定是否调用 `search_knowledge` 工具检索知识库；命中时基于片段回答并按工具返回顺序生成引用卡片，未命中时先声明知识库没有相关内容、再用通用知识回答并明确标注来源。
- `/internal/conversations`：会话线程的读取、删除与标题更新，供后端会话列表使用。
- `/internal/health`：就绪检查。

## 运行

```shell
pnpm run dev
```

Mastra Studio 默认监听 `http://localhost:4111`，仅用于本地开发。

## 与 zhixu 后端联调

Go 后端（`apps/zhixu-backend`）启动本服务时需设置：

- `INTERNAL_API_SECRET`：与后端共享的密钥。`/chat`、`/vectorize`、`/internal/conversations`、`/internal/conversations/*` 与 `/internal/health` 未配置密钥或请求未携带 `Authorization: Bearer <secret>` 时返回 `403`。
- `MASTRA_HOST`：监听地址。默认 `127.0.0.1`；后端运行在容器内并通过宿主网关访问时设为 `0.0.0.0`。绑定 `0.0.0.0` 会暴露所有网卡，请勿在不可信网络中运行。
- `CHAT_BACKEND_URL`：Go 后端 origin（默认 `http://127.0.0.1:9000`）。知识检索请求只允许发往它的 `/internal/tools/v1/search` 路径。

### Outbound address guard

Every outbound `fetch` in this process goes through a guarded undici dispatcher. Two checks run before a connection is made:

- Each request origin, including every redirect hop, is validated before dispatch. Literal IP addresses are rejected when they are private, loopback, link-local, CGNAT, multicast, or cloud-metadata addresses. The WHATWG URL parser normalizes alternative IPv4 spellings first, so `2130706433`, `0x7f000001`, `127.1`, and `127.0.0.1.` are all treated as `127.0.0.1` and rejected.
- Hostnames are resolved at connect time and rejected when they resolve to those same ranges, which closes the DNS-rebinding window between validation and connection.

A model base URL therefore cannot be used to reach the local network, even when its hostname passes the Go-side validation.

Two narrow exceptions exist:

- The backend exemption is the **exact `CHAT_BACKEND_URL` origin** (protocol, host, and port). `localhost` is not exempt by itself, so a redirect to another loopback port is still rejected.
- `EGRESS_ALLOWED_HOSTS` (comma-separated) trusts specific hosts (any port) for local model servers. Those hosts may use plain HTTP and may resolve to private addresses; every other hop must be HTTPS.

Literal private IPs need both entries: `MODEL_ALLOWED_PRIVATE_ENDPOINTS` on the Go side and `EGRESS_ALLOWED_HOSTS` here.

## Storage

默认使用 `file:./mastra.db` 保存会话记忆；设置 `TURSO_DATABASE_URL` 与 `TURSO_AUTH_TOKEN` 可切换到 Turso。

## 代码结构

- `src/mastra/chat.ts`：问答 Agent、系统提示词（命中/未命中双模式）与 `search_knowledge` 工具。
- `src/mastra/vectorize.ts`：向量化任务处理。
- `src/mastra/conversations.ts`、`src/mastra/health.ts`：会话与健康检查路由。
- `src/mastra/index.ts`：Mastra 实例、路由注册、存储与可观测性。
- `src/lib/`：请求模式校验、会话历史、内部鉴权与出站地址守卫。

## Learn more

Mastra 文档：<https://mastra.ai/docs/>。API 用法见 <https://mastra.ai/learn>，社区见 <https://discord.gg/mastra-ai>。
