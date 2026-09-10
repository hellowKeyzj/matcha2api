package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type cloudKeyRouteSettings struct {
	service.SettingRepository
}

func (cloudKeyRouteSettings) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyPanelRateLimitSettings {
		return `{"enabled":true,"public_ip_rpm":1}`, nil
	}
	return "", service.ErrSettingNotFound
}

func TestSealedCloudKeyRouteIsAnonymousAndPublicIPLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	panelLimiter := servermiddleware.NewPanelRateLimiter(client, service.NewSettingService(cloudKeyRouteSettings{}, nil))
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	RegisterUserRoutes(
		router.Group("/api/v1"),
		&handler.Handlers{Package: handler.NewPackageRegistryHandler(service.NewMatchaPackageRegistryService(nil, nil, "", nil))},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
		panelLimiter,
	)

	request := func(method, path, remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	const path = "/api/v1/packages/sealed-cloud-key"
	const counter = "rate_limit:panel:public:ip:203.0.113.9"

	// No key is configured: reaching this handler error proves JWT is not required.
	// Successful key export is covered by handler tests, not by this routing check.
	anonymous := request(http.MethodGet, path, "203.0.113.9:1000")
	require.Equal(t, http.StatusServiceUnavailable, anonymous.Code)
	require.Contains(t, anonymous.Body.String(), "MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED")
	count, err := server.Get(counter)
	require.NoError(t, err)
	require.Equal(t, "1", count)

	limited := request(http.MethodGet, path, "203.0.113.9:2000")
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Contains(t, limited.Body.String(), "RATE_LIMITED")
	require.NotEmpty(t, limited.Header().Get("Retry-After"))

	otherIP := request(http.MethodGet, path, "198.51.100.7:1000")
	require.Equal(t, http.StatusServiceUnavailable, otherIP.Code)
	require.Contains(t, otherIP.Body.String(), "MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED")
	otherCount, err := server.Get("rate_limit:panel:public:ip:198.51.100.7")
	require.NoError(t, err)
	require.Equal(t, "1", otherCount)
	require.Equal(t, http.StatusTooManyRequests, request(http.MethodGet, path, "198.51.100.7:2000").Code)
	require.Equal(t, http.StatusServiceUnavailable, request(http.MethodGet, path, "127.0.0.1:1000").Code)
	require.False(t, server.Exists("rate_limit:panel:public:ip:127.0.0.1"))

	for _, protected := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/client/bootstrap"},
		{http.MethodGet, "/api/v1/packages/mine"},
		{http.MethodGet, "/api/v1/packages/market"},
		{http.MethodPost, "/api/v1/packages/upload"},
		{http.MethodPost, "/api/v1/packages/1/publish"},
		{http.MethodPost, "/api/v1/packages/1/authorization"},
		{http.MethodGet, "/api/v1/packages/1/download"},
	} {
		t.Run(protected.path, func(t *testing.T) {
			require.Equal(t, http.StatusUnauthorized, request(protected.method, protected.path, "203.0.113.9:1000").Code)
		})
	}
}
