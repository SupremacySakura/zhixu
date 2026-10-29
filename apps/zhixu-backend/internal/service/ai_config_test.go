package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"zhixu-backend/internal/model"
	"zhixu-backend/internal/repository"

	"github.com/stretchr/testify/require"
)

type memoryAIConfig struct {
	mu     sync.Mutex
	states map[uint]*repository.AIConfigState
	next   uint
}

func cloneConfig(state *repository.AIConfigState) *repository.AIConfigState {
	copy := *state
	copy.Providers = append([]model.ModelProvider(nil), state.Providers...)
	copy.Models = append([]model.ProviderModel(nil), state.Models...)
	return &copy
}
func (m *memoryAIConfig) state(user uint) *repository.AIConfigState {
	if m.states[user] == nil {
		m.states[user] = &repository.AIConfigState{Settings: model.UserAISettings{UserID: user}}
	}
	return m.states[user]
}
func (m *memoryAIConfig) Read(_ context.Context, user uint) (*repository.AIConfigState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneConfig(m.state(user)), nil
}
func (m *memoryAIConfig) Mutate(_ context.Context, user uint, fn func(*repository.AIConfigState) error) (*repository.AIConfigState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := cloneConfig(m.state(user))
	if err := fn(state); err != nil {
		return nil, err
	}
	for i := range state.Providers {
		if state.Providers[i].ID == 0 {
			m.next++
			state.Providers[i].ID = m.next
			for j := range state.Models {
				if state.Models[j].ProviderID == 0 {
					state.Models[j].ProviderID = m.next
				}
			}
		}
	}
	for i := range state.Models {
		if state.Models[i].ID == 0 {
			m.next++
			state.Models[i].ID = m.next
		}
	}
	state.Settings.ConfigVersion++
	m.states[user] = cloneConfig(state)
	return state, nil
}
func newAIConfigTestService(t *testing.T) *AIConfigService {
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	credentials, err := NewModelCredentials(key)
	require.NoError(t, err)
	return &AIConfigService{Store: &memoryAIConfig{states: map[uint]*repository.AIConfigState{}}, Credentials: credentials}
}
func testProviderInput() ProviderInput {
	key := "test-secret-not-for-production"
	return ProviderInput{ProviderID: "gateway", Name: "网关", BaseURL: "https://example.com/v1", Protocol: "chat", AuthType: "key", APIKey: &key,
		Models: []ModelInput{{ModelID: "chat", Kind: "chat"}, {ModelID: "embed", Kind: "embedding", Dimension: 1024}}}
}

func TestModelCredentialsAuthenticatedEncryption(t *testing.T) {
	s := newAIConfigTestService(t)
	a, err := s.Credentials.Encrypt(7, "gateway", "secret")
	require.NoError(t, err)
	b, err := s.Credentials.Encrypt(7, "gateway", "secret")
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	value, err := s.Credentials.Decrypt(7, "gateway", a)
	require.NoError(t, err)
	require.Equal(t, "secret", value)
	_, err = s.Credentials.Decrypt(8, "gateway", a)
	require.Error(t, err)
	_, err = s.Credentials.Decrypt(7, "other", a)
	require.Error(t, err)
	bytes, _ := base64.StdEncoding.DecodeString(a)
	bytes[len(bytes)-1] ^= 1
	_, err = s.Credentials.Decrypt(7, "gateway", base64.StdEncoding.EncodeToString(bytes))
	require.Error(t, err)
	empty, err := NewModelCredentials("")
	require.NoError(t, err)
	_, err = empty.Encrypt(7, "gateway", "secret")
	require.ErrorIs(t, err, ErrModelKeyUnavailable)
	_, err = NewModelCredentials("invalid")
	require.Error(t, err)
}

func TestAIConfigLifecycleAndIsolation(t *testing.T) {
	ctx := context.Background()
	s := newAIConfigTestService(t)
	input := testProviderInput()
	view, err := s.SaveProvider(ctx, 7, 0, input)
	require.NoError(t, err)
	id := view.Providers[0].ID
	require.True(t, view.Providers[0].CredentialConfigured)
	require.Empty(t, view.Providers[0].Ciphertext)
	data, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(data), *input.APIKey)
	require.NotContains(t, string(data), "ciphertext")
	other, err := s.Get(ctx, 8)
	require.NoError(t, err)
	require.Empty(t, other.Providers)
	input.ExpectedVersion = 0
	_, err = s.SaveProvider(ctx, 8, id, input)
	require.ErrorIs(t, err, ErrAIConfigNotFound)
	input.ExpectedVersion = 1
	input.APIKey = nil
	view, err = s.SaveProvider(ctx, 7, id, input)
	require.NoError(t, err)
	require.True(t, view.Providers[0].CredentialConfigured)
	chat, embed := view.Providers[0].Models[0].ID, view.Providers[0].Models[1].ID
	view, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, ExpectedVersion: 2})
	require.NoError(t, err)
	require.Equal(t, uint64(1), view.Settings.EmbeddingVersion)
	require.False(t, view.Settings.NeedsReindex)
	_, err = s.DeleteProvider(ctx, 7, id, 3)
	require.ErrorIs(t, err, ErrAIConfigConflict)
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: embed, ExpectedVersion: 3})
	require.ErrorIs(t, err, ErrAIConfigConflict)
	_, err = s.SaveDefaults(ctx, 8, DefaultsInput{ChatModelID: chat})
	require.ErrorIs(t, err, ErrAIConfigConflict)
	input.ExpectedVersion = 3
	input.Models[1].Dimension = 1536
	_, err = s.SaveProvider(ctx, 7, id, input)
	require.ErrorIs(t, err, ErrAIConfigConflict)
	input.ConfirmReindex = true
	view, err = s.SaveProvider(ctx, 7, id, input)
	require.NoError(t, err)
	require.Equal(t, embed, view.Providers[0].Models[1].ID)
	require.True(t, view.Settings.NeedsReindex)
	require.Equal(t, uint64(2), view.Settings.EmbeddingVersion)
	input.ExpectedVersion = 4
	input.ClearAPIKey = true
	view, err = s.SaveProvider(ctx, 7, id, input)
	require.NoError(t, err)
	require.False(t, view.Providers[0].CredentialConfigured)
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, ExpectedVersion: 5})
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	view, err = s.SaveDefaults(ctx, 7, DefaultsInput{ExpectedVersion: 5, ConfirmReindex: true})
	require.NoError(t, err)
	view, err = s.DeleteProvider(ctx, 7, id, 6)
	require.NoError(t, err)
	require.Empty(t, view.Providers)
}

func TestAIConfigValidation(t *testing.T) {
	for _, mutate := range []func(*ProviderInput){
		func(p *ProviderInput) { p.ProviderID = "Bad ID" },
		func(p *ProviderInput) { p.BaseURL = "https://secret:pass@example.com/v1" },
		func(p *ProviderInput) { p.BaseURL = "http://169.254.169.254/v1" },
		func(p *ProviderInput) { p.BaseURL = "https://example.com/v1/embeddings/" },
		func(p *ProviderInput) { p.Protocol = "anthropic" },
		func(p *ProviderInput) { p.Models[1].Dimension = 0 },
		func(p *ProviderInput) { p.Models[0].Dimension = 1 },
		func(p *ProviderInput) { p.Models[1].ModelID = "CHAT" },
		func(p *ProviderInput) { p.ClearAPIKey = true },
		func(p *ProviderInput) { p.CatalogID = "openai" },
	} {
		p := testProviderInput()
		mutate(&p)
		require.ErrorIs(t, validateProvider(&p), ErrAIConfigInvalid)
	}
	p := testProviderInput()
	p.BaseURL = "http://127.0.0.1:11434/v1"
	p.AuthType = "none"
	p.APIKey = nil
	require.NoError(t, validateProvider(&p))
}

func TestAIConfigConcurrentExpectedVersion(t *testing.T) {
	s := newAIConfigTestService(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.SaveProvider(context.Background(), 7, 0, testProviderInput())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errorsIsConflict(err) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, conflicts)
}

func TestSaveDefaultsRagSettings(t *testing.T) {
	ctx := context.Background()
	s := newAIConfigTestService(t)
	view, err := s.SaveProvider(ctx, 7, 0, testProviderInput())
	require.NoError(t, err)
	chat, embed := view.Providers[0].Models[0].ID, view.Providers[0].Models[1].ID

	topK, score := 0, 0.01
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, RagTopK: &topK, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	topK = RagMaxTopK + 1
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, RagTopK: &topK, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, RagMinScore: &score, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	score = 1.5
	_, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, RagMinScore: &score, ExpectedVersion: 1})
	require.ErrorIs(t, err, ErrAIConfigInvalid)

	topK, score = 20, 0.7
	view, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, RagTopK: &topK, RagMinScore: &score, ExpectedVersion: 1})
	require.NoError(t, err)
	require.Equal(t, 20, view.Settings.RagTopK)
	require.InDelta(t, 0.7, view.Settings.RagMinScore, 1e-9)

	// A save without retrieval fields keeps the stored values.
	view, err = s.SaveDefaults(ctx, 7, DefaultsInput{ChatModelID: chat, EmbeddingModelID: embed, ExpectedVersion: 2})
	require.NoError(t, err)
	require.Equal(t, 20, view.Settings.RagTopK)
	require.InDelta(t, 0.7, view.Settings.RagMinScore, 1e-9)
}

func errorsIsConflict(err error) bool { return errors.Is(err, ErrAIConfigConflict) }
