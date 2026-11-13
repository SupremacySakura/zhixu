import { FileType } from '../types/file'
import type { FileListItem } from '../services/types/file'

const textPreviewExtensions = [
  'txt',
  'md',
  'json',
  'csv',
  'log',
  'xml',
  'yaml',
  'yml',
  'html',
  'css',
  'js',
  'jsx',
  'ts',
  'tsx',
  'vue',
]

export const getFileExtension = (name: string) => {
  const idx = name.lastIndexOf('.')
  if (idx < 0 || idx === name.length - 1) return ''
  return name.slice(idx + 1).toLowerCase()
}

export const normalizeMimeType = (mimeType: string) => {
  return (mimeType || '').split(';')[0].trim().toLowerCase()
}

export const isTextPreviewFileName = (name: string) => {
  return textPreviewExtensions.includes(getFileExtension(name))
}

export const inferPreviewMimeType = (name: string, mimeType: string) => {
  const ext = getFileExtension(name)
  if (isTextPreviewFileName(name)) return 'text/plain'

  const normalized = normalizeMimeType(mimeType)
  if (normalized && normalized !== 'application/octet-stream') return normalized

  if (ext === 'pdf') return 'application/pdf'
  if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg'].includes(ext))
    return `image/${ext === 'jpg' ? 'jpeg' : ext}`
  if (['mp4', 'webm', 'ogg', 'mov', 'm4v', 'avi', 'mkv', 'mpeg'].includes(ext))
    return `video/${ext === 'mov' ? 'mp4' : ext}`
  if (['mp3', 'wav', 'ogg', 'aac', 'flac', 'm4a'].includes(ext)) return `audio/${ext}`
  return 'application/octet-stream'
}

export const isTextPreviewable = (mimeType: string, name: string) => {
  if (isTextPreviewFileName(name)) return true
  return normalizeMimeType(mimeType).startsWith('text/')
}

/**
 * 格式化字节数为人类可读的字符串
 * @param bytes 字节数
 * @returns 格式化后的字符串，例如 "1.23 MB"
 */
export const formatBytes = (bytes: number) => {
  const v = Number.isFinite(bytes) ? bytes : 0
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = Math.max(0, v)
  let idx = 0
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024
    idx += 1
  }
  const fixed = idx === 0 ? 0 : 1
  return `${size.toFixed(fixed)} ${units[idx]}`
}

/**
 * 检测文件类型
 * @param file 文件对象
 * @returns 文件类型枚举值
 */
export const detectFileType = (file: File) => {
  const t = file.type || ''
  if (isTextPreviewFileName(file.name)) return FileType.Document
  if (t.startsWith('image/')) return FileType.Image
  if (t.startsWith('video/')) return FileType.Video
  if (t.startsWith('audio/')) return FileType.Audio
  if (
    t.includes('pdf') ||
    t.includes('word') ||
    t.includes('excel') ||
    t.includes('powerpoint') ||
    t.startsWith('text/')
  )
    return FileType.Document
  return FileType.Other
}

/**
 * 根据文件类型返回对应的图标和颜色类名
 * @param file 文件对象
 * @returns 包含图标类名、背景颜色类名和前景颜色类名的对象
 */
export const iconForFile = (file: File) => {
  const kind = detectFileType(file)
  if (kind === FileType.Video)
    return { icon: 'material-symbols:movie', bg: 'bg-success-tint', fg: 'text-success' }
  if (kind === FileType.Image)
    return { icon: 'material-symbols:image', bg: 'bg-danger-tint', fg: 'text-danger' }
  if (kind === FileType.Audio)
    return { icon: 'material-symbols:music-note', bg: 'bg-primary-tint', fg: 'text-primary' }
  if (kind === FileType.Document)
    return { icon: 'material-symbols:description', bg: 'bg-primary-tint', fg: 'text-primary' }
  return { icon: 'material-symbols:description', bg: 'bg-primary-tint', fg: 'text-primary' }
}

export const iconForListItem = (item: Pick<FileListItem, 'type' | 'file_type'>) => {
  if (item.type === 'folder') {
    return { icon: 'material-symbols:folder', bg: 'bg-warning-tint', fg: 'text-warning' }
  }
  const kind = item.file_type as FileType
  if (kind === FileType.Video)
    return { icon: 'material-symbols:movie', bg: 'bg-success-tint', fg: 'text-success' }
  if (kind === FileType.Image)
    return { icon: 'material-symbols:image', bg: 'bg-danger-tint', fg: 'text-danger' }
  if (kind === FileType.Audio)
    return { icon: 'material-symbols:music-note', bg: 'bg-primary-tint', fg: 'text-primary' }
  if (kind === FileType.Document)
    return { icon: 'material-symbols:description', bg: 'bg-primary-tint', fg: 'text-primary' }
  return { icon: 'material-symbols:description', bg: 'bg-primary-tint', fg: 'text-primary' }
}

export const typeLabelForListItem = (item: Pick<FileListItem, 'type' | 'file_type'>) => {
  if (item.type === 'folder') return '文件夹'
  const kind = item.file_type as FileType
  if (kind === FileType.Image) return '图片'
  if (kind === FileType.Video) return '视频'
  if (kind === FileType.Audio) return '音频'
  if (kind === FileType.Document) return '文档'
  return '其他'
}

export const formatTime = (value: string) => {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

/**
 * 清理文件名中的控制字符，避免不可见字符影响展示
 * @param name 文件名
 * @returns 清理后的文件名
 */
export const sanitizeFileName = (name: string): string => {
  if (!name) return ''
  return name.replace(/[\x00-\x1f\x7f]/g, '')
}
