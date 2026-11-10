package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"gorm.io/gorm"
)

// 标签相关的业务错误。
var (
	ErrTagInvalid  = errors.New("标签参数不合法")
	ErrTagNotFound = errors.New("标签不存在")
	ErrTagConflict = errors.New("标签名称已存在")
)

const (
	// TagNameMaxRunes 标签名称最大长度（字符）。
	TagNameMaxRunes = 30
	// TagMaxPerKnowledge 单条知识最多绑定的标签数。
	TagMaxPerKnowledge = 20
)

// TagView 标签的接口视图（带引用次数）。
type TagView struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	UsageCount int64  `json:"usage_count"`
}

// TagStore 标签持久层接口，便于测试替换。
type TagStore interface {
	List(context.Context, uint) ([]repository.TagUsage, error)
	FindOrCreate(context.Context, uint, string) (*model.Tag, error)
	Rename(context.Context, uint, uint, string) (*model.Tag, error)
	Delete(context.Context, uint, uint) error
	TagsByKnowledge(context.Context, uint, uint) ([]model.Tag, error)
	ReplaceForKnowledge(context.Context, uint, uint, []string, string) ([]model.Tag, error)
}

type TagService struct{ store TagStore }

func NewTagService(store TagStore) *TagService {
	return &TagService{store: store}
}

// List 返回用户全部标签及引用次数。
func (s *TagService) List(ctx context.Context, userID uint) ([]TagView, error) {
	rows, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]TagView, 0, len(rows))
	for _, row := range rows {
		views = append(views, TagView{ID: row.ID, Name: row.Name, UsageCount: row.UsageCount})
	}
	return views, nil
}

// Create 创建标签；同名标签直接复用（不报错），符合"输入联想已有标签"的交互。
func (s *TagService) Create(ctx context.Context, userID uint, name string) (*TagView, error) {
	clean, err := validateTagName(name)
	if err != nil {
		return nil, err
	}
	tag, err := s.store.FindOrCreate(ctx, userID, clean)
	if err != nil {
		return nil, tagError(err)
	}
	return &TagView{ID: tag.ID, Name: tag.Name}, nil
}

// Rename 重命名标签。
func (s *TagService) Rename(ctx context.Context, userID, id uint, name string) (*TagView, error) {
	clean, err := validateTagName(name)
	if err != nil {
		return nil, err
	}
	tag, err := s.store.Rename(ctx, userID, id, clean)
	if err != nil {
		return nil, tagError(err)
	}
	return &TagView{ID: tag.ID, Name: tag.Name}, nil
}

// Delete 删除标签并解除全部绑定。
func (s *TagService) Delete(ctx context.Context, userID, id uint) error {
	return tagError(s.store.Delete(ctx, userID, id))
}

// TagsForKnowledge 返回知识当前绑定的标签。
func (s *TagService) TagsForKnowledge(ctx context.Context, userID, knowledgeID uint) ([]TagView, error) {
	tags, err := s.store.TagsByKnowledge(ctx, userID, knowledgeID)
	if err != nil {
		return nil, knowledgeTagError(err)
	}
	return tagViews(tags), nil
}

// ReplaceForKnowledge 全量替换知识标签（source=manual），返回替换后的绑定。
func (s *TagService) ReplaceForKnowledge(ctx context.Context, userID, knowledgeID uint, names []string) ([]TagView, error) {
	if len(names) > TagMaxPerKnowledge {
		return nil, fmt.Errorf("%w: 单条知识最多绑定 %d 个标签", ErrTagInvalid, TagMaxPerKnowledge)
	}
	seen := make(map[string]struct{}, len(names))
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		clean, err := validateTagName(name)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, clean)
	}
	tags, err := s.store.ReplaceForKnowledge(ctx, userID, knowledgeID, normalized, model.TagSourceManual)
	if err != nil {
		return nil, knowledgeTagError(err)
	}
	return tagViews(tags), nil
}

func tagViews(tags []model.Tag) []TagView {
	views := make([]TagView, 0, len(tags))
	for _, tag := range tags {
		views = append(views, TagView{ID: tag.ID, Name: tag.Name})
	}
	return views
}

// validateTagName 校验并清洗标签名称：去首尾空白后 1–30 个字符、合法 UTF-8、无 NUL。
func validateTagName(name string) (string, error) {
	clean := strings.TrimSpace(name)
	if !utf8.ValidString(clean) || strings.ContainsRune(clean, 0) || utf8.RuneCountInString(clean) == 0 || utf8.RuneCountInString(clean) > TagNameMaxRunes {
		return "", fmt.Errorf("%w: 标签名称须为 1–%d 个字符", ErrTagInvalid, TagNameMaxRunes)
	}
	return clean, nil
}

// tagError 统一标签持久层错误语义。
func tagError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrTagNotFound
	case errors.Is(err, repository.ErrTagNameConflict), errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrTagConflict
	default:
		return err
	}
}

// knowledgeTagError 处理"知识维度"的标签操作错误：知识不存在应返回 404 而非标签不存在。
func knowledgeTagError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrKnowledgeNotFound
	}
	return tagError(err)
}
