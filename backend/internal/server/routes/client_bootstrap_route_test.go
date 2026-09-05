//go:build unit

package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestUserRoutesClientBootstrapPathIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{Client: handler.NewClientHandler(nil, nil, nil)},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
		nil,
	)

	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/v1/client/bootstrap" {
			return
		}
	}

	t.Fatal("GET /api/v1/client/bootstrap should be registered")
}
