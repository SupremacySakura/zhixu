import { describe, it, expect } from 'vitest'
import {
  detectFileType,
  iconForFile,
  iconForListItem,
  typeLabelForListItem,
  formatTime,
  sanitizeFileName,
  inferPreviewMimeType,
  isTextPreviewable,
} from '../file'
import { FileType } from '../../types/file'

describe('detectFileType', () => {
  it('should detect image files', () => {
    const imageFile = new File([''], 'test.jpg', { type: 'image/jpeg' })
    expect(detectFileType(imageFile)).toBe(FileType.Image)
  })

  it('should detect video files', () => {
    const videoFile = new File([''], 'test.mp4', { type: 'video/mp4' })
    expect(detectFileType(videoFile)).toBe(FileType.Video)
  })

  it('should detect audio files', () => {
    const audioFile = new File([''], 'test.mp3', { type: 'audio/mpeg' })
    expect(detectFileType(audioFile)).toBe(FileType.Audio)
  })

  it('should detect PDF files as documents', () => {
    const pdfFile = new File([''], 'test.pdf', { type: 'application/pdf' })
    expect(detectFileType(pdfFile)).toBe(FileType.Document)
  })

  it('should detect Word files as documents', () => {
    const docFile = new File([''], 'test.docx', {
      type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
    })
    expect(detectFileType(docFile)).toBe(FileType.Document)
  })

  it('should detect text files as documents', () => {
    const txtFile = new File([''], 'test.txt', { type: 'text/plain' })
    expect(detectFileType(txtFile)).toBe(FileType.Document)
  })

  it('should detect TypeScript declaration files as documents even with video mime type', () => {
    const declarationFile = new File([''], 'next-env.d.ts', { type: 'video/mp2t' })
    expect(detectFileType(declarationFile)).toBe(FileType.Document)
  })

  it('should detect unknown files as other', () => {
    const unknownFile = new File([''], 'test.xyz', { type: 'application/x-unknown' })
    expect(detectFileType(unknownFile)).toBe(FileType.Other)
  })

  it('should handle files without type', () => {
    const noTypeFile = new File([''], 'test')
    expect(detectFileType(noTypeFile)).toBe(FileType.Other)
  })
})

describe('iconForFile', () => {
  it('should return video icon for video files', () => {
    const videoFile = new File([''], 'test.mp4', { type: 'video/mp4' })
    const result = iconForFile(videoFile)
    expect(result.icon).toBe('material-symbols:movie')
    expect(result.bg).toBe('bg-success-tint')
    expect(result.fg).toBe('text-success')
  })

  it('should return image icon for image files', () => {
    const imageFile = new File([''], 'test.png', { type: 'image/png' })
    const result = iconForFile(imageFile)
    expect(result.icon).toBe('material-symbols:image')
    expect(result.bg).toBe('bg-danger-tint')
    expect(result.fg).toBe('text-danger')
  })

  it('should return music icon for audio files', () => {
    const audioFile = new File([''], 'test.mp3', { type: 'audio/mpeg' })
    const result = iconForFile(audioFile)
    expect(result.icon).toBe('material-symbols:music-note')
    expect(result.bg).toBe('bg-primary-tint')
    expect(result.fg).toBe('text-primary')
  })

  it('should return document icon for document files', () => {
    const pdfFile = new File([''], 'test.pdf', { type: 'application/pdf' })
    const result = iconForFile(pdfFile)
    expect(result.icon).toBe('material-symbols:description')
    expect(result.bg).toBe('bg-primary-tint')
    expect(result.fg).toBe('text-primary')
  })

  it('should return default icon for other files', () => {
    const otherFile = new File([''], 'test.xyz', { type: 'application/x-unknown' })
    const result = iconForFile(otherFile)
    expect(result.icon).toBe('material-symbols:description')
  })
})

describe('iconForListItem', () => {
  it('should return folder icon for folders', () => {
    const folder = { type: 'folder' as const, file_type: '' }
    const result = iconForListItem(folder)
    expect(result.icon).toBe('material-symbols:folder')
    expect(result.bg).toBe('bg-warning-tint')
    expect(result.fg).toBe('text-warning')
  })

  it('should return video icon for video type', () => {
    const video = { type: 'file' as const, file_type: FileType.Video }
    const result = iconForListItem(video)
    expect(result.icon).toBe('material-symbols:movie')
    expect(result.bg).toBe('bg-success-tint')
    expect(result.fg).toBe('text-success')
  })

  it('should return image icon for image type', () => {
    const image = { type: 'file' as const, file_type: FileType.Image }
    const result = iconForListItem(image)
    expect(result.icon).toBe('material-symbols:image')
    expect(result.bg).toBe('bg-danger-tint')
    expect(result.fg).toBe('text-danger')
  })

  it('should return music icon for audio type', () => {
    const audio = { type: 'file' as const, file_type: FileType.Audio }
    const result = iconForListItem(audio)
    expect(result.icon).toBe('material-symbols:music-note')
    expect(result.bg).toBe('bg-primary-tint')
    expect(result.fg).toBe('text-primary')
  })

  it('should return document icon for document type', () => {
    const doc = { type: 'file' as const, file_type: FileType.Document }
    const result = iconForListItem(doc)
    expect(result.icon).toBe('material-symbols:description')
    expect(result.bg).toBe('bg-primary-tint')
    expect(result.fg).toBe('text-primary')
  })

  it('should return default icon for other type', () => {
    const other = { type: 'file' as const, file_type: FileType.Other }
    const result = iconForListItem(other)
    expect(result.icon).toBe('material-symbols:description')
  })
})

describe('typeLabelForListItem', () => {
  it('should return 文件夹 for folders', () => {
    const folder = { type: 'folder' as const, file_type: '' }
    expect(typeLabelForListItem(folder)).toBe('文件夹')
  })

  it('should return 图片 for image type', () => {
    const image = { type: 'file' as const, file_type: FileType.Image }
    expect(typeLabelForListItem(image)).toBe('图片')
  })

  it('should return 视频 for video type', () => {
    const video = { type: 'file' as const, file_type: FileType.Video }
    expect(typeLabelForListItem(video)).toBe('视频')
  })

  it('should return 音频 for audio type', () => {
    const audio = { type: 'file' as const, file_type: FileType.Audio }
    expect(typeLabelForListItem(audio)).toBe('音频')
  })

  it('should return 文档 for document type', () => {
    const doc = { type: 'file' as const, file_type: FileType.Document }
    expect(typeLabelForListItem(doc)).toBe('文档')
  })

  it('should return 其他 for custom file_type values', () => {
    const other = { type: 'file' as const, file_type: 'custom' }
    expect(typeLabelForListItem(other)).toBe('其他')
  })

  it('should return 其他 for undefined file_type', () => {
    const undefinedType = { type: 'file' as const, file_type: undefined as any }
    expect(typeLabelForListItem(undefinedType)).toBe('其他')
  })
})

describe('formatTime', () => {
  it('should return dash for empty value', () => {
    expect(formatTime('')).toBe('-')
  })

  it('should return dash for null/undefined', () => {
    expect(formatTime(null as any)).toBe('-')
    expect(formatTime(undefined as any)).toBe('-')
  })

  it('should format valid ISO date', () => {
    const isoDate = '2024-01-15T10:30:00Z'
    const result = formatTime(isoDate)
    expect(result).not.toBe('-')
    expect(result).not.toBe(isoDate)
    // Locale and timezone follow the user's environment.
    expect(result).toBe(new Date(isoDate).toLocaleString())
  })

  it('should return original value for invalid date', () => {
    const invalidDate = 'not-a-date'
    expect(formatTime(invalidDate)).toBe(invalidDate)
  })
})

describe('sanitizeFileName', () => {
  it('should remove control characters', () => {
    const name = 'test\x00file\x1fname\x7f.txt'
    expect(sanitizeFileName(name)).toBe('testfilename.txt')
  })

  it('should return empty string for empty input', () => {
    expect(sanitizeFileName('')).toBe('')
  })

  it('should return empty string for null/undefined', () => {
    expect(sanitizeFileName(null as any)).toBe('')
    expect(sanitizeFileName(undefined as any)).toBe('')
  })

  it('should keep valid characters intact', () => {
    const name = 'normal-file_name.txt'
    expect(sanitizeFileName(name)).toBe(name)
  })

  it('should handle special Unicode characters', () => {
    const name = '文件📄名称.txt'
    expect(sanitizeFileName(name)).toBe(name)
  })
})

describe('preview file type helpers', () => {
  it('should prefer TypeScript declaration extension over video/mp2t mime type', () => {
    expect(inferPreviewMimeType('next-env.d.ts', 'video/mp2t')).toBe('text/plain')
    expect(isTextPreviewable('video/mp2t', 'next-env.d.ts')).toBe(true)
  })
})
