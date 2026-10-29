---
task_id: knowledge-operation-log-20261029
status: completed
created: 2026-10-29
last_reviewed: 2026-10-29
applicable_to: "知识库、操作日志、仪表盘动态、级联删除"
---

# 知识库操作接入操作日志

## 背景

操作日志（仪表盘 feed）此前只覆盖文件域 12 种 action，知识库域一条都没有：加入、删除、重试导入都不可见，文件/文件夹级联删除知识后用户也无从感知。

## 结果

- `OperationLogTargetType` 新增 `knowledge`；新增 `knowledge_add`（加入知识库）、`knowledge_delete`（移出知识库）、`knowledge_retry`（重试导入）三个 action，纯字符串枚举，无 schema 迁移。
- 抽取包级 `writeOperationLog`（service/operation_log.go），`fileService.recordOperation` 保持原签名内部改调；`OperationLogWriter` 接口供测试替身注入。`KnowledgeService.Logs` 为可选依赖（对齐 `Vectors VectorizeStore` 装配模式），nil 时静默跳过。
- `KnowledgeService.Add` 仅 `created == true` 时记日志（幂等重复不重复记），detail 带 `source_file`；`Delete` 先 `Get` 取快照再删，**不存在的 id 由静默 200 变为 404**（与 Get/Retry 语义一致，按缺陷修复）；`Retry` 成功后记。
- 级联删除不逐条写知识日志（防文件夹删除冲刷 feed），改为 `DeleteKnowledgeRows` 返回 `(int, error)`，文件/文件夹 `delete` 日志 detail 增加 `knowledge_removed: n`（n>0 时）；计数来自既有的行锁查询，零额外开销。
- 前端 `describeOperationLog` 新增三个 case 与 `knowledge` 回退标题「未知知识」；`delete` 在 detail 含正数 `knowledge_removed` 时副标题追加「，含 N 条知识」。DashboardPage 泛型渲染无需改动。
- Swagger 三文件由 `swag init` 重新生成（枚举增量）。

## 决策与理由

- 日志目标名/目录取知识条目上的来源快照（`Title`/`SourceFolderName`），天然抗后续改名删除，无需现查。
- `Delete` 改为先读后删：既为快照，也顺带修正"删除不存在 id 返回 200"的语义缺陷。
- 知识域服务通过导出字段装配日志依赖而非改构造函数：`NewKnowledgeService` 签名不动，测试与旧装配点零破坏。

## 教训

- 共享日志助手的参数若用具体类型，nil 检查会失效（Go typed-nil 接口陷阱）：调用方必须先判 nil 再传入接口参数。
- 抽取通用函数时注意调用方副作用差异：文件域 recordOperation 原先 nil repo 时不查目录名，抽取后要保留这个提前返回，否则 sqlmock 测试会多出意外查询。

## 候选特性（未实施）

- **重建向量索引记日志**：模型设置中更换 Embedding 并 `confirm_reindex` 触发全量重建，属于设置域操作，跨服务装配（AIConfigService → 日志仓库），单独立项。

## 验证

- 后端 `go build/vet/test ./...` 全绿；新增用例：知识 add/delete/retry 日志快照与幂等/冲突不记、nil Logs 静默、文件与文件夹级联删除日志 detail 计数、`DeleteKnowledgeRows` 计数与空集零值。
- 前端 vitest 全绿；新增用例：三个知识 action 映射、级联计数副标题、knowledge 回退标题。
- 未做浏览器真机验证；需要重启后端加载新逻辑（无 schema 变更）。
