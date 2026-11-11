package service

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatInputAndUserIsolation(t *testing.T) {
	s := newAIConfigTestService(t)
	view, err := s.SaveProvider(context.Background(), 7, 0, testProviderInput())
	require.NoError(t, err)
	chat := &ChatService{Store: s.Store, Credentials: s.Credentials}
	input := ChatInput{ModelID: view.Providers[0].Models[0].ID, Messages: []ChatMessage{{Role: "user", Content: "你好"}}}
	cap, err := chat.Resolve(context.Background(), 7, input)
	require.NoError(t, err)
	require.Equal(t, "test-secret-not-for-production", cap.Key)
	_, err = chat.Resolve(context.Background(), 8, input)
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	input.ModelID = view.Providers[0].Models[1].ID
	_, err = chat.Resolve(context.Background(), 7, input)
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	for _, msgs := range [][]ChatMessage{nil, {{Role: "system", Content: "x"}}, {{Role: "user", Content: " "}}, {{Role: "assistant", Content: "x"}}, {{Role: "user", Content: "x"}, {Role: "assistant", Content: "y"}}} {
		require.Error(t, ValidateChat(ChatInput{Messages: msgs}))
	}
}
func TestValidateChatMessageLimits(t *testing.T) {
	long := func(n int) string { return strings.Repeat("字", n) }
	user := func(content string) ChatMessage { return ChatMessage{Role: "user", Content: content} }
	assistant := func(content string) ChatMessage { return ChatMessage{Role: "assistant", Content: content} }

	// The turn being sent is capped at 2000 characters.
	require.NoError(t, ValidateChat(ChatInput{Messages: []ChatMessage{user(long(ChatMessageMaxRunes))}}))
	require.Error(t, ValidateChat(ChatInput{Messages: []ChatMessage{user(long(ChatMessageMaxRunes + 1))}}))

	// Assistant replies keep the larger legacy cap.
	require.NoError(t, ValidateChat(ChatInput{Messages: []ChatMessage{user("问"), assistant(long(ChatHistoryMaxRunes)), user("再问")}}))
	require.Error(t, ValidateChat(ChatInput{Messages: []ChatMessage{user("问"), assistant(long(ChatHistoryMaxRunes + 1)), user("再问")}}))

	// Historical user messages are not re-capped, so reopened conversations stay usable.
	require.NoError(t, ValidateChat(ChatInput{Messages: []ChatMessage{user(long(ChatHistoryMaxRunes)), assistant("答"), user("再问")}}))
	require.Error(t, ValidateChat(ChatInput{Messages: []ChatMessage{user(long(ChatHistoryMaxRunes + 1)), assistant("答"), user("再问")}}))
}

func TestChatCapabilityLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &ChatService{}
	cap := &ChatCapability{Context: ctx}
	token, revoke, err := s.Register(cap)
	require.NoError(t, err)
	require.Len(t, token, 64)
	require.Equal(t, cap, s.Capability(token))
	cancel()
	require.Nil(t, s.Capability(token))
	revoke()
	require.Nil(t, s.Capability(token))
}
func TestChatRuntimeEndpointPolicy(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "::1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "100.64.0.1"} {
		require.True(t, unsafeModelIP(net.ParseIP(ip)))
	}
	require.False(t, unsafeModelIP(net.ParseIP("8.8.8.8")))

	ai := newAIConfigTestService(t)
	// Saving stays permissive; the deployment policy is enforced when a provider is used.
	input := testProviderInput()
	input.BaseURL = "http://127.0.0.1:11434/v1"
	input.AuthType = "none"
	input.APIKey = nil
	view, err := ai.SaveProvider(context.Background(), 7, 0, input)
	require.NoError(t, err)
	chat := &ChatService{Store: ai.Store, Credentials: ai.Credentials}
	req := ChatInput{ModelID: view.Providers[0].Models[0].ID, Messages: []ChatMessage{{Role: "user", Content: "你好"}}}
	_, err = chat.Resolve(context.Background(), 7, req)
	require.ErrorIs(t, err, ErrAIConfigInvalid)
	chat.AllowedPrivateEndpoints = "http://127.0.0.1:11434/v1"
	_, err = chat.Resolve(context.Background(), 7, req)
	require.NoError(t, err)

	chat.AllowedPrivateEndpoints = ""
	require.Error(t, chat.validateRuntimeModelURL("http://example.com/v1"))
	require.NoError(t, chat.validateRuntimeModelURL("https://example.com/v1"))
	require.Error(t, chat.validateRuntimeModelURL("http://localhost:11434/v1"))
	chat.AllowedPrivateEndpoints = "http://localhost:11434/v1"
	require.NoError(t, chat.validateRuntimeModelURL("http://localhost:11434/v1"))
}

func TestNormalizeChatHistoryDropsInterruptedTurns(t *testing.T) {
	cases := []struct {
		name string
		in   []ChatMessage
		want []ChatMessage
	}{
		{
			name: "healthy history is untouched",
			in: []ChatMessage{
				{Role: "user", Content: "问一"},
				{Role: "assistant", Content: "答一"},
				{Role: "user", Content: "问二"},
			},
			want: []ChatMessage{
				{Role: "user", Content: "问一"},
				{Role: "assistant", Content: "答一"},
				{Role: "user", Content: "问二"},
			},
		},
		{
			name: "trailing interrupted answer is dropped",
			in: []ChatMessage{
				{Role: "user", Content: "问一"},
				{Role: "assistant", Content: "答一"},
				{Role: "user", Content: "问二"},
				{Role: "assistant", Content: "   "},
			},
			want: []ChatMessage{
				{Role: "user", Content: "问一"},
				{Role: "assistant", Content: "答一"},
				{Role: "user", Content: "问二"},
			},
		},
		{
			name: "interrupted answer in the middle merges adjacent user turns",
			in: []ChatMessage{
				{Role: "user", Content: "问一"},
				{Role: "assistant", Content: ""},
				{Role: "user", Content: "问二"},
			},
			want: []ChatMessage{{Role: "user", Content: "问一\n\n问二"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, NormalizeChatHistory(tc.in))
		})
	}
}
