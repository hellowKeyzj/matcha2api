//go:build embed

package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/web"
	"github.com/gin-gonic/gin"
)

type mountTestSettings struct{}

func (mountTestSettings) GetPublicSettingsForInjection(context.Context) (any, error) {
	return map[string]any{"site_name": "Mount test", "site_logo": "/logo.svg"}, nil
}

func TestEmbeddedRuntimeMount(t *testing.T) {
	// Both cases use the exact same embedded build, not separate frontend outputs.
	for _, prefix := range []string{"", "/sub2api"} {
		t.Run(prefix+"/", func(t *testing.T) {
			frontend, err := web.NewFrontendServer(mountTestSettings{}, prefix)
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.Use(frontend.Middleware())
			r.GET("/api/v1/test", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
			h := MountBasePath(r, prefix)
			for _, path := range []string{"/", "/login", "/admin/dashboard"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", prefix+path, nil))
				html := w.Body.String()
				if w.Code != 200 || !strings.Contains(html, `<base href="`+prefix+`/" />`) || !strings.Contains(html, `href="`+prefix+`/logo.svg"`) {
					t.Fatalf("%s: %d %s", path, w.Code, html)
				}
				asset := regexp.MustCompile(`src="\./(assets/[^"]+\.js)"`).FindStringSubmatch(html)
				if len(asset) != 2 {
					t.Fatal("missing relative module entry")
				}
				aw := httptest.NewRecorder()
				h.ServeHTTP(aw, httptest.NewRequest("GET", prefix+"/"+asset[1], nil))
				if aw.Code != 200 || strings.Contains(aw.Header().Get("Content-Type"), "text/html") {
					t.Fatal(aw.Code, aw.Header())
				}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", prefix+"/api/v1/test", nil))
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ok":true`) {
				t.Fatal(w.Body.String())
			}
		})
	}
}
