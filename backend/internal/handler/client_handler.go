package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ClientHandler struct {
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	cfg            *config.Config
}

func NewClientHandler(apiKeyService *service.APIKeyService, settingService *service.SettingService, cfg *config.Config) *ClientHandler {
	return &ClientHandler{apiKeyService: apiKeyService, settingService: settingService, cfg: cfg}
}

type ClientBootstrapResponse struct {
	SchemaVersion int                     `json:"schema_version"`
	Ready         bool                    `json:"ready"`
	NeedsSetup    bool                    `json:"needs_setup"`
	SetupReason   string                  `json:"setup_reason,omitempty"`
	BaseURL       string                  `json:"base_url"`
	RootURL       string                  `json:"root_url"`
	Gateway       ClientGatewayConfig     `json:"gateway"`
	APIKey        *ClientGatewayAPIKey    `json:"api_key"`
	Clients       map[string]ClientConfig `json:"clients"`
}

type ClientGatewayConfig struct {
	RootURL        string                 `json:"root_url"`
	BaseURL        string                 `json:"base_url"`
	BaseURLAliases []string               `json:"base_url_aliases"`
	Endpoints      ClientGatewayEndpoints `json:"endpoints"`
}

type ClientGatewayEndpoints struct {
	Models          string `json:"models"`
	Usage           string `json:"usage"`
	Messages        string `json:"messages"`
	Responses       string `json:"responses"`
	ChatCompletions string `json:"chat_completions"`
	Embeddings      string `json:"embeddings"`
	GeminiModels    string `json:"gemini_models"`
}

type ClientGatewayAPIKey struct {
	ID        int64      `json:"id"`
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	GroupID   *int64     `json:"group_id"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Group     *dto.Group `json:"group"`
}

type ClientConfig struct {
	BaseURL     string `json:"base_url"`
	APIKey      string `json:"api_key,omitempty"`
	ModelsURL   string `json:"models_url,omitempty"`
	MessagesURL string `json:"messages_url,omitempty"`
}

func (h *ClientHandler) Bootstrap(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	rootURL, baseURL := h.gatewayURLs(c)
	cfg := newClientBootstrapResponse(rootURL, baseURL, nil)

	apiKey, err := h.apiKeyService.GetDefaultGatewayAPIKey(c.Request.Context(), subject.UserID)
	if err != nil {
		if errors.Is(err, service.ErrGatewayAPIKeyUnavailable) {
			cfg.NeedsSetup = true
			cfg.SetupReason = "gateway_api_key_unavailable"
			response.Success(c, cfg)
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	cfg = newClientBootstrapResponse(rootURL, baseURL, apiKey)
	response.Success(c, cfg)
}

func newClientBootstrapResponse(rootURL, baseURL string, apiKey *service.APIKey) ClientBootstrapResponse {
	key := ""
	if apiKey != nil {
		key = apiKey.Key
	}
	return ClientBootstrapResponse{
		SchemaVersion: 1,
		Ready:         apiKey != nil,
		NeedsSetup:    apiKey == nil,
		BaseURL:       baseURL,
		RootURL:       rootURL,
		Gateway: ClientGatewayConfig{
			RootURL:        rootURL,
			BaseURL:        baseURL,
			BaseURLAliases: []string{rootURL},
			Endpoints: ClientGatewayEndpoints{
				Models:          joinURLPath(baseURL, "/models"),
				Usage:           joinURLPath(baseURL, "/usage"),
				Messages:        joinURLPath(baseURL, "/messages"),
				Responses:       joinURLPath(baseURL, "/responses"),
				ChatCompletions: joinURLPath(baseURL, "/chat/completions"),
				Embeddings:      joinURLPath(baseURL, "/embeddings"),
				GeminiModels:    joinURLPath(rootURL, "/v1beta/models"),
			},
		},
		APIKey:  clientGatewayAPIKeyFromService(apiKey),
		Clients: clientConfigs(rootURL, baseURL, key),
	}
}

func clientGatewayAPIKeyFromService(apiKey *service.APIKey) *ClientGatewayAPIKey {
	if apiKey == nil {
		return nil
	}
	return &ClientGatewayAPIKey{
		ID:        apiKey.ID,
		Key:       apiKey.Key,
		Name:      apiKey.Name,
		Status:    apiKey.Status,
		GroupID:   apiKey.GroupID,
		ExpiresAt: apiKey.ExpiresAt,
		Group:     dto.GroupFromServiceShallow(apiKey.Group),
	}
}

func clientConfigs(rootURL, baseURL, apiKey string) map[string]ClientConfig {
	return map[string]ClientConfig{
		"openai": {
			BaseURL:   baseURL,
			APIKey:    apiKey,
			ModelsURL: joinURLPath(baseURL, "/models"),
		},
		"anthropic": {
			BaseURL:     rootURL,
			APIKey:      apiKey,
			ModelsURL:   joinURLPath(baseURL, "/models"),
			MessagesURL: joinURLPath(baseURL, "/messages"),
		},
		"gemini": {
			BaseURL:   rootURL,
			APIKey:    apiKey,
			ModelsURL: joinURLPath(rootURL, "/v1beta/models"),
		},
	}
}

func (h *ClientHandler) gatewayURLs(c *gin.Context) (string, string) {
	if h != nil && h.settingService != nil && c != nil && c.Request != nil {
		if root, base := normalizeGatewayBaseURL(h.settingService.GetAPIBaseURL(c.Request.Context())); root != "" {
			return root, base
		}
	}
	if h != nil && h.cfg != nil {
		if root := originFromAbsoluteURL(h.cfg.Server.FrontendURL); root != "" {
			return root, joinURLPath(root, "/v1")
		}
	}
	root := requestRootURL(c)
	return root, joinURLPath(root, "/v1")
}

func originFromAbsoluteURL(raw string) string {
	root, _ := normalizeGatewayBaseURL(raw)
	return root
}

func requestRootURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return "http://localhost"
	}

	scheme := requestScheme(c.Request)
	host := requestHost(c.Request)
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func normalizeGatewayBaseURL(raw string) (string, string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ""
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	base := strings.TrimRight(u.String(), "/")
	root := strings.TrimRight(u.Scheme+"://"+u.Host, "/")
	if base == root {
		base = joinURLPath(root, "/v1")
	}
	return root, base
}

func requestScheme(r *http.Request) string {
	if scheme := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); scheme == "http" || scheme == "https" {
		return scheme
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestHost(r *http.Request) string {
	if host := cleanRequestHost(firstForwardedValue(r.Header.Get("X-Forwarded-Host"))); host != "" {
		return host
	}
	return cleanRequestHost(r.Host)
}

func firstForwardedValue(raw string) string {
	value := strings.TrimSpace(raw)
	if i := strings.IndexByte(value, ','); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	return strings.ToLower(value)
}

func cleanRequestHost(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" || strings.ContainsAny(host, "/\\?#@") {
		return ""
	}
	for _, r := range host {
		if r <= 31 || r == 127 {
			return ""
		}
	}
	return host
}

func joinURLPath(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}
