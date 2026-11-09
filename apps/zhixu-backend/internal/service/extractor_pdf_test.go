package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zhixu-backend/internal/model"

	"github.com/stretchr/testify/require"
)

// buildTestPDF 生成带 xref 表的最小多页 PDF（内容文本仅支持 ASCII）。
func buildTestPDF(t *testing.T, pages []string) []byte {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	fontObj := 3 + 2*len(pages)
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	obj("<</Type/Catalog/Pages 2 0 R>>")
	var kids strings.Builder
	for i := range pages {
		fmt.Fprintf(&kids, "%d 0 R ", 3+2*i)
	}
	obj(fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>", strings.TrimSpace(kids.String()), len(pages)))
	for i, text := range pages {
		content := fmt.Sprintf("BT /F1 18 Tf 72 720 Td (%s) Tj ET", text)
		obj(fmt.Sprintf("<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]/Contents %d 0 R/Resources<</Font<</F1 %d 0 R>>>>>>", 4+2*i, fontObj))
		obj(fmt.Sprintf("<</Length %d>>\nstream\n%s\nendstream", len(content), content))
	}
	obj("<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>")
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(offsets)+1)
	b.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return b.Bytes()
}

func writeSourceFile(t *testing.T, root, hash string, data []byte) {
	t.Helper()
	dir := filepath.Join(root, hash[:2], hash[2:4])
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, hash), data, 0600))
}

func TestPDFExtractorLifecycle(t *testing.T) {
	t.Run("extracts page text with page headings", func(t *testing.T) {
		root := t.TempDir()
		hash := "aabbccddeeff"
		writeSourceFile(t, root, hash, buildTestPDF(t, []string{"Hello World", "Second page text"}))
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "report.pdf", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: 1, SourceHash: hash, SourceType: "pdf"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "", f.code)
		require.Contains(t, f.content, "## 第 1 页")
		require.Contains(t, f.content, "Hello World")
		require.Contains(t, f.content, "## 第 2 页")
		require.Contains(t, f.content, "Second page text")
	})

	t.Run("add sets pdf source type", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "报告.PDF", FileHash: "aabbccdd", Size: 1}}
		item, created, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.NoError(t, err)
		require.True(t, created)
		require.Equal(t, "pdf", item.SourceType)
		require.Equal(t, "报告", item.Title)
	})

	t.Run("textless page reports scanned_pdf_no_text", func(t *testing.T) {
		root := t.TempDir()
		hash := "bbccddeeff00"
		writeSourceFile(t, root, hash, buildTestPDF(t, []string{""}))
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "scan.pdf", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: 1, SourceHash: hash, SourceType: "pdf"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "scanned_pdf_no_text", f.code)
	})

	t.Run("corrupt file reports parse_failed", func(t *testing.T) {
		root := t.TempDir()
		hash := "ccddeeff0011"
		writeSourceFile(t, root, hash, []byte("this is not a pdf at all"))
		f := &knowledgeFake{
			file: &model.FileModel{ID: 1, UserID: 2, Name: "broken.pdf", FileHash: hash},
			item: &model.KnowledgeItem{ID: 10, UserID: 2, SourceFileID: 1, SourceHash: hash, SourceType: "pdf"},
			task: &model.KnowledgeImportTask{ID: 20, KnowledgeID: 10, UserID: 2},
		}
		processed, err := NewKnowledgeService(f, root).ProcessNext(context.Background())
		require.NoError(t, err)
		require.True(t, processed)
		require.Equal(t, "parse_failed", f.code)
	})

	t.Run("oversize pdf rejected at add", func(t *testing.T) {
		f := &knowledgeFake{file: &model.FileModel{ID: 1, UserID: 2, Name: "big.pdf", FileHash: "aabbccdd", Size: maxPDFBytes + 1}}
		_, _, err := NewKnowledgeService(f, t.TempDir()).Add(context.Background(), 2, 1, "")
		require.ErrorIs(t, err, ErrKnowledgeInvalid)
	})
}
