package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	ErrMatchaPackageNotFound            = infraerrors.NotFound("MATCHA_PACKAGE_NOT_FOUND", "matcha package not found")
	ErrMatchaPackageVersionNotFound     = infraerrors.NotFound("MATCHA_PACKAGE_VERSION_NOT_FOUND", "matcha package version not found")
	ErrMatchaPackageEntitlementNotFound = infraerrors.NotFound(
		"MATCHA_PACKAGE_ENTITLEMENT_NOT_FOUND",
		"matcha package entitlement not found",
	)
	ErrMatchaPackageExists        = infraerrors.Conflict("MATCHA_PACKAGE_EXISTS", "matcha package already exists")
	ErrMatchaPackageVersionExists = infraerrors.Conflict(
		"MATCHA_PACKAGE_VERSION_EXISTS",
		"matcha package version already exists",
	)
	ErrMatchaPackageStorageNotConfigured = infraerrors.BadRequest(
		"MATCHA_PACKAGE_STORAGE_NOT_CONFIGURED",
		"matcha package object storage is not configured",
	)
	ErrMatchaPackageVersionNotPublishable = infraerrors.Conflict(
		"MATCHA_PACKAGE_VERSION_NOT_PUBLISHABLE",
		"only draft package versions can be published",
	)
	ErrMatchaPackageVersionNotDownloadable = infraerrors.Forbidden(
		"MATCHA_PACKAGE_VERSION_NOT_DOWNLOADABLE",
		"package version is not downloadable",
	)
	ErrMatchaPackageBackupS3NotConfigured = infraerrors.BadRequest(
		"MATCHA_PACKAGE_BACKUP_S3_NOT_CONFIGURED",
		"backup S3 storage is not configured for package storage reuse",
	)
)

type MatchaPackageRepository interface {
	CreatePackage(ctx context.Context, pkg *model.MatchaPackage) (*model.MatchaPackage, error)
	GetPackage(ctx context.Context, id int64) (*model.MatchaPackage, error)
	DeletePackage(ctx context.Context, id int64) error
	GetPackageByKey(ctx context.Context, packageKey string) (*model.MatchaPackage, error)
	ListPackages(ctx context.Context, filter MatchaPackageListFilter) ([]*model.MatchaPackage, error)
	ListPackageVersions(ctx context.Context, filter MatchaPackageVersionListFilter) (*MatchaPackageVersionListResult, error)
	UpdatePackage(ctx context.Context, pkg *model.MatchaPackage) (*model.MatchaPackage, error)
	CreateVersion(ctx context.Context, version *model.MatchaPackageVersion) (*model.MatchaPackageVersion, error)
	CreateVersionWithEntitlement(ctx context.Context, version *model.MatchaPackageVersion, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageVersion, *model.MatchaPackageEntitlement, error)
	GetVersion(ctx context.Context, id int64) (*model.MatchaPackageVersion, error)
	GetVersionByName(ctx context.Context, packageID int64, version string) (*model.MatchaPackageVersion, error)
	ListVersions(ctx context.Context, packageID int64) ([]*model.MatchaPackageVersion, error)
	UpdateVersionStatus(ctx context.Context, id int64, status string) (*model.MatchaPackageVersion, error)
	PublishDraftVersion(ctx context.Context, id int64) (*model.MatchaPackageVersion, error)
	MergeVersionMetadata(ctx context.Context, id int64, metadata map[string]any) (*model.MatchaPackageVersion, error)
	GrantEntitlement(ctx context.Context, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageEntitlement, error)
	GetActiveEntitlement(ctx context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error)
	RevokeEntitlement(ctx context.Context, id int64) error
	RecordDownload(ctx context.Context, download *model.MatchaPackageDownload) (*model.MatchaPackageDownload, error)
	ListDownloads(ctx context.Context, filter MatchaPackageDownloadFilter) ([]*model.MatchaPackageDownload, error)
	CountDownloadsByPackage(ctx context.Context, packageIDs []int64) (map[int64]MatchaPackageDownloadStats, error)
	UpsertBillingPolicy(ctx context.Context, policy *model.MatchaPackageBillingPolicy) (*model.MatchaPackageBillingPolicy, error)
	ListActiveBillingPolicies(ctx context.Context, packageID int64, versionID *int64) ([]*model.MatchaPackageBillingPolicy, error)
	RecordUsage(ctx context.Context, usage *model.MatchaPackageUsageRecord, ledger *model.MatchaPackageRevenueLedgerEntry) (*model.MatchaPackageUsageRecord, error)
	ListRevenueLedger(ctx context.Context, filter MatchaPackageRevenueFilter) ([]*model.MatchaPackageRevenueLedgerEntry, error)
}

type MatchaPackageObjectStore interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) (sizeBytes int64, err error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignURL(ctx context.Context, key string) (string, error)
}

type MatchaPackageObjectStoreFactory func(ctx context.Context, cfg *MatchaPackageStorageConfig) (MatchaPackageObjectStore, error)

type MatchaPackageObjectStoreResolver func(ctx context.Context) (MatchaPackageObjectStore, error)

type MatchaPackageStorageConfig struct {
	Driver             string `json:"driver"`
	BasePath           string `json:"base_path"`
	Endpoint           string `json:"endpoint"`
	Region             string `json:"region"`
	Bucket             string `json:"bucket"`
	AccessKeyID        string `json:"access_key_id"`
	SecretAccessKey    string `json:"secret_access_key,omitempty"`
	Prefix             string `json:"prefix"`
	ForcePathStyle     bool   `json:"force_path_style"`
	PublicBaseURL      string `json:"public_base_url"`
	PresignExpiryHours int    `json:"presign_expiry_hours"`
}

func (c *MatchaPackageStorageConfig) IsConfigured() bool {
	if c == nil {
		return false
	}
	if c.Driver == "" || c.Driver == "local" {
		return strings.TrimSpace(c.BasePath) != ""
	}
	return c.Bucket != "" && c.AccessKeyID != "" && c.SecretAccessKey != ""
}

type MatchaPackageListFilter struct {
	OwnerUserID *int64
	Status      string
	Kind        string
	Runtime     string
	Search      string
}

type MatchaPackageVersionListFilter struct {
	OwnerUserID   *int64
	Status        string
	VersionStatus string
	Kind          string
	Runtime       string
	Search        string
	UserID        *int64
	Page          int
	PageSize      int
}

type MatchaPackageVersionListRow struct {
	Package           *model.MatchaPackage
	Version           *model.MatchaPackageVersion
	EntitlementStatus string
	Downloadable      bool
}

type MatchaPackageVersionListResult struct {
	Rows  []MatchaPackageVersionListRow
	Total int64
}

type MatchaPackageRevenueFilter struct {
	PackageID   *int64
	OwnerUserID *int64
	Status      string
}

type MatchaPackageDownloadFilter struct {
	PackageID *int64
	Limit     int
}

type MatchaPackageDownloadStats struct {
	DownloadCount    int64
	LastDownloadedAt *time.Time
}

type CreateMatchaPackageRequest struct {
	PackageKey  string         `json:"package_key"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	OwnerUserID *int64         `json:"owner_user_id"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata"`
}

type UpdateMatchaPackageRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	Status      *string        `json:"status"`
	Metadata    map[string]any `json:"metadata"`
}

type CreateMatchaPackageVersionRequest struct {
	PackageID       int64           `json:"package_id"`
	Version         string          `json:"version"`
	Kind            string          `json:"kind"`
	Runtime         string          `json:"runtime"`
	SHA256          string          `json:"sha256"`
	SizeBytes       int64           `json:"size_bytes"`
	ObjectKey       string          `json:"object_key"`
	OwnerUserID     *int64          `json:"owner_user_id"`
	Status          string          `json:"status"`
	PriceAmount     decimal.Decimal `json:"price_amount"`
	PriceCurrency   string          `json:"price_currency"`
	SurchargePolicy map[string]any  `json:"surcharge_policy"`
	Metadata        map[string]any  `json:"metadata"`
}

type GrantMatchaPackageEntitlementRequest struct {
	PackageID       int64  `json:"package_id"`
	VersionID       *int64 `json:"version_id"`
	UserID          int64  `json:"user_id"`
	GrantedByUserID *int64 `json:"granted_by_user_id"`
	Source          string `json:"source"`
}

type UpsertMatchaPackageBillingPolicyRequest struct {
	PackageID       int64           `json:"package_id"`
	VersionID       *int64          `json:"version_id"`
	Kind            string          `json:"kind"`
	Currency        string          `json:"currency"`
	Amount          decimal.Decimal `json:"amount"`
	Unit            string          `json:"unit"`
	Policy          map[string]any  `json:"policy"`
	Active          bool            `json:"active"`
	CreatedByUserID *int64          `json:"created_by_user_id"`
}

type RecordMatchaPackageUsageRequest struct {
	VersionID     int64           `json:"version_id"`
	UserID        int64           `json:"user_id"`
	EntitlementID *int64          `json:"entitlement_id"`
	RequestID     string          `json:"request_id"`
	Binding       string          `json:"binding"`
	Quantity      decimal.Decimal `json:"quantity"`
	Unit          string          `json:"unit"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Metadata      map[string]any  `json:"metadata"`
}

type MatchaPackageUploadRequest struct {
	PackageID   int64
	Version     string
	Kind        string
	Runtime     string
	SHA256      string
	ContentType string
	Body        io.Reader
}

type UploadSealedPackageRequest struct {
	UserID      int64
	Filename    string
	Raw         []byte
	ContentType string
}

type PublishMatchaPackageVersionRequest struct {
	UserID    int64
	VersionID int64
}

type UploadSealedPackageResult struct {
	Package     *model.MatchaPackage
	Version     *model.MatchaPackageVersion
	Entitlement *model.MatchaPackageEntitlement
}

type MatchaPackageUploadResult struct {
	ObjectKey string `json:"object_key"`
	SizeBytes int64  `json:"size_bytes"`
}

type MatchaPackageDownloadObject struct {
	Reader      io.ReadCloser
	Package     *model.MatchaPackage
	Version     *model.MatchaPackageVersion
	Entitlement *model.MatchaPackageEntitlement
}

type MatchaPackageAuthorizationLease struct {
	Package        *model.MatchaPackage
	Version        *model.MatchaPackageVersion
	Entitlement    *model.MatchaPackageEntitlement
	DeviceEnvelope SealedPackageEnvelope
	LeaseExpiresAt time.Time
}

type MatchaPackageDownloadAccess struct {
	Package     *model.MatchaPackage
	Version     *model.MatchaPackageVersion
	Entitlement *model.MatchaPackageEntitlement
}

type MatchaPackageRegistryService struct {
	repo      MatchaPackageRepository
	resolver  MatchaPackageObjectStoreResolver
	keyPrefix string
	cloudKeys *MatchaPackageCloudKeyService
}

func NewMatchaPackageRegistryService(repo MatchaPackageRepository, resolver MatchaPackageObjectStoreResolver, keyPrefix string, cloudKeys *MatchaPackageCloudKeyService) *MatchaPackageRegistryService {
	return &MatchaPackageRegistryService{
		repo:      repo,
		resolver:  resolver,
		keyPrefix: strings.Trim(keyPrefix, "/"),
		cloudKeys: cloudKeys,
	}
}

func (s *MatchaPackageRegistryService) CreatePackage(ctx context.Context, req CreateMatchaPackageRequest) (*model.MatchaPackage, error) {
	pkg := &model.MatchaPackage{
		PackageKey:  req.PackageKey,
		Name:        req.Name,
		Description: req.Description,
		OwnerUserID: req.OwnerUserID,
		Status:      req.Status,
		Metadata:    req.Metadata,
	}
	pkg.Normalize()
	if err := pkg.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreatePackage(ctx, pkg)
	if err != nil {
		return nil, fmt.Errorf("create matcha package: %w", err)
	}
	return created, nil
}

func (s *MatchaPackageRegistryService) GetPackage(ctx context.Context, id int64) (*model.MatchaPackage, error) {
	return s.repo.GetPackage(ctx, id)
}

func (s *MatchaPackageRegistryService) DeletePackage(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("INVALID_MATCHA_PACKAGE", "package id is required")
	}
	return s.repo.DeletePackage(ctx, id)
}

func (s *MatchaPackageRegistryService) GetPackageByKey(ctx context.Context, packageKey string) (*model.MatchaPackage, error) {
	return s.repo.GetPackageByKey(ctx, strings.TrimSpace(packageKey))
}

func (s *MatchaPackageRegistryService) SealedCloudKey(ctx context.Context) (*SealedPackageCloudKey, error) {
	if s == nil || s.cloudKeys == nil {
		return nil, ErrSealedPackageCloudKeyNotConfigured
	}
	return s.cloudKeys.Get(ctx)
}

func (s *MatchaPackageRegistryService) UploadSealedPackage(ctx context.Context, req UploadSealedPackageRequest) (*UploadSealedPackageResult, error) {
	if req.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_UPLOAD", "user_id is required")
	}
	var keyErr error
	meta, err := InspectSealedPackageUpload(req.Filename, req.Raw, func(keyID string) (*SealedPackageCloudKey, error) {
		var key *SealedPackageCloudKey
		key, keyErr = s.cloudKeys.Resolve(ctx, keyID)
		return key, keyErr
	})
	if err != nil {
		if keyErr != nil || errors.Is(err, ErrMatchaPackageCloudEnvelopeRequired) {
			return nil, err
		}
		return nil, infraerrors.BadRequest("MATCHA_PACKAGE_INVALID_UPLOAD", err.Error())
	}
	pkg, packageCreated, err := s.ensureOwnedPackage(ctx, req.UserID, meta)
	if err != nil {
		return nil, err
	}
	if version, err := s.repo.GetVersionByName(ctx, pkg.ID, meta.PackageVersion); err == nil {
		return s.reuseExistingUpload(ctx, pkg, version, req.UserID, req.Filename, meta)
	} else if !errors.Is(err, ErrMatchaPackageVersionNotFound) {
		return nil, err
	}

	uploaded, err := s.UploadPackageObject(ctx, MatchaPackageUploadRequest{
		PackageID:   pkg.ID,
		Version:     meta.PackageVersion,
		Kind:        meta.Kind,
		Runtime:     meta.Runtime,
		SHA256:      meta.PackageSHA256,
		ContentType: req.ContentType,
		Body:        bytes.NewReader(req.Raw),
	})
	if err != nil {
		return nil, err
	}
	versionModel := &model.MatchaPackageVersion{
		PackageID:       pkg.ID,
		Version:         meta.PackageVersion,
		Kind:            meta.Kind,
		Runtime:         meta.Runtime,
		SHA256:          meta.PackageSHA256,
		SizeBytes:       uploaded.SizeBytes,
		ObjectKey:       uploaded.ObjectKey,
		OwnerUserID:     &req.UserID,
		Status:          model.MatchaPackageVersionStatusDraft,
		PriceAmount:     decimal.Zero,
		PriceCurrency:   "USD",
		SurchargePolicy: map[string]any{},
		Metadata:        PackageUploadVersionMetadata(req.Filename, meta),
	}
	versionModel.Normalize()
	if err := versionModel.Validate(); err != nil {
		_ = s.DeletePackageObject(ctx, uploaded.ObjectKey)
		if packageCreated {
			_ = s.DeletePackage(ctx, pkg.ID)
		}
		return nil, err
	}
	entitlementModel := &model.MatchaPackageEntitlement{
		PackageID:       pkg.ID,
		UserID:          req.UserID,
		GrantedByUserID: &req.UserID,
		Source:          "owner_upload",
		Status:          model.MatchaPackageEntitlementStatusActive,
	}
	version, entitlement, err := s.repo.CreateVersionWithEntitlement(ctx, versionModel, entitlementModel)
	if err != nil {
		if errors.Is(err, ErrMatchaPackageVersionExists) {
			if existing, getErr := s.repo.GetVersionByName(ctx, pkg.ID, meta.PackageVersion); getErr == nil {
				if existing.ObjectKey != uploaded.ObjectKey {
					_ = s.DeletePackageObject(ctx, uploaded.ObjectKey)
				}
				return s.reuseExistingUpload(ctx, pkg, existing, req.UserID, req.Filename, meta)
			}
		}
		_ = s.DeletePackageObject(ctx, uploaded.ObjectKey)
		if packageCreated {
			_ = s.DeletePackage(ctx, pkg.ID)
		}
		return nil, err
	}
	return &UploadSealedPackageResult{Package: pkg, Version: version, Entitlement: entitlement}, nil
}

func (s *MatchaPackageRegistryService) PublishPackageVersion(ctx context.Context, req PublishMatchaPackageVersionRequest) (*model.MatchaPackageVersion, error) {
	if req.UserID <= 0 || req.VersionID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_PUBLISH", "user_id and version_id are required")
	}
	version, err := s.repo.GetVersion(ctx, req.VersionID)
	if err != nil {
		return nil, err
	}
	if version.OwnerUserID == nil || *version.OwnerUserID != req.UserID {
		return nil, infraerrors.Forbidden("MATCHA_PACKAGE_FORBIDDEN", "package version belongs to another user")
	}
	pkg, err := s.repo.GetPackage(ctx, version.PackageID)
	if err != nil {
		return nil, err
	}
	if pkg.OwnerUserID == nil || *pkg.OwnerUserID != req.UserID {
		return nil, infraerrors.Forbidden("MATCHA_PACKAGE_FORBIDDEN", "package belongs to another user")
	}
	if version.Status != model.MatchaPackageVersionStatusDraft && version.Status != model.MatchaPackageVersionStatusPublished {
		return nil, ErrMatchaPackageVersionNotPublishable
	}
	return s.repo.PublishDraftVersion(ctx, version.ID)
}

func (s *MatchaPackageRegistryService) ensureOwnedPackage(ctx context.Context, userID int64, meta SealedPackageUploadMetadata) (*model.MatchaPackage, bool, error) {
	pkg, err := s.repo.GetPackageByKey(ctx, meta.PackageKey)
	if err == nil {
		if pkg.OwnerUserID == nil || *pkg.OwnerUserID != userID {
			return nil, false, infraerrors.Forbidden("MATCHA_PACKAGE_FORBIDDEN", "package belongs to another user")
		}
		return pkg, false, nil
	}
	if !errors.Is(err, ErrMatchaPackageNotFound) {
		return nil, false, err
	}
	created, err := s.CreatePackage(ctx, CreateMatchaPackageRequest{
		PackageKey:  meta.PackageKey,
		Name:        meta.DisplayName,
		Description: meta.Description,
		OwnerUserID: &userID,
		Status:      model.MatchaPackageStatusActive,
	})
	if err != nil && errors.Is(err, ErrMatchaPackageExists) {
		pkg, getErr := s.repo.GetPackageByKey(ctx, meta.PackageKey)
		if getErr != nil {
			return nil, false, getErr
		}
		if pkg.OwnerUserID == nil || *pkg.OwnerUserID != userID {
			return nil, false, infraerrors.Forbidden("MATCHA_PACKAGE_FORBIDDEN", "package belongs to another user")
		}
		return pkg, false, nil
	}
	return created, err == nil, err
}

func (s *MatchaPackageRegistryService) reuseExistingUpload(ctx context.Context, pkg *model.MatchaPackage, version *model.MatchaPackageVersion, userID int64, filename string, meta SealedPackageUploadMetadata) (*UploadSealedPackageResult, error) {
	if version == nil || version.OwnerUserID == nil || *version.OwnerUserID != userID {
		return nil, infraerrors.Forbidden("MATCHA_PACKAGE_FORBIDDEN", "package version belongs to another user")
	}
	if version.SHA256 != meta.PackageSHA256 || version.Kind != meta.Kind || version.Runtime != meta.Runtime {
		return nil, infraerrors.Conflict("MATCHA_PACKAGE_VERSION_EXISTS", "existing package version does not match the uploaded object")
	}
	if _, err := packageVersionCloudEnvelope(version); err != nil {
		// Repair metadata only from the fully authenticated, byte-identical upload.
		version, err = s.repo.MergeVersionMetadata(ctx, version.ID, PackageUploadVersionMetadata(filename, meta))
		if err != nil {
			return nil, err
		}
	}
	entitlement, err := s.grantOwnerUploadEntitlement(ctx, pkg.ID, version.ID, userID)
	if err != nil {
		return nil, err
	}
	return &UploadSealedPackageResult{Package: pkg, Version: version, Entitlement: entitlement}, nil
}

func (s *MatchaPackageRegistryService) grantOwnerUploadEntitlement(ctx context.Context, packageID, versionID, userID int64) (*model.MatchaPackageEntitlement, error) {
	return s.GrantEntitlement(ctx, GrantMatchaPackageEntitlementRequest{
		PackageID:       packageID,
		VersionID:       &versionID,
		UserID:          userID,
		GrantedByUserID: &userID,
		Source:          "owner_upload",
	})
}

func PackageUploadVersionMetadata(filename string, meta SealedPackageUploadMetadata) map[string]any {
	metadata := map[string]any{
		"fileName":       safeMatchaPackageFilename(filename),
		"manifestSha256": meta.ManifestSHA256,
		"resourceKey":    meta.ResourceKey,
		"cloudEnvelope": map[string]any{
			"keyId":            meta.CloudEnvelope.KeyID,
			"algorithm":        meta.CloudEnvelope.Algorithm,
			"ciphertextBase64": meta.CloudEnvelope.CiphertextBase64,
		},
	}
	if strings.TrimSpace(meta.Description) != "" {
		metadata["description"] = strings.TrimSpace(meta.Description)
	}
	return metadata
}

func safeMatchaPackageFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".matcha-skillpkg") || strings.HasSuffix(lower, ".matcha-agentpkg") {
		return name
	}
	return ""
}

func (s *MatchaPackageRegistryService) ListPackages(ctx context.Context, filter MatchaPackageListFilter) ([]*model.MatchaPackage, error) {
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Kind = strings.TrimSpace(filter.Kind)
	filter.Runtime = strings.TrimSpace(filter.Runtime)
	filter.Search = strings.TrimSpace(filter.Search)
	return s.repo.ListPackages(ctx, filter)
}

func (s *MatchaPackageRegistryService) ListPackageVersions(ctx context.Context, filter MatchaPackageVersionListFilter) (*MatchaPackageVersionListResult, error) {
	filter.Status = strings.TrimSpace(filter.Status)
	filter.VersionStatus = strings.TrimSpace(filter.VersionStatus)
	filter.Kind = strings.TrimSpace(filter.Kind)
	filter.Runtime = strings.TrimSpace(filter.Runtime)
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 1000 {
		filter.PageSize = 1000
	}
	return s.repo.ListPackageVersions(ctx, filter)
}

func (s *MatchaPackageRegistryService) UpdatePackage(ctx context.Context, id int64, req UpdateMatchaPackageRequest) (*model.MatchaPackage, error) {
	pkg, err := s.repo.GetPackage(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		pkg.Name = *req.Name
	}
	if req.Description != nil {
		pkg.Description = *req.Description
	}
	if req.Status != nil {
		pkg.Status = *req.Status
	}
	if req.Metadata != nil {
		pkg.Metadata = req.Metadata
	}
	pkg.Normalize()
	if err := pkg.Validate(); err != nil {
		return nil, err
	}
	return s.repo.UpdatePackage(ctx, pkg)
}

func (s *MatchaPackageRegistryService) CreateVersion(ctx context.Context, req CreateMatchaPackageVersionRequest) (*model.MatchaPackageVersion, error) {
	version := &model.MatchaPackageVersion{
		PackageID:       req.PackageID,
		Version:         req.Version,
		Kind:            req.Kind,
		Runtime:         req.Runtime,
		SHA256:          req.SHA256,
		SizeBytes:       req.SizeBytes,
		ObjectKey:       req.ObjectKey,
		OwnerUserID:     req.OwnerUserID,
		Status:          req.Status,
		PriceAmount:     req.PriceAmount,
		PriceCurrency:   req.PriceCurrency,
		SurchargePolicy: req.SurchargePolicy,
		Metadata:        req.Metadata,
	}
	version.Normalize()
	if err := version.Validate(); err != nil {
		return nil, err
	}
	created, err := s.repo.CreateVersion(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("create matcha package version: %w", err)
	}
	return created, nil
}

func (s *MatchaPackageRegistryService) GetVersion(ctx context.Context, id int64) (*model.MatchaPackageVersion, error) {
	return s.repo.GetVersion(ctx, id)
}

func (s *MatchaPackageRegistryService) GetVersionByName(ctx context.Context, packageID int64, version string) (*model.MatchaPackageVersion, error) {
	return s.repo.GetVersionByName(ctx, packageID, strings.TrimSpace(version))
}

func (s *MatchaPackageRegistryService) GetActiveEntitlement(ctx context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error) {
	return s.repo.GetActiveEntitlement(ctx, packageID, userID, versionID)
}

func (s *MatchaPackageRegistryService) RecordDownload(ctx context.Context, download *model.MatchaPackageDownload) (*model.MatchaPackageDownload, error) {
	return s.repo.RecordDownload(ctx, download)
}

func (s *MatchaPackageRegistryService) ListDownloads(ctx context.Context, filter MatchaPackageDownloadFilter) ([]*model.MatchaPackageDownload, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 500 {
		filter.Limit = 500
	}
	return s.repo.ListDownloads(ctx, filter)
}

func (s *MatchaPackageRegistryService) CountDownloadsByPackage(ctx context.Context, packageIDs []int64) (map[int64]MatchaPackageDownloadStats, error) {
	return s.repo.CountDownloadsByPackage(ctx, packageIDs)
}

func (s *MatchaPackageRegistryService) DownloadPackageObject(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	storage, err := s.objectStore(ctx)
	if err != nil {
		return nil, err
	}
	return storage.Download(ctx, strings.TrimSpace(objectKey))
}

func (s *MatchaPackageRegistryService) DeletePackageObject(ctx context.Context, objectKey string) error {
	storage, err := s.objectStore(ctx)
	if err != nil {
		return err
	}
	return storage.Delete(ctx, strings.TrimSpace(objectKey))
}

func (s *MatchaPackageRegistryService) CreateAuthorizationLease(ctx context.Context, versionID, userID int64, devicePublicKey string) (*MatchaPackageAuthorizationLease, error) {
	access, err := s.CheckDownloadAccess(ctx, versionID, userID)
	if err != nil {
		return nil, err
	}
	cloudEnvelope, err := packageVersionCloudEnvelope(access.Version)
	if err != nil {
		return nil, err
	}
	cloudKey, err := s.cloudKeys.Resolve(ctx, cloudEnvelope.KeyID)
	if err != nil {
		return nil, err
	}
	contentKey, err := UnwrapSealedPackageCloudEnvelope(cloudKey, cloudEnvelope)
	if err != nil {
		return nil, infraerrors.BadRequest("MATCHA_PACKAGE_CLOUD_ENVELOPE_INVALID", err.Error())
	}
	defer clear(contentKey)
	deviceEnvelope, err := WrapSealedPackageDeviceEnvelope(contentKey, devicePublicKey)
	if err != nil {
		return nil, infraerrors.BadRequest("MATCHA_PACKAGE_DEVICE_PUBLIC_KEY_INVALID", err.Error())
	}
	now := time.Now().UTC()
	leaseExpiresAt := now.Add(15 * time.Minute)
	if expiresAt := access.Entitlement.ExpiresAt; expiresAt != nil && expiresAt.Before(leaseExpiresAt) {
		leaseExpiresAt = *expiresAt
	}
	if !leaseExpiresAt.After(now) {
		return nil, ErrMatchaPackageEntitlementNotFound
	}
	return &MatchaPackageAuthorizationLease{Package: access.Package, Version: access.Version, Entitlement: access.Entitlement, DeviceEnvelope: deviceEnvelope, LeaseExpiresAt: leaseExpiresAt}, nil
}

func (s *MatchaPackageRegistryService) CheckDownloadAccess(ctx context.Context, versionID, userID int64) (*MatchaPackageDownloadAccess, error) {
	if userID <= 0 {
		return nil, ErrMatchaPackageEntitlementNotFound
	}
	version, err := s.repo.GetVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	pkg, err := s.repo.GetPackage(ctx, version.PackageID)
	if err != nil {
		return nil, err
	}
	if pkg.Status != model.MatchaPackageStatusActive || version.Status != model.MatchaPackageVersionStatusPublished {
		return nil, ErrMatchaPackageVersionNotDownloadable
	}
	entitlement, err := s.repo.GetActiveEntitlement(ctx, pkg.ID, userID, &version.ID)
	if err != nil {
		return nil, err
	}
	if _, err := packageVersionCloudEnvelope(version); err != nil {
		return nil, err
	}
	return &MatchaPackageDownloadAccess{Package: pkg, Version: version, Entitlement: entitlement}, nil
}

func (s *MatchaPackageRegistryService) DownloadPackageObjectForUser(ctx context.Context, versionID, userID int64, remoteAddr, userAgent string) (*MatchaPackageDownloadObject, error) {
	access, err := s.CheckDownloadAccess(ctx, versionID, userID)
	if err != nil {
		return nil, err
	}
	storage, err := s.objectStore(ctx)
	if err != nil {
		return nil, err
	}
	pkg, version, entitlement := access.Package, access.Version, access.Entitlement
	reader, err := storage.Download(ctx, version.ObjectKey)
	if err != nil {
		return nil, fmt.Errorf("download matcha package object: %w", err)
	}
	if _, err := s.repo.RecordDownload(ctx, &model.MatchaPackageDownload{
		PackageID:     pkg.ID,
		VersionID:     version.ID,
		UserID:        &userID,
		EntitlementID: &entitlement.ID,
		ObjectKey:     version.ObjectKey,
		SHA256:        version.SHA256,
		SizeBytes:     version.SizeBytes,
		RemoteAddr:    strings.TrimSpace(remoteAddr),
		UserAgent:     strings.TrimSpace(userAgent),
	}); err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("record matcha package download: %w", err)
	}
	return &MatchaPackageDownloadObject{Reader: reader, Package: pkg, Version: version, Entitlement: entitlement}, nil
}

func MatchaPackageVersionDownloadable(pkg *model.MatchaPackage, version *model.MatchaPackageVersion) bool {
	if pkg == nil || version == nil || pkg.Status != model.MatchaPackageStatusActive || version.Status != model.MatchaPackageVersionStatusPublished {
		return false
	}
	_, err := packageVersionCloudEnvelope(version)
	return err == nil
}

func packageVersionCloudEnvelope(version *model.MatchaPackageVersion) (SealedPackageEnvelope, error) {
	if version == nil {
		return SealedPackageEnvelope{}, ErrMatchaPackageCloudEnvelopeRequired
	}
	value, ok := version.Metadata["cloudEnvelope"].(map[string]any)
	if !ok || len(value) == 0 {
		return SealedPackageEnvelope{}, ErrMatchaPackageCloudEnvelopeRequired
	}
	envelope := SealedPackageEnvelope{
		KeyID:            strings.TrimSpace(stringMapValue(value, "keyId")),
		Algorithm:        strings.TrimSpace(stringMapValue(value, "algorithm")),
		CiphertextBase64: strings.TrimSpace(stringMapValue(value, "ciphertextBase64")),
	}
	if envelope == (SealedPackageEnvelope{}) || envelope.Algorithm == "local" {
		return SealedPackageEnvelope{}, ErrMatchaPackageCloudEnvelopeRequired
	}
	if !validSealedPackageEnvelope(envelope) {
		return SealedPackageEnvelope{}, infraerrors.BadRequest("MATCHA_PACKAGE_CLOUD_ENVELOPE_INVALID", "package cloud envelope is invalid")
	}
	return envelope, nil
}

func stringMapValue(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return text
}

func (s *MatchaPackageRegistryService) UpdateVersionStatus(ctx context.Context, id int64, status string) (*model.MatchaPackageVersion, error) {
	version, err := s.repo.GetVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	version.Status = strings.TrimSpace(status)
	version.Normalize()
	if err := version.Validate(); err != nil {
		return nil, err
	}
	return s.repo.UpdateVersionStatus(ctx, id, version.Status)
}

func (s *MatchaPackageRegistryService) ListVersions(ctx context.Context, packageID int64) ([]*model.MatchaPackageVersion, error) {
	return s.repo.ListVersions(ctx, packageID)
}

func (s *MatchaPackageRegistryService) GrantEntitlement(ctx context.Context, req GrantMatchaPackageEntitlementRequest) (*model.MatchaPackageEntitlement, error) {
	if req.PackageID <= 0 || req.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_ENTITLEMENT", "package_id and user_id are required")
	}
	entitlement := &model.MatchaPackageEntitlement{
		PackageID:       req.PackageID,
		VersionID:       req.VersionID,
		UserID:          req.UserID,
		GrantedByUserID: req.GrantedByUserID,
		Source:          strings.TrimSpace(req.Source),
		Status:          model.MatchaPackageEntitlementStatusActive,
	}
	return s.repo.GrantEntitlement(ctx, entitlement)
}

func (s *MatchaPackageRegistryService) RevokeEntitlement(ctx context.Context, id int64) error {
	return s.repo.RevokeEntitlement(ctx, id)
}

func (s *MatchaPackageRegistryService) UploadPackageObject(ctx context.Context, req MatchaPackageUploadRequest) (*MatchaPackageUploadResult, error) {
	storage, err := s.objectStore(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_OBJECT", "package object body is required")
	}
	if req.PackageID <= 0 || strings.TrimSpace(req.Version) == "" || !model.IsMatchaPackageKind(strings.TrimSpace(req.Kind)) || strings.TrimSpace(req.Runtime) == "" {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_OBJECT", "package_id, version, kind and runtime are required")
	}
	key := s.objectKey(req)
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	sizeBytes, err := storage.Upload(ctx, key, req.Body, contentType)
	if err != nil {
		return nil, fmt.Errorf("upload matcha package object: %w", err)
	}
	return &MatchaPackageUploadResult{ObjectKey: key, SizeBytes: sizeBytes}, nil
}

func (s *MatchaPackageRegistryService) UpsertBillingPolicy(ctx context.Context, req UpsertMatchaPackageBillingPolicyRequest) (*model.MatchaPackageBillingPolicy, error) {
	policy := &model.MatchaPackageBillingPolicy{
		PackageID:       req.PackageID,
		VersionID:       req.VersionID,
		Kind:            strings.TrimSpace(req.Kind),
		Currency:        strings.ToUpper(strings.TrimSpace(req.Currency)),
		Amount:          req.Amount,
		Unit:            strings.TrimSpace(req.Unit),
		Policy:          req.Policy,
		Active:          req.Active,
		CreatedByUserID: req.CreatedByUserID,
	}
	if policy.PackageID <= 0 || policy.Unit == "" {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_BILLING_POLICY", "package_id and unit are required")
	}
	if policy.Kind == "" {
		policy.Kind = model.MatchaPackageBillingPolicySurcharge
	}
	if policy.Currency == "" {
		policy.Currency = "USD"
	}
	if policy.Amount.IsNegative() {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_BILLING_POLICY", "amount must be >= 0")
	}
	if policy.Policy == nil {
		policy.Policy = map[string]any{}
	}
	return s.repo.UpsertBillingPolicy(ctx, policy)
}

func (s *MatchaPackageRegistryService) RecordUsage(ctx context.Context, req RecordMatchaPackageUsageRequest) (*model.MatchaPackageUsageRecord, error) {
	version, err := s.repo.GetVersion(ctx, req.VersionID)
	if err != nil {
		return nil, err
	}
	usage := &model.MatchaPackageUsageRecord{
		PackageID:     version.PackageID,
		VersionID:     version.ID,
		UserID:        req.UserID,
		EntitlementID: req.EntitlementID,
		RequestID:     strings.TrimSpace(req.RequestID),
		Binding:       strings.TrimSpace(req.Binding),
		Quantity:      req.Quantity,
		Unit:          strings.TrimSpace(req.Unit),
		Amount:        req.Amount,
		Currency:      strings.ToUpper(strings.TrimSpace(req.Currency)),
		Metadata:      req.Metadata,
	}
	if usage.UserID <= 0 || usage.RequestID == "" || usage.Binding == "" || usage.Unit == "" {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_USAGE", "user_id, request_id, binding and unit are required")
	}
	if usage.Currency == "" {
		usage.Currency = "USD"
	}
	if usage.Quantity.IsNegative() || usage.Amount.IsNegative() {
		return nil, infraerrors.BadRequest("INVALID_MATCHA_PACKAGE_USAGE", "quantity and amount must be >= 0")
	}
	if usage.Metadata == nil {
		usage.Metadata = map[string]any{}
	}
	ledger := &model.MatchaPackageRevenueLedgerEntry{
		PackageID:   usage.PackageID,
		VersionID:   usage.VersionID,
		OwnerUserID: version.OwnerUserID,
		UserID:      usage.UserID,
		EntryType:   model.MatchaPackageRevenueEntryUsage,
		Amount:      usage.Amount,
		Currency:    usage.Currency,
		Status:      model.MatchaPackageRevenueStatusPending,
		Metadata:    usage.Metadata,
	}
	return s.repo.RecordUsage(ctx, usage, ledger)
}

func (s *MatchaPackageRegistryService) ListRevenueLedger(ctx context.Context, filter MatchaPackageRevenueFilter) ([]*model.MatchaPackageRevenueLedgerEntry, error) {
	filter.Status = strings.TrimSpace(filter.Status)
	return s.repo.ListRevenueLedger(ctx, filter)
}

func (s *MatchaPackageRegistryService) objectStore(ctx context.Context) (MatchaPackageObjectStore, error) {
	if s == nil || s.resolver == nil {
		return nil, ErrMatchaPackageStorageNotConfigured
	}
	storage, err := s.resolver(ctx)
	if err != nil {
		return nil, err
	}
	if storage == nil {
		return nil, ErrMatchaPackageStorageNotConfigured
	}
	return storage, nil
}

func (s *MatchaPackageRegistryService) objectKey(req MatchaPackageUploadRequest) string {
	parts := []string{s.keyPrefix, fmt.Sprintf("%d", req.PackageID), strings.TrimSpace(req.Kind), strings.TrimSpace(req.Runtime), strings.TrimSpace(req.Version), strings.ToLower(strings.TrimSpace(req.SHA256)) + ".tar.gz"}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.Trim(part, "/"); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, "/")
}
