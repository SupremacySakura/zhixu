# 后端结构说明

## 来源

- `apps/zhixu-backend/AGENTS.md`（入口、分层、鉴权、存储与命令）。
- 根目录 `AGENTS.md`（整体架构、后端端口与关键风险提示）。

## 适用范围

- 适用于 `apps/zhixu-backend` 的目录结构、分层链路、鉴权流程与数据存储入口说明。

## 更新入口

- 当后端目录分层、路由注册方式、中间件策略、模型结构或存储路径变更时更新本文。
- 接口形态变化需同步检查前端调用层说明，见 `frontend-structure.md`。

## 启动入口与目录骨架

- 启动入口：`cmd/server/main.go`，负责初始化数据库、注册路由并启动 HTTP 服务。
- 核心目录：
- `internal/router/`：路由注册与模块入口。
- `internal/handler/`：HTTP 层参数解析与响应封装。
- `internal/service/`：业务逻辑编排。
- `internal/repository/`：数据访问层。
- `internal/model/`：GORM 模型定义。
- `internal/middleware/`：鉴权、跨域等中间件。

## 分层调用链路

- 典型链路为 `router -> handler -> service -> repository -> database`。
- `handler` 不承载复杂业务规则，主要负责输入输出与错误映射。
- `service` 聚焦业务流程与领域规则，组合多个仓储操作。
- `repository` 聚焦数据库读写与查询组织，避免泄漏 HTTP 语义。

## 认证与文件模块主流程

- 认证模块：登录/注册接口由 `auth` 相关 handler 暴露，JWT 校验由中间件完成；`GET /auth/profile` 返回当前登录用户的基本信息与存储用量（不含密码哈希）。JWT 无状态、无黑名单，退出登录仅由前端清除本地 token。
- 受保护接口从 `Authorization: Bearer <token>` 读取 token，通过中间件解析后注入上下文。
- 文件模块：由路由入口接收上传或管理请求，经 service 层处理分片/元数据逻辑，最终落盘到存储路径并写入数据库记录。

## 数据库与文件存储关键路径

- 数据库连接初始化位于 `internal/database/db.go`，配置由 `internal/config` 从环境变量或配置文件加载。
- 本地 `pnpm dev` 显式设置 `ZHIXU_ENV_FILE` 读取 `.env.development`，不会回退到 Docker `.env`；文件存储路径由配置注入。
- `internal/database/migrate.go` 在创建应用表后补建文件查询索引，兼容首次初始化和已有数据库。
- 启动、端口与旧数据库升级约束见根 [README](../README.md#两种启动方式)。

## Markdown 知识模块

- `internal/model/knowledge.go` 定义独立知识与导入任务表；启动迁移自动补建。
- `internal/repository/knowledge_repo.go` 管理用户隔离、事务、唯一约束与任务租约。
- `internal/service/knowledge.go` 处理 Markdown 读取校验与后台队列，`cmd/server/main.go` 接入启动和退出生命周期。
- 接口由 `internal/router/knowledge.go` 注册 JWT 鉴权。数据库字段、接口及边界见 [Markdown 知识后端](./markdown-knowledge-backend.md)。

## 模型配置模块

- `internal/model/ai_config.go` 定义供应商、模型清单和用户默认设置三张表。
- `internal/repository/ai_config_repo.go` 使用用户行锁实现配置写入事务；`internal/service/ai_config.go` 管理版本、用途、默认模型与重建校验。
- `internal/service/model_credentials.go` 提供绑定用户和供应商的 AES-GCM 凭据加密。
- JWT 配置接口由 `internal/router/ai_config.go` 注册；完整契约见 [模型配置后端](./model-settings-backend.md)。本轮不调用模型或向量库。
