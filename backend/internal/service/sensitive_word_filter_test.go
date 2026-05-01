//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSensitiveWordFilterMatched(t *testing.T) {
	tests := []struct {
		name     string
		settings SensitiveWordFilterSettings
		body     string
		want     bool
	}{
		{
			name:     "disabled does not block",
			settings: SensitiveWordFilterSettings{Enabled: false, Words: []string{"blocked"}},
			body:     `{"messages":[{"content":"blocked"}]}`,
		},
		{
			name:     "empty words does not block",
			settings: SensitiveWordFilterSettings{Enabled: true, Words: []string{}},
			body:     `{"messages":[{"content":"blocked"}]}`,
		},
		{
			name:     "english case insensitive match",
			settings: SensitiveWordFilterSettings{Enabled: true, Words: []string{"SecretTerm"}},
			body:     `{"input":"this has a secretterm inside"}`,
			want:     true,
		},
		{
			name:     "chinese match",
			settings: SensitiveWordFilterSettings{Enabled: true, Words: []string{"敏感词"}},
			body:     `{"content":"这里有敏感词"}`,
			want:     true,
		},
		{
			name:     "empty and duplicate words ignored",
			settings: SensitiveWordFilterSettings{Enabled: true, Words: []string{" ", "Block", "block"}},
			body:     `{"content":"please BLOCK this"}`,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SensitiveWordFilterMatched(tt.settings, []byte(tt.body)))
		})
	}
}

func TestSensitiveWordFilterBlockedMessageDoesNotLeakWord(t *testing.T) {
	require.NotContains(t, SensitiveWordBlockedMessage, "SecretTerm")
	require.Equal(t, "Request blocked by sensitive word policy", SensitiveWordBlockedMessage)
}

func TestSensitiveWordFilterWordsNormalizeAndMarshal(t *testing.T) {
	words := NormalizeSensitiveWordFilterWords([]string{"  Alpha  ", "", "alpha", "中文", " 中文 "})
	require.Equal(t, []string{"Alpha", "中文"}, words)
	require.JSONEq(t, `["Alpha","中文"]`, MarshalSensitiveWordFilterWords(words))
	require.Equal(t, words, ParseSensitiveWordFilterWords(`[" Alpha ","alpha","中文",""]`))
}

func TestSettingService_SensitiveWordFilterDefaultsAndUpdate(t *testing.T) {
	svc := NewSettingService(&settingUpdateRepoStub{}, &config.Config{})

	defaults := svc.parseSettings(map[string]string{})
	require.False(t, defaults.SensitiveWordFilterEnabled)
	require.Empty(t, defaults.SensitiveWordFilterWords)

	repo := &settingUpdateRepoStub{}
	svc = NewSettingService(repo, &config.Config{})
	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		SensitiveWordFilterEnabled: true,
		SensitiveWordFilterWords:   []string{"  Alpha  ", "", "alpha", "中文"},
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeySensitiveWordFilterEnabled])
	require.JSONEq(t, `["Alpha","中文"]`, repo.updates[SettingKeySensitiveWordFilterWords])

	parsed := svc.parseSettings(map[string]string{
		SettingKeySensitiveWordFilterEnabled: repo.updates[SettingKeySensitiveWordFilterEnabled],
		SettingKeySensitiveWordFilterWords:   repo.updates[SettingKeySensitiveWordFilterWords],
	})
	require.True(t, parsed.SensitiveWordFilterEnabled)
	require.Equal(t, []string{"Alpha", "中文"}, parsed.SensitiveWordFilterWords)
}
