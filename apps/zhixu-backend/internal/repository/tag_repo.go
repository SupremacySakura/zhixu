package repository

import (
	"context"
	"errors"
	"strings"

	"zhixu-backend/internal/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrTagNameConflict 同一用户下标签名称重复。
var ErrTagNameConflict = errors.New("标签名称已存在")

// TagUsage 标签及其被引用次数（标签列表展示用）。
type TagUsage struct {
	model.Tag
	UsageCount int64 `json:"usage_count"`
}

type TagRepository struct{ DB *gorm.DB }

func NewTagRepository(db *gorm.DB) *TagRepository { return &TagRepository{DB: db} }

// List 返回用户全部标签及引用次数，按名称排序。
func (r *TagRepository) List(ctx context.Context, userID uint) ([]TagUsage, error) {
	var rows []TagUsage
	err := r.DB.WithContext(ctx).
		Model(&model.Tag{}).
		Select("tags.*, COUNT(knowledge_tags.knowledge_id) AS usage_count").
		Joins("LEFT JOIN knowledge_tags ON knowledge_tags.tag_id = tags.id AND knowledge_tags.user_id = tags.user_id").
		Where("tags.user_id = ?", userID).
		Group("tags.id").
		Order("tags.name").
		Scan(&rows).Error
	return rows, err
}

// FindOrCreate 按名称查找标签，不存在时创建。名称比较依赖数据库排序规则的大小写不敏感语义。
func (r *TagRepository) FindOrCreate(ctx context.Context, userID uint, name string) (*model.Tag, error) {
	var tag model.Tag
	err := r.DB.WithContext(ctx).Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	tag = model.Tag{UserID: userID, Name: name}
	if err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&tag).Error; err != nil {
		return nil, err
	}
	// 并发创建命中唯一键时 OnConflict 不写回主键，回读一次拿到真实记录。
	if err := r.DB.WithContext(ctx).Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// Rename 重命名标签；同用户下重名返回 ErrTagNameConflict。
func (r *TagRepository) Rename(ctx context.Context, userID, id uint, name string) (*model.Tag, error) {
	res := r.DB.WithContext(ctx).Model(&model.Tag{}).Where("id = ? AND user_id = ?", id, userID).Update("name", name)
	if res.Error != nil {
		if isDuplicateTagName(res.Error) {
			return nil, ErrTagNameConflict
		}
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var tag model.Tag
	if err := r.DB.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Take(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// Delete 删除标签并解绑全部知识；知识本体不受影响。
func (r *TagRepository) Delete(ctx context.Context, userID, id uint) error {
	return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Tag{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("tag_id = ? AND user_id = ?", id, userID).Delete(&model.KnowledgeTag{}).Error
	})
}

// TagsByKnowledge 返回知识当前绑定的标签；知识不存在或不属于该用户时返回 gorm.ErrRecordNotFound。
func (r *TagRepository) TagsByKnowledge(ctx context.Context, userID, knowledgeID uint) ([]model.Tag, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&model.KnowledgeItem{}).
		Where("id = ? AND user_id = ?", knowledgeID, userID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var tags []model.Tag
	err := r.DB.WithContext(ctx).
		Model(&model.Tag{}).
		Joins("JOIN knowledge_tags ON knowledge_tags.tag_id = tags.id").
		Where("knowledge_tags.user_id = ? AND knowledge_tags.knowledge_id = ? AND tags.user_id = ?", userID, knowledgeID, userID).
		Order("tags.name").
		Find(&tags).Error
	return tags, err
}

// ReplaceForKnowledge 全量替换知识标签：解绑旧标签、按需创建新标签、写入新绑定。
// 知识不存在时返回 gorm.ErrRecordNotFound；名称去重由调用方完成。
func (r *TagRepository) ReplaceForKnowledge(ctx context.Context, userID, knowledgeID uint, names []string, source string) ([]model.Tag, error) {
	var tags []model.Tag
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", knowledgeID, userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Where("user_id = ? AND knowledge_id = ?", userID, knowledgeID).Delete(&model.KnowledgeTag{}).Error; err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(names))
		for _, name := range names {
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			tag := model.Tag{UserID: userID, Name: name}
			if err := tx.Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				tag = model.Tag{UserID: userID, Name: name}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tag).Error; err != nil {
					return err
				}
				if err := tx.Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error; err != nil {
					return err
				}
			}
			link := model.KnowledgeTag{KnowledgeID: knowledgeID, TagID: tag.ID, UserID: userID, Source: source}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
				return err
			}
			tags = append(tags, tag)
		}
		return nil
	})
	return tags, err
}

// AttachForKnowledge 只新增标签绑定，不删除知识已有的绑定。
// AI 推荐标签的采纳必须走这条路径：全量替换（ReplaceForKnowledge）会把用户手工标签一起删掉。
// tagIDs 按用户校验归属（不属于该用户的 ID 被忽略）；names 不存在时创建。返回绑定后的完整标签列表。
func (r *TagRepository) AttachForKnowledge(ctx context.Context, userID, knowledgeID uint, tagIDs []uint, names []string, source string) ([]model.Tag, error) {
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.KnowledgeItem{}).Where("id = ? AND user_id = ?", knowledgeID, userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
		resolved := make([]model.Tag, 0, len(tagIDs)+len(names))
		seen := make(map[string]struct{}, len(tagIDs)+len(names))
		if len(tagIDs) > 0 {
			var byID []model.Tag
			if err := tx.Where("user_id = ? AND id IN ?", userID, tagIDs).Find(&byID).Error; err != nil {
				return err
			}
			for _, tag := range byID {
				seen[strings.ToLower(tag.Name)] = struct{}{}
				resolved = append(resolved, tag)
			}
		}
		for _, name := range names {
			key := strings.ToLower(name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			var tag model.Tag
			if err := tx.Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error; err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				tag = model.Tag{UserID: userID, Name: name}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&tag).Error; err != nil {
					return err
				}
				if err := tx.Where("user_id = ? AND name = ?", userID, name).Take(&tag).Error; err != nil {
					return err
				}
			}
			resolved = append(resolved, tag)
		}
		for _, tag := range resolved {
			link := model.KnowledgeTag{KnowledgeID: knowledgeID, TagID: tag.ID, UserID: userID, Source: source}
			// DoNothing 保留已有绑定：手工标签不会被 AI 采纳降级成 ai 来源。
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.TagsByKnowledge(ctx, userID, knowledgeID)
}

// isDuplicateTagName 判断数据库唯一键冲突。
func isDuplicateTagName(err error) bool {
	var duplicate *mysqlDriver.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &duplicate) && duplicate.Number == 1062)
}
