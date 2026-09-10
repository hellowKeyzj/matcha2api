package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func sealedTestCloudKey(t testing.TB) *SealedPackageCloudKey {
	t.Helper()
	privatePEM, err := os.ReadFile("testdata/sealed-v1/cloud-test.pem")
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewSealedPackageCloudKey("cloud-test", string(privatePEM))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func sealedTestResolver(key *SealedPackageCloudKey) SealedPackageCloudKeyResolver {
	return func(string) (*SealedPackageCloudKey, error) { return key, nil }
}

func TestInspectSealedPackageUploadAcceptsSealedPackageV1(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		raw         []byte
		kind        string
		resourceKey string
		displayName string
		description string
	}{
		{
			name:        "agent",
			filename:    "writer.matcha-agentpkg",
			raw:         sealedAgentPackageFixture(t, "writer", "openclaw"),
			kind:        "agent",
			resourceKey: "writer",
			displayName: "writer",
		},
		{
			name:        "skill",
			filename:    "calendar.matcha-skillpkg",
			raw:         sealedSkillPackageFixture(t, "calendar", "openclaw", false),
			kind:        "skill",
			resourceKey: "calendar",
			displayName: "Calendar",
			description: "Calendar skill",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := InspectSealedPackageUpload(tc.filename, tc.raw, sealedTestResolver(sealedTestCloudKey(t)))
			if err != nil {
				t.Fatalf("InspectSealedPackageUpload returned error: %v", err)
			}
			if meta.Kind != tc.kind || meta.Runtime != "openclaw" || meta.ResourceKey != tc.resourceKey || meta.DisplayName != tc.displayName {
				t.Fatalf("unexpected metadata: %+v", meta)
			}
			if meta.PackageKey != tc.kind+":openclaw:"+tc.resourceKey {
				t.Fatalf("unexpected package key: %q", meta.PackageKey)
			}
			if meta.Description != tc.description {
				t.Fatalf("unexpected description: %q", meta.Description)
			}
		})
	}
}

func TestInspectSealedPackageUploadRejectsLegacyV1WithoutCloudEnvelope(t *testing.T) {
	manifest := sealedSkillPackageSignedManifest{
		SkillKey:   "calendar",
		Target:     "openclaw",
		Descriptor: sealedPackageDescriptor{Name: "Calendar", Description: "Calendar skill"},
		Files:      []sealedPackageManifestFile{{Path: "SKILL.md", SizeBytes: 5, SHA256: strings.Repeat("a", 64)}},
		SHA256:     "",
	}
	digest := sha256JSONHex(manifest)
	manifest.SHA256 = digest
	raw := mustMarshalJSON(t, map[string]any{"format": "matcha-skillpkg", "version": 1, "skillKey": "calendar", "target": "openclaw", "manifestSha256": digest, "manifest": manifest, "payload": sealedPayloadFixture()})
	_, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if !errors.Is(err, ErrMatchaPackageCloudEnvelopeRequired) {
		t.Fatalf("expected cloud envelope error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsManifestDigestMismatch(t *testing.T) {
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	raw = []byte(strings.Replace(string(raw), "Calendar skill", "Tampered", 1))
	_, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if err == nil || !strings.Contains(err.Error(), "manifest 摘要不一致") {
		t.Fatalf("expected manifest digest error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsInvalidPayloadBase64(t *testing.T) {
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	raw = []byte(strings.Replace(string(raw), "\"nonceBase64\":\"bm9uY2UxMjM0NTY3\"", "\"nonceBase64\":\"nonce=\"", 1))
	_, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if err == nil || !strings.Contains(err.Error(), "payload 无效") {
		t.Fatalf("expected payload error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsLegacyRuntimeTarget(t *testing.T) {
	manifest := sealedAgentPackageLegacySignedManifest{AgentKey: "writer", RuntimeTarget: "openclaw", Files: []sealedPackageManifestFile{{Path: "AGENTS.md", SizeBytes: 5, SHA256: strings.Repeat("b", 64)}}, SHA256: ""}
	digest := sha256JSONHex(manifest)
	manifest.SHA256 = digest
	raw := mustMarshalJSON(t, map[string]any{"format": "matcha-agentpkg", "version": 1, "manifestSha256": digest, "manifest": manifest, "payload": sealedPayloadFixture()})
	_, err := InspectSealedPackageUpload("writer.matcha-agentpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if err == nil || !strings.Contains(err.Error(), "文件格式无效") {
		t.Fatalf("expected missing Rust target field error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsSkillTopLevelMismatch(t *testing.T) {
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	raw = []byte(strings.Replace(string(raw), "\"skillKey\":\"calendar\"", "\"skillKey\":\"other\"", 1))
	_, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if err == nil || !strings.Contains(err.Error(), "manifest 摘要不一致") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsManifestFileRules(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{
			name: "agent disallows non bootstrap path",
			raw: sealedAgentPackageFixtureWithFiles(t, "writer", []sealedPackageManifestFile{
				{Path: "AGENTS.md", SizeBytes: 5, SHA256: strings.Repeat("b", 64)},
				{Path: "tools/run.sh", SizeBytes: 5, SHA256: strings.Repeat("c", 64)},
			}),
		},
		{
			name: "agent requires AGENTS",
			raw:  sealedAgentPackageFixtureWithFiles(t, "writer", []sealedPackageManifestFile{{Path: "SOUL.md", SizeBytes: 5, SHA256: strings.Repeat("b", 64)}}),
		},
		{
			name: "skill requires SKILL",
			raw:  sealedSkillPackageFixtureWithFiles(t, "calendar", []sealedPackageManifestFile{{Path: "README.md", SizeBytes: 5, SHA256: strings.Repeat("a", 64)}}),
		},
		{
			name: "rejects traversal path",
			raw:  sealedSkillPackageFixtureWithFiles(t, "calendar", []sealedPackageManifestFile{{Path: "SKILL.md", SizeBytes: 5, SHA256: strings.Repeat("a", 64)}, {Path: "../secret", SizeBytes: 5, SHA256: strings.Repeat("b", 64)}}),
		},
		{
			name: "rejects bad file sha",
			raw:  sealedSkillPackageFixtureWithFiles(t, "calendar", []sealedPackageManifestFile{{Path: "SKILL.md", SizeBytes: 5, SHA256: strings.Repeat("A", 64)}}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := InspectSealedPackageUpload("pkg."+strings.TrimPrefix(strings.Split(tc.name, " ")[0], "")+".matcha-skillpkg", tc.raw, sealedTestResolver(sealedTestCloudKey(t)))
			if strings.HasPrefix(tc.name, "agent") {
				_, err = InspectSealedPackageUpload("writer.matcha-agentpkg", tc.raw, sealedTestResolver(sealedTestCloudKey(t)))
			}
			if err == nil {
				t.Fatalf("expected manifest file validation error")
			}
		})
	}
}

func TestInspectSealedPackageUploadRejectsInvalidDescriptor(t *testing.T) {
	raw := sealedSkillPackageFixtureWithDescriptor(t, "calendar", sealedPackageDescriptor{Name: " ", Description: "Calendar skill"})
	_, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", raw, sealedTestResolver(sealedTestCloudKey(t)))
	if err == nil || !strings.Contains(err.Error(), "descriptor 无效") {
		t.Fatalf("expected descriptor error, got %v", err)
	}
}

func TestInspectSealedPackageUploadRejectsOversizedPackage(t *testing.T) {
	cases := []struct {
		filename string
		raw      []byte
		message  string
	}{
		{"writer.matcha-agentpkg", make([]byte, SealedAgentPackageObjectMaxBytes+1), "超过 256KB"},
		{"calendar.matcha-skillpkg", make([]byte, SealedSkillPackageObjectMaxBytes+1), "超过 10MB"},
	}
	for _, tc := range cases {
		_, err := InspectSealedPackageUpload(tc.filename, tc.raw, sealedTestResolver(sealedTestCloudKey(t)))
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("expected package size error containing %q, got %v", tc.message, err)
		}
	}
}

type sealedAgentPackageLegacySignedManifest struct {
	AgentKey      string                      `json:"agentKey"`
	RuntimeTarget string                      `json:"runtimeTarget"`
	Files         []sealedPackageManifestFile `json:"files"`
	SHA256        string                      `json:"sha256"`
}

func sealedSkillPackageFixture(t *testing.T, key, runtime string, includeOptionalFalse bool) []byte {
	t.Helper()
	var userInvocable *bool
	if includeOptionalFalse {
		v := false
		userInvocable = &v
	}
	return sealedSkillPackageFixtureWithDescriptor(t, key, sealedPackageDescriptor{
		Name:          "Calendar",
		Description:   "Calendar skill",
		UserInvocable: userInvocable,
	})
}

func sealedSkillPackageFixtureWithDescriptor(t *testing.T, key string, descriptor sealedPackageDescriptor) []byte {
	t.Helper()
	return sealedSkillPackageFixtureWithFilesAndDescriptor(t, key, []sealedPackageManifestFile{{Path: "SKILL.md", SizeBytes: 5, SHA256: sealedTestSHA256([]byte("hello"))}}, descriptor)
}

func sealedSkillPackageFixtureWithFiles(t *testing.T, key string, files []sealedPackageManifestFile) []byte {
	t.Helper()
	return sealedSkillPackageFixtureWithFilesAndDescriptor(t, key, files, sealedPackageDescriptor{Name: "Calendar", Description: "Calendar skill"})
}

func sealedSkillPackageFixtureWithFilesAndDescriptor(t *testing.T, key string, files []sealedPackageManifestFile, descriptor sealedPackageDescriptor) []byte {
	t.Helper()
	manifest := sealedSkillPackageSignedManifest{
		SkillKey:   key,
		Target:     "openclaw",
		Descriptor: descriptor,
		Files:      files,
		SHA256:     "",
	}
	digest := sha256JSONHex(manifest)
	manifest.SHA256 = digest
	return mustMarshalJSON(t, map[string]any{
		"format":         "matcha-skillpkg",
		"version":        1,
		"skillKey":       key,
		"target":         "openclaw",
		"manifestSha256": digest,
		"manifest":       manifest,
		"payload":        sealedEncryptedPayloadFixture(t, sealedSkillPackageFormat, digest, files),
		"cloudEnvelope":  sealedCloudEnvelopeFixture(t, "cloud-test"),
	})
}

func sealedAgentPackageFixture(t *testing.T, key, runtime string) []byte {
	t.Helper()
	return sealedAgentPackageFixtureWithFiles(t, key, []sealedPackageManifestFile{{Path: "AGENTS.md", SizeBytes: 5, SHA256: sealedTestSHA256([]byte("hello"))}})
}

func sealedAgentPackageFixtureWithFiles(t *testing.T, key string, files []sealedPackageManifestFile) []byte {
	t.Helper()
	manifest := sealedAgentPackageSignedManifest{AgentKey: key, Target: "openclaw", Files: files, SHA256: ""}
	digest := sha256JSONHex(manifest)
	manifest.SHA256 = digest
	return mustMarshalJSON(t, map[string]any{"format": "matcha-agentpkg", "version": 1, "manifestSha256": digest, "manifest": manifest, "payload": sealedEncryptedPayloadFixture(t, sealedAgentPackageFormat, digest, files), "cloudEnvelope": sealedCloudEnvelopeFixture(t, "cloud-test")})
}

func sealedTestSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func sealedEncryptedPayloadFixture(t *testing.T, format, digest string, files []sealedPackageManifestFile) sealedPackagePayload {
	t.Helper()
	entries := make([]map[string]string, 0, len(files))
	for _, file := range files {
		entries = append(entries, map[string]string{"path": file.Path, "dataBase64": base64.StdEncoding.EncodeToString([]byte("hello"))})
	}
	return sealedEncryptPlaintext(t, format, digest, mustMarshalJSON(t, entries))
}

func sealedEncryptPlaintext(t *testing.T, format, digest string, plaintext []byte) sealedPackagePayload {
	t.Helper()
	block, err := aes.NewCipher([]byte(strings.Repeat("k", sealedPackageContentKeyBytes)))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("nonce1234567")
	// Deliberately independent of the production AAD serializer.
	aad := []byte(`{"format":"` + format + `","version":1,"manifestSha256":"` + digest + `"}`)
	return sealedPackagePayload{
		Algorithm:        sealedPackagePayloadAlgo,
		NonceBase64:      base64.RawURLEncoding.EncodeToString(nonce),
		CiphertextBase64: base64.RawURLEncoding.EncodeToString(gcm.Seal(nil, nonce, plaintext, aad)),
	}
}

func sealedPayloadFixture() sealedPackagePayload {
	return sealedPackagePayload{
		Algorithm:        "aes-256-gcm",
		NonceBase64:      base64.RawURLEncoding.EncodeToString([]byte("nonce1234567")),
		CiphertextBase64: base64.RawURLEncoding.EncodeToString([]byte("ciphertext1234567890")),
	}
}

func sealedCloudEnvelopeFixture(t *testing.T, keyID string) SealedPackageEnvelope {
	t.Helper()
	key := sealedTestCloudKey(t).PrivateKey
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, []byte(strings.Repeat("k", sealedPackageContentKeyBytes)), nil)
	if err != nil {
		t.Fatal(err)
	}
	return SealedPackageEnvelope{KeyID: keyID, Algorithm: sealedPackageEnvelopeAlgo, CiphertextBase64: base64.RawURLEncoding.EncodeToString(ciphertext)}
}

// Generated by the actual Matcha-claw package/{common,agent,skill}.rs in the
// standalone Rust harness, then opened there before writing. The committed PEM
// is test-only. Fixtures cover serde HTML/line-separator escaping, literal
// backslash-u sequences, optional false fields, empty files and binary content.
func TestInspectSealedPackageUploadRustV1Fixtures(t *testing.T) {
	key := sealedTestCloudKey(t)
	key.KeyID = "rust-fixture"
	for _, tc := range []struct {
		filename, kind, resourceKey, manifestSHA256 string
	}{
		{"agent.matcha-agentpkg", "agent", "writer<>& \\u2028", "9b338b0c7f05d227a7da378af5933f124596bdea12c65a9f0997bf8eb8d11bd0"},
		{"skill.matcha-skillpkg", "skill", "calendar<>& \\u2029", "d783466be67a9c6b20361261acd9f8be738b38008b1e35d354edc0853e575046"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/sealed-v1/" + tc.filename)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := InspectSealedPackageUpload(tc.filename, raw, sealedTestResolver(key))
			if err != nil {
				t.Fatal(err)
			}
			if meta.Kind != tc.kind || meta.ResourceKey != tc.resourceKey || meta.PackageSHA256 != sealedTestSHA256(raw) || meta.PackageVersion != meta.PackageSHA256 {
				t.Fatalf("unexpected Rust fixture metadata: %+v", meta)
			}
			if tc.manifestSHA256 != "" && meta.ManifestSHA256 != tc.manifestSHA256 {
				t.Fatalf("serde digest mismatch: %s", meta.ManifestSHA256)
			}
		})
	}
}

func TestInspectSealedPackageUploadRejectsEnvelopeAndPayloadTampering(t *testing.T) {
	key := sealedTestCloudKey(t)
	key.KeyID = "rust-fixture"
	skillBytes, err := os.ReadFile("testdata/sealed-v1/skill.matcha-skillpkg")
	if err != nil {
		t.Fatal(err)
	}
	agentBytes, err := os.ReadFile("testdata/sealed-v1/agent.matcha-agentpkg")
	if err != nil {
		t.Fatal(err)
	}
	var agent sealedAgentPackageWire
	if err := json.Unmarshal(agentBytes, &agent); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*sealedSkillPackageWire)
	}{
		{"transplanted envelope with same key ID", func(w *sealedSkillPackageWire) { w.CloudEnvelope = agent.CloudEnvelope }},
		{"random envelope with same key ID", func(w *sealedSkillPackageWire) {
			w.CloudEnvelope.CiphertextBase64 = base64.RawURLEncoding.EncodeToString(make([]byte, key.PrivateKey.Size()))
		}},
		{"wrong key ID", func(w *sealedSkillPackageWire) { w.CloudEnvelope.KeyID = "other" }},
		{"random payload", func(w *sealedSkillPackageWire) {
			w.Payload.CiphertextBase64 = base64.RawURLEncoding.EncodeToString(make([]byte, 128))
		}},
		{"nonce tampering", func(w *sealedSkillPackageWire) {
			w.Payload.NonceBase64 = base64.RawURLEncoding.EncodeToString(make([]byte, 12))
		}},
		{"manifest changed with valid new digest", func(w *sealedSkillPackageWire) {
			var manifest sealedSkillPackageManifest
			if err := json.Unmarshal(w.Manifest, &manifest); err != nil {
				t.Fatal(err)
			}
			manifest.Descriptor.Name = "Different manifest"
			manifest.SHA256 = signedSkillManifestDigest(manifest)
			w.ManifestSHA256 = manifest.SHA256
			w.Manifest = mustMarshalJSON(t, manifest)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wire sealedSkillPackageWire
			if err := json.Unmarshal(skillBytes, &wire); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&wire)
			if _, err := InspectSealedPackageUpload("skill.matcha-skillpkg", mustMarshalJSON(t, wire), sealedTestResolver(key)); err == nil {
				t.Fatal("tampered package accepted")
			}
		})
	}
}

func TestInspectSealedPackageUploadRejectsAuthenticatedInvalidFiles(t *testing.T) {
	var wire sealedSkillPackageWire
	if err := json.Unmarshal(sealedSkillPackageFixture(t, "calendar", "openclaw", false), &wire); err != nil {
		t.Fatal(err)
	}
	for _, plaintext := range []string{
		`null`, `[]`, `{}`, `not-json`,
		`[null]`,
		`[{"dataBase64":"aGVsbG8="}]`,
		`[{"path":null,"dataBase64":"aGVsbG8="}]`,
		`[{"Path":"SKILL.md","dataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md","DataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md","path":"SKILL.md","dataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md","dataBase64":"wrong","dataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8=","dataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md"}]`,
		`[{"path":"SKILL.md","dataBase64":null}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8"}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8_"}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8=\n"}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8=\r"}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8=\r\n"}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbA=="}]`,
		`[{"path":"SKILL.md","dataBase64":"d29ybGQ="}]`,
		`[{"path":"../SKILL.md","dataBase64":"aGVsbG8="}]`,
		`[{"path":"OTHER.md","dataBase64":"aGVsbG8="}]`,
		`[{"path":"SKILL.md","dataBase64":"aGVsbG8="},{"path":"SKILL.md","dataBase64":"aGVsbG8="}]`,
	} {
		t.Run(plaintext, func(t *testing.T) {
			wire.Payload = sealedEncryptPlaintext(t, wire.Format, wire.ManifestSHA256, []byte(plaintext))
			if _, err := InspectSealedPackageUpload("skill.matcha-skillpkg", mustMarshalJSON(t, wire), sealedTestResolver(sealedTestCloudKey(t))); err == nil {
				t.Fatal("authenticated invalid files accepted")
			}
		})
	}
	// A duplicate replaces a second manifest file without changing the entry count.
	var manifest sealedSkillPackageManifest
	if err := json.Unmarshal(wire.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Files = append(manifest.Files, sealedPackageManifestFile{Path: "assets/other", SizeBytes: 5, SHA256: sealedTestSHA256([]byte("hello"))})
	manifest.SHA256 = signedSkillManifestDigest(manifest)
	wire.ManifestSHA256, wire.Manifest = manifest.SHA256, mustMarshalJSON(t, manifest)
	wire.Payload = sealedEncryptPlaintext(t, wire.Format, wire.ManifestSHA256, []byte(`[{"path":"SKILL.md","dataBase64":"aGVsbG8="},{"path":"SKILL.md","dataBase64":"aGVsbG8="}]`))
	if _, err := InspectSealedPackageUpload("skill.matcha-skillpkg", mustMarshalJSON(t, wire), sealedTestResolver(sealedTestCloudKey(t))); err == nil {
		t.Fatal("duplicate paths accepted")
	}
}

func TestInspectSealedPackageUploadRejectsLocalEnvelope(t *testing.T) {
	var wire sealedSkillPackageWire
	if err := json.Unmarshal(sealedSkillPackageFixture(t, "calendar", "openclaw", false), &wire); err != nil {
		t.Fatal(err)
	}
	wire.CloudEnvelope = SealedPackageEnvelope{KeyID: "local", Algorithm: "local"}
	if _, err := InspectSealedPackageUpload("skill.matcha-skillpkg", mustMarshalJSON(t, wire), nil); !errors.Is(err, ErrMatchaPackageCloudEnvelopeRequired) {
		t.Fatalf("expected structured legacy error, got %v", err)
	}
}

func TestInspectSealedPackageUploadResolverBoundary(t *testing.T) {
	key := sealedTestCloudKey(t)
	for _, tc := range []struct {
		filename string
		raw      []byte
	}{
		{"writer.matcha-agentpkg", sealedAgentPackageFixture(t, "writer", "openclaw")},
		{"calendar.matcha-skillpkg", sealedSkillPackageFixture(t, "calendar", "openclaw", false)},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			calls := 0
			resolve := func(keyID string) (*SealedPackageCloudKey, error) {
				calls++
				if keyID != key.KeyID {
					t.Fatalf("resolved wrong key ID %q", keyID)
				}
				return key, nil
			}
			if _, err := InspectSealedPackageUpload(tc.filename, tc.raw, resolve); err != nil || calls != 1 {
				t.Fatalf("valid package: calls=%d, error=%v", calls, err)
			}
			want := errors.New("resolver failure")
			calls = 0
			_, err := InspectSealedPackageUpload(tc.filename, tc.raw, func(string) (*SealedPackageCloudKey, error) {
				calls++
				return nil, want
			})
			if err != want || calls != 1 {
				t.Fatalf("resolver error must be unchanged: calls=%d, error=%v", calls, err)
			}
			for _, raw := range [][]byte{
				[]byte(`{"format":`),
				bytes.Replace(tc.raw, []byte(`"format":"matcha-`), []byte(`"format":"wrong-`), 1),
				bytes.Replace(tc.raw, []byte(`"sha256":"`), []byte(`"sha256":"bad`), 1),
				bytes.Replace(tc.raw, []byte(`"nonceBase64":"`), []byte(`"nonceBase64":"\n`), 1),
				bytes.Replace(tc.raw, []byte(`"keyId":"cloud-test"`), []byte(`"keyId":""`), 1),
			} {
				calls = 0
				if _, err := InspectSealedPackageUpload(tc.filename, raw, resolve); err == nil || calls != 0 {
					t.Fatalf("invalid structure resolved key: calls=%d, error=%v", calls, err)
				}
			}
		})
	}
}

func TestSealedPackageObjectsMatchSerdeFields(t *testing.T) {
	// All required fields are present, including legitimately zero-sized files.
	for _, tc := range []struct {
		name   string
		raw    string
		fields []string
		new    func() any
	}{
		{"agent wire", `{"format":"matcha-agentpkg","version":1,"manifestSha256":"x","manifest":{},"payload":{"algorithm":"x","nonceBase64":"x","ciphertextBase64":"x"},"cloudEnvelope":{"keyId":"x","algorithm":"x","ciphertextBase64":"x"}}`, []string{"format", "version", "manifestSha256", "manifest", "payload", "cloudEnvelope"}, func() any { return &sealedAgentPackageWire{} }},
		{"skill wire", `{"format":"matcha-skillpkg","version":1,"skillKey":"x","target":"openclaw","manifestSha256":"x","manifest":{},"payload":{"algorithm":"x","nonceBase64":"x","ciphertextBase64":"x"},"cloudEnvelope":{"keyId":"x","algorithm":"x","ciphertextBase64":"x"}}`, []string{"format", "version", "manifestSha256", "manifest", "payload", "cloudEnvelope"}, func() any { return &sealedSkillPackageWire{} }},
		{"agent manifest", `{"agentKey":"writer","target":"openclaw","files":[],"sha256":"x"}`, []string{"agentKey", "target", "files", "sha256"}, func() any { return &sealedAgentPackageManifest{} }},
		{"skill manifest", `{"skillKey":"calendar","target":"openclaw","descriptor":{"name":"Calendar","description":"Calendar skill"},"files":[],"sha256":"x"}`, []string{"skillKey", "target", "descriptor", "files", "sha256"}, func() any { return &sealedSkillPackageManifest{} }},
		{"payload", `{"algorithm":"aes-256-gcm","nonceBase64":"x","ciphertextBase64":"x"}`, []string{"algorithm", "nonceBase64", "ciphertextBase64"}, func() any { return &sealedPackagePayload{} }},
		{"envelope", `{"keyId":"cloud-test","algorithm":"rsa-oaep-sha256","ciphertextBase64":"x"}`, []string{"keyId", "algorithm", "ciphertextBase64"}, func() any { return &SealedPackageEnvelope{} }},
		{"descriptor", `{"name":"Calendar","description":"Calendar skill"}`, []string{"name", "description"}, func() any { return &sealedPackageDescriptor{} }},
		{"manifest file", `{"path":"SKILL.md","sizeBytes":0,"sha256":"x"}`, []string{"path", "sizeBytes", "sha256"}, func() any { return &sealedPackageManifestFile{} }},
		{"entry", `{"path":"SKILL.md","dataBase64":""}`, []string{"path", "dataBase64"}, func() any { return &sealedPackagePayloadEntry{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tc.raw), tc.new()); err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tc.raw), &fields); err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.fields {
				t.Run(name, func(t *testing.T) {
					// Duplicate escaped names resolve to the same serde field too.
					for _, spelling := range []string{name, `\u00` + hex.EncodeToString([]byte(name[:1])) + name[1:]} {
						raw := tc.raw[:len(tc.raw)-1] + `,"` + spelling + `":` + string(fields[name]) + `}`
						if err := json.Unmarshal([]byte(raw), tc.new()); err == nil {
							t.Fatalf("accepted duplicate %s", spelling)
						}
					}
					for _, mutation := range []string{"missing", "null", "case"} {
						var changed map[string]json.RawMessage
						if err := json.Unmarshal([]byte(tc.raw), &changed); err != nil {
							t.Fatal(err)
						}
						switch mutation {
						case "missing":
							delete(changed, name)
						case "null":
							changed[name] = json.RawMessage("null")
						case "case":
							changed[strings.ToUpper(name)] = changed[name]
							delete(changed, name)
						}
						// A missing envelope is the one intentional legacy upload exception.
						if name == "cloudEnvelope" && mutation != "null" {
							continue
						}
						if err := json.Unmarshal(mustMarshalJSON(t, changed), tc.new()); err == nil {
							t.Fatalf("accepted %s %s", mutation, name)
						}
					}
				})
			}
			unknown := tc.raw[:len(tc.raw)-1] + `,"unknown":{"nested":[null,{"path":false}]},"unknown":0}`
			if err := json.Unmarshal([]byte(unknown), tc.new()); err != nil {
				t.Fatalf("Rust ignores unknown fields (including duplicates): %v", err)
			}
		})
	}
}

func TestInspectSealedPackageUploadUnknownAndOptionalFields(t *testing.T) {
	key := sealedTestCloudKey(t)
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	for _, extra := range []string{
		`,"unknown":{"nested":[null,{"name":false}]}`,
		`,"userInvocable":null,"disableModelInvocation":null`,
		`,"USERINVOCABLE":true`,
	} {
		changed := bytes.Replace(raw, []byte(`"description":"Calendar skill"`), []byte(`"description":"Calendar skill"`+extra), 1)
		if _, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", changed, sealedTestResolver(key)); err != nil {
			t.Fatalf("optional/unknown descriptor field changed digest: %v", err)
		}
	}
	// Some(false) must remain in the unsigned manifest, unlike None/null.
	if _, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", sealedSkillPackageFixture(t, "calendar", "openclaw", true), sealedTestResolver(key)); err != nil {
		t.Fatal(err)
	}
	var wire sealedSkillPackageWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	wire.Payload = sealedEncryptPlaintext(t, wire.Format, wire.ManifestSHA256, []byte(`[{"path":"SKILL.md","dataBase64":"aGVsbG8=","unknown":null,"unknown":{},"Path":"ignored"}]`))
	if _, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", mustMarshalJSON(t, wire), sealedTestResolver(key)); err != nil {
		t.Fatalf("unknown entry fields rejected: %v", err)
	}
	for _, field := range []string{"userInvocable", "disableModelInvocation"} {
		for _, value := range []string{"null", "false", "true"} {
			var descriptor sealedPackageDescriptor
			data := `{"name":"Calendar","description":"Calendar skill","` + field + `":` + value + `}`
			if err := json.Unmarshal([]byte(data), &descriptor); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(data[:len(data)-1]+`,"`+field+`":null}`), &descriptor); err == nil {
				t.Fatalf("accepted duplicate optional %s after %s", field, value)
			}
		}
	}
	// The agent wire shares Rust's optional top-level String fields, even though
	// agent open_wire does not use their values.
	agent := sealedAgentPackageFixture(t, "writer", "openclaw")
	for _, extra := range []string{`,"skillKey":null,"target":null`, `,"skillKey":"ignored","target":"ignored"`} {
		changed := append(append([]byte{}, agent[:len(agent)-1]...), []byte(extra+`}`)...)
		if _, err := InspectSealedPackageUpload("writer.matcha-agentpkg", changed, sealedTestResolver(key)); err != nil {
			t.Fatal(err)
		}
	}
	for _, extra := range []string{`,"skillKey":false`, `,"target":42`, `,"skillKey":null,"skillKey":null`, `,"target":null,"target":"ignored"`} {
		changed := append(append([]byte{}, agent[:len(agent)-1]...), []byte(extra+`}`)...)
		if _, err := InspectSealedPackageUpload("writer.matcha-agentpkg", changed, sealedTestResolver(key)); err == nil {
			t.Fatalf("accepted invalid agent optional field %s", extra)
		}
	}
}

func TestInspectSealedPackageUploadUnicodeMatchesSerde(t *testing.T) {
	key := sealedTestCloudKey(t)
	key.KeyID = "rust-fixture"
	agent, err := os.ReadFile("testdata/sealed-v1/agent.matcha-agentpkg")
	if err != nil {
		t.Fatal(err)
	}
	agent = bytes.TrimSpace(agent)
	for _, tc := range []struct {
		name, extra string
		accepted    bool
	}{
		{"lone high surrogate", `,"skillKey":"\ud800"`, false},
		{"lone low surrogate", `,"target":"\udfff"`, false},
		{"wrong pair", `,"skillKey":"\ud800A"`, false},
		{"two high surrogates", `,"skillKey":"\ud800\ud800"`, false},
		{"invalid UTF8 value", ",\"skillKey\":\"\xff\"", false},
		{"UTF8 encoded surrogate", ",\"target\":\"\xed\xa0\x80\"", false},
		{"invalid UTF8 key", ",\"\xff\":null", false},
		{"lone surrogate key", `,"\ud800":null`, false},
		{"replacement literal", `,"skillKey":"�"`, true},
		{"replacement escape", `,"skillKey":"` + `\` + `ufffd"`, true},
		{"valid pair", `,"target":"` + `\` + `ud83d` + `\` + `ude00"`, true},
		{"replacement and valid pair", `,"skillKey":"�` + `\` + `ud83d` + `\` + `ude00"`, true},
		{"escaped backslash", `,"skillKey":"�\\ud800"`, true},
		{"replacement key", `,"�":null`, true},
		{"valid pair key", `,"😀":null`, true},
		// serde's IgnoredAny uses ignore_str/ignore_escape: neither validates
		// Unicode in ignored values or their nested object keys.
		{"unknown lone surrogate", `,"unknown":"\ud800"`, true},
		{"unknown invalid UTF8", ",\"unknown\":\"\xff\"", true},
		{"unknown nested key", `,"unknown":{"\ud800":"\udfff"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := append(append([]byte{}, agent[:len(agent)-1]...), []byte(tc.extra+`}`)...)
			calls := 0
			_, err := InspectSealedPackageUpload("agent.matcha-agentpkg", raw, func(string) (*SealedPackageCloudKey, error) {
				calls++
				return key, nil
			})
			if (err == nil) != tc.accepted || (!tc.accepted && calls != 0) {
				t.Fatalf("accepted=%v, calls=%d, error=%v", tc.accepted, calls, err)
			}
		})
	}
	// A valid signed replacement-character manifest must not authenticate a
	// malformed string merely because Go decodes it to the same U+FFFD.
	signedReplacement := sealedAgentPackageFixture(t, "writer�", "openclaw")
	for _, malformed := range [][]byte{[]byte(`\ud800`), {0xff}} {
		raw := bytes.Replace(signedReplacement, []byte("�"), malformed, 1)
		if _, err := InspectSealedPackageUpload("agent.matcha-agentpkg", raw, sealedTestResolver(sealedTestCloudKey(t))); err == nil {
			t.Fatal("malformed Unicode authenticated a replacement-character manifest")
		}
	}
	// Known fields in nested descriptor/entry objects must reject the same
	// malformed strings, even when replacement would match a signed digest.
	for _, malformed := range []string{`"\ud800"`, `"\udfff"`, "\"\xff\""} {
		var descriptor sealedPackageDescriptor
		if err := json.Unmarshal([]byte(`{"name":`+malformed+`,"description":"description"}`), &descriptor); err == nil {
			t.Fatalf("accepted malformed descriptor %s", malformed)
		}
		var entry sealedPackagePayloadEntry
		if err := json.Unmarshal([]byte(`{"path":`+malformed+`,"dataBase64":""}`), &entry); err == nil {
			t.Fatalf("accepted malformed entry %s", malformed)
		}
	}
}

func TestInspectSealedPackageUploadRejectsNoncanonicalBase64(t *testing.T) {
	key := sealedTestCloudKey(t)
	var original sealedSkillPackageWire
	if err := json.Unmarshal(sealedSkillPackageFixture(t, "calendar", "openclaw", false), &original); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"nonce", "payload", "envelope"} {
		for _, suffix := range []string{"\r", "\n", "\r\n", "="} {
			t.Run(field+suffix, func(t *testing.T) {
				wire := original
				switch field {
				case "nonce":
					wire.Payload.NonceBase64 += suffix
				case "payload":
					wire.Payload.CiphertextBase64 += suffix
				case "envelope":
					wire.CloudEnvelope.CiphertextBase64 += suffix
					if validSealedPackageEnvelope(wire.CloudEnvelope) {
						t.Fatal("noncanonical envelope validated")
					}
					if _, err := UnwrapSealedPackageCloudEnvelope(key, wire.CloudEnvelope); err == nil {
						t.Fatal("noncanonical envelope unwrapped")
					}
				}
				calls := 0
				if _, err := InspectSealedPackageUpload("calendar.matcha-skillpkg", mustMarshalJSON(t, wire), func(string) (*SealedPackageCloudKey, error) {
					calls++
					return key, nil
				}); err == nil || calls != 0 {
					t.Fatalf("noncanonical base64: calls=%d, error=%v", calls, err)
				}
			})
		}
	}
	for _, value := range []string{"Zh", "Zm9", "aGVsbG9="} {
		encoding := base64.RawURLEncoding
		if strings.HasSuffix(value, "=") {
			encoding = base64.StdEncoding
		}
		if _, err := decodeSealedPackageBase64(encoding, value); err == nil {
			t.Fatalf("accepted nonzero trailing bits %q", value)
		}
	}
}

func BenchmarkInspectSealedPackageUploadLargePayload(b *testing.B) {
	key := sealedTestCloudKey(b)
	content := bytes.Repeat([]byte("x"), sealedSkillPackageMaxFileBytes)
	files := make([]sealedPackageManifestFile, 5)
	entries := make([]sealedPackagePayloadEntry, len(files))
	for i := range files {
		path := "assets/" + string(rune('a'+i))
		if i == 0 {
			path = "SKILL.md"
		}
		files[i] = sealedPackageManifestFile{Path: path, SizeBytes: int64(len(content)), SHA256: sealedTestSHA256(content)}
		entries[i] = sealedPackagePayloadEntry{Path: path, DataBase64: base64.StdEncoding.EncodeToString(content)}
	}
	manifest := sealedSkillPackageManifest{SkillKey: "benchmark", Target: "openclaw", Descriptor: sealedPackageDescriptor{Name: "Benchmark", Description: "Large payload"}, Files: files}
	manifest.SHA256 = signedSkillManifestDigest(manifest)
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		b.Fatal(err)
	}
	plaintext, err := json.Marshal(entries)
	if err != nil {
		b.Fatal(err)
	}
	block, err := aes.NewCipher([]byte(strings.Repeat("k", sealedPackageContentKeyBytes)))
	if err != nil {
		b.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		b.Fatal(err)
	}
	nonce := []byte("nonce1234567")
	aad := []byte(`{"format":"matcha-skillpkg","version":1,"manifestSha256":"` + manifest.SHA256 + `"}`)
	envelope, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PrivateKey.PublicKey, []byte(strings.Repeat("k", sealedPackageContentKeyBytes)), nil)
	if err != nil {
		b.Fatal(err)
	}
	wire := sealedSkillPackageWire{Format: sealedSkillPackageFormat, Version: 1, SkillKey: "benchmark", Target: "openclaw", ManifestSHA256: manifest.SHA256, Manifest: manifestJSON,
		Payload:       sealedPackagePayload{Algorithm: sealedPackagePayloadAlgo, NonceBase64: base64.RawURLEncoding.EncodeToString(nonce), CiphertextBase64: base64.RawURLEncoding.EncodeToString(gcm.Seal(nil, nonce, plaintext, aad))},
		CloudEnvelope: SealedPackageEnvelope{KeyID: key.KeyID, Algorithm: sealedPackageEnvelopeAlgo, CiphertextBase64: base64.RawURLEncoding.EncodeToString(envelope)}}
	raw, err := json.Marshal(wire)
	if err != nil {
		b.Fatal(err)
	}
	resolve := sealedTestResolver(key)
	if _, err := InspectSealedPackageUpload("benchmark.matcha-skillpkg", raw, resolve); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := InspectSealedPackageUpload("benchmark.matcha-skillpkg", raw, resolve); err != nil {
			b.Fatal(err)
		}
	}
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
