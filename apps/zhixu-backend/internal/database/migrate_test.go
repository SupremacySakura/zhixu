package database

import (
	"reflect"
	"sync"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/utils"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestBuildUploadIdentityBackfills_PreservesOneCanonicalTaskAndSeparatesDuplicates(t *testing.T) {
	tasks := []model.UploadTask{
		{
			ID: 1, UserID: 7, FolderID: 9, FileName: " report.pdf ", FileHash: "ABCDEF",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		},
		{
			ID: 2, UserID: 7, FolderID: 9, FileName: " report.pdf ", FileHash: "ABCDEF",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
		},
	}

	backfills, err := buildUploadIdentityBackfills(tasks)

	require.NoError(t, err)
	require.Len(t, backfills, 2)
	assert.NotEmpty(t, backfills[0].IdempotencyKey)
	assert.NotEmpty(t, backfills[0].RequestHash)
	assert.NotEqual(t, backfills[0].IdempotencyKey, backfills[1].IdempotencyKey)
	assert.Equal(t, backfills[0].RequestHash, backfills[1].RequestHash)
}

func TestBuildUploadIdentityBackfills_DoesNotReplaceAnExistingKey(t *testing.T) {
	tasks := []model.UploadTask{
		{
			ID: 1, UserID: 7, FileName: "report.pdf", FileHash: "abcdef",
			FileSize: 1024, ChunkSize: 512, TotalChunks: 2, FileType: "application/pdf",
			IdempotencyKey: "already-assigned",
		},
	}

	backfills, err := buildUploadIdentityBackfills(tasks)

	require.NoError(t, err)
	require.Len(t, backfills, 1)
	assert.Equal(t, "already-assigned", backfills[0].IdempotencyKey)
	assert.NotEmpty(t, backfills[0].RequestHash)
}

func TestBuildUploadIdentityBackfills_SkipsTasksWhoseIdentityIsAlreadyBackfilled(t *testing.T) {
	task := model.UploadTask{
		ID: 1, UserID: 7, FileName: "report.pdf", FileHash: "abcdef",
		FileSize: 5, ChunkSize: 5, TotalChunks: 1, FileType: "text/plain",
	}
	key, requestHash, _, err := utils.BuildUploadIdentity(&task)
	require.NoError(t, err)
	task.IdempotencyKey = key
	task.RequestHash = requestHash

	backfills, err := buildUploadIdentityBackfills([]model.UploadTask{task})

	require.NoError(t, err)
	assert.Empty(t, backfills)
}

func TestNormalizeUploadIdentityIndexError_IgnoresDuplicateIndex(t *testing.T) {
	err := fmt.Errorf("create index: %w", &mysqlDriver.MySQLError{
		Number:  1061,
		Message: "Duplicate key name 'uniq_upload_tasks_idempotency_key'",
	})

	assert.NoError(t, normalizeUploadIdentityIndexError(err))
}

func TestNormalizeUploadIdentityIndexError_PreservesOtherErrors(t *testing.T) {
	want := errors.New("connection lost")

	assert.ErrorIs(t, normalizeUploadIdentityIndexError(want), want)
}

func TestEnsureQueryIndexes(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		existing int
		ddlError error
		wantErr  bool
	}{
		{"creates missing indexes", 0, nil, false},
		{"skips existing indexes", 1, nil, false},
		{"tolerates concurrent creation", 0, &mysqlDriver.MySQLError{Number: 1061}, false},
		{"propagates creation failures", 0, errors.New("permission denied"), true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer sqlDB.Close()
			db, err := gorm.Open(mysql.New(mysql.Config{
				Conn: sqlDB, SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			require.NoError(t, err)

			indexes := append(queryIndexDDLs(), fulltextIndexDDLs()...)
			for _, index := range indexes {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?")).
					WithArgs(index.table, index.name).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(scenario.existing))
				if scenario.existing == 0 {
					exec := mock.ExpectExec(regexp.QuoteMeta(index.ddl))
					if scenario.ddlError != nil {
						exec.WillReturnError(scenario.ddlError)
					} else {
						exec.WillReturnResult(sqlmock.NewResult(0, 0))
					}
				}
				if scenario.wantErr {
					break
				}
			}
			err = ensureQueryIndexes(db)
			if err == nil {
				err = ensureFulltextIndexes(db)
			}
			if scenario.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// 保留行的选择：仍待办的优先于已裁决的，同为待办时取最新（ID 最大）。
func TestChooseSurvivingAlertPrefersOpenThenNewest(t *testing.T) {
	resolved := model.EvolutionAlert{ID: 9, Status: model.EvolutionAlertResolved}
	pending := model.EvolutionAlert{ID: 4, Status: model.EvolutionAlertPending}
	deferred := model.EvolutionAlert{ID: 6, Status: model.EvolutionAlertDeferred}

	assert.Equal(t, uint(4), chooseSurvivingAlert([]model.EvolutionAlert{resolved, pending}).ID,
		"待办优先，即使它更旧")
	assert.Equal(t, uint(6), chooseSurvivingAlert([]model.EvolutionAlert{pending, deferred}).ID,
		"同为待办视野时取最新的一条")
	assert.Equal(t, uint(9), chooseSurvivingAlert([]model.EvolutionAlert{
		{ID: 3, Status: model.EvolutionAlertResolved}, resolved,
	}).ID, "都已裁决时取最新的一条")
}

// 历史回填的索引变更：唯一索引建在 pair_key 上，旧索引要显式删除。
func TestEvolutionPairMigrationTargetsPairKey(t *testing.T) {
	require.Len(t, evolutionPairMigrationDDL, 2)
	assert.Contains(t, evolutionPairMigrationDDL[0].ddl, "UNIQUE INDEX")
	assert.Contains(t, evolutionPairMigrationDDL[0].ddl, "`pair_key`")
	assert.Equal(t, "uniq_evolution_alert_dedupe", evolutionLegacyIndex,
		"旧唯一索引名要写对，否则删不掉，新索引会因为历史重复行建不起来")
}

// AutoMigrate 会在回填之前跑，所以唯一索引不能由模型 tag 声明。
//
// 库里已有的提醒行 pair_key 都是空串，tag 上的唯一索引会在 AutoMigrate 阶段
// 直接建在历史数据上并因重复值失败，整个 Migrate 返回错误、服务起不来。
// 唯一索引只能由迁移末尾的显式 DDL 建，那时回填已经跑完。
func TestEvolutionAlertModelLeavesPairIndexToMigration(t *testing.T) {
	parsed, err := schema.Parse(&model.EvolutionAlert{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)

	pairKey := parsed.LookUpField("PairKey")
	require.NotNil(t, pairKey)
	for _, index := range parsed.ParseIndexes() {
		if index.Name == "uniq_evolution_alert_pair" {
			t.Fatalf("PairKey 的 GORM tag 声明了唯一索引 %q，AutoMigrate 会在回填前把它建在历史空值上",
				index.Name)
		}
	}
}

// 给已有表追加 NOT NULL 的 JSON 列会让 MySQL 用空串回填历史行并报 ERROR 3140。
// pair_versions 必须能安全追加：字符串列，且 GORM tag 里显式给了默认值。
//
// 断言原始 tag 而不是 field.DefaultValue：default:'' 会被 GORM 解析成空字符串，
// 与「没有默认值」无法区分，只有 tag 本身能表达「显式写了默认值」。
func TestEvolutionAlertPairVersionsIsSafeToAppend(t *testing.T) {
	field, ok := reflect.TypeOf(model.EvolutionAlert{}).FieldByName("PairVersions")
	require.True(t, ok)
	tag := field.Tag.Get("gorm")

	assert.NotContains(t, tag, "type:json", "JSON 列不能安全追加到已有表")
	assert.Contains(t, tag, "default:", "追加到已有表的 NOT NULL 列必须有默认值")
}
