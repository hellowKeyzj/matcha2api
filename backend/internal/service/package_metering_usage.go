package service

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
)

const (
	MatchaPackageMeteringHeader = "x-matcha-package-metering"

	matchaPackageMeteringPrefix         = "m1."
	maxPackageMeteringBindingLength     = 512
	maxPackageMeteringDecodedLength     = 512
	maxPackageMeteringRuntimeLength     = 64
	maxPackageMeteringResourceKeyLength = 160
)

type PackageMeteringUsage struct {
	Bindings []string
}

type PackageMeteringBindingPayload struct {
	V             int    `json:"v"`
	Kind          string `json:"kind"`
	Runtime       string `json:"runtime"`
	PackageSHA256 string `json:"packageSha256"`
	Key           string `json:"key"`
	Use           string `json:"use"`
}

func ParsePackageMeteringHeader(value string) PackageMeteringUsage {
	value = strings.TrimSpace(value)
	if value == "" {
		return PackageMeteringUsage{}
	}
	parts := splitPackageMeteringHeaderValue(value)
	return NormalizePackageMeteringUsage(PackageMeteringUsage{Bindings: parts})
}

func ParsePackageMeteringRequestHeader(h http.Header) PackageMeteringUsage {
	if len(h) == 0 {
		return PackageMeteringUsage{}
	}
	values := h.Values(MatchaPackageMeteringHeader)
	if len(values) == 0 {
		values = h.Values(http.CanonicalHeaderKey(MatchaPackageMeteringHeader))
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, splitPackageMeteringHeaderValue(value)...)
	}
	return NormalizePackageMeteringUsage(PackageMeteringUsage{Bindings: parts})
}

func NormalizePackageMeteringUsage(usage PackageMeteringUsage) PackageMeteringUsage {
	bindings := make([]string, 0, len(usage.Bindings))
	seen := make(map[string]struct{}, len(usage.Bindings))
	for _, part := range usage.Bindings {
		_, binding, ok := ParsePackageMeteringBinding(part)
		if !ok {
			continue
		}
		if _, ok := seen[binding]; ok {
			continue
		}
		seen[binding] = struct{}{}
		bindings = append(bindings, binding)
	}
	sort.Strings(bindings)
	return PackageMeteringUsage{Bindings: bindings}
}

func ParsePackageMeteringBinding(binding string) (PackageMeteringBindingPayload, string, bool) {
	var payload PackageMeteringBindingPayload
	binding = strings.TrimSpace(binding)
	if len(binding) <= len(matchaPackageMeteringPrefix) || len(binding) > maxPackageMeteringBindingLength || !strings.HasPrefix(binding, matchaPackageMeteringPrefix) {
		return payload, "", false
	}
	encoded := strings.TrimPrefix(binding, matchaPackageMeteringPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		raw, err = base64.URLEncoding.DecodeString(encoded)
		if err != nil {
			return payload, "", false
		}
	}
	if len(raw) == 0 || len(raw) > maxPackageMeteringDecodedLength {
		return payload, "", false
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, "", false
	}
	canonical, ok := BuildPackageMeteringBinding(payload)
	if !ok {
		return payload, "", false
	}
	payload, _, _ = ParsePackageMeteringBindingCanonical(canonical)
	return payload, canonical, true
}

func BuildPackageMeteringBinding(payload PackageMeteringBindingPayload) (string, bool) {
	payload, ok := normalizePackageMeteringPayload(payload)
	if !ok {
		return "", false
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	if len(raw) > maxPackageMeteringDecodedLength {
		return "", false
	}
	binding := matchaPackageMeteringPrefix + base64.RawURLEncoding.EncodeToString(raw)
	if len(binding) > maxPackageMeteringBindingLength {
		return "", false
	}
	return binding, true
}

func ParsePackageMeteringBindingCanonical(binding string) (PackageMeteringBindingPayload, string, bool) {
	var payload PackageMeteringBindingPayload
	binding = strings.TrimSpace(binding)
	if !strings.HasPrefix(binding, matchaPackageMeteringPrefix) || len(binding) > maxPackageMeteringBindingLength {
		return payload, "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(binding, matchaPackageMeteringPrefix))
	if err != nil || len(raw) == 0 || len(raw) > maxPackageMeteringDecodedLength {
		return payload, "", false
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, "", false
	}
	payload, ok := normalizePackageMeteringPayload(payload)
	if !ok {
		return payload, "", false
	}
	return payload, binding, true
}

func StripMatchaHeaders(h http.Header) {
	for key := range h {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "x-matcha-") {
			delete(h, key)
		}
	}
}

func (u PackageMeteringUsage) Empty() bool {
	return len(u.Bindings) == 0
}

func splitPackageMeteringHeaderValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		var array []string
		if err := json.Unmarshal([]byte(value), &array); err == nil {
			return array
		}
	}
	return strings.Split(value, ",")
}

func normalizePackageMeteringPayload(payload PackageMeteringBindingPayload) (PackageMeteringBindingPayload, bool) {
	payload.Kind = strings.ToLower(strings.TrimSpace(payload.Kind))
	payload.Runtime = strings.TrimSpace(payload.Runtime)
	payload.PackageSHA256 = strings.ToLower(strings.TrimSpace(payload.PackageSHA256))
	payload.Key = strings.TrimSpace(payload.Key)
	payload.Use = strings.ToLower(strings.TrimSpace(payload.Use))
	if payload.V != 1 || !validPackageMeteringKind(payload.Kind) || !validPackageMeteringUnit(payload.Kind, payload.Use) {
		return payload, false
	}
	if !isPackageMeteringSHA256(payload.PackageSHA256) || payload.Runtime == "" || len(payload.Runtime) > maxPackageMeteringRuntimeLength {
		return payload, false
	}
	if payload.Key == "" || len(payload.Key) > maxPackageMeteringResourceKeyLength {
		return payload, false
	}
	return payload, true
}

func validPackageMeteringKind(kind string) bool {
	switch kind {
	case "agent", "skill":
		return true
	default:
		return false
	}
}

func validPackageMeteringUnit(kind, unit string) bool {
	if unit == "" {
		return true
	}
	switch unit {
	case "turn", "session":
		return true
	default:
		return false
	}
}

func isPackageMeteringSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
