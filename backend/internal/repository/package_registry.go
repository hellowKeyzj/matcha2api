package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

type matchaPackageRepository struct {
	db *sql.DB
}

var _ service.MatchaPackageRepository = (*matchaPackageRepository)(nil)

func NewMatchaPackageRepository(db *sql.DB) service.MatchaPackageRepository {
	return &matchaPackageRepository{db: db}
}

func (r *matchaPackageRepository) CreatePackage(ctx context.Context, pkg *model.MatchaPackage) (*model.MatchaPackage, error) {
	metadata, err := jsonMapParam(pkg.Metadata)
	if err != nil {
		return nil, err
	}
	created, err := scanMatchaPackage(r.db.QueryRowContext(ctx, `
		INSERT INTO matcha_packages (package_key, name, description, owner_user_id, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, package_key, name, description, owner_user_id, status, metadata, created_at, updated_at`,
		pkg.PackageKey, pkg.Name, pkg.Description, nullableInt64(pkg.OwnerUserID), pkg.Status, metadata,
	))
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrMatchaPackageExists)
	}
	return created, nil
}

func (r *matchaPackageRepository) GetPackage(ctx context.Context, id int64) (*model.MatchaPackage, error) {
	pkg, err := scanMatchaPackage(r.db.QueryRowContext(ctx, `
		SELECT id, package_key, name, description, owner_user_id, status, metadata, created_at, updated_at
		FROM matcha_packages
		WHERE id = $1`, id))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageNotFound, nil)
	}
	return pkg, nil
}

func (r *matchaPackageRepository) DeletePackage(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM matcha_packages WHERE id = $1`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return service.ErrMatchaPackageNotFound
	}
	return nil
}

func (r *matchaPackageRepository) GetPackageByKey(ctx context.Context, packageKey string) (*model.MatchaPackage, error) {
	pkg, err := scanMatchaPackage(r.db.QueryRowContext(ctx, `
		SELECT id, package_key, name, description, owner_user_id, status, metadata, created_at, updated_at
		FROM matcha_packages
		WHERE package_key = $1`, packageKey))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageNotFound, nil)
	}
	return pkg, nil
}

func (r *matchaPackageRepository) ListPackages(ctx context.Context, filter service.MatchaPackageListFilter) ([]*model.MatchaPackage, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if filter.OwnerUserID != nil {
		args = append(args, *filter.OwnerUserID)
		where = append(where, fmt.Sprintf("p.owner_user_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("p.status = $%d", len(args)))
	}
	versionWhere := []string{"v.package_id = p.id"}
	if filter.Kind != "" {
		args = append(args, filter.Kind)
		versionWhere = append(versionWhere, fmt.Sprintf("v.kind = $%d", len(args)))
	}
	if filter.Runtime != "" {
		args = append(args, filter.Runtime)
		versionWhere = append(versionWhere, fmt.Sprintf("v.runtime = $%d", len(args)))
	}
	if len(versionWhere) > 1 {
		where = append(where, "EXISTS (SELECT 1 FROM matcha_package_versions v WHERE "+strings.Join(versionWhere, " AND ")+")")
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		where = append(where, fmt.Sprintf("(LOWER(p.package_key) LIKE $%[1]d OR LOWER(p.name) LIKE $%[1]d OR LOWER(p.description) LIKE $%[1]d OR EXISTS (SELECT 1 FROM matcha_package_versions v WHERE v.package_id = p.id AND LOWER(v.version) LIKE $%[1]d))", len(args)))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.package_key, p.name, p.description, p.owner_user_id, p.status, p.metadata, p.created_at, p.updated_at
		FROM matcha_packages p
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY p.created_at DESC, p.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	packages := []*model.MatchaPackage{}
	for rows.Next() {
		pkg, err := scanMatchaPackage(rows)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}

func (r *matchaPackageRepository) ListPackageVersions(ctx context.Context, filter service.MatchaPackageVersionListFilter) (*service.MatchaPackageVersionListResult, error) {
	where, args := matchaPackageVersionListWhere(filter)
	countQuery := `
		SELECT COUNT(*)
		FROM matcha_package_versions v
		JOIN matcha_packages p ON p.id = v.package_id
		WHERE ` + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 1000 {
		filter.PageSize = 1000
	}
	maxInt := int(^uint(0) >> 1)
	if filter.Page > maxInt/filter.PageSize {
		return &service.MatchaPackageVersionListResult{Rows: []service.MatchaPackageVersionListRow{}, Total: total}, nil
	}
	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, nullableInt64(filter.UserID), filter.PageSize, offset)
	userParam := strconv.Itoa(len(args) + 1)
	limitParam := strconv.Itoa(len(args) + 2)
	offsetParam := strconv.Itoa(len(args) + 3)
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.package_key, p.name, p.description, p.owner_user_id, p.status, p.metadata, p.created_at, p.updated_at,
		       v.id, v.package_id, v.version, v.kind, v.runtime, v.sha256, v.size_bytes, v.object_key, v.owner_user_id,
		       v.status, v.price_amount, v.price_currency, v.surcharge_policy, v.metadata, v.created_at, v.updated_at, v.published_at,
		       e.status
		FROM matcha_package_versions v
		JOIN matcha_packages p ON p.id = v.package_id
		LEFT JOIN LATERAL (
			SELECT status
			FROM matcha_package_entitlements e
			WHERE e.package_id = p.id
			  AND $`+userParam+`::bigint IS NOT NULL
			  AND e.user_id = $`+userParam+`::bigint
			  AND e.status = 'active'
			  AND e.starts_at <= NOW()
			  AND (e.expires_at IS NULL OR e.expires_at > NOW())
			  AND (e.version_id IS NULL OR e.version_id = v.id)
			ORDER BY CASE WHEN e.version_id = v.id THEN 0 ELSE 1 END, e.id DESC
			LIMIT 1
		) e ON TRUE
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY COALESCE(v.published_at, v.created_at) DESC, v.id DESC
		LIMIT $`+limitParam+` OFFSET $`+offsetParam, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.MatchaPackageVersionListRow{}
	for rows.Next() {
		pkg, version, entitlementStatus, err := scanMatchaPackageVersionListRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, service.MatchaPackageVersionListRow{
			Package:           pkg,
			Version:           version,
			EntitlementStatus: entitlementStatus,
			Downloadable:      service.MatchaPackageVersionDownloadable(pkg, version) && entitlementStatus == model.MatchaPackageEntitlementStatusActive,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.MatchaPackageVersionListResult{Rows: out, Total: total}, nil
}

func matchaPackageVersionListWhere(filter service.MatchaPackageVersionListFilter) ([]string, []any) {
	args := []any{}
	where := []string{"1 = 1"}
	if filter.OwnerUserID != nil {
		args = append(args, *filter.OwnerUserID)
		where = append(where, fmt.Sprintf("p.owner_user_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("p.status = $%d", len(args)))
	}
	if filter.VersionStatus != "" {
		args = append(args, filter.VersionStatus)
		where = append(where, fmt.Sprintf("v.status = $%d", len(args)))
	}
	if filter.Kind != "" {
		args = append(args, filter.Kind)
		where = append(where, fmt.Sprintf("v.kind = $%d", len(args)))
	}
	if filter.Runtime != "" {
		args = append(args, filter.Runtime)
		where = append(where, fmt.Sprintf("v.runtime = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		where = append(where, fmt.Sprintf(`(
			LOWER(p.package_key) LIKE $%[1]d OR LOWER(p.name) LIKE $%[1]d OR LOWER(p.description) LIKE $%[1]d OR
			LOWER(v.version) LIKE $%[1]d OR LOWER(v.kind) LIKE $%[1]d OR LOWER(v.status) LIKE $%[1]d OR
			LOWER(COALESCE(v.metadata->>'description', '')) LIKE $%[1]d
		)`, len(args)))
	}
	return where, args
}

func scanMatchaPackageVersionListRow(row rowScanner) (*model.MatchaPackage, *model.MatchaPackageVersion, string, error) {
	pkg := &model.MatchaPackage{}
	version := &model.MatchaPackageVersion{}
	var pkgOwner sql.NullInt64
	var pkgMetadata []byte
	var versionOwner sql.NullInt64
	var published sql.NullTime
	var priceAmount string
	var surchargePolicy []byte
	var versionMetadata []byte
	var entitlementStatus sql.NullString
	if err := row.Scan(
		&pkg.ID, &pkg.PackageKey, &pkg.Name, &pkg.Description, &pkgOwner, &pkg.Status, &pkgMetadata, &pkg.CreatedAt, &pkg.UpdatedAt,
		&version.ID, &version.PackageID, &version.Version, &version.Kind, &version.Runtime, &version.SHA256, &version.SizeBytes, &version.ObjectKey, &versionOwner,
		&version.Status, &priceAmount, &version.PriceCurrency, &surchargePolicy, &versionMetadata, &version.CreatedAt, &version.UpdatedAt, &published,
		&entitlementStatus,
	); err != nil {
		return nil, nil, "", err
	}
	pkg.OwnerUserID = int64Ptr(pkgOwner)
	var err error
	pkg.Metadata, err = scanJSONMap(pkgMetadata)
	if err != nil {
		return nil, nil, "", err
	}
	version.PriceAmount, err = decimal.NewFromString(priceAmount)
	if err != nil {
		return nil, nil, "", err
	}
	version.OwnerUserID = int64Ptr(versionOwner)
	version.PublishedAt = timePtr(published)
	version.SurchargePolicy, err = scanJSONMap(surchargePolicy)
	if err != nil {
		return nil, nil, "", err
	}
	version.Metadata, err = scanJSONMap(versionMetadata)
	if err != nil {
		return nil, nil, "", err
	}
	return pkg, version, entitlementStatus.String, nil
}

func (r *matchaPackageRepository) UpdatePackage(ctx context.Context, pkg *model.MatchaPackage) (*model.MatchaPackage, error) {
	metadata, err := jsonMapParam(pkg.Metadata)
	if err != nil {
		return nil, err
	}
	updated, err := scanMatchaPackage(r.db.QueryRowContext(ctx, `
		UPDATE matcha_packages
		SET name = $2,
		    description = $3,
		    status = $4,
		    metadata = $5::jsonb,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, package_key, name, description, owner_user_id, status, metadata, created_at, updated_at`,
		pkg.ID, pkg.Name, pkg.Description, pkg.Status, metadata,
	))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageNotFound, service.ErrMatchaPackageExists)
	}
	return updated, nil
}

func (r *matchaPackageRepository) CreateVersion(ctx context.Context, version *model.MatchaPackageVersion) (*model.MatchaPackageVersion, error) {
	created, err := createMatchaPackageVersion(ctx, r.db, version)
	if err != nil {
		return nil, translatePersistenceError(err, nil, service.ErrMatchaPackageVersionExists)
	}
	return created, nil
}

func (r *matchaPackageRepository) CreateVersionWithEntitlement(ctx context.Context, version *model.MatchaPackageVersion, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageVersion, *model.MatchaPackageEntitlement, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	createdVersion, err := createMatchaPackageVersion(ctx, tx, version)
	if err != nil {
		return nil, nil, translatePersistenceError(err, nil, service.ErrMatchaPackageVersionExists)
	}
	versionID := createdVersion.ID
	entitlement.PackageID = createdVersion.PackageID
	entitlement.VersionID = &versionID
	createdEntitlement, err := grantMatchaPackageEntitlement(ctx, tx, entitlement)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return createdVersion, createdEntitlement, nil
}

func createMatchaPackageVersion(ctx context.Context, db sqlRowExecutor, version *model.MatchaPackageVersion) (*model.MatchaPackageVersion, error) {
	surchargePolicy, err := jsonMapParam(version.SurchargePolicy)
	if err != nil {
		return nil, err
	}
	metadata, err := jsonMapParam(version.Metadata)
	if err != nil {
		return nil, err
	}
	return scanMatchaPackageVersion(db.QueryRowContext(ctx, `
		INSERT INTO matcha_package_versions (
			package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, published_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::varchar, $10, $11, $12::jsonb, $13::jsonb,
			CASE WHEN $9::varchar = 'published' THEN NOW() ELSE NULL END)
		RETURNING id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at`,
		version.PackageID,
		version.Version,
		version.Kind,
		version.Runtime,
		version.SHA256,
		version.SizeBytes,
		version.ObjectKey,
		nullableInt64(version.OwnerUserID),
		version.Status,
		version.PriceAmount.String(),
		version.PriceCurrency,
		surchargePolicy,
		metadata,
	))
}

func (r *matchaPackageRepository) GetVersion(ctx context.Context, id int64) (*model.MatchaPackageVersion, error) {
	version, err := scanMatchaPackageVersion(r.db.QueryRowContext(ctx, `
		SELECT id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at
		FROM matcha_package_versions
		WHERE id = $1`, id))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageVersionNotFound, nil)
	}
	return version, nil
}

func (r *matchaPackageRepository) GetVersionByName(ctx context.Context, packageID int64, versionName string) (*model.MatchaPackageVersion, error) {
	version, err := scanMatchaPackageVersion(r.db.QueryRowContext(ctx, `
		SELECT id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at
		FROM matcha_package_versions
		WHERE package_id = $1 AND version = $2`, packageID, versionName))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageVersionNotFound, nil)
	}
	return version, nil
}

func (r *matchaPackageRepository) ListVersions(ctx context.Context, packageID int64) ([]*model.MatchaPackageVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at
		FROM matcha_package_versions
		WHERE package_id = $1
		ORDER BY created_at DESC, id DESC`, packageID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	versions := []*model.MatchaPackageVersion{}
	for rows.Next() {
		version, err := scanMatchaPackageVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (r *matchaPackageRepository) UpdateVersionStatus(ctx context.Context, id int64, status string) (*model.MatchaPackageVersion, error) {
	version, err := scanMatchaPackageVersion(r.db.QueryRowContext(ctx, `
		UPDATE matcha_package_versions
		SET status = $2,
		    published_at = CASE WHEN $2 = 'published' THEN COALESCE(published_at, NOW()) ELSE published_at END,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at`, id, status))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageVersionNotFound, nil)
	}
	return version, nil
}

func (r *matchaPackageRepository) MergeVersionMetadata(ctx context.Context, id int64, metadata map[string]any) (*model.MatchaPackageVersion, error) {
	data, err := jsonMapParam(metadata)
	if err != nil {
		return nil, err
	}
	version, err := scanMatchaPackageVersion(r.db.QueryRowContext(ctx, `
		UPDATE matcha_package_versions
		SET metadata = metadata || $2::jsonb, updated_at = NOW()
		WHERE id = $1
		RETURNING id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at`, id, data))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageVersionNotFound, nil)
	}
	return version, nil
}

func (r *matchaPackageRepository) PublishDraftVersion(ctx context.Context, id int64) (*model.MatchaPackageVersion, error) {
	version, err := scanMatchaPackageVersion(r.db.QueryRowContext(ctx, `
		UPDATE matcha_package_versions
		SET status = 'published',
		    published_at = COALESCE(published_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'draft'
		RETURNING id, package_id, version, kind, runtime, sha256, size_bytes, object_key, owner_user_id,
			status, price_amount, price_currency, surcharge_policy, metadata, created_at, updated_at, published_at`, id))
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	version, err = r.GetVersion(ctx, id)
	if err != nil {
		return nil, err
	}
	if version.Status == model.MatchaPackageVersionStatusPublished {
		return version, nil
	}
	return nil, service.ErrMatchaPackageVersionNotPublishable
}

func (r *matchaPackageRepository) GrantEntitlement(ctx context.Context, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageEntitlement, error) {
	return grantMatchaPackageEntitlement(ctx, r.db, entitlement)
}

func grantMatchaPackageEntitlement(ctx context.Context, db sqlRowExecutor, entitlement *model.MatchaPackageEntitlement) (*model.MatchaPackageEntitlement, error) {
	source := strings.TrimSpace(entitlement.Source)
	if source == "" {
		source = "manual"
	}
	granted, err := scanMatchaPackageEntitlement(db.QueryRowContext(ctx, `
		INSERT INTO matcha_package_entitlements (package_id, version_id, user_id, granted_by_user_id, source, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		ON CONFLICT (package_id, user_id, version_key) WHERE status = 'active'
		DO UPDATE SET granted_by_user_id = EXCLUDED.granted_by_user_id,
		              source = EXCLUDED.source,
		              starts_at = NOW(),
		              expires_at = NULL,
		              updated_at = NOW()
		RETURNING id, package_id, version_id, user_id, granted_by_user_id, source, status, starts_at, expires_at, created_at, updated_at`,
		entitlement.PackageID,
		nullableInt64(entitlement.VersionID),
		entitlement.UserID,
		nullableInt64(entitlement.GrantedByUserID),
		source,
	))
	if err != nil {
		return nil, err
	}
	return granted, nil
}

func (r *matchaPackageRepository) GetActiveEntitlement(ctx context.Context, packageID, userID int64, versionID *int64) (*model.MatchaPackageEntitlement, error) {
	args := []any{packageID, userID}
	where := "package_id = $1 AND user_id = $2 AND status = 'active' AND starts_at <= NOW() AND (expires_at IS NULL OR expires_at > NOW())"
	order := "version_id NULLS FIRST"
	if versionID != nil {
		args = append(args, *versionID)
		where += " AND (version_id IS NULL OR version_id = $3)"
		order = "CASE WHEN version_id = $3 THEN 0 ELSE 1 END"
	}
	entitlement, err := scanMatchaPackageEntitlement(r.db.QueryRowContext(ctx, `
		SELECT id, package_id, version_id, user_id, granted_by_user_id, source, status, starts_at, expires_at, created_at, updated_at
		FROM matcha_package_entitlements
		WHERE `+where+`
		ORDER BY `+order+`, id DESC
		LIMIT 1`, args...))
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMatchaPackageEntitlementNotFound, nil)
	}
	return entitlement, nil
}

func (r *matchaPackageRepository) RevokeEntitlement(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE matcha_package_entitlements
		SET status = 'revoked', updated_at = NOW()
		WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrMatchaPackageEntitlementNotFound
	}
	return nil
}

func (r *matchaPackageRepository) RecordDownload(ctx context.Context, download *model.MatchaPackageDownload) (*model.MatchaPackageDownload, error) {
	recorded, err := scanMatchaPackageDownload(r.db.QueryRowContext(ctx, `
		INSERT INTO matcha_package_downloads (
			package_id, version_id, user_id, entitlement_id, object_key, sha256, size_bytes, remote_addr, user_agent
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, package_id, version_id, user_id, entitlement_id, object_key, sha256, size_bytes, remote_addr, user_agent, downloaded_at`,
		download.PackageID,
		download.VersionID,
		nullableInt64(download.UserID),
		nullableInt64(download.EntitlementID),
		download.ObjectKey,
		download.SHA256,
		download.SizeBytes,
		download.RemoteAddr,
		download.UserAgent,
	))
	if err != nil {
		return nil, err
	}
	return recorded, nil
}

func (r *matchaPackageRepository) ListDownloads(ctx context.Context, filter service.MatchaPackageDownloadFilter) ([]*model.MatchaPackageDownload, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if filter.PackageID != nil {
		args = append(args, *filter.PackageID)
		where = append(where, fmt.Sprintf("package_id = $%d", len(args)))
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, package_id, version_id, user_id, entitlement_id, object_key, sha256, size_bytes, remote_addr, user_agent, downloaded_at
		FROM matcha_package_downloads
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY downloaded_at DESC, id DESC
		LIMIT $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*model.MatchaPackageDownload{}
	for rows.Next() {
		download, err := scanMatchaPackageDownload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, download)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *matchaPackageRepository) CountDownloadsByPackage(ctx context.Context, packageIDs []int64) (map[int64]service.MatchaPackageDownloadStats, error) {
	out := make(map[int64]service.MatchaPackageDownloadStats, len(packageIDs))
	ids := make([]int64, 0, len(packageIDs))
	seen := make(map[int64]struct{}, len(packageIDs))
	for _, id := range packageIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT package_id, COUNT(*), MAX(downloaded_at)
		FROM matcha_package_downloads
		WHERE package_id = ANY($1)
		GROUP BY package_id`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var packageID int64
		var count int64
		var last sql.NullTime
		if err := rows.Scan(&packageID, &count, &last); err != nil {
			return nil, err
		}
		out[packageID] = service.MatchaPackageDownloadStats{DownloadCount: count, LastDownloadedAt: timePtr(last)}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *matchaPackageRepository) UpsertBillingPolicy(ctx context.Context, policy *model.MatchaPackageBillingPolicy) (*model.MatchaPackageBillingPolicy, error) {
	policyJSON, err := jsonMapParam(policy.Policy)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if policy.Active {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, matchaPackageBillingPolicyLockKey(policy)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE matcha_package_billing_policies
			SET active = FALSE, updated_at = NOW()
			WHERE package_id = $1
			  AND COALESCE(version_id, 0) = COALESCE($2, 0)
			  AND kind = $3
			  AND unit = $4
			  AND active = TRUE`,
			policy.PackageID,
			nullableInt64(policy.VersionID),
			policy.Kind,
			policy.Unit,
		); err != nil {
			return nil, err
		}
	}

	created, err := scanMatchaPackageBillingPolicy(tx.QueryRowContext(ctx, `
		INSERT INTO matcha_package_billing_policies (
			package_id, version_id, kind, currency, amount, unit, policy, active, created_by_user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)
		RETURNING id, package_id, version_id, kind, currency, amount, unit, policy, active, starts_at, ends_at, created_by_user_id, created_at, updated_at`,
		policy.PackageID,
		nullableInt64(policy.VersionID),
		policy.Kind,
		policy.Currency,
		policy.Amount.String(),
		policy.Unit,
		policyJSON,
		policy.Active,
		nullableInt64(policy.CreatedByUserID),
	))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return created, nil
}

func matchaPackageBillingPolicyLockKey(policy *model.MatchaPackageBillingPolicy) string {
	versionID := int64(0)
	if policy.VersionID != nil {
		versionID = *policy.VersionID
	}
	return fmt.Sprintf("matcha-package-billing-policy:%d:%d:%s:%s", policy.PackageID, versionID, policy.Kind, policy.Unit)
}

func (r *matchaPackageRepository) ListActiveBillingPolicies(ctx context.Context, packageID int64, versionID *int64) ([]*model.MatchaPackageBillingPolicy, error) {
	args := []any{packageID}
	where := "package_id = $1 AND active = TRUE AND starts_at <= NOW() AND (ends_at IS NULL OR ends_at > NOW())"
	if versionID != nil {
		args = append(args, *versionID)
		where += " AND (version_id IS NULL OR version_id = $2)"
	} else {
		where += " AND version_id IS NULL"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, package_id, version_id, kind, currency, amount, unit, policy, active, starts_at, ends_at, created_by_user_id, created_at, updated_at
		FROM matcha_package_billing_policies
		WHERE `+where+`
		ORDER BY version_id NULLS LAST, starts_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	policies := []*model.MatchaPackageBillingPolicy{}
	for rows.Next() {
		policy, err := scanMatchaPackageBillingPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *matchaPackageRepository) RecordUsage(ctx context.Context, usage *model.MatchaPackageUsageRecord, ledger *model.MatchaPackageRevenueLedgerEntry) (*model.MatchaPackageUsageRecord, error) {
	metadata, err := jsonMapParam(usage.Metadata)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	recorded, err := scanMatchaPackageUsageRecord(tx.QueryRowContext(ctx, `
		INSERT INTO matcha_package_usage_records (
			package_id, version_id, user_id, entitlement_id, request_id, binding, quantity, unit, amount, currency, metadata
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)
		ON CONFLICT (version_id, user_id, request_id, binding) DO NOTHING
		RETURNING id, package_id, version_id, user_id, entitlement_id, request_id, binding, quantity, unit, amount, currency, metadata, recorded_at, created_at`,
		usage.PackageID,
		usage.VersionID,
		usage.UserID,
		nullableInt64(usage.EntitlementID),
		usage.RequestID,
		usage.Binding,
		usage.Quantity.String(),
		usage.Unit,
		usage.Amount.String(),
		usage.Currency,
		metadata,
	))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		recorded, err = scanMatchaPackageUsageRecord(tx.QueryRowContext(ctx, `
			SELECT id, package_id, version_id, user_id, entitlement_id, request_id, binding, quantity, unit, amount, currency, metadata, recorded_at, created_at
			FROM matcha_package_usage_records
			WHERE version_id = $1 AND user_id = $2 AND request_id = $3 AND binding = $4`,
			usage.VersionID,
			usage.UserID,
			usage.RequestID,
			usage.Binding,
		))
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return recorded, nil
	}

	if ledger != nil {
		ledgerJSON, err := jsonMapParam(ledger.Metadata)
		if err != nil {
			return nil, err
		}
		ledger.UsageRecordID = &recorded.ID
		if _, err := scanMatchaPackageRevenueLedgerEntry(tx.QueryRowContext(ctx, `
			INSERT INTO matcha_package_revenue_ledger (
				package_id, version_id, usage_record_id, owner_user_id, user_id, entry_type, amount, currency, status, metadata
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb)
			RETURNING id, package_id, version_id, usage_record_id, owner_user_id, user_id, entry_type, amount, currency, status, metadata, created_at`,
			ledger.PackageID,
			ledger.VersionID,
			nullableInt64(ledger.UsageRecordID),
			nullableInt64(ledger.OwnerUserID),
			ledger.UserID,
			ledger.EntryType,
			ledger.Amount.String(),
			ledger.Currency,
			ledger.Status,
			ledgerJSON,
		)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return recorded, nil
}

func (r *matchaPackageRepository) ListRevenueLedger(ctx context.Context, filter service.MatchaPackageRevenueFilter) ([]*model.MatchaPackageRevenueLedgerEntry, error) {
	where := []string{"1 = 1"}
	args := []any{}
	if filter.PackageID != nil {
		args = append(args, *filter.PackageID)
		where = append(where, fmt.Sprintf("package_id = $%d", len(args)))
	}
	if filter.OwnerUserID != nil {
		args = append(args, *filter.OwnerUserID)
		where = append(where, fmt.Sprintf("owner_user_id = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, package_id, version_id, usage_record_id, owner_user_id, user_id, entry_type, amount, currency, status, metadata, created_at
		FROM matcha_package_revenue_ledger
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	entries := []*model.MatchaPackageRevenueLedgerEntry{}
	for rows.Next() {
		entry, err := scanMatchaPackageRevenueLedgerEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func scanMatchaPackage(row rowScanner) (*model.MatchaPackage, error) {
	pkg := &model.MatchaPackage{}
	var owner sql.NullInt64
	var metadata []byte
	if err := row.Scan(&pkg.ID, &pkg.PackageKey, &pkg.Name, &pkg.Description, &owner, &pkg.Status, &metadata, &pkg.CreatedAt, &pkg.UpdatedAt); err != nil {
		return nil, err
	}
	pkg.OwnerUserID = int64Ptr(owner)
	var err error
	pkg.Metadata, err = scanJSONMap(metadata)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func scanMatchaPackageVersion(row rowScanner) (*model.MatchaPackageVersion, error) {
	version := &model.MatchaPackageVersion{}
	var owner sql.NullInt64
	var published sql.NullTime
	var priceAmount string
	var surchargePolicy []byte
	var metadata []byte
	if err := row.Scan(
		&version.ID,
		&version.PackageID,
		&version.Version,
		&version.Kind,
		&version.Runtime,
		&version.SHA256,
		&version.SizeBytes,
		&version.ObjectKey,
		&owner,
		&version.Status,
		&priceAmount,
		&version.PriceCurrency,
		&surchargePolicy,
		&metadata,
		&version.CreatedAt,
		&version.UpdatedAt,
		&published,
	); err != nil {
		return nil, err
	}
	amount, err := decimal.NewFromString(priceAmount)
	if err != nil {
		return nil, err
	}
	version.PriceAmount = amount
	version.OwnerUserID = int64Ptr(owner)
	version.PublishedAt = timePtr(published)
	version.SurchargePolicy, err = scanJSONMap(surchargePolicy)
	if err != nil {
		return nil, err
	}
	version.Metadata, err = scanJSONMap(metadata)
	if err != nil {
		return nil, err
	}
	return version, nil
}

func scanMatchaPackageEntitlement(row rowScanner) (*model.MatchaPackageEntitlement, error) {
	entitlement := &model.MatchaPackageEntitlement{}
	var versionID sql.NullInt64
	var grantedBy sql.NullInt64
	var expiresAt sql.NullTime
	if err := row.Scan(
		&entitlement.ID,
		&entitlement.PackageID,
		&versionID,
		&entitlement.UserID,
		&grantedBy,
		&entitlement.Source,
		&entitlement.Status,
		&entitlement.StartsAt,
		&expiresAt,
		&entitlement.CreatedAt,
		&entitlement.UpdatedAt,
	); err != nil {
		return nil, err
	}
	entitlement.VersionID = int64Ptr(versionID)
	entitlement.GrantedByUserID = int64Ptr(grantedBy)
	entitlement.ExpiresAt = timePtr(expiresAt)
	return entitlement, nil
}

func scanMatchaPackageDownload(row rowScanner) (*model.MatchaPackageDownload, error) {
	download := &model.MatchaPackageDownload{}
	var userID sql.NullInt64
	var entitlementID sql.NullInt64
	if err := row.Scan(
		&download.ID,
		&download.PackageID,
		&download.VersionID,
		&userID,
		&entitlementID,
		&download.ObjectKey,
		&download.SHA256,
		&download.SizeBytes,
		&download.RemoteAddr,
		&download.UserAgent,
		&download.DownloadedAt,
	); err != nil {
		return nil, err
	}
	download.UserID = int64Ptr(userID)
	download.EntitlementID = int64Ptr(entitlementID)
	return download, nil
}

func scanMatchaPackageBillingPolicy(row rowScanner) (*model.MatchaPackageBillingPolicy, error) {
	policy := &model.MatchaPackageBillingPolicy{}
	var versionID sql.NullInt64
	var endsAt sql.NullTime
	var createdBy sql.NullInt64
	var amount string
	var policyJSON []byte
	if err := row.Scan(
		&policy.ID,
		&policy.PackageID,
		&versionID,
		&policy.Kind,
		&policy.Currency,
		&amount,
		&policy.Unit,
		&policyJSON,
		&policy.Active,
		&policy.StartsAt,
		&endsAt,
		&createdBy,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	); err != nil {
		return nil, err
	}
	parsed, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	policy.Amount = parsed
	policy.VersionID = int64Ptr(versionID)
	policy.EndsAt = timePtr(endsAt)
	policy.CreatedByUserID = int64Ptr(createdBy)
	policy.Policy, err = scanJSONMap(policyJSON)
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func scanMatchaPackageUsageRecord(row rowScanner) (*model.MatchaPackageUsageRecord, error) {
	usage := &model.MatchaPackageUsageRecord{}
	var entitlementID sql.NullInt64
	var quantity string
	var amount string
	var metadata []byte
	if err := row.Scan(
		&usage.ID,
		&usage.PackageID,
		&usage.VersionID,
		&usage.UserID,
		&entitlementID,
		&usage.RequestID,
		&usage.Binding,
		&quantity,
		&usage.Unit,
		&amount,
		&usage.Currency,
		&metadata,
		&usage.RecordedAt,
		&usage.CreatedAt,
	); err != nil {
		return nil, err
	}
	parsedQuantity, err := decimal.NewFromString(quantity)
	if err != nil {
		return nil, err
	}
	parsedAmount, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	usage.Quantity = parsedQuantity
	usage.Amount = parsedAmount
	usage.EntitlementID = int64Ptr(entitlementID)
	usage.Metadata, err = scanJSONMap(metadata)
	if err != nil {
		return nil, err
	}
	return usage, nil
}

func scanMatchaPackageRevenueLedgerEntry(row rowScanner) (*model.MatchaPackageRevenueLedgerEntry, error) {
	entry := &model.MatchaPackageRevenueLedgerEntry{}
	var usageRecordID sql.NullInt64
	var ownerUserID sql.NullInt64
	var amount string
	var metadata []byte
	if err := row.Scan(
		&entry.ID,
		&entry.PackageID,
		&entry.VersionID,
		&usageRecordID,
		&ownerUserID,
		&entry.UserID,
		&entry.EntryType,
		&amount,
		&entry.Currency,
		&entry.Status,
		&metadata,
		&entry.CreatedAt,
	); err != nil {
		return nil, err
	}
	parsed, err := decimal.NewFromString(amount)
	if err != nil {
		return nil, err
	}
	entry.Amount = parsed
	entry.UsageRecordID = int64Ptr(usageRecordID)
	entry.OwnerUserID = int64Ptr(ownerUserID)
	entry.Metadata, err = scanJSONMap(metadata)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func jsonMapParam(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func scanJSONMap(data []byte) (map[string]any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	if value == nil {
		return map[string]any{}, nil
	}
	return value, nil
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func int64Ptr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	v := value.Int64
	return &v
}

func timePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time
	return &v
}
