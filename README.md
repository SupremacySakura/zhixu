# 知序 Zhixu

> 基于大语言模型的个人知识管理与演化系统（毕业设计）

知序是一个个人知识管理与演化系统：以文件存储与管理为底座，逐步构建采集 → 理解 → 关联 → 演化的知识管理闭环，并通过大语言模型实现知识库的智能检索问答与演化分析。

**当前状态**：已完成文件存储与管理底座（v1），包含完整的账号体系、分片上传 / 秒传、分享取件码、操作日志与仪表盘能力。AI 知识管理能力（混合检索问答、知识关联图谱、知识演化追踪）在路线图中规划。

## 功能一览（已实现）

| 模块 | 说明 |
| --- | --- |
| 账号体系 | 注册 / 登录 / JWT 鉴权 / bcrypt 密码加密 |
| 文件管理 | 上传（分片断点续传）、下载、秒传、新建文件夹、移动、重命名、删除、搜索、列表 / 网格视图 |
| 文件分享 | 生成取件码、取件码列表 / 撤销、有效期与次数控制、公开取件页 |
| 操作日志 | 关键操作记录、当前用户隔离、分页查询 |
| 仪表盘 | 存储占用概览、容量进度、系统状态卡片 |

## 技术栈

| 层 | 技术 |
| --- | --- |
| 前端 | Vue 3 + TypeScript + Vite + Pinia + Vue Router + Axios + Vitest |
| 后端 | Go 1.25 + Gin + GORM + Viper + JWT，自实现分片上传协议 |
| 数据库 | MySQL 8.4 |
| 部署 | Docker / Docker Compose，GitHub Actions 自动部署到自有服务器（GHCR 镜像） |
| AI 规划 | Mastra（TypeScript）AI 服务 + Qdrant 向量检索，与 Go 网关协同 |

## 项目结构

```text
zhixu/
├── apps/
│   ├── zhixu-frontend/   # 前端 SPA（页面 / 路由 / 状态管理 / API 层）
│   └── zhixu-backend/    # 后端 HTTP API（路由 / 业务服务 / 数据访问 / 鉴权中间件）
├── docs/                 # 文档中心（brain 全局认知 / tasks 学习日志 / skills 能力沉淀）
├── design/               # UI 设计稿（HTML 静态稿）
├── ops/mysql/            # MySQL 初始化与导入脚本
├── scripts/              # 本地开发辅助脚本
├── docker-compose.yml    # 编排：MySQL + 后端 + 前端（按需启用 profile）
└── .github/workflows/    # CI/CD：push main 自动构建镜像并部署
```

## 两种启动方式

所有命令在仓库根目录执行。两种方式的配置、端口和开发数据相互独立，可以同时运行。

| 项目 | 本地开发 `pnpm dev` | 全栈 Docker |
| --- | --- | --- |
| 配置文件 | `.env.development` | `.env` |
| 前端地址 | http://localhost:5173 | http://localhost:8080 |
| 后端宿主机端口 | `9000` | `9001`（容器内 `9000`） |
| MySQL 宿主机端口 | `3306`（本机服务） | `3307`（容器内 `3306`） |
| 新环境数据库 | `zhixu` | `zhixu` |
| MySQL 数据 | 本机 MySQL 服务的数据目录 | `data/mysql` |
| 文件存储 | `data/dev/files` | 已有 `backend-data` 卷 |

上述为示例配置的默认值。已有 `.env` 中的端口仍会覆盖默认值；要同时运行，请将 Docker 的 `BACKEND_HOST_PORT` 改为 `9001`、`FRONTEND_PORT` 改为 `8080`。容器内部代理仍然使用 `backend:9000`。

### 方式一：本地开发（Run dev）

前置要求：Node.js 22.12+、pnpm 9、Go 1.25、本机 MySQL 8.4、Qdrant。**此模式不依赖 Docker，前后端与 Mastra 均在本机运行。**

macOS 可使用 Homebrew 安装并启动 MySQL（已有本机服务则跳过）：

```bash
brew install mysql@8.4
brew services start mysql@8.4
export PATH="$(brew --prefix mysql@8.4)/bin:$PATH"
# 按提示设置本机 root 密码
mysql_secure_installation
mysql -h 127.0.0.1 -P 3306 -u root -p \
  -e 'CREATE DATABASE IF NOT EXISTS zhixu CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;'
```

Qdrant 需要单独启动。`pnpm dev` 不会代管它，也不会自动启动容器；可以使用官方镜像（仅此一个容器，与全栈 Docker 模式无关）：

```bash
docker run -d --name zhixu-qdrant \
  -p 127.0.0.1:6333:6333 \
  -v zhixu-qdrant-data:/qdrant/storage \
  qdrant/qdrant:v1.15.5
```

首次准备：

```bash
pnpm install
cp .env.development.example .env.development
# 编辑 .env.development：设置本机 DB_USER / DB_PASSWORD / DB_PORT
# QDRANT_URL 默认 http://127.0.0.1:6333；Qdrant 开启鉴权时再填 QDRANT_API_KEY
```

知识入库后的自动向量化由 Mastra 完成，需在另一个终端启动 AI 服务：

```bash
pnpm dev:ai
```

Mastra 默认监听 http://127.0.0.1:4111。首次使用还需登录后在「模型设置」配置可用的 Embedding 模型与凭据，否则任务会停在待向量化并提示配置缺失。

会话历史由 Mastra Memory 直存 MySQL（与业务同库），需在 `apps/zhixu-ai/.env` 配置 `MYSQL_*` 连接信息；会话管理接口（列表/详情/改名/删除）由 Go 后端鉴权后转发 Mastra 内网路由，两端通过 `INTERNAL_API_SECRET` 共享密钥认证（`.env.development` 与 `apps/zhixu-ai/.env` 必须填同一值，可用 `openssl rand -hex 32` 生成），为空时内网路由拒绝服务。

日常启动，在 IDE 中运行根 `package.json` 的 `dev` 脚本，或执行：

```bash
pnpm dev
```

脚本连接已启动的本机 MySQL，编译并启动本机 Go 后端，等待健康后启动带热更新的本机 Vite。访问 http://localhost:5173，`/api` 请求代理到本地后端。它不会读取根 Docker `.env`，不会启动任何容器，也不会管理或停止本机 MySQL 服务。

`Ctrl+C` 停止前后端；任一进程退出也会清理另一端。需要停止本机 MySQL 时，由你单独执行：

```bash
brew services stop mysql@8.4
```

自定义本地端口时修改 `.env.development` 的 `PORT`、`DEV_FRONTEND_PORT`、`DB_PORT`；代理目标自动跟随后端端口，端口被占用时不会悄悄切换。
开发文件为 shell 兼容的 `KEY=value` 格式，带空格或特殊字符的值需加引号；不要放入不可信的 shell 命令。
`DB_USER` 和 `DB_PASSWORD` 必须对应实际本机 MySQL 账号，且该账号需有 `zhixu` 库的建表、建索引与读写权限。脚本不创建数据库账号或数据库；首次建库后，应用表及索引由后端迁移创建。`DB_HOST` 使用 `127.0.0.1`、`localhost` 或 `::1`。

### 方式二：Docker 全栈启动

前置要求：Docker 与 Docker Compose 插件。新环境执行：

```bash
cp .env.example .env
# 编辑 .env：设置一致的 MYSQL_ROOT_PASSWORD / DB_PASSWORD 和强 JWT_SECRET
docker compose --profile full up -d --build
```

访问 http://localhost:8080；后端健康检查为 http://localhost:9001/healthz，经过前端代理为 http://localhost:8080/api/healthz。
`.env.example` 已设置 `COMPOSE_PROFILES=full`，因此 `docker compose up -d --build` 也会启动全栈。只启动 MySQL 可执行 `docker compose up -d mysql`。

MySQL 仅在空数据目录首次初始化所配置的数据库；应用表和查询索引由后端启动迁移创建。停服务用 `docker compose down`，重建不需要清空数据。

### 已有环境升级

**不要覆盖已有 `.env`，不要清空 `data/mysql` 或删除文件存储卷。项目更名不要求数据库更名。**

已有 `cloud-drive` 数据库可继续使用，在原 `.env` 保留：

```dotenv
MYSQL_DATABASE=cloud-drive
DB_NAME=cloud-drive
```

以上仅适用于仍有历史数据的环境，不是当前推荐配置。本地开发、后端及 Docker 的默认数据库名统一为 `zhixu`；本机与容器使用不同的 MySQL 实例和端口，数据仍然隔离。历史环境如需暂时使用旧库，必须显式指定库名，并保留原数据目录、卷名、密码和 JWT 密钥，不依赖默认值。
升级前备份数据库和文件卷，再执行 `docker compose --profile full up -d --build`；后端自动补建缺失索引，不会更名或删除旧库。
需要主动迁移到 `zhixu` 时，先停写、备份并导入新库、核对数据与文件关联，再同时切换两个库名配置；保留旧库和备份以便回滚。详见 [MySQL 升级说明](ops/mysql/README.md)。

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `pnpm install` | 安装前端依赖 |
| `pnpm build` | 前端生产构建 |
| `pnpm test` | 前端单元测试（Vitest） |
| `pnpm test:backend` | 后端单元测试（Go） |
| `pnpm typecheck` | 前端 `vue-tsc` 类型检查 |
| `pnpm lint` | ESLint（根目录与前端，只报告不修复） |
| `pnpm --filter zhixu-frontend format` | 前端 Prettier 格式化 |
| `pnpm build:backend` | 后端编译检查 |
| `pnpm dev` | 本机后端 + Vite，连接已运行的本机 MySQL，无 Docker 依赖 |
| `docker compose config` | 校验编排配置 |
| `docker compose up -d --build` | 重建并启动服务 |
| `docker compose ps` / `logs -f` | 查看状态 / 跟踪日志 |
| `docker compose down` | 停止并移除容器 |

## 导入外部 SQL 到 MySQL 容器

```bash
./scripts/mysql-import.sh ./ops/mysql/import/your_dump.sql zhixu
```

说明见 `ops/mysql/README.md`。

## 服务器部署

仓库内置 GitHub Actions 工作流 `.github/workflows/deploy.yml`：push 到 `main` 时自动构建 `zhixu-backend` / `zhixu-frontend` 镜像并推送到 GHCR，然后 SSH 登录服务器拉取新镜像重启容器。

需要在仓库 Settings → Secrets and variables → Actions 中配置：

| Secret | 说明 |
| --- | --- |
| `SERVER_HOST` | 服务器 IP 或域名 |
| `SERVER_USER` | SSH 用户名 |
| `SSH_PRIVATE_KEY` | SSH 私钥 |
| `DEPLOY_DIR` | 服务器上存放 `docker-compose.yml` 的目录（例如 `/opt/zhixu`） |

> 工作流使用默认 SSH 端口 `22`，用内置 `GITHUB_TOKEN` 登录 GHCR，镜像名来自工作流 `env`，标签为当前提交 SHA。服务器侧需有仓库 checkout、Docker 与 Docker Compose 插件，并预先放好生产 `.env`。升级遵循上面的数据保留步骤。

## 排障

| 现象 | 排查 |
| --- | --- |
| 端口冲突 | 本地修改 `.env.development`；Docker 修改 `.env` 的宿主机端口；不要修改容器内部代理端口 |
| MySQL 未就绪或密码错误 | 本机模式检查 MySQL 服务及 `.env.development` 账号；Docker 模式查看 Compose 日志，已有密码不会随环境变量自动更改 |
| 旧数据启动提示 Unknown database | 保留原库名，或完成备份迁移后切换；不要清空数据卷 |
| 容器异常退出 | `docker compose logs -f` 定位；`docker compose up -d --build` 重建 |

## 文档导航

- `AGENTS.md` — AI 协作协议与文档入口
- `docs/` — 文档中心（全局认知 / 结构说明 / 任务记录 / 技能沉淀）
- `apps/zhixu-frontend/AGENTS.md`、`apps/zhixu-backend/AGENTS.md` — 子应用导航
