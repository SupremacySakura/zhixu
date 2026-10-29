---
last_reviewed: 2026-10-30
---

本文为 Brain 层知识文件，AI 工具进入项目时应优先读取 docs/brain/

# 全局架构视图

- 目标：提供系统级的高层架构图景，清晰展现前后端连接、数据流和鉴权链路。
- 参考链接：前端结构请参阅 ../frontend-structure.md，后端结构请参阅 ../backend-structure.md。
- 数据流概览：从前端发起请求到后端服务执行、数据库落盘，以及从数据库返回结果到前端渲染的完整路径。
- 鉴权链路：前端在每次请求中注入 Authorization: Bearer <token>，后端通过中间件完成校验并注入用户上下文。
- 服务划分：前端 UI/路由/状态管理 -> 后端路由 -> 服务编排 -> 数据访问层。
- AI 协同：Go 后端经内网路由调用 Mastra AI 服务（`INTERNAL_API_SECRET` 共享密钥认证），Mastra 负责向量化与对话，向量检索走 Qdrant，会话记忆直存 MySQL。
- 安全与观测：日志脱敏、错误码设计、指标暴露、追踪示例等要点。

- 参考：前端结构见 ../frontend-structure.md；后端结构见 ../backend-structure.md。

## 数据流与鉴权链路要点
- 请求从浏览器发送，携带 JWT Bearer Token。
- 服务端中间件解析 Token，注入上下文，统一处理权限校验。
- 数据落盘路径、缓存策略与 API 边界在后端实现中体现。

## 连接与交互要点
- 前端与后端通过统一的 API 风格进行通信，前端路由触发的操作落到后端服务实现。
- 前后端联调应遵循 docs/README.md 的导航与引用规则，避免正文冗余。

## 参考引用
- 前端结构： ../frontend-structure.md
- 后端结构： ../backend-structure.md

(End of file - total 40 lines)
