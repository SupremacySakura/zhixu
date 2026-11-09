---
task_id: knowledge-multiformat-20261109
status: completed
created: 2026-11-09
last_reviewed: 2026-11-09
applicable_to: "知识库入库、文件格式提取、DOCX/PDF/XLSX/CSV 解析、上传白名单、前端文件能力表"
---

# 知识库多格式入库：提取器抽象 + TXT/PDF/Word/CSV/Excel

## 背景

知识库原本只接受 `.md/.markdown`：`validMarkdown` 按后缀放行、`readMarkdown` 手工解析编码、`source_type` 固定写 `"markdown"`。目标是让多种格式加入知识库，且不把格式差异扩散到分块、向量化与前端渲染。方案定为“统一提取为规范化 Markdown”：新增 Extractor 提取层，所有格式在校验、读取、编码处理后产出同一份 Markdown 正文写入 `knowledge_items.content`，下游零改动。

## 结果

- **后端提取器**（`apps/zhixu-backend/internal/service/`）：
  - `extractor.go`：`Extractor` 接口 `{SourceType; Label; MatchesExt; MaxBytes; Extract}` + `var extractors` 有序注册表；`extractorForExt`/`extractorForSourceType`（空 `source_type` 兜底 markdown）；`validateSource` 校验扩展名与元数据大小；`openSourceFile` 统一受限根目录读取（hex 校验、hash 分片、旧 `hash.extension` 回退）；`callExtractor` recover → `parse_failed`；`extractSource` 实际大小预检后分派。
  - `extractor_markdown.go`：Markdown 行为迁移进提取器，语义不变（BOM、UTF-8+NUL 校验、换行统一、空正文 `empty_content`）。
  - `extractor_text.go`：`.txt`，编码探测 BOM → UTF-16LE/BE → UTF-8 → GBK。
  - `extractor_pdf.go`：`github.com/ledongthuc/pdf` 逐页取文本层，按 `## 第 N 页` 分段；加密 → `encrypted_file`；单页失败 → `parse_failed`；无文本 → `scanned_pdf_no_text`；非法 UTF-8 用 `ToValidUTF8` 剥离。
  - `extractor_ooxml.go` + `extractor_docx.go` + `extractor_xlsx.go`：标准库 `archive/zip`+`encoding/xml` 解析 OOXML；docx 标题样式（`styles.xml` 按样式名兼容 heading/标题/標題）→ `#` 层级、`numbering.xml` 区分有序/无序列表并缩进、外链 → Markdown 链接（保留文本与目标 URL）、表格 → GFM；xlsx 逐工作表输出 `## 表名` + GFM 表（共享字符串、inlineStr、布尔、首行表头、空表跳过）；解压总量 50 MiB、单部件 30 MiB 防炸弹。
  - `extractor_csv.go`：编码探测后 `encoding/csv`（`FieldsPerRecord=-1`、`LazyQuotes`）转 GFM 表，首行表头。
  - `knowledge.go`：`Add/Retry` 用 `validateSource`，`source_type` 取自提取器；`ProcessNext` 用 `extractSource`；handler swagger 摘要泛化为“支持格式由后端能力表决定”。
  - `file.go`：上传白名单增加 docx/xlsx 的 OOXML MIME。
- **前端**：`utils/knowledge.ts` 的 `KNOWLEDGE_FORMATS` 能力表登记 markdown/text/pdf/docx/csv/xlsx（label 短名：Markdown/文本/PDF/Word/CSV/Excel），门控、大小限制、默认标题剥扩展名、详情页来源徽章全部由表驱动；`renderKnowledgeContent(content, sourceType)` 对 `text` 使用 breaks 渲染，其余统一 markdown-it（表格已横向滚动、PDF 页标题即 h2）。
- **测试**：后端新增 `extractor_text_test.go`、`extractor_pdf_test.go`（手工构造带 xref 的最小多页 PDF）、`extractor_office_test.go`（内存 zip 夹具：docx 标题/列表/外链/表格、xlsx 多表/共享字符串/空表、csv 编码/参差行/转义；含 zip 炸弹用例）；前端 `knowledge_format.test.ts` 覆盖六格式匹配与标题剥离，`markdown.test.ts` 覆盖 breaks。全量：后端 `go test ./...` 全绿，前端 227 测试 + lint + typecheck 通过。
- **文档**：`markdown-knowledge-backend.md` 改为多格式正文（新增“支持格式与提取器”表、安全限额、新增错误码），`markdown-knowledge-frontend.md` 同步能力表与渲染差异。

## 决策与理由

- **统一提取为 Markdown，而不是按格式分叉存储**：分块器只认 Markdown 标题，向量化只消费 `content`；让提取层吸收格式差异，新增格式只需实现一个接口，不必改知识表结构、分块与向量化链路。
- **不引入外部转换服务/二进制（LibreOffice、pdftotext）**：提取全在 Go 内完成，部署不新增运行时；pdf/docx/xlsx/csv 都能用库或标准库覆盖 RAG 需要的正文与结构。
- **版式与图片刻意丢弃**：RAG 只检索文本；保真转换成本高且会引入图片资源管理。PDF 页标题保留是唯一“位置”信息，用于定位与分块锚点。
- **扫描件/加密明确失败而非产出空知识**：`scanned_pdf_no_text`/`encrypted_file` 让用户知道该做什么（换文本版 PDF），避免“入库成功但正文为空”的隐性失败。
- **OOXML 防炸弹放在解压层**：文件大小上限挡不住高压缩比 zip；解压总量/单部件双限额在读取路径上直接执行，`encoding/xml` 本身不解析 DTD 与外部实体，无需额外 XXE 处理。
- **能力表放前端常量而非向后端要接口**：当前格式集合由编译期决定，前端写死同一张表最简单；后端仍以自己的提取器注册表为准，形成双保险（前端门控、后端校验）。
- **`source_type` 兜底 markdown**：历史数据与旧测试 fixture 的 `source_type` 为空，兜底保证行为不变，避免数据迁移。

## 教训

- **测试里的“不支持格式”样例会随能力表过期**：`TestKnowledgeAdd/unsupported` 原用 `note.pdf`，PDF 支持后该用例静默失去意义；应改用 `note.exe` 这类永久不支持的扩展名。前端同理（`a.pdf` → `a.exe`）。
- **Go 沙箱下的模块与校验缓存**：`~/go/pkg/mod` 与 `~/go/pkg/sumdb` 不可写，需 `GOCACHE`/`GOMODCACHE` 指向 `/tmp`；`proxy.golang.org` 不通时用 `GOPROXY=https://goproxy.cn,direct` + `GOSUMDB=off`，哈希仍会固定进 `go.sum`。
- **PDF/docx 夹具要真实可解析**：PDF 需精确 xref 偏移（`buildTestPDF`），OOXML 用 `archive/zip` 内存构造；这类测试能在无外部样本时验证真实解析路径。
- **GBK 与 UTF-8 的歧义**：GBK“中文”恰好在偶数字节下也是合法 UTF-8；测试夹具要用全角逗号（`A3 AC`）这类含非法 UTF-8 起始字节的字符强制走 GBK 分支，否则用例会假通过。
- **`transform.Reader` 是类型不是构造函数**：编码解码要 `transform.NewReader`；错误应通过错误路径返回而非 panic。
- **XML 解码“任何错误都当结束”等于静默丢数据**：截断输入返回的是 `unexpected EOF` 而非 `io.EOF`，必须区分；同理“容错收尾半行”会把损坏文件包装成成功入库。
- **`failMarkdown` 返回 `error` 而非 `*markdownFailure`**：给内部 helper 定签名时先看 `failMarkdown` 的返回类型，避免接口/指针混用导致编译错误。

## 代码审查修复（2026-11-09）

对 staged diff 的 code review 提出 4 项，逐项核实结论：

- **CR-001 [P1] XLSX 列引用/展开无上限（真实）**：`xlsxColIndex("ZZZZZZZZ1")` 会算出 217,180,147,157，`for len(cells) < col` 补齐空单元格约需 3.4 TB 内存；共享字符串被大量引用时 Markdown 输出也无上限，解压限额拦不住。修复：列号超过 Excel 上限（XFD=16384）返回 `parse_failed`；全文件单元格数 ≤ 1,000,000（`file_too_large`）；展开后 Markdown ≤ 20 MiB（`cappedBuilder` 限写，`file_too_large`）；DOCX 正文同样以 20 MiB 为界。
- **CR-002 [P2] 截断 XML 被当成成功（真实）**：`xml.Decoder` 对截断返回 `unexpected EOF`（非 `io.EOF`），原实现把任何错误都当作结束并返回已解析内容；XLSX 还会 flush 半行。修复：所有内容 XML（docx `document.xml`、xlsx 工作簿/工作表/共享字符串）区分 `io.EOF` 与其他错误，后者报 `parse_failed`。
- **CR-003 [P2] 参差表格列丢失（真实）**：用 `markdown-it` 实测 `| a | b |` + `| 1 | 2 | 3 |`，第三格不渲染。修复：`writeMarkdownTable` 先取最大列数，表头/分隔行/数据行统一补齐；CSV 用例补 3 列分隔行断言。
- **CR-004 [P3] 历史空 `source_type` 标签（不成立）**：`source_type` 列为 `not null`，`Add` 自初版起固定写入类型，空值只存在于测试 fake。仍做防御性修复：详情页用 `||` 兜底 Markdown 标签。

回归测试：`TestXlsxColIndex`（含 `XFD1`/`XFE1`/`ZZZZZZZZ1` 边界）、DOCX/XLSX 截断 → `parse_failed`、XLSX 超大列引用 → `parse_failed`、共享字符串重复展开 → `file_too_large`、CSV 参差行整表补齐。后端 `go test ./...`、前端 227 测试 + lint + typecheck 全绿。

## 第二轮代码审查修复（2026-11-09）

第二轮 review 提出 2 项 P1，均核实为真实存在并已修复：

- **CR-001 [P1] XLSX 稀疏列补齐绕过单元格上限（真实）**：`maxMarkdownCells` 原先只在解析到真实单元格时递增，而 `for len(row) < col` 会按列号为稀疏行补出空单元格。构造"大量行、每行只有一个位于最右列 XFD 的单元格"时，10 万行即尝试分配约 16 亿个字符串槽位，在渲染上限生效前就会耗尽内存。修复：把补齐出的空单元格计入同一上限（`pad := col - len(row)` 先计数后分配），超限立即报 `file_too_large`（"XLSX 单元格数量过多"）。回归测试 `TestXlsxSparseColumnsBounded`（200 行 XFD 稀疏行 → 解析期失败，0.01s，不再做大分配）。
- **CR-005 [P1] DOCX 正文大小检查晚于完整构建（真实）**：外链文本节点会重复输出（每个文本节点都会附带完整 URL），超长 URL 配合大量文本节点可在 `docxToMarkdown` 构建期放大内存，而 20 MiB 检查发生在构建完成之后。修复：正文缓冲 `out` 与单段落缓冲 `para` 均改为 `cappedBuilder`（新增 `cappedBuilder.Reset` 支持段落复用），构建期超限即返回 `file_too_large`；同时限制外链目标 ≤ `maxDocxLinkBytes`（2048 字节），超长链接退化为纯文本。回归测试 `TestDocxLinkAmplificationBounded`（单段落 / 多段落两种放大路径）、`TestDocxOverlongLinkFallsBackToText`（超长链接保留文本、不生成 Markdown 链接）。

验证：`go test ./...` 全绿（含新增 3 个回归测试），`gofmt` 干净。

## 第三轮代码审查修复（2026-11-09）

第三轮 review 提出 1 项 P1，核实为真实存在并已修复：

- **CR-001 [P1] 表格按最大列宽补齐仍可触发巨量遍历/分配（真实）**：解析期计数（真实单元格 + 行内补齐）仍不等于渲染期展开量——`writeMarkdownTable` 会把每一行补到全表最大列宽，因此"大量窄行 + 一个 XFD 超宽行"在解析期计数远低于 100 万的情况下，渲染要遍历 `16384 × 行数` 个单元格；`cappedBuilder` 只能截断输出，循环本身不会停（写满后仍继续遍历），可长时间占住唯一导入 worker。DOCX 更严重：表格先写入无上限的 `strings.Builder` 临时缓冲，再交给限写正文构建器，放大发生在限额生效之前。修复：① `writeMarkdownTable(out, rows, maxCells int64) error` 先按 `width×行数` 设限（`maxTableCells = 4_000_000`，与 20 MiB 输出上限相称），超限报 `file_too_large`("表格按最大列宽补齐后的内容过大")；② 写入循环检测 `overflowed(out)`，写满上限立即停止遍历；③ DOCX 表格临时缓冲改为 `cappedBuilder`，表格单元格总数同样受 `maxTableCells` 限制，并区分 XML 截断（非 `io.EOF` 返回 `failTruncatedXML`）；④ CSV 渲染也改用 `cappedBuilder`（展开 >20 MiB → `file_too_large`），与其它格式的统一限额对齐。
- 新增回归测试：`TestXlsxTablePaddingBounded`（10000 窄行 + 1 个 XFD 行 → 渲染期 0.02s 内拒绝）、`TestDocxTablePaddingBounded`（3000 窄行 + 1 行 2000 单元格 → 0.01s 内拒绝），均断言不再做巨量分配/遍历。

验证：`go test ./...` 全绿，`go vet ./...`、`gofmt` 干净；前端 227 测试 + lint + typecheck 通过。

## 差异清单（待人工审核）

- `docs/brain/project.md` 仍写“Markdown 知识入库”，实际已支持多格式；`docs/brain/` 对 AI 只读，需人工审核后更新。
- 遗留局限（候选特性）：`.doc/.xls`（OLE2 旧格式）不支持；PDF 不做 OCR 与版面还原（多栏顺序可能错乱）；xlsx 日期/货币显示格式不还原（显示序列值）；docx 图片、页眉页脚、脚注丢弃；表格单元格内换行折成空格。
- 待评估风险：PDF 由第三方库 `ledongthuc/pdf` 读取压缩流，解压放大没有显式限额（文件本身 ≤ 20 MiB，但压缩流理论膨胀比不受控）；后续可评估换用带解压上限的读取方式或加超时。

## 验证

```bash
pnpm test:backend   # GOCACHE/GOMODCACHE 指向 /tmp，GOPROXY=goproxy.cn
go test ./...       # 在 apps/zhixu-backend
pnpm test:run && pnpm lint && pnpm typecheck   # 在 apps/zhixu-frontend
```
