package database

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"zhixu-backend/internal/model"
	"zhixu-backend/internal/utils"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type uploadIdentityBackfill struct {
	ID             uint
	IdempotencyKey string
	RequestHash    string
}

var queryIndexes = []struct {
	table, name, columns string
}{
	{"file_models", "idx_files_hash_user", "file_hash(64), user_id"},
	{"file_models", "idx_files_parent_user", "folder_id, user_id"},
	{"folder_models", "idx_folders_parent_user", "parent_id, user_id"},
}

func ensureQueryIndexes(db *gorm.DB) error {
	for _, index := range queryIndexes {
		var count int64
		if err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			index.table, index.name,
		).Scan(&count).Error; err != nil {
			return fmt.Errorf("check index %s: %w", index.name, err)
		}
		if count != 0 {
			continue
		}
		// Identifiers come only from the fixed list above, never from user input.
		ddl := fmt.Sprintf("CREATE INDEX `%s` ON `%s` (%s)", index.name, index.table, index.columns)
		if err := normalizeUploadIdentityIndexError(db.Exec(ddl).Error); err != nil {
			return fmt.Errorf("create index %s: %w", index.name, err)
		}
	}
	return nil
}

func normalizeUploadIdentityIndexError(err error) error {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1061 {
		return nil
	}
	return err
}

func legacyUploadIdentity(prefix string, id uint, attempt int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", prefix, id, attempt)))
	return hex.EncodeToString(sum[:])
}

func buildUploadIdentityBackfills(tasks []model.UploadTask) ([]uploadIdentityBackfill, error) {
	usedKeys := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		if task.IdempotencyKey != "" {
			usedKeys[task.IdempotencyKey] = struct{}{}
		}
	}

	backfills := make([]uploadIdentityBackfill, 0, len(tasks))
	for _, task := range tasks {
		candidateKey, requestHash, _, identityErr := utils.BuildUploadIdentity(&task)
		if identityErr != nil {
			if requestHash == "" {
				requestHash = legacyUploadIdentity("legacy-request", task.ID, 0)
			}
		}

		idempotencyKey := task.IdempotencyKey
		if idempotencyKey == "" && identityErr == nil {
			if _, exists := usedKeys[candidateKey]; !exists {
				idempotencyKey = candidateKey
			}
		}
		for attempt := 0; idempotencyKey == ""; attempt++ {
			candidate := legacyUploadIdentity("legacy-upload", task.ID, attempt)
			if _, exists := usedKeys[candidate]; !exists {
				idempotencyKey = candidate
			}
		}
		usedKeys[idempotencyKey] = struct{}{}

		if task.RequestHash != "" {
			requestHash = task.RequestHash
		}
		if task.IdempotencyKey == idempotencyKey && task.RequestHash == requestHash {
			continue
		}
		backfills = append(backfills, uploadIdentityBackfill{
			ID: task.ID, IdempotencyKey: idempotencyKey,
			RequestHash: requestHash,
		})
	}
	return backfills, nil
}

// Migrate 数据库迁移
func Migrate() error {
	if err := DB.AutoMigrate(
		&model.UserModel{},
		&model.UploadTask{},
		&model.UploadChunk{},
		&model.FileModel{},
		&model.FolderModel{},
		&model.PickUpCodeModel{},
		&model.PublicShareLinkModel{},
		&model.OperationLogModel{},
		&model.KnowledgeItem{},
		&model.KnowledgeImportTask{},
		&model.KnowledgeChunk{},
		&model.VectorizeTask{},
		&model.VectorCleanupTask{},
		&model.ModelProvider{},
		&model.ProviderModel{},
		&model.UserAISettings{},
		&model.ChatTurnSource{},
		&model.ChatConversationState{},
	); err != nil {
		return err
	}
	if err := ensureQueryIndexes(DB); err != nil {
		return err
	}

	var tasks []model.UploadTask
	if err := DB.Order("id ASC").Find(&tasks).Error; err != nil {
		return err
	}
	backfills, err := buildUploadIdentityBackfills(tasks)
	if err != nil {
		return err
	}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		for _, backfill := range backfills {
			if err := tx.Model(&model.UploadTask{}).Where("id = ?", backfill.ID).UpdateColumns(map[string]interface{}{
				"idempotency_key": backfill.IdempotencyKey,
				"request_hash":    backfill.RequestHash,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if !DB.Migrator().HasIndex(&model.UploadTask{}, "uniq_upload_tasks_idempotency_key") {
		if err := normalizeUploadIdentityIndexError(DB.Exec("CREATE UNIQUE INDEX uniq_upload_tasks_idempotency_key ON upload_tasks (idempotency_key)").Error); err != nil {
			return err
		}
	}
	if err := DB.Exec(`
		UPDATE upload_tasks AS task
		JOIN (
			SELECT user_id, folder_id, file_hash, name, MIN(id) AS file_id
			FROM file_models
			WHERE deleted_at IS NULL
			GROUP BY user_id, folder_id, file_hash, name
		) AS file_match
		ON file_match.user_id = task.user_id
			AND file_match.folder_id = task.folder_id
			AND file_match.file_hash = task.file_hash
			AND file_match.name = task.file_name
		SET task.file_id = file_match.file_id
		WHERE task.status = ? AND task.file_id IS NULL
	`, model.UploadStatusCompleted).Error; err != nil {
		return err
	}
	return nil
}
