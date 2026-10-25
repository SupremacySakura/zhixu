package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zhixu-backend/internal/model"
)

// ConversationService relays conversation management to the AI service's internal API.
// End users are already authenticated by the Go JWT middleware; this service attaches a
// shared internal secret plus the owning user id so the AI service can scope every read
// and write to that user's threads without ever trusting a client-supplied user id.
type ConversationService struct {
	AIURL   string
	Secret  string
	Sources ChatSourceStore
}

// ChatSourceStore persists RAG provenance per turn.
type ChatSourceStore interface {
	SaveTurnSource(ctx context.Context, row *model.ChatTurnSource) error
	ListTurnSources(ctx context.Context, conversationID string, user uint) ([]model.ChatTurnSource, error)
	IsDeleted(ctx context.Context, conversationID string, user uint) (bool, error)
	MarkDeleted(ctx context.Context, conversationID string, user uint) error
	CompleteCleanup(ctx context.Context, conversationID string, user uint) error
	PendingCleanups(ctx context.Context) ([]model.ChatConversationState, error)
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationMessage struct {
	Role      string                  `json:"role"`
	Content   string                  `json:"content"`
	CreatedAt time.Time               `json:"created_at"`
	Query     string                  `json:"query,omitempty"`
	Sources   []model.KnowledgeSource `json:"sources,omitempty"`
}

type ConversationDetail struct {
	Conversation Conversation          `json:"conversation"`
	Messages     []ConversationMessage `json:"messages"`
}

// mastraThread mirrors the AI service's thread payload.
type mastraThread struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type mastraMessage struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	CreatedAt time.Time       `json:"created_at"`
}

// mastraContentV2 is the subset of the V2 message shape we render from.
type mastraContentV2 struct {
	Content string `json:"content"`
	Parts   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"parts"`
}

func (s *ConversationService) call(ctx context.Context, path string, payload any, out any) error {
	if s.Secret == "" {
		return fmt.Errorf("conversation service not configured")
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(s.AIURL, "/")+"/internal/conversations"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Secret)
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("conversation service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return errConversationNotFound
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("conversation service error: %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

var errConversationNotFound = fmt.Errorf("conversation not found")

func (s *ConversationService) List(ctx context.Context, user uint) ([]Conversation, error) {
	var result struct {
		Conversations []mastraThread `json:"conversations"`
	}
	if err := s.call(ctx, "", map[string]string{"resource": ResourceID(user)}, &result); err != nil {
		return nil, err
	}
	conversations := make([]Conversation, 0, len(result.Conversations))
	for _, t := range result.Conversations {
		if err := s.EnsureActive(ctx, user, t.ID); err != nil {
			if errors.Is(err, ErrConversationDeleted) {
				continue
			}
			return nil, err
		}
		conversations = append(conversations, Conversation{
			ID: t.ID, Title: t.Title, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		})
	}
	return conversations, nil
}

func (s *ConversationService) Detail(ctx context.Context, user uint, id string) (*ConversationDetail, error) {
	if err := s.EnsureActive(ctx, user, id); err != nil {
		return nil, err
	}
	var result struct {
		Conversation mastraThread    `json:"conversation"`
		Messages     []mastraMessage `json:"messages"`
	}
	if err := s.call(ctx, "/messages", map[string]string{"resource": ResourceID(user), "id": id}, &result); err != nil {
		return nil, err
	}
	// Recheck after the remote call: deletion may have committed while it was in flight.
	if err := s.EnsureActive(ctx, user, id); err != nil {
		return nil, err
	}
	// Only the exact assistant message id is an association. Old rows and missing
	// saves must not shift citations onto plain, failed, or interrupted turns.
	sources := make(map[string]model.ChatTurnSource)
	if s.Sources != nil {
		if rows, err := s.Sources.ListTurnSources(ctx, id, user); err == nil {
			for _, row := range rows {
				if row.MessageID != "" {
					sources[row.MessageID] = row
				}
			}
		}
	}
	detail := &ConversationDetail{
		Conversation: Conversation{
			ID: result.Conversation.ID, Title: result.Conversation.Title,
			CreatedAt: result.Conversation.CreatedAt, UpdatedAt: result.Conversation.UpdatedAt,
		},
	}
	for _, m := range result.Messages {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		message := ConversationMessage{
			Role: m.Role, Content: renderContent(m.Content), CreatedAt: m.CreatedAt,
		}
		if row, ok := sources[m.ID]; m.Role == "assistant" && ok {
			message.Query = row.Query
			_ = json.Unmarshal([]byte(row.Sources), &message.Sources)
		}
		detail.Messages = append(detail.Messages, message)
	}
	return detail, nil
}

func (s *ConversationService) Rename(ctx context.Context, user uint, id, title string) error {
	if err := s.EnsureActive(ctx, user, id); err != nil {
		return err
	}
	return s.call(ctx, "/rename", map[string]string{"resource": ResourceID(user), "id": id, "title": title}, nil)
}

func (s *ConversationService) Delete(ctx context.Context, user uint, id string) error {
	if s.Sources == nil {
		return fmt.Errorf("conversation source store not configured")
	}
	if err := s.Sources.MarkDeleted(ctx, id, user); err != nil {
		return err
	}
	return s.cleanup(ctx, user, id)
}

func (s *ConversationService) cleanup(ctx context.Context, user uint, id string) error {
	err := s.call(ctx, "/delete", map[string]string{"resource": ResourceID(user), "id": id}, nil)
	if err != nil && !errors.Is(err, errConversationNotFound) {
		return err // The durable pending row is left for Run/restart to retry.
	}
	return s.Sources.CompleteCleanup(ctx, id, user)
}

// SaveTurnSource records the retrieval behind the assistant turn that just finished.
func (s *ConversationService) SaveTurnSource(ctx context.Context, conversationID string, user uint, messageID string, query string, sources []model.KnowledgeSource) error {
	if s.Sources == nil || conversationID == "" || strings.TrimSpace(messageID) == "" {
		return nil
	}
	encoded, err := json.Marshal(sources)
	if err != nil {
		return err
	}
	return s.Sources.SaveTurnSource(ctx, &model.ChatTurnSource{
		ConversationID: conversationID, UserID: user, MessageID: messageID, Query: query, Sources: string(encoded),
	})
}

// renderContent flattens a V2 message body to plain text for the chat UI.
func renderContent(raw json.RawMessage) string {
	var body mastraContentV2
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	var b strings.Builder
	for _, part := range body.Parts {
		if part.Type == "text" {
			b.WriteString(part.Text)
		}
	}
	if b.Len() > 0 {
		return b.String()
	}
	return body.Content
}

// ResourceID is the AI service's resource scope for one user's threads. Keeping it in
// one place guarantees the chat turn writer and the conversation reader agree on it.
func ResourceID(user uint) string {
	return fmt.Sprintf("u%d", user)
}
