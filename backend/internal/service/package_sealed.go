package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrMatchaPackageCloudEnvelopeRequired = infraerrors.BadRequest(
	"MATCHA_PACKAGE_CLOUD_ENVELOPE_REQUIRED",
	"package has no cloud envelope; re-export it with the configured cloud key",
)

const (
	SealedAgentPackageObjectMaxBytes = 256 << 10
	SealedSkillPackageObjectMaxBytes = 10 << 20

	sealedAgentPackageFormat      = "matcha-agentpkg"
	sealedSkillPackageFormat      = "matcha-skillpkg"
	sealedPackageFormatVersion    = 1
	sealedPackageTarget           = "openclaw"
	sealedPackagePayloadAlgo      = "aes-256-gcm"
	sealedPackageEnvelopeAlgo     = "rsa-oaep-sha256"
	sealedPackageContentKeyBytes  = 32
	sealedPackageGCMNonceBytes    = 12
	sealedPackageGCMTagBytes      = 16
	sealedPackageMaxEnvelopeBytes = 8192

	sealedPackageMaxKeyBytes  = 4096
	sealedPackageMaxPathBytes = 240

	sealedAgentPackageMaxFiles      = 4
	sealedAgentPackageMaxFileBytes  = 48 << 10
	sealedAgentPackageMaxTotalBytes = 48 << 10

	sealedSkillPackageMaxFiles      = 512
	sealedSkillPackageMaxFileBytes  = 1 << 20
	sealedSkillPackageMaxTotalBytes = 5 << 20
	sealedSkillDescriptorNameBytes  = 256
	sealedSkillDescriptorDescBytes  = 8 << 10
)

var sealedAgentBootstrapFiles = map[string]struct{}{
	"AGENTS.md": {},
	"SOUL.md":   {},
	"USER.md":   {},
	"MEMORY.md": {},
}

type SealedPackageUploadMetadata struct {
	Kind           string
	Runtime        string
	ResourceKey    string
	PackageKey     string
	PackageVersion string
	PackageSHA256  string
	ManifestSHA256 string
	DisplayName    string
	Description    string
	CloudEnvelope  SealedPackageEnvelope
}

type SealedPackageCloudKeyResolver func(keyID string) (*SealedPackageCloudKey, error)

func InspectSealedPackageUpload(filename string, raw []byte, resolveKey SealedPackageCloudKeyResolver) (SealedPackageUploadMetadata, error) {
	if len(raw) == 0 {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件为空")
	}
	lower := strings.ToLower(strings.TrimSpace(filepath.Base(filename)))
	switch {
	case strings.HasSuffix(lower, ".matcha-agentpkg"):
		if len(raw) > SealedAgentPackageObjectMaxBytes {
			return SealedPackageUploadMetadata{}, fmt.Errorf("agent package 文件超过 256KB")
		}
		return inspectSealedAgentPackageV1(raw, resolveKey)
	case strings.HasSuffix(lower, ".matcha-skillpkg"):
		if len(raw) > SealedSkillPackageObjectMaxBytes {
			return SealedPackageUploadMetadata{}, fmt.Errorf("skill package 文件超过 10MB")
		}
		return inspectSealedSkillPackageV1(raw, resolveKey)
	default:
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 扩展名必须是 .matcha-skillpkg 或 .matcha-agentpkg")
	}
}

func inspectSealedAgentPackageV1(raw []byte, resolveKey SealedPackageCloudKeyResolver) (SealedPackageUploadMetadata, error) {
	var wire sealedAgentPackageWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	if wire.Format != sealedAgentPackageFormat || wire.Version != sealedPackageFormatVersion {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	var manifest sealedAgentPackageManifest
	if err := json.Unmarshal(wire.Manifest, &manifest); err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	if !validSealedPackageTarget(manifest.Target) {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package runtime 无效")
	}
	resourceKey, err := parseSealedPackageKey(manifest.AgentKey)
	if err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package manifest 缺少 target 或 key")
	}
	if err := validateSealedAgentManifestFiles(manifest.Files); err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	if err := validateSealedManifestDigest(wire.ManifestSHA256, manifest.SHA256, signedAgentManifestDigest(manifest)); err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	return finishSealedPackageUpload(raw, SealedPackageUploadMetadata{
		Kind:           model.MatchaPackageKindAgent,
		Runtime:        manifest.Target,
		ResourceKey:    resourceKey,
		ManifestSHA256: manifest.SHA256,
		DisplayName:    resourceKey,
		CloudEnvelope:  wire.CloudEnvelope,
	}, wire.Format, wire.Payload, manifest.Files, resolveKey)
}

func inspectSealedSkillPackageV1(raw []byte, resolveKey SealedPackageCloudKeyResolver) (SealedPackageUploadMetadata, error) {
	var wire sealedSkillPackageWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	if wire.Format != sealedSkillPackageFormat || wire.Version != sealedPackageFormatVersion {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	if !validSealedPackageTarget(wire.Target) {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package runtime 无效")
	}
	topKey, err := parseSealedPackageKey(wire.SkillKey)
	if err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package manifest 缺少 target 或 key")
	}
	var manifest sealedSkillPackageManifest
	if err := json.Unmarshal(wire.Manifest, &manifest); err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package 文件格式无效")
	}
	if manifest.SkillKey != topKey || manifest.Target != wire.Target {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package manifest 摘要不一致")
	}
	resourceKey := topKey
	if err := validateSealedSkillManifestFiles(manifest.Files); err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	displayName, description, err := cleanSealedSkillDescriptor(manifest.Descriptor)
	if err != nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package descriptor 无效")
	}
	if err := validateSealedManifestDigest(wire.ManifestSHA256, manifest.SHA256, signedSkillManifestDigest(manifest)); err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	return finishSealedPackageUpload(raw, SealedPackageUploadMetadata{
		Kind:           model.MatchaPackageKindSkill,
		Runtime:        wire.Target,
		ResourceKey:    resourceKey,
		ManifestSHA256: manifest.SHA256,
		DisplayName:    displayName,
		Description:    description,
		CloudEnvelope:  wire.CloudEnvelope,
	}, wire.Format, wire.Payload, manifest.Files, resolveKey)
}

func finishSealedPackageUpload(raw []byte, meta SealedPackageUploadMetadata, format string, payload sealedPackagePayload, files []sealedPackageManifestFile, resolveKey SealedPackageCloudKeyResolver) (SealedPackageUploadMetadata, error) {
	if meta.Runtime == "" || meta.ResourceKey == "" {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package manifest 缺少 target 或 key")
	}
	if meta.Runtime != sealedPackageTarget {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package runtime 无效")
	}
	meta.PackageKey = meta.Kind + ":" + meta.Runtime + ":" + meta.ResourceKey
	if len(meta.PackageKey) > 160 {
		return SealedPackageUploadMetadata{}, fmt.Errorf("package key 过长")
	}
	if meta.CloudEnvelope == (SealedPackageEnvelope{}) || meta.CloudEnvelope.Algorithm == "local" {
		return SealedPackageUploadMetadata{}, ErrMatchaPackageCloudEnvelopeRequired
	}
	nonce, ciphertext, err := decodeSealedPackagePayload(payload)
	if err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	envelopeCiphertext, err := decodeSealedPackageEnvelope(meta.CloudEnvelope)
	if err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	if resolveKey == nil {
		return SealedPackageUploadMetadata{}, fmt.Errorf("sealed package cloud key is not configured")
	}
	cloudKey, err := resolveKey(meta.CloudEnvelope.KeyID)
	if err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	contentKey, err := unwrapSealedPackageCloudEnvelope(cloudKey, meta.CloudEnvelope, envelopeCiphertext)
	if err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	defer clear(contentKey)
	if err := validateSealedPackagePayloadFiles(format, meta.ManifestSHA256, nonce, ciphertext, contentKey, files); err != nil {
		return SealedPackageUploadMetadata{}, err
	}
	sha := sha256.Sum256(raw)
	meta.PackageSHA256 = hex.EncodeToString(sha[:])
	meta.PackageVersion = meta.PackageSHA256
	return meta, nil
}

func validSealedPackageTarget(target string) bool {
	return target == sealedPackageTarget
}

func parseSealedPackageKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > sealedPackageMaxKeyBytes || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("package key 无效")
	}
	return value, nil
}

func validateSealedAgentManifestFiles(files []sealedPackageManifestFile) error {
	if len(files) == 0 || len(files) > sealedAgentPackageMaxFiles {
		return fmt.Errorf("package manifest 文件列表无效")
	}
	seen := make(map[string]struct{}, len(files))
	total := int64(0)
	hasRequired := false
	for _, file := range files {
		if _, ok := sealedAgentBootstrapFiles[file.Path]; !ok {
			return fmt.Errorf("package manifest 文件路径无效")
		}
		if !validSealedPackageRelativePath(file.Path) {
			return fmt.Errorf("package manifest 文件路径无效")
		}
		if _, ok := seen[file.Path]; ok {
			return fmt.Errorf("package manifest 文件列表无效")
		}
		seen[file.Path] = struct{}{}
		if file.Path == "AGENTS.md" {
			hasRequired = true
		}
		if file.SizeBytes < 0 || file.SizeBytes > sealedAgentPackageMaxFileBytes {
			return fmt.Errorf("package manifest 文件大小无效")
		}
		total += file.SizeBytes
		if total > sealedAgentPackageMaxTotalBytes {
			return fmt.Errorf("package manifest 文件大小无效")
		}
		if !validSealedPackageSHA256(file.SHA256) {
			return fmt.Errorf("package manifest 文件摘要无效")
		}
	}
	if !hasRequired {
		return fmt.Errorf("package manifest 文件列表无效")
	}
	return nil
}

func validateSealedSkillManifestFiles(files []sealedPackageManifestFile) error {
	if len(files) == 0 || len(files) > sealedSkillPackageMaxFiles {
		return fmt.Errorf("package manifest 文件列表无效")
	}
	seen := make(map[string]struct{}, len(files))
	total := int64(0)
	hasManifest := false
	for _, file := range files {
		if !validSealedPackageRelativePath(file.Path) {
			return fmt.Errorf("package manifest 文件路径无效")
		}
		if _, ok := seen[file.Path]; ok {
			return fmt.Errorf("package manifest 文件列表无效")
		}
		seen[file.Path] = struct{}{}
		if file.Path == "SKILL.md" {
			hasManifest = true
		}
		if file.SizeBytes < 0 || file.SizeBytes > sealedSkillPackageMaxFileBytes {
			return fmt.Errorf("package manifest 文件大小无效")
		}
		total += file.SizeBytes
		if total > sealedSkillPackageMaxTotalBytes {
			return fmt.Errorf("package manifest 文件大小无效")
		}
		if !validSealedPackageSHA256(file.SHA256) {
			return fmt.Errorf("package manifest 文件摘要无效")
		}
	}
	if !hasManifest {
		return fmt.Errorf("package manifest 文件列表无效")
	}
	return nil
}

func validSealedPackageRelativePath(path string) bool {
	if path == "" || len(path) > sealedPackageMaxPathBytes || strings.Contains(path, "\\") || strings.ContainsRune(path, '\x00') || strings.HasPrefix(path, "/") {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." || isSealedPackageWindowsDrive(component) {
			return false
		}
	}
	return true
}

func isSealedPackageWindowsDrive(component string) bool {
	if len(component) < 2 || component[1] != ':' {
		return false
	}
	c := component[0]
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func cleanSealedSkillDescriptor(descriptor sealedPackageDescriptor) (string, string, error) {
	name, ok := cleanSealedText(descriptor.Name, sealedSkillDescriptorNameBytes)
	if !ok {
		return "", "", fmt.Errorf("invalid descriptor name")
	}
	description, ok := cleanSealedText(descriptor.Description, sealedSkillDescriptorDescBytes)
	if !ok {
		return "", "", fmt.Errorf("invalid descriptor description")
	}
	return name, description, nil
}

func cleanSealedText(value string, maxBytes int) (string, bool) {
	value = strings.TrimSpace(value)
	return value, value != "" && len(value) <= maxBytes && !strings.ContainsRune(value, '\x00')
}

func validateSealedManifestDigest(topLevel, manifest, computed string) error {
	if topLevel == "" || manifest == "" || !validSealedPackageSHA256(topLevel) || topLevel != manifest || computed != manifest {
		return fmt.Errorf("package manifest 摘要不一致")
	}
	return nil
}

func signedAgentManifestDigest(manifest sealedAgentPackageManifest) string {
	return sha256JSONHex(sealedAgentPackageSignedManifest{
		AgentKey: manifest.AgentKey,
		Target:   manifest.Target,
		Files:    manifest.Files,
		SHA256:   "",
	})
}

func signedSkillManifestDigest(manifest sealedSkillPackageManifest) string {
	return sha256JSONHex(sealedSkillPackageSignedManifest{
		SkillKey:   manifest.SkillKey,
		Target:     manifest.Target,
		Descriptor: manifest.Descriptor,
		Files:      manifest.Files,
		SHA256:     "",
	})
}

func sha256JSONHex(value any) string {
	data, err := marshalSealedPackageJSON(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Match serde_json's compact struct serialization, including literal U+2028/U+2029.
func marshalSealedPackageJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	result := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] == '\\' && i+1 < len(data) {
			if i+6 <= len(data) && data[i+1] == 'u' && string(data[i+2:i+5]) == "202" && (data[i+5] == '8' || data[i+5] == '9') {
				if data[i+5] == '8' {
					result = append(result, "\u2028"...)
				} else {
					result = append(result, "\u2029"...)
				}
				i += 5
				continue
			}
			// Skip escape pairs so a literal backslash-u2028 remains escaped.
			result = append(result, data[i], data[i+1])
			i++
			continue
		}
		result = append(result, data[i])
	}
	return result, nil
}

func validateSealedPackagePayloadFiles(format, manifestSHA256 string, nonce, ciphertext, contentKey []byte, files []sealedPackageManifestFile) error {
	block, err := aes.NewCipher(contentKey)
	if err != nil {
		return fmt.Errorf("package content key 无效")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	aad, err := marshalSealedPackageJSON(struct {
		Format         string `json:"format"`
		Version        int    `json:"version"`
		ManifestSHA256 string `json:"manifestSha256"`
	}{format, sealedPackageFormatVersion, manifestSHA256})
	if err != nil {
		return err
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return fmt.Errorf("package payload 认证失败")
	}
	defer clear(plaintext)
	var entries []sealedPackagePayloadEntry
	if err := json.Unmarshal(plaintext, &entries); err != nil || len(entries) != len(files) {
		return fmt.Errorf("package payload 文件列表无效")
	}
	expected := make(map[string]sealedPackageManifestFile, len(files))
	for _, file := range files {
		expected[file.Path] = file
	}
	for _, entry := range entries {
		file, ok := expected[entry.Path]
		if !ok {
			return fmt.Errorf("package payload 文件路径或内容无效")
		}
		content, err := decodeSealedPackageBase64(base64.StdEncoding, entry.DataBase64)
		if err != nil {
			clear(content)
			return fmt.Errorf("package payload 文件内容无效")
		}
		sum := sha256.Sum256(content)
		matches := int64(len(content)) == file.SizeBytes && hex.EncodeToString(sum[:]) == file.SHA256
		clear(content)
		if !matches {
			return fmt.Errorf("package payload 文件大小或摘要不一致")
		}
		// Deleting also rejects duplicates; count equality ensures no manifest file is omitted.
		delete(expected, entry.Path)
	}
	return nil
}

// Strict rejects nonzero trailing bits; Go's decoder still ignores CR/LF,
// which Rust's STANDARD and URL_SAFE_NO_PAD engines reject.
func decodeSealedPackageBase64(encoding *base64.Encoding, value string) ([]byte, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, fmt.Errorf("invalid sealed package base64")
	}
	return encoding.Strict().DecodeString(value)
}

func decodeSealedPackagePayload(payload sealedPackagePayload) (nonce, ciphertext []byte, err error) {
	if payload.Algorithm != sealedPackagePayloadAlgo {
		return nil, nil, fmt.Errorf("package payload 无效")
	}
	nonce, err = decodeSealedPackageBase64(base64.RawURLEncoding, payload.NonceBase64)
	if err != nil || len(nonce) != sealedPackageGCMNonceBytes {
		return nil, nil, fmt.Errorf("package payload 无效")
	}
	ciphertext, err = decodeSealedPackageBase64(base64.RawURLEncoding, payload.CiphertextBase64)
	if err != nil || len(ciphertext) <= sealedPackageGCMTagBytes {
		return nil, nil, fmt.Errorf("package payload 无效")
	}
	return nonce, ciphertext, nil
}

func decodeSealedPackageEnvelope(envelope SealedPackageEnvelope) ([]byte, error) {
	if strings.TrimSpace(envelope.KeyID) == "" || envelope.Algorithm != sealedPackageEnvelopeAlgo {
		return nil, fmt.Errorf("package cloud envelope 无效")
	}
	ciphertext, err := decodeSealedPackageBase64(base64.RawURLEncoding, envelope.CiphertextBase64)
	if err != nil || len(ciphertext) == 0 || len(ciphertext) > sealedPackageMaxEnvelopeBytes {
		return nil, fmt.Errorf("package cloud envelope 无效")
	}
	return ciphertext, nil
}

func validSealedPackageEnvelope(envelope SealedPackageEnvelope) bool {
	_, err := decodeSealedPackageEnvelope(envelope)
	return err == nil
}

func NewSealedPackageCloudKey(keyID, privateKeyPEM string) (*SealedPackageCloudKey, error) {
	privateKey, err := parseRSAPrivateKeyPEM(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshal sealed package public key: %w", err)
	}
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		sum := sha256.Sum256(publicDER)
		keyID = hex.EncodeToString(sum[:8])
	}
	return &SealedPackageCloudKey{
		KeyID:            keyID,
		Algorithm:        sealedPackageEnvelopeAlgo,
		PublicKeySPKIPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		PrivateKeyPEM:    strings.TrimSpace(privateKeyPEM),
		PrivateKey:       privateKey,
	}, nil
}

func UnwrapSealedPackageCloudEnvelope(cloudKey *SealedPackageCloudKey, envelope SealedPackageEnvelope) ([]byte, error) {
	ciphertext, err := decodeSealedPackageEnvelope(envelope)
	if err != nil {
		return nil, err
	}
	return unwrapSealedPackageCloudEnvelope(cloudKey, envelope, ciphertext)
}

func unwrapSealedPackageCloudEnvelope(cloudKey *SealedPackageCloudKey, envelope SealedPackageEnvelope, ciphertext []byte) ([]byte, error) {
	if cloudKey == nil || cloudKey.PrivateKey == nil {
		return nil, fmt.Errorf("sealed package cloud key is not configured")
	}
	if envelope.KeyID != cloudKey.KeyID || envelope.Algorithm != cloudKey.Algorithm {
		return nil, fmt.Errorf("package cloud envelope key 不匹配")
	}
	contentKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, cloudKey.PrivateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("package cloud envelope 解密失败")
	}
	if len(contentKey) != sealedPackageContentKeyBytes {
		clear(contentKey)
		return nil, fmt.Errorf("package content key 无效")
	}
	return contentKey, nil
}

func WrapSealedPackageDeviceEnvelope(contentKey []byte, devicePublicKeyPEM string) (SealedPackageEnvelope, error) {
	if len(contentKey) != sealedPackageContentKeyBytes {
		return SealedPackageEnvelope{}, fmt.Errorf("package content key 无效")
	}
	publicKey, err := parseRSAPublicKeyPEM(devicePublicKeyPEM)
	if err != nil {
		return SealedPackageEnvelope{}, err
	}
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, contentKey, nil)
	if err != nil {
		return SealedPackageEnvelope{}, fmt.Errorf("package device envelope 加密失败")
	}
	return SealedPackageEnvelope{
		Algorithm:        sealedPackageEnvelopeAlgo,
		CiphertextBase64: base64.RawURLEncoding.EncodeToString(ciphertext),
	}, nil
}

func parseRSAPrivateKeyPEM(value string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, fmt.Errorf("sealed package private key 无效")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sealed package private key 无效")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("sealed package private key must be RSA")
	}
	return key, nil
}

func parseRSAPublicKeyPEM(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(value)))
	if block == nil {
		return nil, fmt.Errorf("devicePublicKey 无效")
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("devicePublicKey 无效")
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("devicePublicKey must be RSA")
	}
	return key, nil
}

func validSealedPackageSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f') {
			return false
		}
	}
	return true
}

type SealedPackageCloudKey struct {
	KeyID            string
	Algorithm        string
	PublicKeySPKIPEM string
	PrivateKeyPEM    string
	PrivateKey       *rsa.PrivateKey
}

type SealedPackageEnvelope struct {
	KeyID            string `json:"keyId"`
	Algorithm        string `json:"algorithm"`
	CiphertextBase64 string `json:"ciphertextBase64"`
}

type sealedAgentPackageWire struct {
	Format         string                `json:"format"`
	Version        int                   `json:"version"`
	ManifestSHA256 string                `json:"manifestSha256"`
	Manifest       json.RawMessage       `json:"manifest"`
	Payload        sealedPackagePayload  `json:"payload"`
	CloudEnvelope  SealedPackageEnvelope `json:"cloudEnvelope"`
	SkillKey       *string               `json:"skillKey,omitempty"`
	Target         *string               `json:"target,omitempty"`
}

type sealedSkillPackageWire struct {
	Format         string                `json:"format"`
	Version        int                   `json:"version"`
	SkillKey       string                `json:"skillKey"`
	Target         string                `json:"target"`
	ManifestSHA256 string                `json:"manifestSha256"`
	Manifest       json.RawMessage       `json:"manifest"`
	Payload        sealedPackagePayload  `json:"payload"`
	CloudEnvelope  SealedPackageEnvelope `json:"cloudEnvelope"`
}

type sealedAgentPackageManifest struct {
	AgentKey string                      `json:"agentKey"`
	Target   string                      `json:"target"`
	Files    []sealedPackageManifestFile `json:"files"`
	SHA256   string                      `json:"sha256"`
}

type sealedSkillPackageManifest struct {
	SkillKey   string                      `json:"skillKey"`
	Target     string                      `json:"target"`
	Descriptor sealedPackageDescriptor     `json:"descriptor"`
	Files      []sealedPackageManifestFile `json:"files"`
	SHA256     string                      `json:"sha256"`
}

type sealedAgentPackageSignedManifest struct {
	AgentKey string                      `json:"agentKey"`
	Target   string                      `json:"target"`
	Files    []sealedPackageManifestFile `json:"files"`
	SHA256   string                      `json:"sha256"`
}

type sealedSkillPackageSignedManifest struct {
	SkillKey   string                      `json:"skillKey"`
	Target     string                      `json:"target"`
	Descriptor sealedPackageDescriptor     `json:"descriptor"`
	Files      []sealedPackageManifestFile `json:"files"`
	SHA256     string                      `json:"sha256"`
}

type sealedPackagePayload struct {
	Algorithm        string `json:"algorithm"`
	NonceBase64      string `json:"nonceBase64"`
	CiphertextBase64 string `json:"ciphertextBase64"`
}

type sealedPackageDescriptor struct {
	Name                   string `json:"name"`
	Description            string `json:"description"`
	UserInvocable          *bool  `json:"userInvocable,omitempty"`
	DisableModelInvocation *bool  `json:"disableModelInvocation,omitempty"`
}

type sealedPackageManifestFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

type sealedPackagePayloadEntry struct {
	Path       string `json:"path"`
	DataBase64 string `json:"dataBase64"`
}

// Decode known fields once, with serde's exact names, duplicate rejection and
// required/non-null semantics. Unknown fields are ignored, not a second scan
// of the package or a RawMessage copy of every (potentially large) value.
func decodeSealedPackageObject(data []byte, required int, nullable uint16, names []string, fields ...any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("invalid sealed package object")
	}
	var seen uint16
	for decoder.More() {
		keyStart := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if name, ok := token.(string); ok && strings.Contains(name, "�") {
			if !validSealedPackageString(data[keyStart:decoder.InputOffset()]) {
				return fmt.Errorf("invalid sealed package field name unicode")
			}
		}
		index := -1
		for i, name := range names {
			if token == name {
				index = i
				break
			}
		}
		if index < 0 {
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return err
			}
			continue
		}
		bit := uint16(1) << index
		if seen&bit != 0 {
			return fmt.Errorf("duplicate sealed package field %s", names[index])
		}
		seen |= bit
		value := bytes.TrimLeft(data[decoder.InputOffset():], " \t\r\n:")
		if nullable&bit == 0 && bytes.HasPrefix(value, []byte("null")) {
			return fmt.Errorf("null sealed package field %s", names[index])
		}
		valueStart := decoder.InputOffset()
		if err := decoder.Decode(fields[index]); err != nil {
			return err
		}
		var text string
		switch field := fields[index].(type) {
		case *string:
			text = *field
		case **string:
			if *field != nil {
				text = **field
			}
		}
		// encoding/json silently replaces malformed Unicode with U+FFFD. Only
		// inspect the original token when that replacement could have occurred.
		if strings.Contains(text, "�") && !validSealedPackageString(data[valueStart:decoder.InputOffset()]) {
			return fmt.Errorf("invalid sealed package field %s unicode", names[index])
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if seen&((1<<required)-1) != (1<<required)-1 {
		return fmt.Errorf("missing sealed package field")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("invalid trailing sealed package data")
	}
	return nil
}

// The token is already valid JSON; check only the Unicode details Go accepts
// but serde's String/identifier decoder rejects. Do not inspect ignored values.
func validSealedPackageString(raw []byte) bool {
	if !utf8.Valid(raw) {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		code, _ := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		i += 4
		if code >= 0xdc00 && code <= 0xdfff {
			return false
		}
		if code < 0xd800 || code > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, _ := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return true
}

func (w *sealedAgentPackageWire) UnmarshalJSON(data []byte) error {
	*w = sealedAgentPackageWire{}
	// Missing cloudEnvelope keeps the existing structured re-export error.
	return decodeSealedPackageObject(data, 5, 1<<6|1<<7,
		[]string{"format", "version", "manifestSha256", "manifest", "payload", "cloudEnvelope", "skillKey", "target"},
		&w.Format, &w.Version, &w.ManifestSHA256, &w.Manifest, &w.Payload, &w.CloudEnvelope, &w.SkillKey, &w.Target)
}

func (w *sealedSkillPackageWire) UnmarshalJSON(data []byte) error {
	*w = sealedSkillPackageWire{}
	return decodeSealedPackageObject(data, 5, 1<<6|1<<7,
		[]string{"format", "version", "manifestSha256", "manifest", "payload", "cloudEnvelope", "skillKey", "target"},
		&w.Format, &w.Version, &w.ManifestSHA256, &w.Manifest, &w.Payload, &w.CloudEnvelope, &w.SkillKey, &w.Target)
}

func (m *sealedAgentPackageManifest) UnmarshalJSON(data []byte) error {
	*m = sealedAgentPackageManifest{}
	return decodeSealedPackageObject(data, 4, 0, []string{"agentKey", "target", "files", "sha256"},
		&m.AgentKey, &m.Target, &m.Files, &m.SHA256)
}

func (m *sealedSkillPackageManifest) UnmarshalJSON(data []byte) error {
	*m = sealedSkillPackageManifest{}
	return decodeSealedPackageObject(data, 5, 0, []string{"skillKey", "target", "descriptor", "files", "sha256"},
		&m.SkillKey, &m.Target, &m.Descriptor, &m.Files, &m.SHA256)
}

func (p *sealedPackagePayload) UnmarshalJSON(data []byte) error {
	*p = sealedPackagePayload{}
	return decodeSealedPackageObject(data, 3, 0, []string{"algorithm", "nonceBase64", "ciphertextBase64"},
		&p.Algorithm, &p.NonceBase64, &p.CiphertextBase64)
}

func (e *SealedPackageEnvelope) UnmarshalJSON(data []byte) error {
	*e = SealedPackageEnvelope{}
	return decodeSealedPackageObject(data, 3, 0, []string{"keyId", "algorithm", "ciphertextBase64"},
		&e.KeyID, &e.Algorithm, &e.CiphertextBase64)
}

func (d *sealedPackageDescriptor) UnmarshalJSON(data []byte) error {
	*d = sealedPackageDescriptor{}
	return decodeSealedPackageObject(data, 2, 1<<2|1<<3,
		[]string{"name", "description", "userInvocable", "disableModelInvocation"},
		&d.Name, &d.Description, &d.UserInvocable, &d.DisableModelInvocation)
}

func (f *sealedPackageManifestFile) UnmarshalJSON(data []byte) error {
	*f = sealedPackageManifestFile{}
	return decodeSealedPackageObject(data, 3, 0, []string{"path", "sizeBytes", "sha256"},
		&f.Path, &f.SizeBytes, &f.SHA256)
}

func (e *sealedPackagePayloadEntry) UnmarshalJSON(data []byte) error {
	*e = sealedPackagePayloadEntry{}
	return decodeSealedPackageObject(data, 2, 0, []string{"path", "dataBase64"}, &e.Path, &e.DataBase64)
}
