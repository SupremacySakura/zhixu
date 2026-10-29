---
last_reviewed: 2026-10-30
---

本文为 Brain 层知识文件，AI 工具进入项目时应优先读取 docs/brain/

# 测试提示

## 来源

- 根目录 `package.json`（当前可用验证命令的权威来源）。
- 根目录 `AGENTS.md` 与 `README.md`（启动方式与端口约定）。
- 前后端 `AGENTS.md`（子域启动命令与关键风险点）。

## 适用范围

- 适用于日常开发改动后的最小可用验证、联调回归与问题复现记录。

## 更新入口

- 当命令入口、端口约定、鉴权流程、核心页面或核心接口变更时更新本文。
- 新增自动化测试框架后，需补充对应执行命令、目录规范与失败排查步骤。

## 测试前置条件

- 本地 MySQL 可连接，后端可正常启动；涉及知识库 / 对话功能时需 Qdrant 与 Mastra AI 服务可用。
- 根目录已执行 `pnpm install`，前端依赖完整。
- 已有自动化测试：前端 Vitest、后端 Go test、AI 服务 node:test；联调类场景仍以手工回归为主。

## 基础验证命令

- 根目录：
- `pnpm lint`（ESLint 检查）
- `pnpm typecheck`（前端 `vue-tsc` 类型检查）
- `pnpm test`（前端 Vitest 单测）
- `pnpm build:backend`（后端编译检查）
- `pnpm test:backend`（后端 Go 测试）
- `pnpm verify`（以上 + 前端构建的全量验证）
- 前端目录 `apps/zhixu-frontend`：
- `pnpm dev`（本地联调）
- `pnpm build`（构建验证）
- 后端目录 `apps/zhixu-backend`：
- `go run cmd/server/main.go`（运行服务）
- `go build/vet/test ./...`（编译、静态检查与测试）
- AI 服务目录 `apps/zhixu-ai`：
- `pnpm typecheck` / `pnpm test` / `pnpm build`

## 建议验证顺序

1. 启动后端并确认端口可访问。
2. 启动前端并检查登录页加载、路由跳转是否正常。
3. 验证登录后受保护路由访问与未登录重定向行为。
4. 按改动范围执行功能回归（文件列表、上传、分享/提取、知识库、模型设置、对话相关流程）。
5. 最后执行 `pnpm lint` 与 `pnpm typecheck` 做静态兜底；跨端改动建议直接跑 `pnpm verify`。

## 场景回归清单

- 登录状态：登录、刷新、退出后 token 与页面权限是否一致。
- 路由守卫：受保护路径是否正确拦截，豁免路径是否保持可访问。
- 文件流程：上传、展示、操作反馈是否与后端响应一致。
- 知识与对话：入库状态流转、RAG 检索阈值 / TopK 设置、拒答文案是否符合预期。
- 接口异常：401、500 等错误是否有可理解提示且不导致页面崩溃。

## 问题定位与复现建议

- 优先记录最小复现路径：环境、输入、接口、预期与实际结果。
- 前端重点看浏览器 Network 与 Console，后端重点看服务启动日志与接口日志。
- 若是联调问题，先验证代理路径与后端端口，再定位请求头与 token 状态。
- Go 无热更新，后端改动须重启服务才生效；AI 服务改动经现有 dev/build 脚本加载。
