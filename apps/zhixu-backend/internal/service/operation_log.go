package service

import (
	"encoding/json"

	"zhixu-backend/internal/log"
	"zhixu-backend/internal/model"
)

// OperationLogWriter 抽象操作日志写入依赖（对齐 Vectors VectorizeStore 的装配模式），便于服务测试使用替身。
// 调用方须保证 repo 非 nil（具体指针传入接口时 nil 检查会失效）。
type OperationLogWriter interface {
	CreateOperationLog(*model.OperationLogModel) error
	PruneOperationLogs(userID uint, keep int) (int64, error)
}

// writeOperationLog 以快照形式落一条操作日志；日志写入失败仅记错误，绝不影响主流程。
// 写入成功后触发保留上限清理，清理失败同样仅记错误。
// folderName 由调用方提供（文件域按 folderID 现查，知识域用知识项上的目录名快照）。
func writeOperationLog(repo OperationLogWriter, userID uint, action model.OperationLogAction, targetType model.OperationLogTargetType, targetID uint, targetName, folderName string, detail map[string]any) {
	detailJSON := "{}"
	if len(detail) > 0 {
		if payload, err := json.Marshal(detail); err == nil {
			detailJSON = string(payload)
		}
	}
	entry := &model.OperationLogModel{
		UserID:     userID,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		FolderName: folderName,
		Detail:     detailJSON,
	}
	if err := repo.CreateOperationLog(entry); err != nil {
		log.Error().Err(err).Uint("user_id", userID).Str("action", string(action)).Msg("记录操作日志失败")
		return
	}
	if _, err := repo.PruneOperationLogs(userID, operationLogKeepCount); err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("清理操作日志失败")
	}
}
