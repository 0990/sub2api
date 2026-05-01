package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const SensitiveWordBlockedMessage = "Request blocked by sensitive word policy"

type SensitiveWordFilterSettings struct {
	Enabled bool
	Words   []string
}

type cachedSensitiveWordFilterSettings struct {
	settings  SensitiveWordFilterSettings
	expiresAt int64 // unix nano
}

var sensitiveWordFilterCache atomic.Value // *cachedSensitiveWordFilterSettings
var sensitiveWordFilterSF singleflight.Group

const sensitiveWordFilterCacheTTL = 60 * time.Second
const sensitiveWordFilterErrorTTL = 5 * time.Second
const sensitiveWordFilterDBTimeout = 5 * time.Second

func NormalizeSensitiveWordFilterWords(words []string) []string {
	if len(words) == 0 {
		return []string{}
	}
	normalized := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		trimmed := strings.TrimSpace(word)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func ParseSensitiveWordFilterWords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return []string{}
	}
	var words []string
	if err := json.Unmarshal([]byte(raw), &words); err != nil {
		return []string{}
	}
	return NormalizeSensitiveWordFilterWords(words)
}

func MarshalSensitiveWordFilterWords(words []string) string {
	normalized := NormalizeSensitiveWordFilterWords(words)
	data, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func SensitiveWordFilterMatched(settings SensitiveWordFilterSettings, body []byte) bool {
	if !settings.Enabled || len(body) == 0 {
		return false
	}
	words := NormalizeSensitiveWordFilterWords(settings.Words)
	if len(words) == 0 {
		return false
	}
	haystack := strings.ToLower(string(body))
	for _, word := range words {
		if strings.Contains(haystack, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func CheckSensitiveWordPolicy(ctx context.Context, settingService *SettingService, body []byte) bool {
	if settingService == nil {
		return false
	}
	return SensitiveWordFilterMatched(settingService.GetSensitiveWordFilterSettings(ctx), body)
}

func (s *SettingService) GetSensitiveWordFilterSettings(ctx context.Context) SensitiveWordFilterSettings {
	if s == nil || s.settingRepo == nil {
		return SensitiveWordFilterSettings{Words: []string{}}
	}
	if cached, ok := sensitiveWordFilterCache.Load().(*cachedSensitiveWordFilterSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.settings
		}
	}
	val, _, _ := sensitiveWordFilterSF.Do("sensitive_word_filter", func() (any, error) {
		if cached, ok := sensitiveWordFilterCache.Load().(*cachedSensitiveWordFilterSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.settings, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sensitiveWordFilterDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeySensitiveWordFilterEnabled,
			SettingKeySensitiveWordFilterWords,
		})
		if err != nil {
			slog.Warn("failed to get sensitive word filter settings", "error", err)
			disabled := SensitiveWordFilterSettings{Words: []string{}}
			sensitiveWordFilterCache.Store(&cachedSensitiveWordFilterSettings{
				settings:  disabled,
				expiresAt: time.Now().Add(sensitiveWordFilterErrorTTL).UnixNano(),
			})
			return disabled, nil
		}
		settings := SensitiveWordFilterSettings{
			Enabled: values[SettingKeySensitiveWordFilterEnabled] == "true",
			Words:   ParseSensitiveWordFilterWords(values[SettingKeySensitiveWordFilterWords]),
		}
		sensitiveWordFilterCache.Store(&cachedSensitiveWordFilterSettings{
			settings:  settings,
			expiresAt: time.Now().Add(sensitiveWordFilterCacheTTL).UnixNano(),
		})
		return settings, nil
	})
	if settings, ok := val.(SensitiveWordFilterSettings); ok {
		return settings
	}
	return SensitiveWordFilterSettings{Words: []string{}}
}
