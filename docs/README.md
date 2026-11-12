# 文档中心（索引页）

> 本页只做导航与阅读顺序，不承载长篇规则正文。

## 1. 三层知识架构

### Layer 1: 全局认知 [docs/brain/](./brain/)

AI 工具进入项目时应优先读取的必读层。

- [project.md](./brain/project.md) — 项目目标与形态
- [architecture.md](./brain/architecture.md) — 全局架构视图
- [tech-stack.md](./brain/tech-stack.md) — 技术栈与命令
- [conventions.md](./brain/conventions.md) — 编码与文档约定
- [constraints.md](./brain/constraints.md) — 安全与配置约束
- [testing.md](./brain/testing.md) — 测试规范
- [ai-execution.md](./brain/ai-execution.md) — AI 执行协议

### Layer 2: 学习日志 [docs/tasks/](./tasks/)

记录任务结果与可复用教训。

- [README.md](./tasks/README.md) — 格式规范与准入标准

### Layer 3: 能力沉淀 [docs/skills/](./skills/)

可复用的操作模式与技能。

- [README.md](./skills/README.md) — 格式规范与入库流程

### 设计文档 [docs/design/](./design/)

毕业设计三份源头文档的入库副本（需求/产品/架构），AI 只读。

- [README.md](./design/README.md) — 文档清单与同步约定

### 功能规划 [docs/plan/](./plan/)

毕业设计需求 → 仓库实现的差距与分阶段实施计划（待办）。

- [README.md](./plan/README.md) — 完成度矩阵、阶段总览、数据库与 API 契约变更总表

### 视觉与产品规范

前端的视觉权威与产品真相，两者都不是实现细节，冲突时的优先级见 [ADR-0003](./adr/0003-visual-world-replacement.md)。

- [../DESIGN.md](../DESIGN.md) — 索引卡盒视觉规范（v2，骨架 + 三章待回填）
- [../apps/zhixu-frontend/PRODUCT.md](../apps/zhixu-frontend/PRODUCT.md) — 产品真相（用户、定位、不可破契约）

### 领域术语与决策记录

- [../GLOSSARY.md](../GLOSSARY.md) — 领域术语表（知识管理、AI 工作流、知识关联、知识图谱）
- [adr/0001-knowledge-relation-model.md](./adr/0001-knowledge-relation-model.md) — ADR-0001 知识关系模型（有向行、五种类型、唯一键含类型）
- [adr/0002-conflict-relation-type.md](./adr/0002-conflict-relation-type.md) — ADR-0002 冲突关系类型 contradicts（对称类型、不进 AI 推荐白名单）
- [adr/0003-visual-world-replacement.md](./adr/0003-visual-world-replacement.md) — ADR-0003 视觉世界替换（redesign 而非 refinement、三份权威边界、扩展—迁移—收缩）

### 详细结构参考

- [../README.md](../README.md#两种启动方式) — 本地开发与 Docker 启动、端口和升级约束
- [frontend-structure.md](./frontend-structure.md) — 前端详细结构
- [backend-structure.md](./backend-structure.md) — 后端详细结构
- [markdown-knowledge-backend.md](./markdown-knowledge-backend.md) — Markdown 入库、数据库迁移与接口契约
- [markdown-knowledge-frontend.md](./markdown-knowledge-frontend.md) — 文件加入、知识阅读与前端验收边界
- [model-settings-backend.md](./model-settings-backend.md) — 用户模型配置、加密凭据与默认模型接口
- [model-settings-frontend.md](./model-settings-frontend.md) — 设置页、供应商表单与真实接口接入
- [service-status.md](./service-status.md) — 设置页基础服务检测、鉴权、缓存与状态边界
- [plain-chat.md](./plain-chat.md) — 统一对话链路（工具常驻、模型自主检索）与启动方式
- [ai-architecture.md](./ai-architecture.md) — BFF / AI 平台 / 模型直连的目标架构、Turn 契约与迁移阶段

## 2. 推荐阅读顺序

1. [brain/project.md](./brain/project.md) — 了解项目是什么
2. [brain/architecture.md](./brain/architecture.md) — 理解全局架构
3. [brain/constraints.md](./brain/constraints.md) — 明确约束边界
4. [frontend-structure.md](./frontend-structure.md) / [backend-structure.md](./backend-structure.md) — 深入子域

## 3. 使用说明

- AI 工具请先读取 brain/ 目录全部文件
- 执行任务前查看 tasks/ 中相关学习日志
- 发现可复用模式时沉淀到 skills/
