package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func TestPackageRegistryAuthorizeRequiresEntitlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	fixture.registry.entitlementErr = service.ErrMatchaPackageEntitlementNotFound
	h := NewPackageRegistryHandler(fixture.registry)
	body := `{"devicePublicKey":` + strconvQuote(testRSAPublicKeyPEM(t)) + `}`
	w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/7/authorization", body, fixture.userID)
	c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}

	h.Authorize(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
	if fixture.registry.recordDownloadCalls != 0 {
		t.Fatalf("authorization must not insert records")
	}
}

func TestPackageRegistryAuthorizeReturnsDeviceEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	h := NewPackageRegistryHandler(fixture.registry)
	body := `{"packageVersionId":"7","devicePublicKey":` + strconvQuote(testRSAPublicKeyPEM(t)) + `}`
	w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/7/authorization", body, fixture.userID)
	c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}

	h.Authorize(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if fixture.registry.recordDownloadCalls != 0 {
		t.Fatalf("authorization must not insert records")
	}
	if !strings.Contains(w.Body.String(), `"deviceEnvelope"`) || !strings.Contains(w.Body.String(), `"leaseExpiresAt"`) || strings.Contains(w.Body.String(), "contentKey") {
		t.Fatalf("expected authorization lease without contentKey, got %s", w.Body.String())
	}
}

func TestPackageRegistryPublishPreservesVersionResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	h := NewPackageRegistryHandler(fixture.registry)
	w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/7/publish", ``, fixture.userID)
	c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}

	h.Publish(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{
		"packageId": `"1"`, "packageVersionId": `"7"`, "packageType": `"skill"`,
		"status": `"published"`, "entitlementStatus": `"active"`, "downloadable": `true`,
	} {
		if got := string(envelope.Data[field]); got != want {
			t.Fatalf("%s: expected %s, got %s", field, want, got)
		}
	}
	for _, field := range []string{"package", "contentKey", "cloudEnvelope", "deviceEnvelope"} {
		if _, ok := envelope.Data[field]; ok {
			t.Fatalf("unexpected field %s", field)
		}
	}
}

func TestPackageRegistryPublishDoesNotHideEntitlementFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	fixture.registry.entitlementErr = infraerrors.ServiceUnavailable("TEST_DATABASE_UNAVAILABLE", "database unavailable")
	h := NewPackageRegistryHandler(fixture.registry)
	w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/7/publish", ``, fixture.userID)
	c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}

	h.Publish(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPackageRegistryDownloadStreamsPackageObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	h := NewPackageRegistryHandler(fixture.registry)
	w, c := packageRegistryTestContext(http.MethodGet, "/api/v1/packages/7/download", ``, fixture.userID)
	c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}

	h.Download(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "body" {
		t.Fatalf("expected package body, got %q", w.Body.String())
	}
	if got := w.Header().Get("X-Package-Version-Id"); got != "7" {
		t.Fatalf("expected package version header, got %q", got)
	}
	if got := w.Header().Get("X-Matcha-Package-Metering"); !strings.HasPrefix(got, "m1.") {
		t.Fatalf("expected metering binding header, got %q", got)
	}
	call := fixture.registry.downloadForUserCall
	if call.versionID != fixture.version.ID || call.userID != fixture.userID {
		t.Fatalf("wrong download call args: %+v", call)
	}
}

func TestPackageRegistryListMarketIncludesUnentitledPublishedPackage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newPackageRegistryFixture(t)
	fixture.registry.listRows = []service.MatchaPackageVersionListRow{{Package: fixture.pkg, Version: fixture.version, Downloadable: false}}
	h := NewPackageRegistryHandler(fixture.registry)
	w, c := packageRegistryTestContext(http.MethodGet, "/api/v1/packages/market?page=1&page_size=20", ``, fixture.userID)

	h.ListMarket(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"items":[`) || !strings.Contains(w.Body.String(), `"downloadable":false`) {
		t.Fatalf("expected unentitled market item, got %s", w.Body.String())
	}
	filter := fixture.registry.lastListFilter
	if filter.Status != model.MatchaPackageStatusActive || filter.VersionStatus != model.MatchaPackageVersionStatusPublished {
		t.Fatalf("unexpected market filter: %+v", filter)
	}
}

func TestPackageVersionDTOUsesVersionDescription(t *testing.T) {
	now := time.Now().UTC()
	pkg := &model.MatchaPackage{ID: 1, PackageKey: "skill:openclaw:calendar", Name: "Calendar", Description: "old package description", Status: model.MatchaPackageStatusActive, CreatedAt: now, UpdatedAt: now}
	version := &model.MatchaPackageVersion{ID: 7, Version: strings.Repeat("a", 64), Kind: model.MatchaPackageKindSkill, Runtime: "openclaw", SHA256: strings.Repeat("a", 64), Status: model.MatchaPackageVersionStatusPublished, Metadata: map[string]any{"description": "Calendar skill"}, CreatedAt: now, UpdatedAt: now}

	item := packageVersionDTO(pkg, version, true, model.MatchaPackageEntitlementStatusActive)

	if item.Description != "Calendar skill" {
		t.Fatalf("expected version description, got %q", item.Description)
	}
}

func TestPackageRegistryUploadCreatesDraftAndPreservesServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{name: "draft", status: http.StatusCreated},
		{name: "invalid package", err: infraerrors.BadRequest("MATCHA_PACKAGE_INVALID_UPLOAD", "invalid package"), status: http.StatusBadRequest},
		{name: "unknown key", err: service.ErrSealedPackageCloudKeyUnavailable, status: http.StatusServiceUnavailable},
		{name: "key conflict", err: service.ErrSealedPackageCloudKeyIDConflict, status: http.StatusServiceUnavailable},
		{name: "database", err: infraerrors.ServiceUnavailable("TEST_DATABASE_UNAVAILABLE", "database unavailable"), status: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newPackageRegistryFixture(t)
			fixture.version.Status = model.MatchaPackageVersionStatusDraft
			fixture.registry.uploadErr = tc.err
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("package", "calendar.matcha-skillpkg")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write([]byte("sealed package bytes")); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/upload", "", fixture.userID)
			c.Request.Body = io.NopCloser(&body)
			c.Request.Header.Set("Content-Type", writer.FormDataContentType())
			NewPackageRegistryHandler(fixture.registry).Upload(c)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d: %s", tc.status, w.Code, w.Body.String())
			}
			req := fixture.registry.uploadReq
			if req.UserID != fixture.userID || req.Filename != "calendar.matcha-skillpkg" || string(req.Raw) != "sealed package bytes" || req.ContentType != "application/octet-stream" {
				t.Fatalf("unexpected upload request: %+v", req)
			}
			if tc.err != nil {
				if !strings.Contains(w.Body.String(), infraerrors.Reason(tc.err)) {
					t.Fatalf("lost service error reason: %s", w.Body.String())
				}
				return
			}
			if !strings.Contains(w.Body.String(), `"status":"draft"`) || !strings.Contains(w.Body.String(), `"downloadable":false`) {
				t.Fatalf("upload must return draft, not published: %s", w.Body.String())
			}
		})
	}
}

func TestPackageRegistryAuthorizePreservesCloudKeyErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, err := range []error{service.ErrSealedPackageCloudKeyUnavailable, service.ErrSealedPackageCloudKeyIDConflict} {
		fixture := newPackageRegistryFixture(t)
		fixture.registry.entitlementErr = err
		w, c := packageRegistryTestContext(http.MethodPost, "/api/v1/packages/7/authorization", `{"devicePublicKey":"device-key"}`, fixture.userID)
		c.Params = gin.Params{{Key: "packageVersionId", Value: "7"}}
		NewPackageRegistryHandler(fixture.registry).Authorize(c)
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), infraerrors.Reason(err)) {
			t.Fatalf("lost cloud key error: %d %s", w.Code, w.Body.String())
		}
	}
}

type packageRegistryFixture struct {
	userID      int64
	pkg         *model.MatchaPackage
	version     *model.MatchaPackageVersion
	entitlement *model.MatchaPackageEntitlement
	registry    *fakePackageRegistry
}

func newPackageRegistryFixture(t *testing.T) packageRegistryFixture {
	t.Helper()
	now := time.Now().UTC()
	userID := int64(42)
	pkg := &model.MatchaPackage{ID: 1, PackageKey: "skill:openclaw:test", Name: "Test", Status: model.MatchaPackageStatusActive, CreatedAt: now, UpdatedAt: now}
	version := &model.MatchaPackageVersion{ID: 7, PackageID: 1, Version: strings.Repeat("a", 64), Kind: model.MatchaPackageKindSkill, Runtime: "openclaw", SHA256: strings.Repeat("a", 64), SizeBytes: 4, ObjectKey: "pkg", OwnerUserID: &userID, Status: model.MatchaPackageVersionStatusPublished, PriceAmount: decimal.Zero, PriceCurrency: "USD", Metadata: map[string]any{"fileName": "test.matcha-skillpkg", "cloudEnvelope": map[string]any{"keyId": "test", "algorithm": "rsa-oaep-sha256", "ciphertextBase64": "YQ"}}, CreatedAt: now, UpdatedAt: now}
	entitlement := &model.MatchaPackageEntitlement{ID: 9, PackageID: 1, VersionID: &version.ID, UserID: userID, Status: model.MatchaPackageEntitlementStatusActive}
	registry := &fakePackageRegistry{pkg: pkg, version: version, entitlement: entitlement, object: "body"}
	registry.listRows = []service.MatchaPackageVersionListRow{{Package: pkg, Version: version, EntitlementStatus: entitlement.Status, Downloadable: true}}
	return packageRegistryFixture{userID: userID, pkg: pkg, version: version, entitlement: entitlement, registry: registry}
}

func packageRegistryTestContext(method, target, body string, userID int64) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, target, reader)
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	return w, c
}

type fakePackageRegistry struct {
	pkg            *model.MatchaPackage
	version        *model.MatchaPackageVersion
	entitlement    *model.MatchaPackageEntitlement
	entitlementErr error
	object         string
	uploadReq      service.UploadSealedPackageRequest
	uploadErr      error

	listRows       []service.MatchaPackageVersionListRow
	lastListFilter service.MatchaPackageVersionListFilter

	recordDownloadCalls int
	downloadForUserCall struct {
		versionID int64
		userID    int64
		remote    string
		ua        string
	}
}

func (f *fakePackageRegistry) GetPackage(_ context.Context, id int64) (*model.MatchaPackage, error) {
	if f.pkg != nil && f.pkg.ID == id {
		return f.pkg, nil
	}
	return nil, service.ErrMatchaPackageNotFound
}

func (f *fakePackageRegistry) GetVersion(_ context.Context, id int64) (*model.MatchaPackageVersion, error) {
	if f.version != nil && f.version.ID == id {
		return f.version, nil
	}
	return nil, service.ErrMatchaPackageVersionNotFound
}

func (f *fakePackageRegistry) GetActiveEntitlement(_ context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error) {
	if f.entitlementErr != nil {
		return nil, f.entitlementErr
	}
	if f.entitlement == nil || f.entitlement.PackageID != packageID || f.entitlement.UserID != userID || versionID == nil || f.entitlement.VersionID == nil || *f.entitlement.VersionID != *versionID {
		return nil, service.ErrMatchaPackageEntitlementNotFound
	}
	return f.entitlement, nil
}

func (f *fakePackageRegistry) ListPackageVersions(_ context.Context, filter service.MatchaPackageVersionListFilter) (*service.MatchaPackageVersionListResult, error) {
	f.lastListFilter = filter
	return &service.MatchaPackageVersionListResult{Rows: f.listRows, Total: int64(len(f.listRows))}, nil
}

func (f *fakePackageRegistry) UploadSealedPackage(_ context.Context, req service.UploadSealedPackageRequest) (*service.UploadSealedPackageResult, error) {
	f.uploadReq = req
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	return &service.UploadSealedPackageResult{Package: f.pkg, Version: f.version, Entitlement: f.entitlement}, nil
}

func (f *fakePackageRegistry) PublishPackageVersion(context.Context, service.PublishMatchaPackageVersionRequest) (*model.MatchaPackageVersion, error) {
	return f.version, nil
}

func (f *fakePackageRegistry) SealedCloudKey(context.Context) (*service.SealedPackageCloudKey, error) {
	return &service.SealedPackageCloudKey{KeyID: "test", Algorithm: "rsa-oaep-sha256", PublicKeySPKIPEM: testRSAPublicKeyPEM(nil)}, nil
}

func (f *fakePackageRegistry) CreateAuthorizationLease(_ context.Context, versionID, userID int64, devicePublicKey string) (*service.MatchaPackageAuthorizationLease, error) {
	if f.entitlementErr != nil {
		return nil, f.entitlementErr
	}
	if f.version == nil || f.version.ID != versionID || f.entitlement == nil || f.entitlement.UserID != userID || strings.TrimSpace(devicePublicKey) == "" {
		return nil, service.ErrMatchaPackageEntitlementNotFound
	}
	return &service.MatchaPackageAuthorizationLease{
		Package:        f.pkg,
		Version:        f.version,
		Entitlement:    f.entitlement,
		DeviceEnvelope: service.SealedPackageEnvelope{Algorithm: "rsa-oaep-sha256", CiphertextBase64: "ciphertext"},
		LeaseExpiresAt: time.Now().UTC().Add(15 * time.Minute),
	}, nil
}

func testRSAPublicKeyPEM(t *testing.T) string {
	if t != nil {
		t.Helper()
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		return ""
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		if t != nil {
			t.Fatal(err)
		}
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func (f *fakePackageRegistry) DownloadPackageObjectForUser(_ context.Context, versionID, userID int64, remoteAddr, userAgent string) (*service.MatchaPackageDownloadObject, error) {
	f.downloadForUserCall.versionID = versionID
	f.downloadForUserCall.userID = userID
	f.downloadForUserCall.remote = remoteAddr
	f.downloadForUserCall.ua = userAgent
	if f.object == "" {
		return nil, infraerrors.ServiceUnavailable("TEST_OBJECT_MISSING", "object missing")
	}
	if f.version == nil || f.version.ID != versionID || f.entitlement == nil || f.entitlement.UserID != userID {
		return nil, service.ErrMatchaPackageEntitlementNotFound
	}
	return &service.MatchaPackageDownloadObject{
		Reader:      io.NopCloser(strings.NewReader(f.object)),
		Package:     f.pkg,
		Version:     f.version,
		Entitlement: f.entitlement,
	}, nil
}
