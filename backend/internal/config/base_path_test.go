package config

import "testing"

func TestNormalizeBasePath(t *testing.T) {
	for input, want := range map[string]string{"": "", "/": "", "/sub2api/": "/sub2api", "/tools/sub2api": "/tools/sub2api"} {
		got, err := NormalizeBasePath(input)
		if err != nil || got != want {
			t.Fatalf("%q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{"sub2api", "//", "///", "//evil/x", "/a/../b", "/a?b", "/a%2fb", "/a\\b"} {
		if _, err := NormalizeBasePath(input); err == nil {
			t.Fatal(input)
		}
	}
}
