package handler

import (
	"context"
	stderrors "errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const packageUploadMaxBytes = service.SealedSkillPackageObjectMaxBytes + (1 << 20)

var errPackageRegistryUnavailable = infraerrors.ServiceUnavailable("PACKAGE_REGISTRY_UNAVAILABLE", "package registry service unavailable")

type packageRegistryService interface {
	GetPackage(ctx context.Context, id int64) (*model.MatchaPackage, error)
	GetActiveEntitlement(ctx context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error)
	ListPackageVersions(ctx context.Context, filter service.MatchaPackageVersionListFilter) (*service.MatchaPackageVersionListResult, error)
	UploadSealedPackage(ctx context.Context, req service.UploadSealedPackageRequest) (*service.UploadSealedPackageResult, error)
	PublishPackageVersion(ctx context.Context, req service.PublishMatchaPackageVersionRequest) (*model.MatchaPackageVersion, error)
	SealedCloudKey(ctx context.Context) (*service.SealedPackageCloudKey, error)
	CreateAuthorizationLease(ctx context.Context, versionID, userID int64, devicePublicKey string) (*service.MatchaPackageAuthorizationLease, error)
	DownloadPackageObjectForUser(ctx context.Context, versionID, userID int64, remoteAddr, userAgent string) (*service.MatchaPackageDownloadObject, error)
}

var _ packageRegistryService = (*service.MatchaPackageRegistryService)(nil)

type PackageRegistryHandler struct {
	registry packageRegistryService
}

func NewPackageRegistryHandler(registry packageRegistryService) *PackageRegistryHandler {
	return &PackageRegistryHandler{registry: registry}
}

func ProvidePackageRegistryHandler(registry *service.MatchaPackageRegistryService) *PackageRegistryHandler {
	return NewPackageRegistryHandler(registry)
}

func (h *PackageRegistryHandler) ListMine(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	query := packageListQuery(c)
	result, err := registry.ListPackageVersions(c.Request.Context(), service.MatchaPackageVersionListFilter{
		OwnerUserID: &subject.UserID,
		Kind:        packageKindFilter(query.PackageType),
		Search:      query.Search,
		UserID:      &subject.UserID,
		Page:        query.Page,
		PageSize:    query.PageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, packageVersionRowsDTO(result.Rows), result.Total, query.Page, query.PageSize)
}

func (h *PackageRegistryHandler) SealedCloudKey(c *gin.Context) {
	registry, ok := h.service(c)
	if !ok {
		return
	}
	key, err := registry.SealedCloudKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.SealedCloudKeyResponse{
		Version:   1,
		KeyID:     key.KeyID,
		Algorithm: key.Algorithm,
		PublicKey: key.PublicKeySPKIPEM,
	})
}

func (h *PackageRegistryHandler) ListMarket(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	query := packageListQuery(c)
	result, err := registry.ListPackageVersions(c.Request.Context(), service.MatchaPackageVersionListFilter{
		Status:        model.MatchaPackageStatusActive,
		VersionStatus: model.MatchaPackageVersionStatusPublished,
		Kind:          packageKindFilter(query.PackageType),
		Search:        query.Search,
		UserID:        &subject.UserID,
		Page:          query.Page,
		PageSize:      query.PageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, packageVersionRowsDTO(result.Rows), result.Total, query.Page, query.PageSize)
}

func (h *PackageRegistryHandler) Upload(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, packageUploadMaxBytes)
	file, header, err := c.Request.FormFile("package")
	if err != nil {
		response.BadRequest(c, "请选择有效的 package 文件")
		return
	}
	defer func() { _ = file.Close() }()
	if !validPackageFilename(header.Filename) {
		response.BadRequest(c, "package 扩展名必须是 .matcha-skillpkg 或 .matcha-agentpkg")
		return
	}
	raw, err := io.ReadAll(file)
	if err != nil {
		response.BadRequest(c, "读取 package 文件失败")
		return
	}
	uploaded, err := registry.UploadSealedPackage(c.Request.Context(), service.UploadSealedPackageRequest{
		UserID:      subject.UserID,
		Filename:    header.Filename,
		Raw:         raw,
		ContentType: "application/octet-stream",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	downloadable := service.MatchaPackageVersionDownloadable(uploaded.Package, uploaded.Version) && uploaded.Entitlement != nil
	entitlementStatus := ""
	if uploaded.Entitlement != nil {
		entitlementStatus = uploaded.Entitlement.Status
	}
	response.Created(c, packageVersionDTO(uploaded.Package, uploaded.Version, downloadable, entitlementStatus))
}

func (h *PackageRegistryHandler) Publish(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	versionID, ok := packageVersionIDParam(c)
	if !ok {
		return
	}
	version, err := registry.PublishPackageVersion(c.Request.Context(), service.PublishMatchaPackageVersionRequest{UserID: subject.UserID, VersionID: versionID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pkg, err := registry.GetPackage(c.Request.Context(), version.PackageID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	entitlement, err := registry.GetActiveEntitlement(c.Request.Context(), pkg.ID, subject.UserID, &version.ID)
	if err != nil && !stderrors.Is(err, service.ErrMatchaPackageEntitlementNotFound) {
		response.ErrorFrom(c, err)
		return
	}
	entitlementStatus := ""
	if entitlement != nil {
		entitlementStatus = entitlement.Status
	}
	response.Success(c, packageVersionDTO(pkg, version, service.MatchaPackageVersionDownloadable(pkg, version) && entitlement != nil, entitlementStatus))
}

func (h *PackageRegistryHandler) Download(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	versionID, ok := packageVersionIDParam(c)
	if !ok {
		return
	}
	object, err := registry.DownloadPackageObjectForUser(c.Request.Context(), versionID, subject.UserID, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		packageDownloadError(c, err)
		return
	}
	if object == nil || object.Reader == nil || object.Package == nil || object.Version == nil {
		response.ErrorFrom(c, infraerrors.InternalServer("MATCHA_PACKAGE_DOWNLOAD_INVALID", "package download object is invalid"))
		return
	}
	h.streamPackageDownload(c, object.Reader, object.Package, object.Version)
}

func (h *PackageRegistryHandler) Authorize(c *gin.Context) {
	subject, ok := packageAuthSubject(c)
	if !ok {
		return
	}
	registry, ok := h.service(c)
	if !ok {
		return
	}
	versionID, ok := packageVersionIDParam(c)
	if !ok {
		return
	}
	var req dto.PackageAuthorizationRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DevicePublicKey) == "" {
		response.BadRequest(c, "devicePublicKey 必填")
		return
	}
	if strings.TrimSpace(req.PackageVersionID) != "" && strings.TrimSpace(req.PackageVersionID) != strconv.FormatInt(versionID, 10) {
		response.BadRequest(c, "packageVersionId 无效")
		return
	}
	lease, err := registry.CreateAuthorizationLease(c.Request.Context(), versionID, subject.UserID, req.DevicePublicKey)
	if err != nil {
		packageDownloadError(c, err)
		return
	}
	response.Success(c, dto.PackageAuthorizationResponse{
		PackageVersionID:  strconv.FormatInt(lease.Version.ID, 10),
		PackageType:       lease.Version.Kind,
		MeteringBinding:   packageMeteringBinding(lease.Package, lease.Version),
		EntitlementStatus: lease.Entitlement.Status,
		DeviceEnvelope: &dto.PackageEnvelope{
			KeyID:            lease.DeviceEnvelope.KeyID,
			Algorithm:        lease.DeviceEnvelope.Algorithm,
			CiphertextBase64: lease.DeviceEnvelope.CiphertextBase64,
		},
		LeaseExpiresAt: lease.LeaseExpiresAt,
	})
}

func (h *PackageRegistryHandler) service(c *gin.Context) (packageRegistryService, bool) {
	if h == nil || h.registry == nil {
		response.ErrorFrom(c, errPackageRegistryUnavailable)
		return nil, false
	}
	return h.registry, true
}

func packageVersionRowsDTO(rows []service.MatchaPackageVersionListRow) []dto.PackageVersion {
	items := make([]dto.PackageVersion, 0, len(rows))
	for _, row := range rows {
		items = append(items, packageVersionDTO(row.Package, row.Version, row.Downloadable, row.EntitlementStatus))
	}
	return items
}

func (h *PackageRegistryHandler) streamPackageDownload(c *gin.Context, reader io.ReadCloser, pkg *model.MatchaPackage, version *model.MatchaPackageVersion) {
	if reader == nil || pkg == nil || version == nil {
		response.ErrorFrom(c, infraerrors.InternalServer("MATCHA_PACKAGE_DOWNLOAD_INVALID", "package download object is invalid"))
		return
	}
	defer func() { _ = reader.Close() }()
	filename := packageVersionFilename(pkg, version)
	if filename != "" {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	}
	c.Header("X-Package-Version-Id", strconv.FormatInt(version.ID, 10))
	if binding := packageMeteringBinding(pkg, version); binding != nil && binding.ID != "" {
		c.Header("X-Matcha-Package-Metering", binding.ID)
	}
	c.DataFromReader(http.StatusOK, version.SizeBytes, "application/octet-stream", reader, nil)
}

func packageListQuery(c *gin.Context) dto.PackageListQuery {
	page, pageSize := response.ParsePagination(c)
	return dto.PackageListQuery{
		Page:        page,
		PageSize:    pageSize,
		Search:      strings.TrimSpace(c.Query("search")),
		PackageType: strings.TrimSpace(c.Query("packageType")),
	}
}

func packageAuthSubject(c *gin.Context) (middleware2.AuthSubject, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return middleware2.AuthSubject{}, false
	}
	return subject, true
}

func packageVersionIDParam(c *gin.Context) (int64, bool) {
	versionID, err := strconv.ParseInt(strings.TrimSpace(c.Param("packageVersionId")), 10, 64)
	if err != nil || versionID <= 0 {
		response.BadRequest(c, "packageVersionId 无效")
		return 0, false
	}
	return versionID, true
}

func validPackageFilename(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasSuffix(lower, ".matcha-skillpkg") || strings.HasSuffix(lower, ".matcha-agentpkg")
}

func safePackageFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if !validPackageFilename(name) {
		return ""
	}
	return name
}

func packageKindFilter(packageType string) string {
	switch strings.ToLower(strings.TrimSpace(packageType)) {
	case model.MatchaPackageKindAgent:
		return model.MatchaPackageKindAgent
	case model.MatchaPackageKindSkill:
		return model.MatchaPackageKindSkill
	default:
		return ""
	}
}

func packageVersionDTO(pkg *model.MatchaPackage, version *model.MatchaPackageVersion, downloadable bool, entitlementStatus string) dto.PackageVersion {
	createdAt := version.CreatedAt
	updatedAt := version.UpdatedAt
	return dto.PackageVersion{
		PackageID:         strconv.FormatInt(pkg.ID, 10),
		PackageVersionID:  strconv.FormatInt(version.ID, 10),
		Name:              pkg.PackageKey,
		DisplayName:       pkg.Name,
		PackageType:       version.Kind,
		Version:           version.Version,
		Description:       packageVersionDescription(pkg, version),
		Status:            version.Status,
		EntitlementStatus: entitlementStatus,
		Downloadable:      downloadable,
		MeteringBinding:   packageMeteringBinding(pkg, version),
		CreatedAt:         &createdAt,
		UpdatedAt:         &updatedAt,
	}
}

func packageVersionDescription(pkg *model.MatchaPackage, version *model.MatchaPackageVersion) string {
	if version != nil {
		if description, _ := version.Metadata["description"].(string); strings.TrimSpace(description) != "" {
			return strings.TrimSpace(description)
		}
	}
	if pkg == nil {
		return ""
	}
	return pkg.Description
}

func packageVersionFilename(pkg *model.MatchaPackage, version *model.MatchaPackageVersion) string {
	if version == nil {
		return ""
	}
	if fileName, _ := version.Metadata["fileName"].(string); strings.TrimSpace(fileName) != "" {
		if safe := safePackageFilename(fileName); safe != "" {
			return safe
		}
	}
	name := strconv.FormatInt(version.ID, 10)
	if pkg != nil && pkg.PackageKey != "" {
		name = strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(pkg.PackageKey)
	}
	if version.Kind == model.MatchaPackageKindAgent {
		return name + ".matcha-agentpkg"
	}
	return name + ".matcha-skillpkg"
}

func packageMeteringBinding(pkg *model.MatchaPackage, version *model.MatchaPackageVersion) *dto.PackageMeteringBinding {
	if pkg == nil || version == nil || version.SHA256 == "" {
		return nil
	}
	usage := "turn"
	if version.Kind == model.MatchaPackageKindAgent {
		usage = "session"
	}
	key := pkg.PackageKey
	if resourceKey, _ := version.Metadata["resourceKey"].(string); strings.TrimSpace(resourceKey) != "" {
		key = strings.TrimSpace(resourceKey)
	}
	binding, ok := service.BuildPackageMeteringBinding(service.PackageMeteringBindingPayload{
		V:             1,
		Kind:          version.Kind,
		Runtime:       version.Runtime,
		PackageSHA256: version.SHA256,
		Key:           key,
		Use:           usage,
	})
	if !ok {
		return nil
	}
	amount, _ := version.PriceAmount.Float64()
	currency := strings.TrimSpace(version.PriceCurrency)
	if currency == "" {
		currency = "USD"
	}
	return &dto.PackageMeteringBinding{
		ID:       binding,
		Type:     version.Kind,
		Unit:     usage,
		Amount:   amount,
		Currency: currency,
	}
}

func packageDownloadError(c *gin.Context, err error) {
	if stderrors.Is(err, service.ErrMatchaPackageEntitlementNotFound) {
		response.Forbidden(c, "package entitlement is required")
		return
	}
	response.ErrorFrom(c, err)
}
