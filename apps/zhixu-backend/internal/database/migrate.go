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

// fulltextIndexes carries the ngram full-text indexes behind keyword retrieval.
// MySQL's ngram parser splits Chinese text into bigrams, which is what makes a
// two-character Chinese query match at all; GORM cannot express this, so it is
// created with raw DDL the same way as the plain indexes above.
var fulltextIndexes = []struct {
	table, name, columns, parser string
}{
	{"knowledge_chunks", "ft_knowledge_chunks_content", "content", "ngram"},
}

func ensureQueryIndexes(db *gorm.DB) error {
	return ensureIndexes(db, queryIndexDDLs())
}

func ensureFulltextIndexes(db *gorm.DB) error {
	return ensureIndexes(db, fulltextIndexDDLs())
}

func queryIndexDDLs() []indexDDL {
	ddls := make([]indexDDL, 0, len(queryIndexes))
	for _, index := range queryIndexes {
		ddls = append(ddls, indexDDL{
			table: index.table,
			name:  index.name,
			ddl:   fmt.Sprintf("CREATE INDEX `%s` ON `%s` (%s)", index.name, index.table, index.columns),
		})
	}
	return ddls
}

func fulltextIndexDDLs() []indexDDL {
	ddls := make([]indexDDL, 0, len(fulltextIndexes))
	for _, index := range fulltextIndexes {
		ddls = append(ddls, indexDDL{
			table: index.table,
			name:  index.name,
			ddl:   fmt.Sprintf("CREATE FULLTEXT INDEX `%s` ON `%s` (%s) WITH PARSER %s", index.name, index.table, index.columns, index.parser),
		})
	}
	return ddls
}

type indexDDL struct {
	table, name, ddl string
}

func ensureIndexes(db *gorm.DB, indexes []indexDDL) error {
	for _, index := range indexes {
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
		// Identifiers come only from the fixed lists above, never from user input.
		if err := normalizeUploadIdentityIndexError(db.Exec(index.ddl).Error); err != nil {
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
		&model.KnowledgeVersion{},
		&model.Tag{},
		&model.KnowledgeTag{},
		&model.KnowledgeRelation{},
		&model.VectorizeTask{},
		&model.VectorCleanupTask{},
		&model.ModelProvider{},
		&model.ProviderModel{},
		&model.UserAISettings{},
		&model.ChatTurnSource{},
		&model.ChatTurnUsage{},
		&model.ChatConversationState{},
		&model.RetrievalLog{},
		&model.AiTask{},
		&model.EvolutionAlert{},
	); err != nil {
		return err
	}
	if err := ensureQueryIndexes(DB); err != nil {
		return err
	}
	if err := ensureFulltextIndexes(DB); err != nil {
		return err
	}

	// content_hash 是演化检测（精确重复）的唯一依据，必须覆盖历史数据。
	// 规范化规则写在 Go 里（去 BOM / 统一换行 / 去行尾空白 / 整体 trim），
	// 用 MySQL 的 SHA2(content, 256) 会算出对不上的值，导致历史知识全部漏判。
	if err := backfillKnowledgeContentHashes(DB); err != nil {
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

// knowledgeContentHashBatch 每批回填的行数：一次性读出全部正文会让大库启动迁移时内存暴涨。
const knowledgeContentHashBatch = 500

// backfillKnowledgeContentHashes 为历史知识补齐正文指纹，只处理 content_hash 为空的行。
// 规范化必须走 Go 侧函数，与新建/编辑/导入时算出的值保持一致，否则同一篇正文会算出两个指纹。
func backfillKnowledgeContentHashes(db *gorm.DB) error {
	var lastID uint
	for {
		var rows []model.KnowledgeItem
		if err := db.Select("id", "content").
			Where("id > ?", lastID).
			Where("content_hash = ? OR content_hash IS NULL", "").
			Order("id ASC").
			Limit(knowledgeContentHashBatch).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, row := range rows {
				hash := utils.KnowledgeContentHash(row.Content)
				if hash == "" {
					continue
				}
				if err := tx.Model(&model.KnowledgeItem{}).Where("id = ?", row.ID).
					UpdateColumn("content_hash", hash).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		lastID = rows[len(rows)-1].ID
	}
}
