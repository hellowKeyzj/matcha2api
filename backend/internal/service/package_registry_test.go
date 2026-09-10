package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPackageRegistryCreateAuthorizationLeaseRequiresEntitlement(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	fixture.repo.entitlementErr = ErrMatchaPackageEntitlementNotFound

	_, err := fixture.service.CreateAuthorizationLease(context.Background(), fixture.version.ID, fixture.userID, fixture.devicePublicKey)
	if !errors.Is(err, ErrMatchaPackageEntitlementNotFound) {
		t.Fatalf("expected entitlement error, got %v", err)
	}
}

func TestPackageRegistryCreateAuthorizationLeaseReturnsDeviceEnvelope(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	before := time.Now().UTC().Add(14 * time.Minute)

	lease, err := fixture.service.CreateAuthorizationLease(context.Background(), fixture.version.ID, fixture.userID, fixture.devicePublicKey)
	if err != nil {
		t.Fatalf("CreateAuthorizationLease returned error: %v", err)
	}
	if lease == nil || lease.DeviceEnvelope.Algorithm != sealedPackageEnvelopeAlgo || lease.DeviceEnvelope.CiphertextBase64 == "" || lease.LeaseExpiresAt.Before(before) {
		t.Fatalf("unexpected lease: %+v", lease)
	}
	if fixture.repo.recordDownloadCalls != 0 {
		t.Fatalf("authorization lease must not record download")
	}
	if strings.Contains(lease.DeviceEnvelope.CiphertextBase64, base64.RawURLEncoding.EncodeToString(fixture.contentKey)) {
		t.Fatalf("device envelope leaked contentKey")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(lease.DeviceEnvelope.CiphertextBase64)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, fixture.devicePrivateKey, ciphertext, nil)
	if err != nil {
		t.Fatalf("decrypt device envelope: %v", err)
	}
	if string(plain) != string(fixture.contentKey) {
		t.Fatalf("unexpected content key")
	}
}

func TestPackageRegistryLeaseRespectsEntitlementExpiry(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	for _, remaining := range []time.Duration{time.Minute, time.Hour, -time.Minute} {
		expiresAt := time.Now().UTC().Add(remaining)
		fixture.repo.entitlement.ExpiresAt = &expiresAt
		before := time.Now().UTC()
		lease, err := fixture.service.CreateAuthorizationLease(context.Background(), fixture.version.ID, fixture.userID, fixture.devicePublicKey)
		if remaining < 0 {
			if !errors.Is(err, ErrMatchaPackageEntitlementNotFound) {
				t.Fatalf("expired entitlement returned lease: %+v %v", lease, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if lease.LeaseExpiresAt.After(expiresAt) || lease.LeaseExpiresAt.After(time.Now().UTC().Add(15*time.Minute)) {
			t.Fatalf("lease exceeds entitlement or maximum duration: %v", lease.LeaseExpiresAt)
		}
		if remaining == time.Minute && !lease.LeaseExpiresAt.Equal(expiresAt) {
			t.Fatal("short entitlement must cap lease exactly")
		}
		if remaining == time.Hour && lease.LeaseExpiresAt.Before(before.Add(15*time.Minute)) {
			t.Fatal("long entitlement should retain the fifteen-minute lease")
		}
	}
}

func TestPackageRegistryCheckDownloadAccessDoesNotWriteRecord(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)

	access, err := fixture.service.CheckDownloadAccess(context.Background(), fixture.version.ID, fixture.userID)
	if err != nil {
		t.Fatalf("CheckDownloadAccess returned error: %v", err)
	}
	if fixture.repo.recordDownloadCalls != 0 || access == nil || access.Entitlement == nil {
		t.Fatalf("expected access check without download record, calls=%d access=%+v", fixture.repo.recordDownloadCalls, access)
	}
}

func TestPackageRegistryPublishUsesConditionalDraftTransition(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	fixture.version.Status = model.MatchaPackageVersionStatusDraft
	req := PublishMatchaPackageVersionRequest{UserID: fixture.userID, VersionID: fixture.version.ID}
	published, err := fixture.service.PublishPackageVersion(context.Background(), req)
	if err != nil || published.Status != model.MatchaPackageVersionStatusPublished || published.PublishedAt == nil {
		t.Fatalf("draft publish failed: %+v %v", published, err)
	}
	firstPublishedAt := *published.PublishedAt
	published, err = fixture.service.PublishPackageVersion(context.Background(), req)
	if err != nil || !published.PublishedAt.Equal(firstPublishedAt) || fixture.repo.publishCalls != 2 || fixture.repo.updateStatusCalls != 0 {
		t.Fatalf("publish must be idempotent via conditional repo method: %v", err)
	}
	for _, status := range []string{model.MatchaPackageVersionStatusDisabled, model.MatchaPackageVersionStatusYanked} {
		fixture.version.Status = status
		if _, err := fixture.service.PublishPackageVersion(context.Background(), req); !errors.Is(err, ErrMatchaPackageVersionNotPublishable) {
			t.Fatalf("%s should not be publishable: %v", status, err)
		}
	}
	fixture.version.Status = model.MatchaPackageVersionStatusDraft
	fixture.repo.publishErr = ErrMatchaPackageVersionNotPublishable // concurrent takedown after service read
	if _, err := fixture.service.PublishPackageVersion(context.Background(), req); !errors.Is(err, ErrMatchaPackageVersionNotPublishable) {
		t.Fatalf("conditional repository error must survive: %v", err)
	}
	fixture.repo.publishErr = nil
	otherUser := fixture.userID + 1
	if _, err := fixture.service.PublishPackageVersion(context.Background(), PublishMatchaPackageVersionRequest{UserID: otherUser, VersionID: fixture.version.ID}); err == nil {
		t.Fatal("another user published version")
	}
	fixture.repo.pkg.OwnerUserID = &otherUser
	if _, err := fixture.service.PublishPackageVersion(context.Background(), req); err == nil {
		t.Fatal("another user's package published")
	}
	fixture.repo.pkg.OwnerUserID = &fixture.userID
	fixture.version.Status = model.MatchaPackageVersionStatusDisabled
	if _, err := fixture.service.UpdateVersionStatus(context.Background(), fixture.version.ID, model.MatchaPackageVersionStatusPublished); err != nil || fixture.repo.updateStatusCalls != 1 {
		t.Fatalf("generic admin transition changed: %v", err)
	}
}

func TestPackageRegistryAllUserAccessPathsShareValidation(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	store := &registryTestObjectStore{}
	fixture.service.resolver = func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }
	checks := []struct {
		name  string
		check func(int64) error
	}{
		{"authorize", func(userID int64) error {
			_, err := fixture.service.CreateAuthorizationLease(context.Background(), fixture.version.ID, userID, fixture.devicePublicKey)
			return err
		}},
		{"check", func(userID int64) error {
			_, err := fixture.service.CheckDownloadAccess(context.Background(), fixture.version.ID, userID)
			return err
		}},
		{"download", func(userID int64) error {
			object, err := fixture.service.DownloadPackageObjectForUser(context.Background(), fixture.version.ID, userID, "", "")
			if object != nil {
				_ = object.Reader.Close()
			}
			return err
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.check(0); !errors.Is(err, ErrMatchaPackageEntitlementNotFound) {
				t.Fatalf("zero user: %v", err)
			}
			fixture.repo.entitlementErr = ErrMatchaPackageEntitlementNotFound
			if err := check.check(fixture.userID); !errors.Is(err, ErrMatchaPackageEntitlementNotFound) {
				t.Fatalf("no entitlement: %v", err)
			}
			fixture.repo.entitlementErr = nil
			for _, status := range []string{model.MatchaPackageVersionStatusDraft, model.MatchaPackageVersionStatusDisabled, model.MatchaPackageVersionStatusYanked} {
				fixture.version.Status = status
				if err := check.check(fixture.userID); !errors.Is(err, ErrMatchaPackageVersionNotDownloadable) {
					t.Fatalf("%s: %v", status, err)
				}
			}
			fixture.version.Status = model.MatchaPackageVersionStatusPublished
			fixture.repo.pkg.Status = model.MatchaPackageStatusDisabled
			if err := check.check(fixture.userID); !errors.Is(err, ErrMatchaPackageVersionNotDownloadable) {
				t.Fatalf("disabled package: %v", err)
			}
			fixture.repo.pkg.Status = model.MatchaPackageStatusActive
			metadata := fixture.version.Metadata
			for _, missing := range []map[string]any{nil, {}, {"cloudEnvelope": nil}, {"cloudEnvelope": map[string]any{}}, {"cloudEnvelope": map[string]any{"keyId": "local", "algorithm": "local"}}} {
				fixture.version.Metadata = missing
				if err := check.check(fixture.userID); !errors.Is(err, ErrMatchaPackageCloudEnvelopeRequired) {
					t.Fatalf("legacy metadata: %v", err)
				}
			}
			fixture.version.Metadata = metadata
		})
	}
	if store.downloadCalls != 0 || fixture.repo.recordDownloadCalls != 0 {
		t.Fatal("denied access touched storage/audit")
	}
}

func TestPackageRegistryUploadAuthenticatesBeforeWritesAndCreatesDraft(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	key := sealedTestCloudKey(t)
	fixture.service.cloudKeys = packageCloudKeyServiceForTest(key)
	store := &registryTestObjectStore{}
	fixture.service.resolver = func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }
	req := UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: sealedSkillPackageFixture(t, "calendar", "openclaw", false)}
	var wire sealedSkillPackageWire
	if err := json.Unmarshal(req.Raw, &wire); err != nil {
		t.Fatal(err)
	}
	good := req.Raw
	wire.Payload = sealedPayloadFixture()
	req.Raw = mustMarshalJSON(t, wire)
	if _, err := fixture.service.UploadSealedPackage(context.Background(), req); err == nil {
		t.Fatal("random payload accepted")
	}
	wire.CloudEnvelope = SealedPackageEnvelope{}
	req.Raw = mustMarshalJSON(t, wire)
	if _, err := fixture.service.UploadSealedPackage(context.Background(), req); !errors.Is(err, ErrMatchaPackageCloudEnvelopeRequired) {
		t.Fatalf("legacy reason lost: %v", err)
	}
	if store.uploadCalls != 0 {
		t.Fatal("invalid package reached storage")
	}
	req.Raw = good
	result, err := fixture.service.UploadSealedPackage(context.Background(), req)
	if err != nil || result.Version.Status != model.MatchaPackageVersionStatusDraft || store.uploadCalls != 1 {
		t.Fatalf("verified upload must create draft: %+v %v", result, err)
	}
	if _, err := packageVersionCloudEnvelope(result.Version); err != nil {
		t.Fatal(err)
	}
	if fixture.repo.publishCalls != 0 || fixture.repo.updateStatusCalls != 0 {
		t.Fatal("upload implicitly published")
	}
}

func TestPackageRegistryDownloadAuditsAfterStorageAndClosesOnAuditFailure(t *testing.T) {
	fixture := newPackageRegistryServiceFixture(t)
	store := &registryTestObjectStore{}
	fixture.service.resolver = func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }
	store.downloadErr = errors.New("storage failed")
	if _, err := fixture.service.DownloadPackageObjectForUser(context.Background(), fixture.version.ID, fixture.userID, "", ""); err == nil || fixture.repo.recordDownloadCalls != 0 {
		t.Fatal("failed storage must not be audited")
	}
	store.downloadErr = nil
	fixture.repo.recordDownloadErr = errors.New("audit failed")
	if _, err := fixture.service.DownloadPackageObjectForUser(context.Background(), fixture.version.ID, fixture.userID, "", ""); err == nil || !store.reader.closed {
		t.Fatal("audit failure must close reader")
	}
	fixture.repo.recordDownloadErr = nil
	before := fixture.repo.recordDownloadCalls
	object, err := fixture.service.DownloadPackageObjectForUser(context.Background(), fixture.version.ID, fixture.userID, "", "")
	if err != nil || fixture.repo.recordDownloadCalls != before+1 {
		t.Fatalf("successful download must record once: %v", err)
	}
	_ = object.Reader.Close()
}

func TestPackageRegistryRustFixtureUploadPublishAuthorizeDownload(t *testing.T) {
	for _, filename := range []string{"agent.matcha-agentpkg", "skill.matcha-skillpkg"} {
		t.Run(filename, func(t *testing.T) {
			fixture := newPackageRegistryServiceFixture(t)
			key := sealedTestCloudKey(t)
			key.KeyID = "rust-fixture"
			fixture.service.cloudKeys = packageCloudKeyServiceForTest(key)
			store := &registryTestObjectStore{}
			fixture.service.resolver = func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }
			raw, err := os.ReadFile("testdata/sealed-v1/" + filename)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			upload, err := fixture.service.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: filename, Raw: raw})
			if err != nil {
				t.Fatal(err)
			}
			if upload.Version.Status != model.MatchaPackageVersionStatusDraft || upload.Version.SHA256 != sealedTestSHA256(raw) {
				t.Fatal("fixture upload did not create verified draft")
			}
			if _, err := fixture.service.CreateAuthorizationLease(ctx, upload.Version.ID, fixture.userID, fixture.devicePublicKey); !errors.Is(err, ErrMatchaPackageVersionNotDownloadable) {
				t.Fatalf("draft authorization: %v", err)
			}
			// A prior server stored the same authenticated object without cloud metadata.
			upload.Version.Status = model.MatchaPackageVersionStatusPublished
			upload.Version.Metadata["adminNote"] = "retain"
			delete(upload.Version.Metadata, "cloudEnvelope")
			if MatchaPackageVersionDownloadable(upload.Package, upload.Version) {
				t.Fatal("missing envelope must not advertise downloadable")
			}
			fixture.repo.mergeMetadataErr = errors.New("metadata unavailable")
			if _, err := fixture.service.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: filename, Raw: raw}); !errors.Is(err, fixture.repo.mergeMetadataErr) {
				t.Fatalf("repair failure must not report success: %v", err)
			}
			fixture.repo.mergeMetadataErr = nil
			reuploaded, err := fixture.service.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: filename, Raw: raw})
			if err != nil {
				t.Fatal(err)
			}
			if reuploaded.Version.ID != upload.Version.ID || reuploaded.Entitlement.ID != upload.Entitlement.ID || store.uploadCalls != 1 || reuploaded.Version.Status != model.MatchaPackageVersionStatusPublished || reuploaded.Version.Metadata["adminNote"] != "retain" {
				t.Fatal("metadata repair replaced version, object, entitlement, status or unrelated metadata")
			}
			if !MatchaPackageVersionDownloadable(reuploaded.Package, reuploaded.Version) {
				t.Fatal("authenticated metadata repair did not restore downloadability")
			}
			upload.Version.Status = model.MatchaPackageVersionStatusDraft
			published, err := fixture.service.PublishPackageVersion(ctx, PublishMatchaPackageVersionRequest{UserID: fixture.userID, VersionID: upload.Version.ID})
			if err != nil || published.Status != model.MatchaPackageVersionStatusPublished {
				t.Fatalf("publish: %v", err)
			}
			lease, err := fixture.service.CreateAuthorizationLease(ctx, published.ID, fixture.userID, fixture.devicePublicKey)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.repo.recordDownloadCalls != 0 {
				t.Fatal("authorize inserted audit")
			}
			object, err := fixture.service.DownloadPackageObjectForUser(ctx, published.ID, fixture.userID, "127.0.0.1", "fixture-test")
			if err != nil {
				t.Fatal(err)
			}
			downloaded, err := io.ReadAll(object.Reader)
			_ = object.Reader.Close()
			if err != nil || !bytes.Equal(downloaded, raw) || fixture.repo.recordDownloadCalls != 1 {
				t.Fatal("download changed fixture or audit count")
			}
			wrapped, err := base64.RawURLEncoding.Strict().DecodeString(lease.DeviceEnvelope.CiphertextBase64)
			if err != nil {
				t.Fatal(err)
			}
			contentKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, fixture.devicePrivateKey, wrapped, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(contentKey)
			var wire sealedAgentPackageWire // common wire fields also apply to skill
			if err := json.Unmarshal(downloaded, &wire); err != nil {
				t.Fatal(err)
			}
			block, err := aes.NewCipher(contentKey)
			if err != nil {
				t.Fatal(err)
			}
			gcm, err := cipher.NewGCM(block)
			if err != nil {
				t.Fatal(err)
			}
			nonce, err := base64.RawURLEncoding.Strict().DecodeString(wire.Payload.NonceBase64)
			if err != nil {
				t.Fatal(err)
			}
			ciphertext, err := base64.RawURLEncoding.Strict().DecodeString(wire.Payload.CiphertextBase64)
			if err != nil {
				t.Fatal(err)
			}
			// Original Rust ordered AAD, not the production helper under test.
			aad := []byte(`{"format":"` + wire.Format + `","version":1,"manifestSha256":"` + wire.ManifestSHA256 + `"}`)
			plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
			if err != nil {
				t.Fatalf("device key cannot authenticate original Rust fixture: %v", err)
			}
			defer clear(plaintext)
			var entries []struct {
				Path       string `json:"path"`
				DataBase64 string `json:"dataBase64"`
			}
			if err := json.Unmarshal(plaintext, &entries); err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Files []sealedPackageManifestFile `json:"files"`
			}
			if err := json.Unmarshal(wire.Manifest, &manifest); err != nil {
				t.Fatal(err)
			}
			if len(entries) != len(manifest.Files) {
				t.Fatal("file count mismatch")
			}
			for i, entry := range entries {
				content, err := base64.StdEncoding.Strict().DecodeString(entry.DataBase64)
				if err != nil {
					t.Fatal(err)
				}
				if entry.Path != manifest.Files[i].Path || int64(len(content)) != manifest.Files[i].SizeBytes || sealedTestSHA256(content) != manifest.Files[i].SHA256 {
					t.Fatal("device-decrypted file differs from original manifest")
				}
				clear(content)
			}
		})
	}
}

func TestPackageRegistryCloudKeyTransitionPreservesOldAndNewPackages(t *testing.T) {
	ctx := context.Background()
	fixture := newPackageRegistryServiceFixture(t)
	k1 := sealedTestCloudKey(t)
	settings := newPackageCloudKeySettingRepo()
	keys := packageCloudKeyServiceForTest(k1)
	keys.settingRepo, keys.encryptor = settings, packageCloudKeyEncryptor{}
	store := &registryTestObjectStore{}
	resolver := func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }
	registry := NewMatchaPackageRegistryService(fixture.repo, resolver, "", keys)
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	req := UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: raw}
	old, err := registry.UploadSealedPackage(ctx, req)
	require.NoError(t, err)
	_, err = registry.PublishPackageVersion(ctx, PublishMatchaPackageVersionRequest{UserID: fixture.userID, VersionID: old.Version.ID})
	require.NoError(t, err)

	// Another instance has no K1 config and creates the immutable database K2.
	writer := NewMatchaPackageCloudKeyService(settings, packageCloudKeyEncryptor{}, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}})
	status, err := writer.Generate(ctx, nil)
	require.NoError(t, err)
	current, err := registry.SealedCloudKey(ctx)
	require.NoError(t, err)
	require.Equal(t, status.KeyID, current.KeyID)
	require.NotEqual(t, k1.KeyID, current.KeyID)
	lease, err := registry.CreateAuthorizationLease(ctx, old.Version.ID, fixture.userID, fixture.devicePublicKey)
	require.NoError(t, err)
	ciphertext, err := base64.RawURLEncoding.DecodeString(lease.DeviceEnvelope.CiphertextBase64)
	require.NoError(t, err)
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, fixture.devicePrivateKey, ciphertext, nil)
	require.NoError(t, err)
	require.Equal(t, fixture.contentKey, plain)
	clear(plain)

	// Authenticated K1 reupload repairs missing metadata without replacing the object.
	delete(old.Version.Metadata, "cloudEnvelope")
	old.Version.Metadata["adminNote"] = "retain"
	reuploaded, err := registry.UploadSealedPackage(ctx, req)
	require.NoError(t, err)
	require.Equal(t, old.Version.ID, reuploaded.Version.ID)
	require.Equal(t, old.Entitlement.ID, reuploaded.Entitlement.ID)
	require.Equal(t, "retain", reuploaded.Version.Metadata["adminNote"])
	require.Equal(t, model.MatchaPackageVersionStatusPublished, reuploaded.Version.Status)
	require.Equal(t, 1, store.uploadCalls)
	envelope, err := packageVersionCloudEnvelope(reuploaded.Version)
	require.NoError(t, err)
	require.Equal(t, k1.KeyID, envelope.KeyID)

	// Rewrap the existing fixture's content key, leaving its v1 payload/AAD intact.
	var wire sealedSkillPackageWire
	require.NoError(t, json.Unmarshal(raw, &wire))
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &current.PrivateKey.PublicKey, fixture.contentKey, nil)
	require.NoError(t, err)
	wire.CloudEnvelope = SealedPackageEnvelope{KeyID: current.KeyID, Algorithm: sealedPackageEnvelopeAlgo, CiphertextBase64: base64.RawURLEncoding.EncodeToString(wrapped)}
	newReq := req
	newReq.Raw = mustMarshalJSON(t, wire)
	newUpload, err := registry.UploadSealedPackage(ctx, newReq)
	require.NoError(t, err)
	require.NotEqual(t, old.Version.ID, newUpload.Version.ID)
	require.Equal(t, model.MatchaPackageVersionStatusDraft, newUpload.Version.Status)
	_, err = registry.PublishPackageVersion(ctx, PublishMatchaPackageVersionRequest{UserID: fixture.userID, VersionID: newUpload.Version.ID})
	require.NoError(t, err)

	restartedKeys := NewMatchaPackageCloudKeyService(settings, packageCloudKeyEncryptor{}, keys.cfg)
	restarted := NewMatchaPackageRegistryService(fixture.repo, resolver, "", restartedKeys)
	for _, uploaded := range []*UploadSealedPackageResult{old, newUpload} {
		lease, err := restarted.CreateAuthorizationLease(ctx, uploaded.Version.ID, fixture.userID, fixture.devicePublicKey)
		require.NoError(t, err)
		ciphertext, err := base64.RawURLEncoding.DecodeString(lease.DeviceEnvelope.CiphertextBase64)
		require.NoError(t, err)
		plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, fixture.devicePrivateKey, ciphertext, nil)
		require.NoError(t, err)
		require.Equal(t, fixture.contentKey, plain)
		clear(plain)
	}
	for _, request := range []UploadSealedPackageRequest{req, newReq} {
		_, err := restarted.UploadSealedPackage(ctx, request)
		require.NoError(t, err)
	}
	require.Equal(t, 2, store.uploadCalls, "restarts/reuploads must reuse both versions")
}

func TestPackageRegistryUploadAndAuthorizationPreserveResolverErrors(t *testing.T) {
	ctx := context.Background()
	fixture := newPackageRegistryServiceFixture(t)
	key := sealedTestCloudKey(t)
	settings := newPackageCloudKeySettingRepo()
	keys := packageCloudKeyServiceForTest(key)
	keys.settingRepo, keys.encryptor = settings, packageCloudKeyEncryptor{}
	store := &registryTestObjectStore{}
	registry := NewMatchaPackageRegistryService(fixture.repo, func(context.Context) (MatchaPackageObjectStore, error) { return store, nil }, "", keys)
	raw := sealedSkillPackageFixture(t, "calendar", "openclaw", false)
	var wire sealedSkillPackageWire
	require.NoError(t, json.Unmarshal(raw, &wire))
	databaseErr := errors.New("database connection failed")
	settings.getErr = databaseErr
	_, err := registry.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: raw})
	require.ErrorIs(t, err, databaseErr)
	require.Equal(t, http.StatusInternalServerError, infraerrors.Code(err), "DB errors are not invalid user packages")
	fixture.version.Metadata["cloudEnvelope"] = PackageUploadVersionMetadata("", SealedPackageUploadMetadata{CloudEnvelope: wire.CloudEnvelope})["cloudEnvelope"]
	_, err = registry.CreateAuthorizationLease(ctx, fixture.version.ID, fixture.userID, fixture.devicePublicKey)
	require.ErrorIs(t, err, databaseErr)
	reads := settings.getCalls
	_, err = registry.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: []byte("invalid JSON")})
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, reads, settings.getCalls, "structural validation precedes key resolution")
	settings.getErr = nil

	wire.CloudEnvelope.KeyID = "unknown"
	unknown := mustMarshalJSON(t, wire)
	_, err = registry.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: unknown})
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	fixture.version.Metadata["cloudEnvelope"].(map[string]any)["keyId"] = "unknown"
	_, err = registry.CreateAuthorizationLease(ctx, fixture.version.ID, fixture.userID, fixture.devicePublicKey)
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable)

	// Database and configured K1 now claim the same ID with different material.
	otherKey, err := fixture.service.cloudKeys.Get(ctx)
	require.NoError(t, err)
	encrypted, err := (packageCloudKeyEncryptor{}).Encrypt(otherKey.PrivateKeyPEM)
	require.NoError(t, err)
	stored := mustMarshalJSON(t, sealedPackageCloudKeySettings{KeyID: key.KeyID, EncryptedPrivateKeyPEM: encrypted})
	require.NoError(t, settings.Set(ctx, settingKeyMatchaPackageCloudKey, string(stored)))
	_, err = registry.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: raw})
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyIDConflict)
	fixture.version.Metadata["cloudEnvelope"].(map[string]any)["keyId"] = key.KeyID
	_, err = registry.CreateAuthorizationLease(ctx, fixture.version.ID, fixture.userID, fixture.devicePublicKey)
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyIDConflict)
	_, err = registry.UploadSealedPackage(ctx, UploadSealedPackageRequest{UserID: fixture.userID, Filename: "calendar.matcha-skillpkg", Raw: unknown})
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable, "unrelated IDs must not be globally marked conflicting")
	require.Zero(t, store.uploadCalls)
}

type registryTestReader struct {
	io.Reader
	closed bool
}

func (r *registryTestReader) Close() error { r.closed = true; return nil }

type registryTestObjectStore struct {
	uploadCalls, downloadCalls int
	downloadErr                error
	reader                     *registryTestReader
	objects                    map[string][]byte
}

func (s *registryTestObjectStore) Upload(_ context.Context, key string, body io.Reader, _ string) (int64, error) {
	s.uploadCalls++
	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	s.objects[key] = data
	return int64(len(data)), nil
}
func (s *registryTestObjectStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	s.downloadCalls++
	if s.downloadErr != nil {
		return nil, s.downloadErr
	}
	data, ok := s.objects[key]
	if !ok {
		data = []byte("package")
	}
	s.reader = &registryTestReader{Reader: bytes.NewReader(data)}
	return s.reader, nil
}
func (*registryTestObjectStore) Delete(context.Context, string) error               { return nil }
func (*registryTestObjectStore) PresignURL(context.Context, string) (string, error) { return "", nil }

type packageRegistryServiceFixture struct {
	userID           int64
	contentKey       []byte
	devicePublicKey  string
	devicePrivateKey *rsa.PrivateKey
	version          *model.MatchaPackageVersion
	service          *MatchaPackageRegistryService
	repo             *fakeMatchaPackageRepository
}

func newPackageRegistryServiceFixture(t *testing.T) packageRegistryServiceFixture {
	t.Helper()
	userID := int64(42)
	cloudPrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	cloudDER := x509.MarshalPKCS1PrivateKey(cloudPrivate)
	cloudKey, err := NewSealedPackageCloudKey("cloud-test", string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: cloudDER})))
	if err != nil {
		t.Fatal(err)
	}
	contentKey := []byte(strings.Repeat("k", sealedPackageContentKeyBytes))
	cloudCiphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &cloudPrivate.PublicKey, contentKey, nil)
	if err != nil {
		t.Fatal(err)
	}
	devicePrivate, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	deviceDER, err := x509.MarshalPKIXPublicKey(&devicePrivate.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	pkg := &model.MatchaPackage{ID: 1, PackageKey: "skill:openclaw:test", Name: "Test", OwnerUserID: &userID, Status: model.MatchaPackageStatusActive, Metadata: map[string]any{}, CreatedAt: now, UpdatedAt: now}
	version := &model.MatchaPackageVersion{ID: 7, PackageID: 1, Version: strings.Repeat("a", 64), Kind: model.MatchaPackageKindSkill, Runtime: "openclaw", SHA256: strings.Repeat("a", 64), SizeBytes: 4, ObjectKey: "pkg", OwnerUserID: &userID, Status: model.MatchaPackageVersionStatusPublished, PriceAmount: decimal.Zero, PriceCurrency: "USD", SurchargePolicy: map[string]any{}, Metadata: map[string]any{"cloudEnvelope": map[string]any{"keyId": "cloud-test", "algorithm": sealedPackageEnvelopeAlgo, "ciphertextBase64": base64.RawURLEncoding.EncodeToString(cloudCiphertext)}}, CreatedAt: now, UpdatedAt: now}
	entitlement := &model.MatchaPackageEntitlement{ID: 9, PackageID: pkg.ID, VersionID: &version.ID, UserID: userID, Status: model.MatchaPackageEntitlementStatusActive}
	repo := &fakeMatchaPackageRepository{pkg: pkg, version: version, entitlement: entitlement}
	return packageRegistryServiceFixture{
		userID:           userID,
		contentKey:       contentKey,
		devicePublicKey:  string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: deviceDER})),
		devicePrivateKey: devicePrivate,
		version:          version,
		service:          NewMatchaPackageRegistryService(repo, nil, "", packageCloudKeyServiceForTest(cloudKey)),
		repo:             repo,
	}
}

type fakeMatchaPackageRepository struct {
	pkg                 *model.MatchaPackage
	version             *model.MatchaPackageVersion
	versions            map[int64]*model.MatchaPackageVersion
	entitlement         *model.MatchaPackageEntitlement
	entitlements        map[int64]*model.MatchaPackageEntitlement
	entitlementErr      error
	recordDownloadCalls int
	publishCalls        int
	publishErr          error
	updateStatusCalls   int
	recordDownloadErr   error
	mergeMetadataErr    error
}

func (f *fakeMatchaPackageRepository) CreatePackage(context.Context, *model.MatchaPackage) (*model.MatchaPackage, error) {
	return nil, ErrMatchaPackageExists
}
func (f *fakeMatchaPackageRepository) GetPackage(_ context.Context, id int64) (*model.MatchaPackage, error) {
	if f.pkg != nil && f.pkg.ID == id {
		return f.pkg, nil
	}
	return nil, ErrMatchaPackageNotFound
}
func (f *fakeMatchaPackageRepository) DeletePackage(context.Context, int64) error { return nil }
func (f *fakeMatchaPackageRepository) GetPackageByKey(context.Context, string) (*model.MatchaPackage, error) {
	return f.pkg, nil
}
func (f *fakeMatchaPackageRepository) ListPackages(context.Context, MatchaPackageListFilter) ([]*model.MatchaPackage, error) {
	return []*model.MatchaPackage{f.pkg}, nil
}
func (f *fakeMatchaPackageRepository) ListPackageVersions(context.Context, MatchaPackageVersionListFilter) (*MatchaPackageVersionListResult, error) {
	return &MatchaPackageVersionListResult{}, nil
}
func (f *fakeMatchaPackageRepository) UpdatePackage(context.Context, *model.MatchaPackage) (*model.MatchaPackage, error) {
	return f.pkg, nil
}
func (f *fakeMatchaPackageRepository) CreateVersion(context.Context, *model.MatchaPackageVersion) (*model.MatchaPackageVersion, error) {
	return f.version, nil
}
func (f *fakeMatchaPackageRepository) CreateVersionWithEntitlement(_ context.Context, version *model.MatchaPackageVersion, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageVersion, *model.MatchaPackageEntitlement, error) {
	if f.versions == nil {
		f.versions = make(map[int64]*model.MatchaPackageVersion)
		f.entitlements = make(map[int64]*model.MatchaPackageEntitlement)
	}
	version.ID = int64(7 + len(f.versions))
	entitlement.ID = int64(9 + len(f.entitlements))
	entitlement.VersionID = &version.ID
	f.versions[version.ID], f.entitlements[version.ID] = version, entitlement
	f.version, f.entitlement = version, entitlement
	return version, entitlement, nil
}
func (f *fakeMatchaPackageRepository) GetVersion(_ context.Context, id int64) (*model.MatchaPackageVersion, error) {
	if version := f.versions[id]; version != nil {
		return version, nil
	}
	if f.version != nil && f.version.ID == id {
		return f.version, nil
	}
	return nil, ErrMatchaPackageVersionNotFound
}
func (f *fakeMatchaPackageRepository) GetVersionByName(_ context.Context, packageID int64, name string) (*model.MatchaPackageVersion, error) {
	for _, version := range f.versions {
		if version.PackageID == packageID && version.Version == name {
			return version, nil
		}
	}
	if f.version != nil && f.version.PackageID == packageID && f.version.Version == name {
		return f.version, nil
	}
	return nil, ErrMatchaPackageVersionNotFound
}
func (f *fakeMatchaPackageRepository) MergeVersionMetadata(_ context.Context, id int64, metadata map[string]any) (*model.MatchaPackageVersion, error) {
	if f.version == nil || f.version.ID != id {
		return nil, ErrMatchaPackageVersionNotFound
	}
	if f.mergeMetadataErr != nil {
		return nil, f.mergeMetadataErr
	}
	for key, value := range metadata {
		f.version.Metadata[key] = value
	}
	return f.version, nil
}
func (f *fakeMatchaPackageRepository) ListVersions(context.Context, int64) ([]*model.MatchaPackageVersion, error) {
	return []*model.MatchaPackageVersion{f.version}, nil
}
func (f *fakeMatchaPackageRepository) UpdateVersionStatus(_ context.Context, _ int64, status string) (*model.MatchaPackageVersion, error) {
	f.updateStatusCalls++
	f.version.Status = status
	return f.version, nil
}
func (f *fakeMatchaPackageRepository) PublishDraftVersion(_ context.Context, id int64) (*model.MatchaPackageVersion, error) {
	if f.version == nil || f.version.ID != id {
		return nil, ErrMatchaPackageVersionNotFound
	}
	f.publishCalls++
	if f.publishErr != nil {
		return nil, f.publishErr
	}
	switch f.version.Status {
	case model.MatchaPackageVersionStatusDraft:
		f.version.Status = model.MatchaPackageVersionStatusPublished
		if f.version.PublishedAt == nil {
			now := time.Now().UTC()
			f.version.PublishedAt = &now
		}
	case model.MatchaPackageVersionStatusPublished:
	default:
		return nil, ErrMatchaPackageVersionNotPublishable
	}
	return f.version, nil
}

func (f *fakeMatchaPackageRepository) GrantEntitlement(_ context.Context, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageEntitlement, error) {
	if entitlement.VersionID != nil {
		if existing := f.entitlements[*entitlement.VersionID]; existing != nil {
			return existing, nil
		}
	}
	return f.entitlement, nil
}
func (f *fakeMatchaPackageRepository) GetActiveEntitlement(_ context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error) {
	if f.entitlementErr != nil {
		return nil, f.entitlementErr
	}
	if versionID != nil {
		if entitlement := f.entitlements[*versionID]; entitlement != nil && entitlement.PackageID == packageID && entitlement.UserID == userID {
			return entitlement, nil
		}
	}
	if f.entitlement != nil && f.entitlement.PackageID == packageID && f.entitlement.UserID == userID && versionID != nil && f.entitlement.VersionID != nil && *f.entitlement.VersionID == *versionID {
		return f.entitlement, nil
	}
	return nil, ErrMatchaPackageEntitlementNotFound
}
func (f *fakeMatchaPackageRepository) RevokeEntitlement(context.Context, int64) error { return nil }
func (f *fakeMatchaPackageRepository) RecordDownload(_ context.Context, download *model.MatchaPackageDownload) (*model.MatchaPackageDownload, error) {
	f.recordDownloadCalls++
	if f.recordDownloadErr != nil {
		return nil, f.recordDownloadErr
	}
	download.ID = int64(f.recordDownloadCalls)
	download.DownloadedAt = time.Now().UTC()
	return download, nil
}
func (f *fakeMatchaPackageRepository) ListDownloads(context.Context, MatchaPackageDownloadFilter) ([]*model.MatchaPackageDownload, error) {
	return nil, nil
}
func (f *fakeMatchaPackageRepository) CountDownloadsByPackage(context.Context, []int64) (map[int64]MatchaPackageDownloadStats, error) {
	return map[int64]MatchaPackageDownloadStats{}, nil
}
func (f *fakeMatchaPackageRepository) UpsertBillingPolicy(context.Context, *model.MatchaPackageBillingPolicy) (*model.MatchaPackageBillingPolicy, error) {
	return nil, nil
}
func (f *fakeMatchaPackageRepository) ListActiveBillingPolicies(context.Context, int64, *int64) ([]*model.MatchaPackageBillingPolicy, error) {
	return nil, nil
}
func (f *fakeMatchaPackageRepository) RecordUsage(context.Context, *model.MatchaPackageUsageRecord, *model.MatchaPackageRevenueLedgerEntry) (*model.MatchaPackageUsageRecord, error) {
	return nil, nil
}
func (f *fakeMatchaPackageRepository) ListRevenueLedger(context.Context, MatchaPackageRevenueFilter) ([]*model.MatchaPackageRevenueLedgerEntry, error) {
	return nil, nil
}
