---
last_reviewed: 2026-10-30
---

本文为 Brain 层知识文件，AI 工具进入项目时应优先读取 docs/brain/

# 项目上下文

## 来源

- 根目录 `AGENTS.md`（项目概览、启动命令、架构与端口关系）。
- 根目录 `README.md`（两种启动方式、端口约定、部署与排障的权威正文）。
- 前后端子项目 `AGENTS.md`（各子域入口与关键运行信息）。

## 适用范围

- 面向首次进入仓库的开发者或 AI，快速建立"这是什么项目、怎么跑起来、从哪里读起"的全局认知。

## 更新入口

- 当项目技术栈、目录拓扑、端口约定、联调方式发生变更时，优先更新本文。
- 变更涉及子域细节时，同时同步到对应主题文档：见 `frontend-structure.md`、`backend-structure.md`、`brain/constraints.md`。

## 项目目标与形态

- 知序（Zhixu）是一个基于大语言模型的个人知识管理与演化系统（毕业设计）：以文件存储与管理为底座，构建采集 → 理解 → 关联 → 演化的知识管理闭环。
- 已实现：账号体系、文件管理（分片上传 / 秒传）、分享取件码、操作日志、仪表盘、Markdown 知识入库、模型设置、对话 RAG 检索问答。
- 仓库采用 `pnpm workspace` 单仓组织，业务代码集中在 `apps/`，`packages/` 预留为共享包目录（当前为空）。

## 技术栈总览

- 前端：Vue 3 + TypeScript + Vite + Pinia + Vue Router + Tailwind CSS v4 + Axios + Vitest。
- 后端：Go 1.25 + Gin + GORM + Viper + JWT + Swaggo，数据库 MySQL 8.4。
- AI 服务：Mastra（TypeScript）+ Qdrant 向量检索，与 Go 网关通过内网路由协同（`INTERNAL_API_SECRET` 共享密钥）。
- 协作方式：前端开发服务器通过代理将 `/api/*` 转发到后端服务。

## 目录与职责

- `apps/zhixu-frontend`：前端 SPA，包含页面、路由、状态管理与 API 调用层。
- `apps/zhixu-backend`：后端 HTTP API，包含路由、业务服务、数据访问与鉴权中间件。
- `apps/zhixu-ai`：Mastra AI 服务，负责向量化、对话与会话记忆（Memory 直存 MySQL）。
- `data/`：运行时文件存储目录（分片与文件内容）。
- `docs/`：文档中心与主题正文。

## 运行与联调关系

- 后端默认端口 `9000`（可通过 `PORT` 环境变量覆盖）。
- 前端开发端口 `5173`，通过 Vite 代理与后端联调。
- Mastra AI 服务默认监听 `4111`，需单独以 `pnpm dev:ai` 启动；Qdrant 默认 `6333`，需单独启动容器。
- 建议启动顺序：MySQL 与 Qdrant → 后端 → 前端 →（按需）AI 服务；随后验证登录态检查与文件相关接口是否可达。
- 两种启动方式（本地开发 / Docker 全栈）的完整约束以根 `README.md` 为权威正文。

## 新成员最短上手路径

1. 在仓库根目录执行 `pnpm install` 安装依赖。
2. 按根 `README.md` 准备本机 MySQL、Qdrant 与 `.env.development`。
3. 在仓库根目录执行 `pnpm dev`（脚本编译并启动本机 Go 后端与 Vite 前端）。
4. 需要知识向量化与对话能力时，另开终端执行 `pnpm dev:ai`。
5. 阅读顺序建议：`brain/constraints.md` → `brain/ai-execution.md` → `frontend-structure.md` / `backend-structure.md` → `brain/testing.md`。
