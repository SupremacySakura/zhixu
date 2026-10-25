package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"zhixu-backend/internal/model"
)

type cleanupStoreFake struct {
	task        *model.VectorCleanupTask
	rescheduled bool
	success     bool
}

func (f *cleanupStoreFake) Next(context.Context) (*model.VectorCleanupTask, error) {
	if f.task == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.task, nil
}
func (f *cleanupStoreFake) Reschedule(_ context.Context, _ *model.VectorCleanupTask, success bool) error {
	f.rescheduled = true
	f.success = success
	return nil
}

type cleanerFake struct {
	collections []string
	deleted     []string
	err         error
}

func (f *cleanerFake) UserCollections(context.Context, uint) ([]string, error) {
	return f.collections, nil
}
func (f *cleanerFake) DeleteKnowledge(_ context.Context, name string, user, knowledge uint) error {
	f.deleted = append(f.deleted, name)
	return f.err
}
func TestCleanupRetriesDurableTombstoneWithoutModelCalls(t *testing.T) {
	store := &cleanupStoreFake{task: &model.VectorCleanupTask{UserID: 7, KnowledgeID: 9}}
	cleaner := &cleanerFake{collections: []string{"zhixu_u7_v1_c1", "zhixu_u7_v3"}, err: errors.New("offline")}
	service := &VectorCleanupService{Store: store, Vectors: cleaner}
	processed, err := service.ProcessNext(context.Background())
	require.True(t, processed)
	require.Error(t, err)
	require.True(t, store.rescheduled)
	require.False(t, store.success)
	cleaner.err = nil
	cleaner.deleted = nil
	processed, err = service.ProcessNext(context.Background())
	require.True(t, processed)
	require.NoError(t, err)
	require.True(t, store.success)
	require.Equal(t, cleaner.collections, cleaner.deleted)
	require.NotNil(t, store.task) // retained for late remote writes
}
