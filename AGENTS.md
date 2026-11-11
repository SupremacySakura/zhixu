# AGENTS.md

## AI 读取协议（所有 AI 工具必读）
1. **读取顺序**: 先读 docs/brain/ 全部 → 再按需读 docs/tasks/ 最近 3 条 → 按匹配条件读 docs/skills/
2. **写入禁止**: docs/brain/ 和 docs/skills/ 对 AI 只读，修改需人工审核
3. **冲突处理**: 代码与文档不一致时以代码为准，但需在约束文件记录差异

## 文档中心入口
- 必读层: [docs/brain/](./docs/brain/) — 项目全局认知
- 学习层: [docs/tasks/](./docs/tasks/) — 任务结果与教训
- 能力层: [docs/skills/](./docs/skills/) — 可复用操作模式
- 详细参考: [docs/frontend-structure.md](./docs/frontend-structure.md) | [docs/backend-structure.md](./docs/backend-structure.md)
- 文档约定: [docs/brain/conventions.md](./docs/brain/conventions.md)

## 子项目索引
- 前端: [apps/zhixu-frontend/AGENTS.md](./apps/zhixu-frontend/AGENTS.md)
- 后端: [apps/zhixu-backend/AGENTS.md](./apps/zhixu-backend/AGENTS.md)

## 工具专属目录
- .trae/ — Trae IDE 工作空间（已 gitignore，不作为项目知识来源）
- .sisyphus/ — OpenCode 执行引擎（plans 是执行计划，非学习日志）

## Agent skills

### Issue tracking
- 议题和规格存在 GitHub Issues（`SupremacySakura/zhixu`），统一用 `gh` CLI 操作。详见 [docs/agents/issue-tracker.md](./docs/agents/issue-tracker.md)。

### Triage labels
- 分诊使用五种规范标签：`needs-triage` / `needs-info` / `ready-for-agent` / `ready-for-human` / `wontfix`。详见 [docs/agents/triage-labels.md](./docs/agents/triage-labels.md)。

### Domain docs
- 单上下文布局：领域术语在根 `GLOSSARY.md`，架构决策在 `docs/adr/`。详见 [docs/agents/domain.md](./docs/agents/domain.md)。

