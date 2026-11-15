package database

import (
	"fmt"
	"sort"

	"zhixu-backend/internal/model"

	"gorm.io/gorm"
)

// evolutionPairBackfillBatch 是一次读取的提醒条数。历史提醒量级在几十到几百，
// 但这个循环按 ID 游标推进，规模变大也不会把整表读进内存。
const evolutionPairBackfillBatch = 200

// evolutionPairMigrationDDL 是知识对收敛需要的索引变更。
//
// 旧唯一索引建在 dedupe_key 上，正是「同一对知识两条提醒」的成因；新唯一索引建在 pair_key 上。
// GORM 的 tag 改动不会删除库里已有的索引，所以这两条必须显式执行。
var evolutionPairMigrationDDL = []indexDDL{
	{
		table: "evolution_alerts",
		name:  "uniq_evolution_alert_pair",
		ddl:   "CREATE UNIQUE INDEX `uniq_evolution_alert_pair` ON `evolution_alerts` (`pair_key`)",
	},
	{
		table: "evolution_alerts",
		name:  "idx_evolution_alert_dedupe",
		ddl:   "CREATE INDEX `idx_evolution_alert_dedupe` ON `evolution_alerts` (`dedupe_key`)",
	},
}

// evolutionLegacyIndex 是迁移后应当消失的旧唯一索引。
const evolutionLegacyIndex = "uniq_evolution_alert_dedupe"

// dropLegacyEvolutionDedupeIndex 删掉建在 dedupe_key 上的旧唯一索引。
//
// 不删它的话，同一对知识的两个方向仍会因为证据引文不同而各自落库，
// 新唯一索引反而会因为「同一对已有两行」而在回填后建不起来。
func dropLegacyEvolutionDedupeIndex(db *gorm.DB) error {
	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		"evolution_alerts", evolutionLegacyIndex,
	).Scan(&count).Error; err != nil {
		return fmt.Errorf("check index %s: %w", evolutionLegacyIndex, err)
	}
	if count == 0 {
		return nil
	}
	if err := db.Exec("DROP INDEX `" + evolutionLegacyIndex + "` ON `evolution_alerts`").Error; err != nil {
		return fmt.Errorf("drop index %s: %w", evolutionLegacyIndex, err)
	}
	return nil
}

// backfillEvolutionPairs 把历史提醒按知识对收敛：同一对只留一条，其余并入保留行。
//
// 保留顺序：仍待办的优先，其次最新的一条。不能简单取最新——那会让一条待办提醒被一条
// 已忽略的行顶掉，用户会以为系统漏报。
//
// 回填可重复执行：已经收敛过的数据再跑一遍不会产生变化，也不会丢结论。
func backfillEvolutionPairs(db *gorm.DB) error {
	var lastID uint
	for {
		var rows []model.EvolutionAlert
		if err := db.Where("id > ?", lastID).
			Order("id ASC").Limit(evolutionPairBackfillBatch).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		lastID = rows[len(rows)-1].ID

		// 这一批内部也可能出现同一对，先按知识对分组再逐组收敛。
		groups := make(map[string][]model.EvolutionAlert)
		for _, row := range rows {
			key := row.PairKey
			if key == "" {
				ids := row.SubjectIDList()
				if len(ids) != 2 {
					continue
				}
				key = model.EvolutionPairKey(ids[0], ids[1])
				// 回填到行上：后面按 pair_key 分组与建唯一索引都依赖它。
				if err := db.Model(&model.EvolutionAlert{}).Where("id = ?", row.ID).
					UpdateColumn("pair_key", key).Error; err != nil {
					return fmt.Errorf("backfill pair_key %d: %w", row.ID, err)
				}
			}
			groups[key] = append(groups[key], row)
		}
		keys := make([]string, 0, len(groups))
		for key := range groups {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, key := range keys {
				if err := convergePairRows(tx, key, groups[key]); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

// convergePairRows 把一组同对提醒收敛到保留行上，并删除其余行。
func convergePairRows(tx *gorm.DB, pairKey string, rows []model.EvolutionAlert) error {
	keep := chooseSurvivingAlert(rows)
	merged := keep
	// 先把所有行按「保留行优先」的顺序并进保留行，保证主结论与结论集合稳定。
	ordered := append([]model.EvolutionAlert{keep}, others(rows, keep.ID)...)
	for i := 1; i < len(ordered); i++ {
		result, _ := model.MergeEvolutionAlert(&merged, &ordered[i])
		if result != nil {
			merged = *result
		}
	}
	merged.PairKey = pairKey
	if err := tx.Model(&model.EvolutionAlert{}).Where("id = ?", keep.ID).Updates(map[string]any{
		"pair_key":        merged.PairKey,
		"pair_versions":   merged.PairVersions,
		"alert_type":      merged.AlertType,
		"severity":        merged.Severity,
		"payload":         merged.Payload,
		"dedupe_key":      merged.DedupeKey,
		"subject_ids":     merged.SubjectIDs,
		"status":          merged.Status,
		"resolved_at":     merged.ResolvedAt,
		"resolved_action": merged.ResolvedAction,
		"resolution_note": merged.ResolutionNote,
	}).Error; err != nil {
		return fmt.Errorf("update alert %d: %w", keep.ID, err)
	}
	for _, row := range rows {
		if row.ID == keep.ID {
			continue
		}
		if err := tx.Delete(&model.EvolutionAlert{}, row.ID).Error; err != nil {
			return fmt.Errorf("delete alert %d: %w", row.ID, err)
		}
	}
	return nil
}

// chooseSurvivingAlert 选出保留行：仍待办的优先，其次 ID 最大（最新）的一条。
// 用 ID 而不是时间比较，避免同一秒内创建的多行排序不确定。
func chooseSurvivingAlert(rows []model.EvolutionAlert) model.EvolutionAlert {
	best := rows[0]
	for _, row := range rows[1:] {
		switch {
		case row.Status.Open() && !best.Status.Open():
			best = row
		case row.Status.Open() == best.Status.Open() && row.ID > best.ID:
			best = row
		}
	}
	return best
}

// others 返回除保留行之外的行，按 ID 升序。
func others(rows []model.EvolutionAlert, keepID uint) []model.EvolutionAlert {
	out := make([]model.EvolutionAlert, 0, len(rows))
	for _, row := range rows {
		if row.ID != keepID {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
