---
task_id: private-model-network-20261115
status: completed
created: 2026-11-15
last_reviewed: 2026-11-15
applicable_to: "私网模型接入、Ollama/vLLM/LM Studio、模型地址策略、出站守卫、Mastra 请求作用域"
---

# 私网模型接入：部署级开关与请求级出站策略

## 背景

用户配置本机 Ollama（`http://127.0.0.1:11434/v1`）后，对话报「私网模型地址未获部署允许，请配置精确的允许地址」。原有机制只能靠 `MODEL_ALLOWED_PRIVATE_ENDPOINTS` 精确匹配 base_url，且必须同时在 `apps/zhixu-ai/.env` 维护 `EGRESS_ALLOWED_HOSTS`；改端口或换地址就要两处同步，实际使用门槛过高。

目标：让部署能一次开启私网模型访问，同时不把 SSRF 防线整体拆掉，也不让用户自行决定访问内网。

## 结果

- **部署开关**：`internal/config/config.go` 新增 `MODEL_ALLOW_PRIVATE_NETWORKS`（默认 false），`docker-compose.yml` 与 `.env.example`、`.env.development.example` 透传。默认关闭，升级不会静默放开内网。
- **Go 运行期策略**：`ChatService.AllowPrivateNetworks` 由配置注入；`validateRuntimeModelURL` 在开关开启时放行 RFC1918、回环、IPv6 ULA 与 CGNAT/Tailscale，`forbiddenModelIP` 仍拦截链路本地、云元数据（`169.254.169.254`、`100.100.100.200`、`168.63.129.16`）、组播与未指定地址。公网明文 HTTP 在 Go 侧不再直接拒绝，交由 Mastra 的解析结果判定。
- **策略下发**：对话、向量化、整理、关联、演化五条调用链的 `model` 字段新增 `allow_private_networks`（仅开启时下发）；Mastra 五个 strict schema 接受该可选字段，旧请求行为不变。
- **Mastra 请求级作用域**：`egress-guard.ts` 用 `AsyncLocalStorage` 实现 `withEgressPolicy` / `enterEgressPolicy`。对话路由在 `ReadableStream.start` 内进入作用域（流比 handler 调用活得更久），其余路由包住 `agent.execute`。并发的不同用户轮次互不串权。
- **明文域名的二次判定**：开关开启时 `assertSafeOrigin` 接受 HTTP 主机名，但 `GuardedAgent.dispatch` 会记录该主机，`guardedLookup` 要求它解析到私网地址；否则按「明文 HTTP 只允许解析到私网地址」拒绝，避免开关把公网供应商降级成明文。
- **可操作的报错**：Go 拒绝文案改为「请让部署管理员开启 MODEL_ALLOW_PRIVATE_NETWORKS，或用 MODEL_ALLOWED_PRIVATE_ENDPOINTS 精确列出该地址」；模型设置页 API 地址下方补同样的说明。
- **文档**：`docs/plain-chat.md`、`docs/ai-architecture.md`、`docs/model-settings-backend.md`、`docs/model-settings-frontend.md`、`apps/zhixu-ai/README.md` 同步两种放行方式与保留的拦截范围。

## 决策与理由

- **不做成用户级开关**：模型地址由用户填写，但能否访问内网是部署信任决策。用户级开关等于把 SSRF 能力开放给任意登录用户；部署级开关与现有 `EGRESS_ALLOWED_HOSTS` 的信任模型一致。
- **不删除精确 allowlist**：只放行个别地址的部署仍需要最小授权，`MODEL_ALLOWED_PRIVATE_ENDPOINTS` + `EGRESS_ALLOWED_HOSTS` 组合保留。
- **开关随请求下发，而不是让 Mastra 读同一份环境变量**：两端环境可能不同（容器/本机混部），请求级字段让策略只有一个事实源，且能按轮次隔离。
- **用 `AsyncLocalStorage` 而非全局可变状态**：Mastra 单进程处理并发轮次，全局布尔值会让一个用户的私网请求影响另一个用户的策略。
- **元数据地址不在放行范围内**：私网模型的真实需求是内网推理服务，云元数据与链路本地不属于该需求，任何开关都不应打开。
- **CGNAT 归入私网**：Tailscale 使用 `100.64/10`，自建组网接入模型是常见场景；但 `100.100.100.200` 是阿里云元数据，必须单独排除。

## 教训

- **精确 origin 豁免会压过更宽的授权**：`CHAT_BACKEND_URL` 落在 `127.0.0.1:9000` 时，`lookupHosts` 会把同主机的其他端口全部挡成「仅允许精确配置的 origin」，私网开关看起来完全无效。授权范围要先比大小再判具体规则，否则宽授权永远走不到。
- **单测覆盖不到“作用域是否真的进入请求”**：`withEgressPolicy` 包错位置（包在 handler 外层而非 `start` 内）时，单元测试仍然全绿。用真实 `fetch` 打本机临时服务做对照，才发现流式路由必须用 `enterWith`。
- **环境里残留的旧白名单会掩盖负向结果**：本机 `EGRESS_ALLOWED_HOSTS=127.0.0.1` 让「关闭开关」也返回 200，差点把未生效当成生效。验证开关必须用不在白名单里的目标，或清空白名单。

## 验证

- 后端：`gofmt`、`go vet ./internal/... ./cmd/server`、`go test ./...`（含新增 `TestChatRuntimePrivateNetworkSwitch`、`TestChatCapabilityCarriesPrivateNetworkPolicy`、`TestPrivateModelNetworkSwitchBinding`）全绿。
- AI：`tsc --noEmit`、`pnpm test`（69 项，含私网放行、元数据拦截、明文域名解析、流式作用域）通过。
- 前端：`pnpm lint`、`pnpm typecheck`、470 项测试通过。
- 真实链路：空白名单下用临时本地服务对照，`withEgressPolicy(true)` 可连通、无作用域被拒、`169.254.169.254` 被拒、公网明文被拒；对运行中的 Mastra 发 `/vectorize`，`127.0.0.1:11434` 在开启时返回 200。
- 未验证：容器内后端 + 宿主 Mastra 的跨网段组合；真实公网明文域名（验证依赖 DNS，当前用 `filterLookupAnswers` 单测覆盖）。
