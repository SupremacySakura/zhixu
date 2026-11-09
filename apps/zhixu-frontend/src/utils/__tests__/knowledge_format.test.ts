import { describe, expect, it } from 'vitest'
import { getKnowledgeFormat, stripKnowledgeExtension } from '../knowledge'

describe('knowledge format capability table', () => {
  it('matches markdown variants case-insensitively and rejects folders', () => {
    expect(getKnowledgeFormat({ type: 'file', name: 'a.MD' })?.type).toBe('markdown')
    expect(getKnowledgeFormat({ type: 'file', name: 'a.markdown' })?.label).toBe('Markdown')
    expect(getKnowledgeFormat({ type: 'folder', name: 'a.md' })).toBeNull()
    expect(getKnowledgeFormat({ type: 'file', name: 'a.exe' })).toBeNull()
  })
  it('matches pdf files case-insensitively', () => {
    expect(getKnowledgeFormat({ type: 'file', name: '报告.pdf' })?.type).toBe('pdf')
    expect(getKnowledgeFormat({ type: 'file', name: 'report.PDF' })?.label).toBe('PDF')
    expect(getKnowledgeFormat({ type: 'file', name: 'report.pdf' })?.maxBytes).toBe(
      20 * 1024 * 1024,
    )
  })
  it('matches text files case-insensitively', () => {
    expect(getKnowledgeFormat({ type: 'file', name: '笔记.txt' })?.type).toBe('text')
    expect(getKnowledgeFormat({ type: 'file', name: 'note.TXT' })?.label).toBe('文本')
    expect(getKnowledgeFormat({ type: 'folder', name: 'note.txt' })).toBeNull()
  })
  it('matches office and csv files case-insensitively', () => {
    expect(getKnowledgeFormat({ type: 'file', name: '报告.docx' })?.type).toBe('docx')
    expect(getKnowledgeFormat({ type: 'file', name: 'REPORT.DOCX' })?.label).toBe('Word')
    expect(getKnowledgeFormat({ type: 'file', name: '数据.csv' })?.type).toBe('csv')
    expect(getKnowledgeFormat({ type: 'file', name: 'DATA.CSV' })?.label).toBe('CSV')
    expect(getKnowledgeFormat({ type: 'file', name: '销售.xlsx' })?.type).toBe('xlsx')
    expect(getKnowledgeFormat({ type: 'file', name: 'SALES.XLSX' })?.label).toBe('Excel')
    expect(getKnowledgeFormat({ type: 'file', name: 'legacy.doc' })).toBeNull()
    expect(getKnowledgeFormat({ type: 'file', name: 'legacy.xls' })).toBeNull()
  })
  it('exposes per-format size limits', () => {
    expect(getKnowledgeFormat({ type: 'file', name: 'a.md' })?.maxBytes).toBe(20 * 1024 * 1024)
  })
  it('strips the matched extension for default titles', () => {
    expect(stripKnowledgeExtension('笔记.MD')).toBe('笔记')
    expect(stripKnowledgeExtension('note.markdown')).toBe('note')
    expect(stripKnowledgeExtension('memo.txt')).toBe('memo')
    expect(stripKnowledgeExtension('报告.pdf')).toBe('报告')
    expect(stripKnowledgeExtension('合同.docx')).toBe('合同')
    expect(stripKnowledgeExtension('数据.csv')).toBe('数据')
    expect(stripKnowledgeExtension('销售.xlsx')).toBe('销售')
    expect(stripKnowledgeExtension('note.exe')).toBe('note.exe')
  })
})
