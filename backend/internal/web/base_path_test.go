package web

import (
	"strings"
	"testing"
)

func TestMountHTML(t *testing.T) {
	for _, prefix := range []string{"", "/sub2api"} {
		got := string(mountHTML([]byte(`<head><!--app-base--><script src="./assets/app.js"></script></head>`), prefix))
		if !strings.Contains(got, `<base href="`+prefix+`/" />`) || !strings.Contains(got, `src="./assets/app.js"`) {
			t.Fatal(got)
		}
	}
}
