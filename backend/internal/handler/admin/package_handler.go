package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	adminPackageStatusDraft         = "draft"
	adminPackageStatusPendingReview = "pending_review"
	adminPackageStatusApproved      = "approved"
	adminPackageStatusRejected      = "rejected"
	adminPackageStatusTakenDown     = "taken_down"

	packageMetadataAdminStatus        = "adminStatus"
	packageMetadataVersion            = "version"
	packageMetadataFileURL            = "fileURL"
	packageMetadataStorageKey         = "storageKey"
	packageMetadataSizeBytes          = "sizeBytes"
	packageMetadataChecksum           = "checksum"
	packageMetadataPlanBindings       = "planBindings"
	packageMetadataPermissionBindings = "permissionBindings"
	packageMetadataCreatedBy          = "createdBy"
	packageMetadataReviewedBy         = "reviewedBy"
	packageMetadataReviewedAt         = "reviewedAt"
	packageMetadataReviewNote         = "reviewNote"
	packageMetadataTakedownReason     = "takedownReason"
)

var errPackageRegistryUnavailable = infraerrors.ServiceUnavailable("PACKAGE_REGISTRY_UNAVAILABLE", "package registry service unavailable")

type PackageHandler struct {
	registry  *service.MatchaPackageRegistryService
	storage   *service.MatchaPackageStorageSettingService
	cloudKeys *service.MatchaPackageCloudKeyService
}

func NewPackageHandler(registry *service.MatchaPackageRegistryService, storage *service.MatchaPackageStorageSettingService, cloudKeys *service.MatchaPackageCloudKeyService) *PackageHandler {
	return &PackageHandler{registry: registry, storage: storage, cloudKeys: cloudKeys}
}

type AdminPackage struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	Version            string     `json:"version"`
	Status             string     `json:"status"`
	FileURL            string     `json:"file_url"`
	StorageKey         string     `json:"storage_key"`
	SizeBytes          int64      `json:"size_bytes"`
	Checksum           string     `json:"checksum"`
	PlanBindings       []string   `json:"plan_bindings"`
	PermissionBindings []string   `json:"permission_bindings"`
	DownloadCount      int64      `json:"download_count"`
	LastDownloadedAt   *time.Time `json:"last_downloaded_at,omitempty"`
	CreatedBy          *int64     `json:"created_by,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ReviewedBy         *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt         *time.Time `json:"reviewed_at,omitempty"`
	ReviewNote         string     `json:"review_note"`
	TakedownReason     string     `json:"takedown_reason"`
}

type PackageStorageConfig struct {
	Driver                    string     `json:"driver"`
	ReuseBackupS3             bool       `json:"reuse_backup_s3"`
	BasePath                  string     `json:"base_path"`
	PublicBaseURL             string     `json:"public_base_url"`
	Endpoint                  string     `json:"endpoint"`
	Region                    string     `json:"region"`
	Bucket                    string     `json:"bucket"`
	AccessKeyID               string     `json:"access_key_id"`
	SecretAccessKey           string     `json:"secret_access_key,omitempty"`
	SecretAccessKeyConfigured bool       `json:"secret_access_key_configured,omitempty"`
	Prefix                    string     `json:"prefix"`
	PresignExpiryHours        int        `json:"presign_expiry_hours"`
	ForcePathStyle            bool       `json:"force_path_style"`
	UpdatedBy                 *int64     `json:"updated_by,omitempty"`
	UpdatedAt                 *time.Time `json:"updated_at,omitempty"`
}

type PackageDownloadAudit struct {
	ID          int64     `json:"id"`
	PackageID   int64     `json:"package_id"`
	PackageName string    `json:"package_name"`
	ActorUserID *int64    `json:"actor_user_id,omitempty"`
	Action      string    `json:"action"`
	ClientIP    string    `json:"client_ip"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

type packageSaveRequest struct {
	Name               string   `json:"name"`
	Description        string   `json:"description"`
	Version            string   `json:"version"`
	Status             string   `json:"status"`
	FileURL            string   `json:"file_url"`
	StorageKey         string   `json:"storage_key"`
	SizeBytes          int64    `json:"size_bytes"`
	Checksum           string   `json:"checksum"`
	PlanBindings       []string `json:"plan_bindings"`
	PermissionBindings []string `json:"permission_bindings"`
}

type packageBindingRequest struct {
	PlanBindings       []string `json:"plan_bindings"`
	PermissionBindings []string `json:"permission_bindings"`
}

type packageReviewRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type packageTakedownRequest struct {
	Reason string `json:"reason"`
}

func (h *PackageHandler) List(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}

	status := strings.TrimSpace(c.Query("status"))
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	packages, err := registry.ListPackages(c.Request.Context(), service.MatchaPackageListFilter{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := downloadStats(c.Request.Context(), registry, packages)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	items := make([]AdminPackage, 0, len(packages))
	for _, pkg := range packages {
		item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[pkg.ID])
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if status != "" && item.Status != status {
			continue
		}
		if search != "" && !packageMatchesSearch(item, search) {
			continue
		}
		items = append(items, item)
	}
	sortAdminPackages(items)
	response.Success(c, items)
}

func (h *PackageHandler) Get(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	pkg, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := registry.CountDownloadsByPackage(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[id])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PackageHandler) Create(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	var request packageSaveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	request.Status = normalizeAdminPackageStatus(request.Status)
	if request.Status == "" {
		response.BadRequest(c, "invalid package status")
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		response.BadRequest(c, "package name is required")
		return
	}
	now := time.Now().UTC()
	metadata := metadataFromPackageRequest(nil, request, now, packageActorID(c))
	pkg, err := registry.CreatePackage(c.Request.Context(), service.CreateMatchaPackageRequest{
		PackageKey:  adminPackageKey(name, now),
		Name:        name,
		Description: strings.TrimSpace(request.Description),
		Status:      modelStatusForAdminPackage(request.Status),
		Metadata:    metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, service.MatchaPackageDownloadStats{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *PackageHandler) Update(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	var request packageSaveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	request.Status = normalizeAdminPackageStatus(request.Status)
	if request.Status == "" {
		response.BadRequest(c, "invalid package status")
		return
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		response.BadRequest(c, "package name is required")
		return
	}
	current, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	metadata := metadataFromPackageRequest(current.Metadata, request, time.Now().UTC(), nil)
	status := modelStatusForAdminPackage(request.Status)
	pkg, err := registry.UpdatePackage(c.Request.Context(), id, service.UpdateMatchaPackageRequest{
		Name:        &name,
		Description: stringPtr(strings.TrimSpace(request.Description)),
		Status:      &status,
		Metadata:    metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := registry.CountDownloadsByPackage(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[id])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PackageHandler) Delete(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	if err := registry.DeletePackage(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "package deleted"})
}

func (h *PackageHandler) UpdateBindings(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	var request packageBindingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	current, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	metadata := cloneMetadata(current.Metadata)
	metadata[packageMetadataPlanBindings] = sanitizeStringList(request.PlanBindings)
	metadata[packageMetadataPermissionBindings] = sanitizeStringList(request.PermissionBindings)
	pkg, err := registry.UpdatePackage(c.Request.Context(), id, service.UpdateMatchaPackageRequest{Metadata: metadata})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := registry.CountDownloadsByPackage(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[id])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PackageHandler) Review(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	var request packageReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	request.Status = strings.TrimSpace(request.Status)
	if request.Status != adminPackageStatusApproved && request.Status != adminPackageStatusRejected {
		response.BadRequest(c, "review status must be approved or rejected")
		return
	}
	current, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	now := time.Now().UTC()
	metadata := cloneMetadata(current.Metadata)
	metadata[packageMetadataAdminStatus] = request.Status
	metadata[packageMetadataReviewNote] = strings.TrimSpace(request.Note)
	metadata[packageMetadataReviewedAt] = now.Format(time.RFC3339Nano)
	if actor := packageActorID(c); actor != nil {
		metadata[packageMetadataReviewedBy] = *actor
	}
	status := modelStatusForAdminPackage(request.Status)
	pkg, err := registry.UpdatePackage(c.Request.Context(), id, service.UpdateMatchaPackageRequest{Status: &status, Metadata: metadata})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := registry.CountDownloadsByPackage(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[id])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PackageHandler) TakeDown(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	var request packageTakedownRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	current, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	metadata := cloneMetadata(current.Metadata)
	metadata[packageMetadataAdminStatus] = adminPackageStatusTakenDown
	metadata[packageMetadataTakedownReason] = strings.TrimSpace(request.Reason)
	status := model.MatchaPackageStatusDisabled
	pkg, err := registry.UpdatePackage(c.Request.Context(), id, service.UpdateMatchaPackageRequest{Status: &status, Metadata: metadata})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := registry.CountDownloadsByPackage(c.Request.Context(), []int64{id})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.adminPackageDTO(c.Request.Context(), registry, pkg, stats[id])
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *PackageHandler) ListDownloadAudits(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	packageID, _ := strconv.ParseInt(strings.TrimSpace(c.Query("package_id")), 10, 64)
	filter := service.MatchaPackageDownloadFilter{Limit: packageAuditLimit(c.Query("limit"))}
	if packageID > 0 {
		filter.PackageID = &packageID
	}
	downloads, err := registry.ListDownloads(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]PackageDownloadAudit, 0, len(downloads))
	packageNames := map[int64]string{}
	for _, download := range downloads {
		if download == nil {
			continue
		}
		name := packageNames[download.PackageID]
		if name == "" {
			pkg, err := registry.GetPackage(c.Request.Context(), download.PackageID)
			if err == nil && pkg != nil {
				name = pkg.Name
			}
			packageNames[download.PackageID] = name
		}
		items = append(items, PackageDownloadAudit{
			ID:          download.ID,
			PackageID:   download.PackageID,
			PackageName: name,
			ActorUserID: download.UserID,
			Action:      "download",
			ClientIP:    download.RemoteAddr,
			UserAgent:   download.UserAgent,
			CreatedAt:   download.DownloadedAt,
		})
	}
	response.Success(c, items)
}

func (h *PackageHandler) RecordDownloadAudit(c *gin.Context) {
	registry, ok := h.registryService(c)
	if !ok {
		return
	}
	id, ok := packageIDParam(c)
	if !ok {
		return
	}
	pkg, err := registry.GetPackage(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	version, err := latestPackageVersion(c.Request.Context(), registry, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if version == nil {
		response.ErrorFrom(c, service.ErrMatchaPackageVersionNotDownloadable)
		return
	}
	recorded, err := registry.RecordDownload(c.Request.Context(), &model.MatchaPackageDownload{
		PackageID:  pkg.ID,
		VersionID:  version.ID,
		UserID:     packageActorID(c),
		ObjectKey:  version.ObjectKey,
		SHA256:     version.SHA256,
		SizeBytes:  version.SizeBytes,
		RemoteAddr: c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, PackageDownloadAudit{
		ID:          recorded.ID,
		PackageID:   pkg.ID,
		PackageName: pkg.Name,
		ActorUserID: recorded.UserID,
		Action:      "download",
		ClientIP:    recorded.RemoteAddr,
		UserAgent:   recorded.UserAgent,
		CreatedAt:   recorded.DownloadedAt,
	})
}

func (h *PackageHandler) GetStorage(c *gin.Context) {
	settings := h.storageSettings(c.Request.Context())
	response.Success(c, packageStorageConfigFromService(settings))
}

func (h *PackageHandler) UpdateStorage(c *gin.Context) {
	var request PackageStorageConfig
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.storage == nil {
		response.ErrorFrom(c, errPackageRegistryUnavailable)
		return
	}
	updated, err := h.storage.Update(c.Request.Context(), service.MatchaPackageStorageSettings{
		Driver:             request.Driver,
		ReuseBackupS3:      request.ReuseBackupS3,
		BasePath:           request.BasePath,
		PublicBaseURL:      request.PublicBaseURL,
		Endpoint:           request.Endpoint,
		Region:             request.Region,
		Bucket:             request.Bucket,
		AccessKeyID:        request.AccessKeyID,
		SecretAccessKey:    request.SecretAccessKey,
		Prefix:             request.Prefix,
		PresignExpiryHours: request.PresignExpiryHours,
		ForcePathStyle:     request.ForcePathStyle,
	}, packageActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, packageStorageConfigFromService(updated))
}

func (h *PackageHandler) GetCloudKey(c *gin.Context) {
	if h == nil || h.cloudKeys == nil {
		response.ErrorFrom(c, errPackageRegistryUnavailable)
		return
	}
	status, err := h.cloudKeys.Status(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *PackageHandler) GenerateCloudKey(c *gin.Context) {
	if h == nil || h.cloudKeys == nil {
		response.ErrorFrom(c, errPackageRegistryUnavailable)
		return
	}
	status, err := h.cloudKeys.Generate(c.Request.Context(), packageActorID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, status)
}

func (h *PackageHandler) registryService(c *gin.Context) (*service.MatchaPackageRegistryService, bool) {
	if h == nil || h.registry == nil {
		response.ErrorFrom(c, errPackageRegistryUnavailable)
		return nil, false
	}
	return h.registry, true
}

func (h *PackageHandler) storageSettings(ctx context.Context) *service.MatchaPackageStorageSettings {
	if h == nil || h.storage == nil {
		return service.DefaultMatchaPackageStorageSettings()
	}
	settings, err := h.storage.Get(ctx)
	if err != nil {
		return service.DefaultMatchaPackageStorageSettings()
	}
	return settings
}

func (h *PackageHandler) adminPackageDTO(ctx context.Context, registry *service.MatchaPackageRegistryService, pkg *model.MatchaPackage, stats service.MatchaPackageDownloadStats) (AdminPackage, error) {
	if pkg == nil {
		return AdminPackage{}, service.ErrMatchaPackageNotFound
	}
	version, err := latestPackageVersion(ctx, registry, pkg.ID)
	if err != nil {
		return AdminPackage{}, err
	}
	return adminPackageFromModel(pkg, version, stats), nil
}

func adminPackageFromModel(pkg *model.MatchaPackage, version *model.MatchaPackageVersion, stats service.MatchaPackageDownloadStats) AdminPackage {
	metadata := pkg.Metadata
	item := AdminPackage{
		ID:                 pkg.ID,
		Name:               pkg.Name,
		Description:        pkg.Description,
		Version:            metadataString(metadata, packageMetadataVersion),
		Status:             adminStatusFromPackage(pkg),
		FileURL:            metadataString(metadata, packageMetadataFileURL),
		StorageKey:         metadataString(metadata, packageMetadataStorageKey),
		SizeBytes:          metadataInt64(metadata, packageMetadataSizeBytes),
		Checksum:           metadataString(metadata, packageMetadataChecksum),
		PlanBindings:       metadataStringSlice(metadata, packageMetadataPlanBindings),
		PermissionBindings: metadataStringSlice(metadata, packageMetadataPermissionBindings),
		DownloadCount:      stats.DownloadCount,
		LastDownloadedAt:   stats.LastDownloadedAt,
		CreatedBy:          metadataInt64Ptr(metadata, packageMetadataCreatedBy),
		CreatedAt:          pkg.CreatedAt,
		UpdatedAt:          pkg.UpdatedAt,
		ReviewedBy:         metadataInt64Ptr(metadata, packageMetadataReviewedBy),
		ReviewedAt:         metadataTimePtr(metadata, packageMetadataReviewedAt),
		ReviewNote:         metadataString(metadata, packageMetadataReviewNote),
		TakedownReason:     metadataString(metadata, packageMetadataTakedownReason),
	}
	if version != nil {
		if item.Version == "" {
			item.Version = version.Version
		}
		if item.StorageKey == "" {
			item.StorageKey = version.ObjectKey
		}
		if item.Checksum == "" {
			item.Checksum = version.SHA256
		}
		if item.SizeBytes == 0 {
			item.SizeBytes = version.SizeBytes
		}
	}
	return item
}

func latestPackageVersion(ctx context.Context, registry *service.MatchaPackageRegistryService, packageID int64) (*model.MatchaPackageVersion, error) {
	versions, err := registry.ListVersions(ctx, packageID)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}
	return versions[0], nil
}

func downloadStats(ctx context.Context, registry *service.MatchaPackageRegistryService, packages []*model.MatchaPackage) (map[int64]service.MatchaPackageDownloadStats, error) {
	ids := make([]int64, 0, len(packages))
	for _, pkg := range packages {
		if pkg != nil {
			ids = append(ids, pkg.ID)
		}
	}
	return registry.CountDownloadsByPackage(ctx, ids)
}

func metadataFromPackageRequest(current map[string]any, request packageSaveRequest, now time.Time, createdBy *int64) map[string]any {
	metadata := cloneMetadata(current)
	metadata[packageMetadataAdminStatus] = request.Status
	metadata[packageMetadataVersion] = strings.TrimSpace(request.Version)
	metadata[packageMetadataFileURL] = strings.TrimSpace(request.FileURL)
	metadata[packageMetadataStorageKey] = strings.TrimSpace(request.StorageKey)
	metadata[packageMetadataSizeBytes] = maxInt64(request.SizeBytes, 0)
	metadata[packageMetadataChecksum] = strings.TrimSpace(request.Checksum)
	metadata[packageMetadataPlanBindings] = sanitizeStringList(request.PlanBindings)
	metadata[packageMetadataPermissionBindings] = sanitizeStringList(request.PermissionBindings)
	if createdBy != nil {
		metadata[packageMetadataCreatedBy] = *createdBy
	}
	if metadata[packageMetadataCreatedBy] == nil && createdBy == nil {
		delete(metadata, packageMetadataCreatedBy)
	}
	_ = now
	return metadata
}

func adminStatusFromPackage(pkg *model.MatchaPackage) string {
	if pkg == nil {
		return adminPackageStatusDraft
	}
	if status := normalizeAdminPackageStatus(metadataString(pkg.Metadata, packageMetadataAdminStatus)); status != "" {
		return status
	}
	switch pkg.Status {
	case model.MatchaPackageStatusActive:
		return adminPackageStatusApproved
	case model.MatchaPackageStatusDisabled, model.MatchaPackageStatusArchived:
		return adminPackageStatusTakenDown
	default:
		return adminPackageStatusDraft
	}
}

func modelStatusForAdminPackage(status string) string {
	switch status {
	case adminPackageStatusApproved:
		return model.MatchaPackageStatusActive
	case adminPackageStatusRejected, adminPackageStatusTakenDown:
		return model.MatchaPackageStatusDisabled
	default:
		return model.MatchaPackageStatusDraft
	}
}

func normalizeAdminPackageStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return adminPackageStatusDraft
	}
	if validPackageStatus(status) {
		return status
	}
	return ""
}

func validPackageStatus(status string) bool {
	switch status {
	case adminPackageStatusDraft, adminPackageStatusPendingReview, adminPackageStatusApproved, adminPackageStatusRejected, adminPackageStatusTakenDown:
		return true
	default:
		return false
	}
}

func packageMatchesSearch(item AdminPackage, search string) bool {
	return strings.Contains(strings.ToLower(item.Name), search) ||
		strings.Contains(strings.ToLower(item.Description), search) ||
		strings.Contains(strings.ToLower(item.Version), search) ||
		strings.Contains(strings.ToLower(item.StorageKey), search) ||
		strings.Contains(strings.ToLower(item.FileURL), search) ||
		strings.Contains(strings.ToLower(item.Checksum), search)
}

func sortAdminPackages(items []AdminPackage) {
	for i := 1; i < len(items); i++ {
		item := items[i]
		j := i - 1
		for j >= 0 && items[j].UpdatedAt.Before(item.UpdatedAt) {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = item
	}
}

func sanitizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func packageAuditLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func packageIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid package ID")
		return 0, false
	}
	return id, true
}

func packageActorID(c *gin.Context) *int64 {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		id := subject.UserID
		return &id
	}
	return nil
}

func packageStorageConfigFromService(settings *service.MatchaPackageStorageSettings) PackageStorageConfig {
	if settings == nil {
		settings = service.DefaultMatchaPackageStorageSettings()
	}
	return PackageStorageConfig{
		Driver:                    settings.Driver,
		ReuseBackupS3:             settings.ReuseBackupS3,
		BasePath:                  settings.BasePath,
		PublicBaseURL:             settings.PublicBaseURL,
		Endpoint:                  settings.Endpoint,
		Region:                    settings.Region,
		Bucket:                    settings.Bucket,
		AccessKeyID:               settings.AccessKeyID,
		SecretAccessKeyConfigured: settings.SecretAccessKeyConfigured,
		Prefix:                    settings.Prefix,
		PresignExpiryHours:        settings.PresignExpiryHours,
		ForcePathStyle:            settings.ForcePathStyle,
		UpdatedBy:                 settings.UpdatedBy,
		UpdatedAt:                 settings.UpdatedAt,
	}
}

func adminPackageKey(name string, now time.Time) string {
	slug := strings.Trim(strings.ToLower(strings.TrimSpace(name)), "-")
	if slug == "" {
		slug = "package"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug = strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "package"
	}
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	return fmt.Sprintf("admin:%s:%s", slug, strconv.FormatInt(now.UnixNano(), 36))
}

func cloneMetadata(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+8)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func metadataString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	values, ok := metadata[key].([]any)
	if ok {
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok {
				out = append(out, s)
			}
		}
		return sanitizeStringList(out)
	}
	stringsValue, ok := metadata[key].([]string)
	if !ok {
		return []string{}
	}
	return sanitizeStringList(stringsValue)
}

func metadataInt64(metadata map[string]any, key string) int64 {
	if v := metadataInt64Ptr(metadata, key); v != nil {
		return *v
	}
	return 0
}

func metadataInt64Ptr(metadata map[string]any, key string) *int64 {
	switch value := metadata[key].(type) {
	case int64:
		return &value
	case int:
		v := int64(value)
		return &v
	case float64:
		v := int64(value)
		return &v
	case json.Number:
		if v, err := value.Int64(); err == nil {
			return &v
		}
	case string:
		if v, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
			return &v
		}
	}
	return nil
}

func metadataTimePtr(metadata map[string]any, key string) *time.Time {
	raw := metadataString(metadata, key)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func maxInt64(value, min int64) int64 {
	if value < min {
		return min
	}
	return value
}

func stringPtr(value string) *string { return &value }
