//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type sensitiveWordSettingRepoStub struct {
	values map[string]string
}

func (r *sensitiveWordSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *sensitiveWordSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (r *sensitiveWordSettingRepoStub) Set(ctx context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *sensitiveWordSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if v, ok := r.values[key]; ok {
			out[key] = v
		}
	}
	return out, nil
}

func (r *sensitiveWordSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *sensitiveWordSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *sensitiveWordSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func newSensitiveWordSettingService(t *testing.T) *service.SettingService {
	t.Helper()
	repo := &sensitiveWordSettingRepoStub{values: map[string]string{}}
	settingService := service.NewSettingService(repo, &config.Config{})
	err := settingService.UpdateSettings(context.Background(), &service.SystemSettings{
		SensitiveWordFilterEnabled: true,
		SensitiveWordFilterWords:   []string{"blockedword"},
	})
	require.NoError(t, err)
	return settingService
}

func newSensitiveWordRequestContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(1)
	user := &service.User{ID: 7, Concurrency: 1, Balance: 10, Status: service.StatusActive}
	group := &service.Group{ID: groupID, Platform: domain.PlatformAnthropic, Hydrated: true}
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
		ID:      11,
		UserID:  user.ID,
		User:    user,
		GroupID: &groupID,
		Group:   group,
		Status:  service.StatusActive,
	})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: user.ID, Concurrency: user.Concurrency})
	return c, w
}

func TestGatewayMessagesSensitiveWordBlockedBeforeDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingService := newSensitiveWordSettingService(t)
	h := &GatewayHandler{settingService: settingService}
	c, w := newSensitiveWordRequestContext(http.MethodPost, "/v1/messages", `{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"blockedword"}]}`)

	h.Messages(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), service.SensitiveWordBlockedMessage)
	require.NotContains(t, w.Body.String(), "blockedword")
}

func TestOpenAIResponsesSensitiveWordBlockedBeforeDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingService := newSensitiveWordSettingService(t)
	gatewayService := service.NewOpenAIGatewayService(nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, settingService)
	h := &OpenAIGatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: &service.BillingCacheService{},
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper:   NewConcurrencyHelper(&service.ConcurrencyService{}, SSEPingFormatComment, 0),
		maxAccountSwitches:  1,
	}
	c, w := newSensitiveWordRequestContext(http.MethodPost, "/openai/v1/responses", `{"model":"gpt-4o","input":"blockedword"}`)

	h.Responses(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), service.SensitiveWordBlockedMessage)
	require.NotContains(t, w.Body.String(), "blockedword")
}
