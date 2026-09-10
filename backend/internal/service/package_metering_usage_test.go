package service

import (
	"encoding/base64"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePackageMeteringHeaderCanonicalizesSortsAndDedupes(t *testing.T) {
	bindingA := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "skill",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("a", 64),
		Key:           "skill:openclaw:test-a",
		Use:           "turn",
	})
	bindingB := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "agent",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("b", 64),
		Key:           "agent:openclaw:test-b",
		Use:           "session",
	})

	usage := ParsePackageMeteringHeader(" invalid, " + bindingB + ",," + paddedBinding(bindingA) + "," + bindingB)

	expected := []string{bindingA, bindingB}
	require.ElementsMatch(t, expected, usage.Bindings)
	require.True(t, sortStringsEqual(expected, usage.Bindings), "bindings must be sorted for stable request fingerprints")
}

func TestParsePackageMeteringHeaderJSONArray(t *testing.T) {
	bindingA := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "skill",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("a", 64),
		Key:           "skill:openclaw:test-a",
		Use:           "turn",
	})
	bindingB := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "skill",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("c", 64),
		Key:           "skill:openclaw:test-c",
		Use:           "turn",
	})

	usage := ParsePackageMeteringHeader(`[" ` + bindingA + ` ", "` + bindingB + `", "", "` + bindingA + `", "bad"]`)

	expected := []string{bindingA, bindingB}
	require.ElementsMatch(t, expected, usage.Bindings)
	require.True(t, sortStringsEqual(expected, usage.Bindings), "bindings must be sorted for stable request fingerprints")
}

func TestParsePackageMeteringRequestHeaderUsesAllPhysicalHeaderValues(t *testing.T) {
	bindingA := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "skill",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("a", 64),
		Key:           "skill:openclaw:test-a",
		Use:           "turn",
	})
	bindingB := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "agent",
		Runtime:       "openclaw",
		PackageSHA256: strings.Repeat("d", 64),
		Key:           "agent:openclaw:test-d",
		Use:           "session",
	})

	headers := http.Header{}
	headers.Add(MatchaPackageMeteringHeader, bindingA)
	headers.Add(MatchaPackageMeteringHeader, bindingB+", invalid")

	usage := ParsePackageMeteringRequestHeader(headers)

	expected := []string{bindingA, bindingB}
	require.ElementsMatch(t, expected, usage.Bindings)
	require.True(t, sortStringsEqual(expected, usage.Bindings), "bindings must be sorted for stable request fingerprints")
}

func TestParsePackageMeteringBindingNormalizesURLPadding(t *testing.T) {
	binding := mustPackageMeteringBinding(t, PackageMeteringBindingPayload{
		V:             1,
		Kind:          "SKILL",
		Runtime:       " openclaw ",
		PackageSHA256: strings.ToUpper(strings.Repeat("e", 64)),
		Key:           " skill:openclaw:test-e ",
		Use:           "TURN",
	})

	payload, canonical, ok := ParsePackageMeteringBinding(paddedBinding(binding))

	require.True(t, ok)
	require.Equal(t, binding, canonical)
	require.Equal(t, "skill", payload.Kind)
	require.Equal(t, "openclaw", payload.Runtime)
	require.Equal(t, strings.Repeat("e", 64), payload.PackageSHA256)
	require.Equal(t, "skill:openclaw:test-e", payload.Key)
	require.Equal(t, "turn", payload.Use)
}

func TestParsePackageMeteringBindingRejectsInvalidPayloads(t *testing.T) {
	cases := []PackageMeteringBindingPayload{
		{V: 2, Kind: "skill", Runtime: "openclaw", PackageSHA256: strings.Repeat("a", 64), Key: "skill:bad", Use: "turn"},
		{V: 1, Kind: "workflow", Runtime: "openclaw", PackageSHA256: strings.Repeat("a", 64), Key: "skill:bad", Use: "turn"},
		{V: 1, Kind: "skill", Runtime: "openclaw", PackageSHA256: "not-a-sha", Key: "skill:bad", Use: "turn"},
		{V: 1, Kind: "skill", Runtime: "openclaw", PackageSHA256: strings.Repeat("a", 64), Key: "skill:bad", Use: "request"},
		{V: 1, Kind: "skill", Runtime: strings.Repeat("r", maxPackageMeteringRuntimeLength+1), PackageSHA256: strings.Repeat("a", 64), Key: "skill:bad", Use: "turn"},
		{V: 1, Kind: "skill", Runtime: "openclaw", PackageSHA256: strings.Repeat("a", 64), Key: strings.Repeat("k", maxPackageMeteringResourceKeyLength+1), Use: "turn"},
	}

	for _, payload := range cases {
		binding := rawPackageMeteringBinding(t, payload)
		_, _, ok := ParsePackageMeteringBinding(binding)
		require.False(t, ok, "payload should be rejected: %+v", payload)
	}
}

func TestStripMatchaHeaders(t *testing.T) {
	headers := http.Header{
		"X-Matcha-Package-Metering": []string{"binding1"},
		"X-Matcha-Trace":            []string{"trace"},
		"Authorization":             []string{"Bearer upstream"},
	}

	StripMatchaHeaders(headers)

	require.Empty(t, headers.Values("X-Matcha-Package-Metering"))
	require.Empty(t, headers.Values("X-Matcha-Trace"))
	require.Equal(t, []string{"Bearer upstream"}, headers.Values("Authorization"))
}

func mustPackageMeteringBinding(t *testing.T, payload PackageMeteringBindingPayload) string {
	t.Helper()
	binding, ok := BuildPackageMeteringBinding(payload)
	require.True(t, ok)
	return binding
}

func rawPackageMeteringBinding(t *testing.T, payload PackageMeteringBindingPayload) string {
	t.Helper()
	raw := `{"v":` + intString(payload.V) + `,"kind":"` + payload.Kind + `","runtime":"` + payload.Runtime + `","packageSha256":"` + payload.PackageSHA256 + `","key":"` + payload.Key + `","use":"` + payload.Use + `"}`
	return matchaPackageMeteringPrefix + base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func paddedBinding(binding string) string {
	encoded := strings.TrimPrefix(binding, matchaPackageMeteringPrefix)
	if padding := len(encoded) % 4; padding != 0 {
		encoded += strings.Repeat("=", 4-padding)
	}
	return matchaPackageMeteringPrefix + encoded
}

func sortStringsEqual(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	sorted := append([]string(nil), expected...)
	sort.Strings(sorted)
	for i := range sorted {
		if sorted[i] != actual[i] {
			return false
		}
	}
	return true
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
