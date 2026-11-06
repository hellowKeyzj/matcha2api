package server

import (
	"bufio"
	"net"
	"net/http"
	"strings"
)

// MountBasePath mounts the entire HTTP application, including streaming and WebSocket endpoints.
func MountBasePath(next http.Handler, prefix string) http.Handler {
	if prefix == "" {
		return next
	}
	stripped := http.StripPrefix(prefix, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r) // Container health checks remain independent of the public mount.
			return
		}
		if r.URL.Path == prefix {
			target := *r.URL
			target.Path += "/"
			http.Redirect(w, r, target.String(), http.StatusPermanentRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, prefix+"/") {
			http.NotFound(w, r)
			return
		}
		stripped.ServeHTTP(&basePathWriter{ResponseWriter: w, prefix: prefix}, r)
	})
}

type basePathWriter struct {
	http.ResponseWriter
	prefix      string
	wroteHeader bool
}

func (w *basePathWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *basePathWriter) WriteHeader(status int) {
	// Informational responses do not commit the final headers (except upgrades).
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if !w.wroteHeader {
		w.wroteHeader = true
		if location := w.Header().Get("Location"); strings.HasPrefix(location, "/") && !strings.HasPrefix(location, "//") {
			path := location
			if i := strings.IndexAny(path, "?#"); i >= 0 {
				path = path[:i]
			}
			if path != w.prefix && !strings.HasPrefix(path, w.prefix+"/") {
				w.Header().Set("Location", w.prefix+location)
			}
		}
		cookies := (&http.Response{Header: w.Header()}).Cookies()
		w.Header().Del("Set-Cookie")
		for _, cookie := range cookies {
			if cookie.Path != "" && cookie.Path != w.prefix && !strings.HasPrefix(cookie.Path, w.prefix+"/") {
				cookie.Path = w.prefix + cookie.Path
			}
			http.SetCookie(w.ResponseWriter, cookie)
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *basePathWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *basePathWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *basePathWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *basePathWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}
