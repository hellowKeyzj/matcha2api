package web

import (
	"bytes"
	"html"
)

// Vite emits relative assets; an explicit base also fixes deep-link reloads.
func mountHTML(content []byte, prefix string) []byte {
	base := prefix + "/"
	tag := []byte(`<base href="` + html.EscapeString(base) + `" />`)
	return bytes.Replace(content, []byte("<!--app-base-->"), tag, 1)
}
