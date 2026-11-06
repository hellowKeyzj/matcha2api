package server

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMountBasePath(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/login")
		http.SetCookie(w, &http.Cookie{Name: "state", Value: "x", Path: "/api/v1/auth"})
		w.WriteHeader(302)
		_, _ = w.Write([]byte(r.URL.RequestURI()))
	})
	for _, prefix := range []string{"", "/sub2api"} {
		h := MountBasePath(next, prefix)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", prefix+"/api/v1/auth?x=1", nil))
		if w.Body.String() != "/api/v1/auth?x=1" {
			t.Fatal(w.Body.String())
		}
		if w.Header().Get("Location") != prefix+"/login" {
			t.Fatal(w.Header())
		}
		if w.Result().Cookies()[0].Path != prefix+"/api/v1/auth" {
			t.Fatal(w.Header())
		}
	}
	h := MountBasePath(next, "/sub2api")
	for path, status := range map[string]int{"/sub2api": 308, "/sub2apix/": 404, "/api/v1/auth": 404, "/health": 302} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}

func TestMountBasePathStreamingAndUpgrade(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: ready\n\n")
			w.(http.Flusher).Flush()
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: test\r\n\r\n")
		_ = rw.Flush()
	})
	s := httptest.NewServer(MountBasePath(next, "/sub2api"))
	defer s.Close()
	resp, err := s.Client().Get(s.URL + "/sub2api/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != "data: ready\n\n" {
		t.Fatal(string(data), err)
	}
	conn, err := net.DialTimeout("tcp", s.Listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = io.WriteString(conn, "GET /sub2api/upgrade HTTP/1.1\r\nHost: localhost\r\nConnection: Upgrade\r\nUpgrade: test\r\n\r\n")
	upgrade, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil || upgrade.StatusCode != 101 {
		t.Fatal(upgrade, err)
	}
}

func TestMountBasePathInformationalResponse(t *testing.T) {
	s := httptest.NewServer(MountBasePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		w.Header().Set("Location", "/login")
		http.SetCookie(w, &http.Cookie{Name: "state", Value: "ready", Path: "/api/v1/auth"})
		w.WriteHeader(http.StatusFound)
	}), "/sub2api"))
	defer s.Close()
	client := s.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(s.URL + "/sub2api/start")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/sub2api/login" {
		t.Fatal(resp.StatusCode, resp.Header)
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Path != "/sub2api/api/v1/auth" {
		t.Fatal(cookies)
	}
}

func TestMountBasePathRedirectBoundaries(t *testing.T) {
	for location, want := range map[string]string{
		"/login?next=/purchase#form":  "/sub2api/login?next=/purchase#form",
		"/sub2api?from=login":         "/sub2api?from=login",
		"/sub2api#top":                "/sub2api#top",
		"/sub2api/login":              "/sub2api/login",
		"/sub2apix/login":             "/sub2api/sub2apix/login",
		"//external.test/login":       "//external.test/login",
		"https://external.test/login": "https://external.test/login",
	} {
		t.Run(location, func(t *testing.T) {
			h := MountBasePath(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", location)
				w.WriteHeader(http.StatusFound)
			}), "/sub2api")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/sub2api/start", nil))
			if got := w.Header().Get("Location"); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}
