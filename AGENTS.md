# AGENTS.md

## AI 读取协议（所有 AI 工具必读）
1. **读取顺序**: 先读 docs/brain/ 全部 → 再按需读 docs/tasks/ 最近 3 条 → 按匹配条件读 docs/skills/
2. **写入禁止**: docs/brain/ 和 docs/skills/ 对 AI 只读，修改需人工审核
3. **冲突处理**: 代码与文档不一致时以代码为准，但需在约束文件记录差异

## 过程播报（每轮都适用）

多步任务中，**每完成一个阶段就出声**：探查完、写完一批文件、验完一轮，用一两句话说明「现在在做什么 / 到哪一步了 / 下一步是什么」，然后继续做。不要让用户面对几十个工具调用而屏幕空白——他们看不到工具输出，只看得到你的正文。

判定标准：连续 5 个工具调用没有任何正文，就是播报漏了。

任务结束时再按 `docs/brain/ai-execution.md` 的第 4 步汇报（改动文件 / 行为变化 / 验证结果 / 残余风险）。

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

### Environment
- 本机环境事实（限流 60 req/min、无 playwright、GOCACHE 坑、worktree 软链）：[docs/agents/environment.md](./docs/agents/environment.md)。**要跑截图、写并发任务、或遇到"莫名被弹回登录页"时先读它。**

### Browser screenshots
- 视觉改动的验证配方（headless Chrome + CDP、登录态、多视口、断网验证、错误态注入）：[docs/agents/browser-screenshots.md](./docs/agents/browser-screenshots.md)。**任何改动涉及 CSS/令牌/布局/图标/字体时读它。**

### Issue tracking
- 议题和规格存在 GitHub Issues（`SupremacySakura/zhixu`），统一用 `gh` CLI 操作。详见 [docs/agents/issue-tracker.md](./docs/agents/issue-tracker.md)。

### Triage labels
- 分诊使用五种规范标签：`needs-triage` / `needs-info` / `ready-for-agent` / `ready-for-human` / `wontfix`。详见 [docs/agents/triage-labels.md](./docs/agents/triage-labels.md)。

### Domain docs
- 单上下文布局：领域术语在根 `GLOSSARY.md`，架构决策在 `docs/adr/`。详见 [docs/agents/domain.md](./docs/agents/domain.md)。

