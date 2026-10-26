# 设置页服务状态

## 范围与含义

`/home/settings` 的服务状态页提供 Mastra 与 Qdrant 的只读连接检查，不调用 LLM/Embedding，不创建集合或写入向量，不修改用户模型设置。基础服务正常仅表示本次探测通过，不等同于模型可调用、向量索引完整、工作流执行成功或知识已全部向量化。

- Mastra：检查内部健康端点与共享密钥鉴权，不检测聊天记忆数据库。
- Qdrant：检查就绪状态与当前部署凭据的集合列表读取权限，不承诺具有写入权限。
- AI 依赖异常不会由本功能停止文件管理或知识阅读。

## 接口与配置

前端 `GET /api/settings/services/status` 通过现有代理到 Go `GET /settings/services/status`，必须通过 JWT 鉴权。地址与凭据复用部署参数 `CHAT_AI_URL`、`INTERNAL_API_SECRET`、`QDRANT_URL`、`QDRANT_API_KEY`，不接受客户端检测地址，不下发内网地址或凭据。

成功检测返回现有 `{code: 0, msg: "success", data}` 包裹，`data` 包括 `checked_at`（RFC3339）和 `services` 数组。每项字段：

| 字段 | 内容 |
| --- | --- |
| id | `mastra` / `qdrant` |
| status | `healthy` / `unconfigured` / `unauthorized` / `unreachable` / `timeout` / `unhealthy` |
| latency_ms | 探测耗时；未配置时为 null |
| message | 安全、可读的状态说明，不回显上游正文或网络异常 |

依赖故障作为检测结果返回，不作为整个 HTTP 请求失败；未登录仍按现有认证中间件处理。

Go 对两项服务并行探测，每项总预算 3 秒；Qdrant 两次访问共享该预算。10 秒进程内缓存合并并发检查，不持久化、不构建监控历史。禁止 HTTP 重定向和环境代理，限制响应读取长度。

Mastra 新增 `GET /internal/health`，使用现有 `Authorization: Bearer INTERNAL_API_SECRET` 校验，成功仅返回 `{status: "ok"}`，缺失/错误密钥返回 403，设置 `Cache-Control: no-store`。该接口仅供后端探测。

## 前端行为

每次进入服务状态页检测一次，支持手动刷新，不持续轮询。切换页签或卸载取消请求，并用请求身份防止旧响应回写。显示每项状态、耗时和检测时间；刷新失败保留上次成功结果并明确标为旧结果，不把请求失败伪装成服务离线。

## 验证边界

自动化测试使用 HTTP/SDK 替身，不调用真实付费模型。真实部署联调应验证：两项正常、停止其中一项、共享密钥错误、Qdrant API Key 错误、未登录访问、刷新及离开页面取消。Go 无热更新，新增接口须重启业务服务；AI 新路由须经现有 AI dev/build 脚本加载。

## 文档差异

Brain 中旧 `cloud-drive` 路径与无自动化测试描述仍与现有代码不一致，Brain/skills 对 AI 只读，留待人工审核；本功能以实际 `zhixu` 代码与测试为准。
