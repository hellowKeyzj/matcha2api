//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type clientBootstrapSettingRepoStub struct {
	values map[string]string
}

func (s *clientBootstrapSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *clientBootstrapSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *clientBootstrapSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *clientBootstrapSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = s.values[key]
	}
	return out, nil
}

func (s *clientBootstrapSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *clientBootstrapSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *clientBootstrapSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type clientBootstrapGroupRepoStub struct {
	service.GroupRepository
	group *service.Group
	err   error
}

func (r *clientBootstrapGroupRepoStub) GetDefaultShardGroup(context.Context) (*service.Group, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.group == nil {
		return nil, nil
	}
	clone := *r.group
	return &clone, nil
}

type clientBootstrapAPIKeyRepoStub struct {
	service.APIKeyRepository
	keys []service.APIKey
}

func (r *clientBootstrapAPIKeyRepoStub) ListAllByUserID(context.Context, int64, service.APIKeyListFilters) ([]service.APIKey, error) {
	out := make([]service.APIKey, len(r.keys))
	copy(out, r.keys)
	return out, nil
}

func (r *clientBootstrapAPIKeyRepoStub) ListByUserID(_ context.Context, _ int64, params pagination.PaginationParams, _ service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	out := make([]service.APIKey, len(r.keys))
	copy(out, r.keys)
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func TestClientHandlerBootstrapRequiresAuthSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClientHandler(nil, nil, nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/client/bootstrap", nil)

	h.Bootstrap(c)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestClientHandlerBootstrapReturnsSetupStateWhenDefaultGatewayKeyUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newClientBootstrapTestHandler(
		map[string]string{service.SettingKeyAPIBaseURL: "https://gateway.example.com/v1"},
		nil,
		service.ErrDefaultShardGroupNotConfigured,
		nil,
	)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/client/bootstrap", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})

	h.Bootstrap(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Ready       bool                    `json:"ready"`
			NeedsSetup  bool                    `json:"needs_setup"`
			SetupReason string                  `json:"setup_reason"`
			RootURL     string                  `json:"root_url"`
			BaseURL     string                  `json:"base_url"`
			APIKey      map[string]any          `json:"api_key"`
			Clients     map[string]ClientConfig `json:"clients"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.False(t, resp.Data.Ready)
	require.True(t, resp.Data.NeedsSetup)
	require.Equal(t, "gateway_api_key_unavailable", resp.Data.SetupReason)
	require.Equal(t, "https://gateway.example.com", resp.Data.RootURL)
	require.Equal(t, "https://gateway.example.com/v1", resp.Data.BaseURL)
	require.Nil(t, resp.Data.APIKey)
	require.Equal(t, "https://gateway.example.com/v1", resp.Data.Clients["openai"].BaseURL)
}

func TestClientHandlerBootstrapReturnsGatewayKeyAndClientConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(42)
	group := &service.Group{
		ID:                    groupID,
		Name:                  "default-supply",
		Platform:              service.PlatformComposite,
		Status:                service.StatusActive,
		ShardEnabled:          true,
		SubscriptionType:      service.SubscriptionTypeSubscriptionBalance,
		AllowMessagesDispatch: true,
	}
	h := newClientBootstrapTestHandler(
		map[string]string{service.SettingKeyAPIBaseURL: "https://gateway.example.com"},
		group,
		nil,
		[]service.APIKey{{ID: 5, UserID: 7, Name: "Default API Key", Key: "sk-live", Status: service.StatusActive, GroupID: &groupID}},
	)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/client/bootstrap", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})

	h.Bootstrap(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int                     `json:"code"`
		Data ClientBootstrapResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, 1, resp.Data.SchemaVersion)
	require.True(t, resp.Data.Ready)
	require.False(t, resp.Data.NeedsSetup)
	require.Equal(t, "https://gateway.example.com", resp.Data.RootURL)
	require.Equal(t, "https://gateway.example.com/v1", resp.Data.BaseURL)
	require.Equal(t, "https://gateway.example.com/v1/models", resp.Data.Gateway.Endpoints.Models)
	require.Equal(t, "https://gateway.example.com/v1/usage", resp.Data.Gateway.Endpoints.Usage)
	require.Equal(t, "https://gateway.example.com/v1beta/models", resp.Data.Gateway.Endpoints.GeminiModels)
	require.NotNil(t, resp.Data.APIKey)
	require.Equal(t, int64(5), resp.Data.APIKey.ID)
	require.Equal(t, "sk-live", resp.Data.APIKey.Key)
	require.NotNil(t, resp.Data.APIKey.Group)
	require.Equal(t, groupID, resp.Data.APIKey.Group.ID)
	require.Equal(t, service.PlatformComposite, resp.Data.APIKey.Group.Platform)
	require.Equal(t, "https://gateway.example.com/v1", resp.Data.Clients["openai"].BaseURL)
	require.Equal(t, "sk-live", resp.Data.Clients["openai"].APIKey)
	require.Equal(t, "https://gateway.example.com", resp.Data.Clients["anthropic"].BaseURL)
	require.Equal(t, "https://gateway.example.com", resp.Data.Clients["gemini"].BaseURL)
}

func TestClientHandlerBootstrapDerivesURLsFromForwardedRequestWhenUnconfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(42)
	h := newClientBootstrapTestHandler(
		map[string]string{},
		&service.Group{ID: groupID, Status: service.StatusActive, ShardEnabled: true},
		nil,
		[]service.APIKey{{ID: 5, UserID: 7, Key: "sk-live", Status: service.StatusActive, GroupID: &groupID}},
	)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/client/bootstrap", nil)
	c.Request.Host = "internal.local:8080"
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "public.example.com, internal.local")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})

	h.Bootstrap(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Data ClientBootstrapResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, "https://public.example.com", resp.Data.RootURL)
	require.Equal(t, "https://public.example.com/v1", resp.Data.BaseURL)
}

func newClientBootstrapTestHandler(settings map[string]string, group *service.Group, groupErr error, keys []service.APIKey) *ClientHandler {
	settingService := service.NewSettingService(&clientBootstrapSettingRepoStub{values: settings}, &config.Config{})
	apiKeyService := service.NewAPIKeyService(
		&clientBootstrapAPIKeyRepoStub{keys: keys},
		nil,
		&clientBootstrapGroupRepoStub{group: group, err: groupErr},
		nil,
		nil,
		nil,
		nil,
	)
	return NewClientHandler(apiKeyService, settingService, &config.Config{})
}
