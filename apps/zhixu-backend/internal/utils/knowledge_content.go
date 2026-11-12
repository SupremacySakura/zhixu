package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// knowledgeContentBOM 是 UTF-8 的字节序标记。它不可见，但会让「看起来一样」的两段正文哈希不同。
const knowledgeContentBOM = "\ufeff"

// NormalizeKnowledgeContent 把正文规约到可比较形态，只做四件事：
// 去 UTF-8 BOM、CRLF/CR 统一为 LF、逐行去行尾空白、整体 trim。
//
// 刻意不做连续空行压缩、大小写折叠与标点归一：前两者会把结构不同的文档判成同一篇，
// 后者会让「内容确实改过」的编辑被误判为重复。
func NormalizeKnowledgeContent(content string) string {
	content = strings.TrimPrefix(content, knowledgeContentBOM)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// KnowledgeContentHash 返回规范化正文的 sha256（小写十六进制）。
// 空正文返回空串：没有正文的条目不该参与精确重复判定，也不该和另一条空条目互相命中。
func KnowledgeContentHash(content string) string {
	normalized := NormalizeKnowledgeContent(content)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
