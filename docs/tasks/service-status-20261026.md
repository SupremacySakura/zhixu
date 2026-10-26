---
task_id: service-status
status: completed
created: 2026-10-26
last_reviewed: 2026-10-26
applicable_to: Vue 设置页与 Go 聚合内部服务只读健康检测
---

## 基础服务状态接入

## 结果

设置页接入 Mastra/Qdrant 真实检测，具备 JWT、内部 Bearer、固定部署地址、并行 3 秒限时、10 秒缓存与并发合并；前端手动刷新、离开取消、旧响应防回写、失败保留结果与检测时间。未调用付费模型，未新增依赖或改变已有 `/healthz`。

- 前端全量 30 文件 195/195 测试通过，typecheck/lint/build 通过。
- Go `go test ./...`、服务/router 定向 race 检测、`go build ./...` 通过。
- AI typecheck 与 14/14 测试通过；运行中 dev watcher 已加载健康接口，匿名真实请求返回 403/no-store。
- 真实本地 Qdrant `/readyz` 返回 `all shards are ready`；已有 Vite 提供更新后的组件。未进行登录后的完整浏览器联调。
- AI `pnpm build` 因现有 dev server PID 8079 活跃被 Mastra 安全保护拒绝：`A \`mastra dev\` server is running in this directory`。未强制构建或终止用户进程。
- 当前 Go 开发服务仍为旧二进制，新接口返回 404，需用户重启 `pnpm dev`；不将此状态冒充联调通过。

## 教训

- 健康 200 不等于服务正常：Mastra 验证 `{status:"ok"}`，Qdrant readyz 验证真实文本，collections 验证 JSON 结构，避免 HTML 路由兜底假健康。
- 缓存合并不能绑定第一个请求的 context：每个等待者取消只影响自己，末等待者离开才取消探测且不缓存取消结果。
- Qdrant API Key 在无认证本地部署允许为空；不能把空 key 一律归类未配置，应通过只读访问验证。
- 真实服务存在时构建/重启影响用户环境；优先利用 watcher 验证和只读探测，明确报告尚未重启的服务。

## 可沉淀模式

固定地址与凭据仅驻留后端，禁重定向/环境代理、限制 headers/body，不返回原始异常或上游正文。前端请求取消加身份 guard 与 runtime schema 校验，刷新失败保留旧结果而不是显示服务离线。服务健康与模型连接测试分开，不承诺写权限或索引完整。
