package service

import (
	"context"
	"strings"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type tagFake struct {
	rows []repository.TagUsage

	findOrCreate *model.Tag
	createName   string
	createErr    error

	renameTag  *model.Tag
	renameName string
	renameErr  error

	deleteErr error

	knowledgeTags []model.Tag
	knowledgeErr  error

	replacedNames  []string
	replacedSource string
	replaceTags    []model.Tag
	replaceErr     error
}

func (f *tagFake) List(context.Context, uint) ([]repository.TagUsage, error) { return f.rows, nil }

func (f *tagFake) FindOrCreate(_ context.Context, _ uint, name string) (*model.Tag, error) {
	f.createName = name
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.findOrCreate != nil {
		return f.findOrCreate, nil
	}
	return &model.Tag{ID: 1, Name: name}, nil
}

func (f *tagFake) Rename(_ context.Context, _ uint, _ uint, name string) (*model.Tag, error) {
	f.renameName = name
	return f.renameTag, f.renameErr
}

func (f *tagFake) Delete(context.Context, uint, uint) error { return f.deleteErr }

func (f *tagFake) TagsByKnowledge(context.Context, uint, uint) ([]model.Tag, error) {
	return f.knowledgeTags, f.knowledgeErr
}

func (f *tagFake) ReplaceForKnowledge(_ context.Context, _ uint, _ uint, names []string, source string) ([]model.Tag, error) {
	f.replacedNames = names
	f.replacedSource = source
	return f.replaceTags, f.replaceErr
}

func TestTagList(t *testing.T) {
	svc := NewTagService(&tagFake{rows: []repository.TagUsage{
		{Tag: model.Tag{ID: 1, Name: "工作"}, UsageCount: 3},
		{Tag: model.Tag{ID: 2, Name: "学习"}},
	}})
	views, err := svc.List(context.Background(), 2)
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "工作", views[0].Name)
	require.Equal(t, int64(3), views[0].UsageCount)
	require.Equal(t, int64(0), views[1].UsageCount)
}

func TestTagCreateValidatesName(t *testing.T) {
	fake := &tagFake{}
	svc := NewTagService(fake)
	for _, tc := range []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"blank", "   "},
		{"too long", strings.Repeat("长", TagNameMaxRunes+1)},
		{"nul byte", "工作\x00"},
		{"invalid utf8", "\xff\xfe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), 2, tc.input)
			require.ErrorIs(t, err, ErrTagInvalid)
			require.Empty(t, fake.createName)
		})
	}
}

func TestTagCreateTrimsAndReturnsExisting(t *testing.T) {
	fake := &tagFake{findOrCreate: &model.Tag{ID: 7, Name: "工作"}}
	svc := NewTagService(fake)
	view, err := svc.Create(context.Background(), 2, "  工作  ")
	require.NoError(t, err)
	require.Equal(t, uint(7), view.ID)
	require.Equal(t, "工作", fake.createName)
}

func TestTagRenameErrors(t *testing.T) {
	svc := NewTagService(&tagFake{renameErr: gorm.ErrRecordNotFound})
	_, err := svc.Rename(context.Background(), 2, 5, "工作")
	require.ErrorIs(t, err, ErrTagNotFound)

	svc = NewTagService(&tagFake{renameErr: repository.ErrTagNameConflict})
	_, err = svc.Rename(context.Background(), 2, 5, "工作")
	require.ErrorIs(t, err, ErrTagConflict)

	svc = NewTagService(&tagFake{renameTag: &model.Tag{ID: 5, Name: "新标签"}})
	view, err := svc.Rename(context.Background(), 2, 5, " 新标签 ")
	require.NoError(t, err)
	require.Equal(t, "新标签", view.Name)
}

func TestTagDeleteNotFound(t *testing.T) {
	svc := NewTagService(&tagFake{deleteErr: gorm.ErrRecordNotFound})
	require.ErrorIs(t, svc.Delete(context.Background(), 2, 5), ErrTagNotFound)
}

func TestTagKnowledgeTags(t *testing.T) {
	svc := NewTagService(&tagFake{knowledgeErr: gorm.ErrRecordNotFound})
	_, err := svc.TagsForKnowledge(context.Background(), 2, 99)
	require.ErrorIs(t, err, ErrKnowledgeNotFound)

	svc = NewTagService(&tagFake{knowledgeTags: []model.Tag{{ID: 1, Name: "工作"}}})
	views, err := svc.TagsForKnowledge(context.Background(), 2, 10)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, "工作", views[0].Name)
}

func TestTagReplaceForKnowledge(t *testing.T) {
	fake := &tagFake{replaceTags: []model.Tag{{ID: 5, Name: "工作"}}}
	svc := NewTagService(fake)
	views, err := svc.ReplaceForKnowledge(context.Background(), 2, 10, []string{" 工作 ", "工作", "学习"})
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, []string{"工作", "学习"}, fake.replacedNames)
	require.Equal(t, model.TagSourceManual, fake.replacedSource)

	// 超过单条知识标签上限。
	names := make([]string, TagMaxPerKnowledge+1)
	for i := range names {
		names[i] = "标签"
	}
	_, err = svc.ReplaceForKnowledge(context.Background(), 2, 10, names)
	require.ErrorIs(t, err, ErrTagInvalid)

	// 单个标签名不合法。
	_, err = svc.ReplaceForKnowledge(context.Background(), 2, 10, []string{""})
	require.ErrorIs(t, err, ErrTagInvalid)

	// 知识不存在。
	svc = NewTagService(&tagFake{replaceErr: gorm.ErrRecordNotFound})
	_, err = svc.ReplaceForKnowledge(context.Background(), 2, 99, []string{"工作"})
	require.ErrorIs(t, err, ErrKnowledgeNotFound)
}
