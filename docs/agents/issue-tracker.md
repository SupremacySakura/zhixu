# 议题跟踪器：GitHub

本仓库的议题与规格保存在 GitHub Issues（`SupremacySakura/zhixu`）。所有操作使用 `gh` CLI，在仓库根目录执行。

## 约定

- **创建议题**：`gh issue create --title "..." --body "..."`。多行正文使用 heredoc。
- **读取议题**：`gh issue view <number> --json number,title,body,labels,comments`。
- **列出议题**：`gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`，按需加 `--label`、`--state`。
- **子议题**：`gh issue create --parent <parent> ...`，或事后 `gh issue edit <parent> --add-sub-issue <child>`（gh 2.94+）。旧版走 API 且需子议题的数据库 ID；不支持时在子议题正文顶部写 `Part of #<parent>`。
- **阻塞关系**：`gh issue create --blocked-by <n>,<n>` / `gh issue edit <child> --add-blocked-by <blocker>`（gh 2.94+）。旧版 `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`，其中 `<blocker-db-id>` 是数字数据库 ID（`gh api repos/<owner>/<repo>/issues/<n> --jq .id`）。无法使用依赖关系时，在正文顶部写 `Blocked by: #<n>, #<n>`。
- **统计未关闭的阻塞项**：`gh api repos/<owner>/<repo>/issues/<n> --jq .issue_dependencies_summary.blocked_by`（0 表示可以开始）。
- **评论议题**：`gh issue comment <number> --body "..."`。
- **标签**：`gh issue edit <number> --add-label "..."` / `--remove-label "..."`。
- **关闭议题**：先 `gh issue comment` 说明，再 `gh issue close <number>`。

仓库由 `git remote -v`（origin = `SupremacySakura/zhixu`）确定，在克隆目录内 `gh` 会自动识别。只在用户自己的仓库创建议题；每次会话首次 `gh issue create` 前用 `gh repo view --json nameWithOwner` 确认目标仓库。

## 将拉取请求作为分诊入口

**将 PR 作为请求入口：否。**（若置为 `yes`，`/triage` 会把外部 PR 当作功能请求，用同样的标签与状态，命令换成 `gh pr view` / `gh pr diff` / `gh pr edit --add-label` / `gh pr close`。）

## 当技能要求“发布到议题跟踪器”

创建 GitHub 议题。

## 当技能要求“获取相关任务单”

按上文**读取议题**执行。

## Wayfinder 操作

- **地图**：一个 `wayfinder:map` 标签的议题，正文含 Notes / Decisions-so-far / Fog。
- **子任务单**：作为子议题挂在地图下，标签 `wayfinder:<type>`（research/prototype/grilling/task）；认领后分配给推进者。
- **当前边界查询**：列出地图下未关闭、无未关闭阻塞项、无负责人的子议题，取顺序最靠前者。
- **认领**：`gh issue edit <n> --add-assignee @me`（会话首次写入）。
- **解决**：`gh issue comment <n> --body "<answer>"` → `gh issue close <n>` → 在地图 Decisions-so-far 加一条上下文指针。
