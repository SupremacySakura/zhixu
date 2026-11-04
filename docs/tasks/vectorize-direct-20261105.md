---
task_id: vectorize-direct-20261105
status: completed
created: 2026-11-05
last_reviewed: 2026-11-05
applicable_to: "向量化、embedding、Mastra、BFF、安全边界、架构迁移"
---

# 向量化直连：Mastra `/vectorize` 去掉 Go 网关（P2）

## 背景

P1 已把对话链路改为 Mastra 直连 provider，但向量化仍走旧路径：Go worker 为每批文本签发 capability，经 `/internal/chat/v1/embeddings` 转发到 provider，Mastra 的 `/vectorize` 只是计算层。P2 的目标是让 embedding 与 chat 走同一套直连方式：Go 只传递「用哪个模型」，Mastra 直接调 provider，Go 不再转发 provider 流量。

## 结果

- **Mastra**：`src/lib/vectorize.ts` 的 wire 改为 `{model:{protocol,model_id,base_url,api_key}, dimension, texts}`；删除 `allowedGateway`；`src/mastra/vectorize.ts` 改用 `allowedProviderURL` 校验模型地址（403 `Invalid model URL`），超时 100s、`Cache-Control: no-store`、错误不回显 provider 响应等行为不变。
- **Go**：`MastraEmbedder` 只保留 `AIURL`，每批请求携带 embedding 模型配置（含本轮解密的 key），不再 `Register`/`revoke` capability、不再拼 gateway URL；删除 `/internal/chat/v1/*` 路由、`EmbeddingGateway`/`embeddingGateway` handler 与 `handler/embedding.go`；`ChatCapability` 删除 `Texts`/`claimed`，删除 `ClaimCapability` 与 `ModelClient`。
- **地址策略**：Go 不再拨号 provider，原先只在 `ModelClient`（拨号前）执行的私网/元数据拦截失去执行点。改为在解析模型时静态校验：`validateRuntimeModelURL` 复用 `validateModelURL` 的格式规则，并拒绝私网、回环、链路本地、云元数据与 `100.64/10`，除非 `MODEL_ALLOWED_PRIVATE_ENDPOINTS` 精确匹配该 base_url；非 allowlist 的公网地址必须是 HTTPS。保存配置仍然宽松，便于先写配置再开 allowlist。
- **测试**：新增 `apps/zhixu-ai/src/tests/vectorize.test.mjs`（直连 wire 通过、旧 gateway wire 被 strict 拒绝、非法模型/批量被拒）；Go 新增 `TestChatRuntimeEndpointPolicy`（未 allowlist 时解析失败、精确 allowlist 后通过、`localhost` 与明文 HTTP 规则）与 `TestMastraEmbedderSendsDirectProviderBatch`/`TestMastraEmbedderRejectsMismatchedResponse`（直连 wire、无 gateway/capability 字段、数量/维度/零向量校验）；删除 embedding gateway 测试；现有 handler fixture 补 `AllowedPrivateEndpoints`。
- **文档**：`ai-architecture.md` 状态改为 P1、P2 已落地，删除 `/internal/chat/v1/*` 路由行，契约 4 改为已落地 wire，迁移阶段与差异清单同步；`plain-chat.md` 安全段改为描述解析期静态校验。

## 决策与理由

- **Go 保留入库编排**：切块、任务状态机、失败分块重试、Qdrant 写入与 `vector_state` 全在 Go；只有「文本→向量」这一计算动作交给 Mastra。Qdrant 写入方唯一，避免双写导致 schema 归属分裂。
- **不把 capability 用在 embedding 上**：capability 的存在意义是「Go 作为代理时约束代理目标」；直连后没有再需要授权的中间跳，模型配置本身随请求下发即可。
- **删除 `ModelClient` 而不是留作死代码**：它的唯一用途是拨号 provider；Go 不再拨号后，保留它会让后来者误以为 SSRF 防线仍在拨号路径上。把策略搬到解析期，语义更诚实。
- **保存宽松、解析严格**：`validateProvider` 的既有测试明确允许保存 `http://127.0.0.1:11434/v1`（先配置后开 allowlist 的部署顺序），因此策略只在运行时执行，不改保存体验。

## 教训

- **删掉一条链路时，先找出它承载的隐式安全职责**：`ModelClient` 不只是「发请求的工具」，还承担了私网/元数据/HTTPS 策略；P1 移除对话网关后该策略在对话路径已经失效，P2 才补齐到解析期。迁移类任务要把「删代码」和「搬不变量」分开列。
- **静态校验的判断边界要写进注释**：`validateRuntimeModelURL` 只能判断字面 IP 与 localhost，域名解析发生在 Mastra 侧，注释里必须说明这一点，否则会被误读成完整 SSRF 防护。
- **测试 fixture 是策略变更的探测器**：新增运行期校验后，凡是用 `127.0.0.1` 造 provider 的 handler 测试都会失败；这类失败是有效的，应改 fixture 显式声明 allowlist，而不是放宽校验。
- **strict schema 是删除字段的保障**：Mastra 侧 `vectorizeSchema` 保持 `.strict()`，旧 gateway 字段会直接 400，避免「字段被忽略但仍可用」的静默兼容。

## 候选特性

- P3：`done.usage` 逐轮记账；Mastra embedding 的 token 用量回传（当前 `/vectorize` 不返回 usage）。
- 域名型 provider 的运行期地址校验：需要在 Mastra 侧或部署网关注入解析结果，Go 静态校验无法覆盖。
- 向量化批次与并发可配置化（当前固定 16 条/批）。

## 验证

- Go：`gofmt`、`go build ./...`、`go vet ./...`、`go test -count=1 ./internal/handler ./internal/service ./internal/repository` 全绿。
- Mastra：`pnpm test` 18/18（新增 3 例）、`pnpm typecheck` 通过（`mastra build` 仍被运行中的 dev server 拒绝，未执行）。
- 未执行（需真机）：改配置后重新向量化一条知识（成功分块与失败分块重试各一次），确认 Qdrant 写入、`vector_state` 与详情面板一致。
