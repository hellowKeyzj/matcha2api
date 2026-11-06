package config

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeBasePath returns an empty prefix for root deployments.
func NormalizeBasePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "//") {
		return "", fmt.Errorf("APP_BASE_PATH must be an absolute URL path")
	}
	value = strings.TrimRight(value, "/")
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#\\") {
		return "", fmt.Errorf("APP_BASE_PATH must be an absolute URL path")
	}
	for _, segment := range strings.Split(value[1:], "/") {
		if segment == "" || segment == "." || segment == ".." || url.PathEscape(segment) != segment {
			return "", fmt.Errorf("APP_BASE_PATH contains an invalid path segment")
		}
	}
	return value, nil
}
