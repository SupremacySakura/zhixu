---
last_reviewed: 2026-10-30
---

本文为 Brain 层知识文件，AI 工具进入项目时应优先读取 docs/brain/

# 技术栈总览

- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router + Tailwind CSS v4（CSS-first）+ motion-v + Axios + Vitest。
- 后端：Go 1.25 + Gin + GORM + Viper + MySQL 8.4 + JWT + Swaggo。
- AI 服务：Mastra 1.x（TypeScript，Node ≥ 22.13），Memory 直存 MySQL，向量检索使用 Qdrant。
- 协作方式：前端开发服务器通过代理将 `/api/*` 转发到后端服务；Go 后端经内网路由调用 Mastra（`INTERNAL_API_SECRET` 共享密钥认证）。

## AI 执行命令清单（根目录 package.json 为权威来源）

- 依赖安装：`pnpm install`（仓库根目录）。
- 本机开发：`pnpm dev`（仓库根目录，脚本启动本机 Go 后端 + Vite 前端）。
- AI 服务开发：`pnpm dev:ai`（`apps/zhixu-ai`，Mastra 监听 4111）。
- 前端构建：`pnpm build`；前端类型检查：`pnpm typecheck`；前端单测：`pnpm test`（Vitest）。
- 根目录校验：`pnpm lint`。
- 后端编译：`pnpm build:backend`；后端测试：`pnpm test:backend`（`apps/zhixu-backend` 下 `go test ./...`）。
- 后端直接运行：`go run cmd/server/main.go`（`apps/zhixu-backend`）。
- AI 服务校验：`pnpm typecheck:ai`、`pnpm build:ai`；AI 测试：`pnpm --filter zhixu-ai test`。
- 全量验证：`pnpm verify`（lint + typecheck + test + build + build:backend + test:backend）。
