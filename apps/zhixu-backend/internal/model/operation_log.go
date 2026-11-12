package model

import "time"

// OperationLogAction 操作日志的动作类型
type OperationLogAction string

const (
	OperationLogActionUpload               OperationLogAction = "upload"                    // 分片上传完成
	OperationLogActionUploadInstant        OperationLogAction = "upload_instant"            // 秒传（命中已有文件）
	OperationLogActionMkdir                OperationLogAction = "mkdir"                     // 新建文件夹
	OperationLogActionRename               OperationLogAction = "rename"                    // 重命名
	OperationLogActionMove                 OperationLogAction = "move"                      // 移动
	OperationLogActionDelete               OperationLogAction = "delete"                    // 删除
	OperationLogActionDownload             OperationLogAction = "download"                  // 下载
	OperationLogActionPreview              OperationLogAction = "preview"                   // 预览
	OperationLogActionShareLinkCreate      OperationLogAction = "share_link_create"         // 创建分享链接
	OperationLogActionShareLinkDelete      OperationLogAction = "share_link_delete"         // 删除分享链接
	OperationLogActionPickupCodeCreate     OperationLogAction = "pickup_code_create"        // 创建取件码
	OperationLogActionPickupCodeDelete     OperationLogAction = "pickup_code_delete"        // 删除取件码
	OperationLogActionKnowledgeAdd         OperationLogAction = "knowledge_add"             // 加入知识库
	OperationLogActionKnowledgeCreate      OperationLogAction = "knowledge_create"          // 手动创建知识
	OperationLogActionKnowledgeUpdate      OperationLogAction = "knowledge_update"          // 编辑知识正文
	OperationLogActionKnowledgeRestore     OperationLogAction = "knowledge_restore"         // 恢复历史版本
	OperationLogActionKnowledgeDelete      OperationLogAction = "knowledge_delete"          // 移出知识库
	OperationLogActionKnowledgeRetry       OperationLogAction = "knowledge_retry"           // 重试知识导入
	OperationLogActionKnowledgeVectorRetry OperationLogAction = "knowledge_vector_retry"    // 重试知识向量化
	OperationLogActionKnowledgeFavorite    OperationLogAction = "knowledge_favorite"        // 收藏知识
	OperationLogActionKnowledgeArchive     OperationLogAction = "knowledge_archive"         // 归档知识
	OperationLogActionKnowledgeUnarchive   OperationLogAction = "knowledge_unarchive"       // 恢复归档
	OperationLogActionKnowledgeWebImport   OperationLogAction = "knowledge_web_import"      // 从网页导入知识
	OperationLogActionKnowledgeOrganize    OperationLogAction = "knowledge_organize_accept" // 采纳 AI 整理结果
	OperationLogActionEvolutionResolve     OperationLogAction = "evolution_resolve"         // 裁决演化提醒
	OperationLogActionKnowledgeOutdated    OperationLogAction = "knowledge_outdated"        // 标记知识可能过时
	OperationLogActionKnowledgeVerified    OperationLogAction = "knowledge_verified"        // 标记知识已核实（清除过时）
)

// OperationLogTargetType 操作目标的实体类型
type OperationLogTargetType string

const (
	OperationLogTargetTypeFile      OperationLogTargetType = "file"
	OperationLogTargetTypeFolder    OperationLogTargetType = "folder"
	OperationLogTargetTypeKnowledge OperationLogTargetType = "knowledge"
	OperationLogTargetTypeEvolution OperationLogTargetType = "evolution"
)

// OperationLogModel 用户操作日志。
// TargetName / FolderName 为操作发生时的名称快照，目标后续被改名或删除后日志仍可读。
type OperationLogModel struct {
	ID         uint                   `gorm:"primaryKey" json:"id"`
	UserID     uint                   `gorm:"not null;index:idx_op_logs_user_time,priority:1" json:"user_id"` // 操作者
	Action     OperationLogAction     `gorm:"type:varchar(32);not null" json:"action"`                        // 操作类型
	TargetType OperationLogTargetType `gorm:"type:varchar(16);not null" json:"target_type"`                   // 目标实体类型
	TargetID   uint                   `gorm:"not null;default:0" json:"target_id"`                            // 目标实体 ID
	TargetName string                 `gorm:"type:varchar(255);not null" json:"target_name"`                  // 目标名称快照
	FolderName string                 `gorm:"type:varchar(255);not null;default:''" json:"folder_name"`       // 所在目录快照，空=根目录/不适用
	Detail     string                 `gorm:"type:json" json:"detail"`                                        // 扩展信息 JSON：size/old_name/from/to/code 等
	CreatedAt  time.Time              `gorm:"not null;index:idx_op_logs_user_time,priority:2" json:"created_at"`
}
