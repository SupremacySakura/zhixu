package service

import (
	"context"
	"fmt"
	"time"

	"zhixu-backend/internal/model"

	"github.com/rs/zerolog/log"
)

var ErrConversationDeleted = model.ErrConversationDeleted

// EnsureActive fences new sends to a deleted thread. The AI service must still
// enforce thread ownership and serialize message writes against remote deletion;
// this preflight is not a lock held for the lifetime of a stream.
func (s *ConversationService) EnsureActive(ctx context.Context, user uint, id string) error {
	if id == "" {
		return nil
	}
	if s.Sources == nil {
		return fmt.Errorf("conversation source store not configured")
	}
	deleted, err := s.Sources.IsDeleted(ctx, id, user)
	if err != nil {
		return err
	}
	if deleted {
		return ErrConversationDeleted
	}
	return nil
}

// RetryPendingCleanups loads persisted work, so restarting the service does not
// lose failed remote deletions. Concurrent workers may issue duplicate DELETEs;
// those are idempotent, and completion uses the same durable row lock as saves.
func (s *ConversationService) RetryPendingCleanups(ctx context.Context) error {
	if s.Sources == nil {
		return fmt.Errorf("conversation source store not configured")
	}
	rows, err := s.Sources.PendingCleanups(ctx)
	if err != nil {
		return err
	}
	var firstErr error
	for _, row := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.cleanup(ctx, row.UserID, row.ConversationID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Run retries durable pending cleanup on startup and every five seconds until
// the application lifecycle context is canceled. It never clears tombstones.
func (s *ConversationService) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := s.RetryPendingCleanups(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("会话清理重试失败")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
