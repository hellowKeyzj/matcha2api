package model

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	MatchaPackageKindAgent = "agent"
	MatchaPackageKindSkill = "skill"
)

const (
	MatchaPackageStatusDraft    = "draft"
	MatchaPackageStatusActive   = "active"
	MatchaPackageStatusDisabled = "disabled"
	MatchaPackageStatusArchived = "archived"
)

const (
	MatchaPackageVersionStatusDraft     = "draft"
	MatchaPackageVersionStatusPublished = "published"
	MatchaPackageVersionStatusDisabled  = "disabled"
	MatchaPackageVersionStatusYanked    = "yanked"
)

const (
	MatchaPackageEntitlementStatusActive  = "active"
	MatchaPackageEntitlementStatusRevoked = "revoked"
	MatchaPackageEntitlementStatusExpired = "expired"
)

const (
	MatchaPackageBillingPolicyPrice     = "price"
	MatchaPackageBillingPolicySurcharge = "surcharge"
)

const (
	MatchaPackageRevenueEntryUsage  = "usage"
	MatchaPackageRevenueEntryPayout = "payout"
)

const (
	MatchaPackageRevenueStatusPending = "pending"
	MatchaPackageRevenueStatusPosted  = "posted"
	MatchaPackageRevenueStatusVoid    = "void"
)

type MatchaPackage struct {
	ID          int64          `json:"id"`
	PackageKey  string         `json:"package_key"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	OwnerUserID *int64         `json:"owner_user_id,omitempty"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type MatchaPackageVersion struct {
	ID              int64           `json:"id"`
	PackageID       int64           `json:"package_id"`
	Version         string          `json:"version"`
	Kind            string          `json:"kind"`
	Runtime         string          `json:"runtime"`
	SHA256          string          `json:"sha256"`
	SizeBytes       int64           `json:"size_bytes"`
	ObjectKey       string          `json:"object_key"`
	OwnerUserID     *int64          `json:"owner_user_id,omitempty"`
	Status          string          `json:"status"`
	PriceAmount     decimal.Decimal `json:"price_amount"`
	PriceCurrency   string          `json:"price_currency"`
	SurchargePolicy map[string]any  `json:"surcharge_policy"`
	Metadata        map[string]any  `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
}

type MatchaPackageEntitlement struct {
	ID              int64      `json:"id"`
	PackageID       int64      `json:"package_id"`
	VersionID       *int64     `json:"version_id,omitempty"`
	UserID          int64      `json:"user_id"`
	GrantedByUserID *int64     `json:"granted_by_user_id,omitempty"`
	Source          string     `json:"source"`
	Status          string     `json:"status"`
	StartsAt        time.Time  `json:"starts_at"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type MatchaPackageDownload struct {
	ID            int64     `json:"id"`
	PackageID     int64     `json:"package_id"`
	VersionID     int64     `json:"version_id"`
	UserID        *int64    `json:"user_id,omitempty"`
	EntitlementID *int64    `json:"entitlement_id,omitempty"`
	ObjectKey     string    `json:"object_key"`
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	RemoteAddr    string    `json:"remote_addr"`
	UserAgent     string    `json:"user_agent"`
	DownloadedAt  time.Time `json:"downloaded_at"`
}

type MatchaPackageBillingPolicy struct {
	ID              int64           `json:"id"`
	PackageID       int64           `json:"package_id"`
	VersionID       *int64          `json:"version_id,omitempty"`
	Kind            string          `json:"kind"`
	Currency        string          `json:"currency"`
	Amount          decimal.Decimal `json:"amount"`
	Unit            string          `json:"unit"`
	Policy          map[string]any  `json:"policy"`
	Active          bool            `json:"active"`
	StartsAt        time.Time       `json:"starts_at"`
	EndsAt          *time.Time      `json:"ends_at,omitempty"`
	CreatedByUserID *int64          `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type MatchaPackageUsageRecord struct {
	ID            int64           `json:"id"`
	PackageID     int64           `json:"package_id"`
	VersionID     int64           `json:"version_id"`
	UserID        int64           `json:"user_id"`
	EntitlementID *int64          `json:"entitlement_id,omitempty"`
	RequestID     string          `json:"request_id"`
	Binding       string          `json:"binding"`
	Quantity      decimal.Decimal `json:"quantity"`
	Unit          string          `json:"unit"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Metadata      map[string]any  `json:"metadata"`
	RecordedAt    time.Time       `json:"recorded_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

type MatchaPackageRevenueLedgerEntry struct {
	ID            int64           `json:"id"`
	PackageID     int64           `json:"package_id"`
	VersionID     int64           `json:"version_id"`
	UsageRecordID *int64          `json:"usage_record_id,omitempty"`
	OwnerUserID   *int64          `json:"owner_user_id,omitempty"`
	UserID        int64           `json:"user_id"`
	EntryType     string          `json:"entry_type"`
	Amount        decimal.Decimal `json:"amount"`
	Currency      string          `json:"currency"`
	Status        string          `json:"status"`
	Metadata      map[string]any  `json:"metadata"`
	CreatedAt     time.Time       `json:"created_at"`
}

func (p *MatchaPackage) Normalize() {
	if p == nil {
		return
	}
	p.PackageKey = strings.TrimSpace(p.PackageKey)
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.Status = strings.TrimSpace(p.Status)
	if p.Status == "" {
		p.Status = MatchaPackageStatusDraft
	}
	if p.Metadata == nil {
		p.Metadata = map[string]any{}
	}
}

func (p *MatchaPackage) Validate() error {
	if p == nil {
		return &ValidationError{Field: "package", Message: "package is required"}
	}
	if p.PackageKey == "" {
		return &ValidationError{Field: "package_key", Message: "package_key is required"}
	}
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if !isMatchaPackageStatus(p.Status) {
		return &ValidationError{Field: "status", Message: "invalid package status"}
	}
	return nil
}

func (v *MatchaPackageVersion) Normalize() {
	if v == nil {
		return
	}
	v.Version = strings.TrimSpace(v.Version)
	v.Kind = strings.TrimSpace(v.Kind)
	v.Runtime = strings.TrimSpace(v.Runtime)
	v.SHA256 = strings.ToLower(strings.TrimSpace(v.SHA256))
	v.ObjectKey = strings.TrimSpace(v.ObjectKey)
	v.Status = strings.TrimSpace(v.Status)
	if v.Status == "" {
		v.Status = MatchaPackageVersionStatusDraft
	}
	v.PriceCurrency = strings.ToUpper(strings.TrimSpace(v.PriceCurrency))
	if v.PriceCurrency == "" {
		v.PriceCurrency = "USD"
	}
	if v.SurchargePolicy == nil {
		v.SurchargePolicy = map[string]any{}
	}
	if v.Metadata == nil {
		v.Metadata = map[string]any{}
	}
}

func (v *MatchaPackageVersion) Validate() error {
	if v == nil {
		return &ValidationError{Field: "version", Message: "version is required"}
	}
	if v.PackageID <= 0 {
		return &ValidationError{Field: "package_id", Message: "package_id is required"}
	}
	if v.Version == "" {
		return &ValidationError{Field: "version", Message: "version is required"}
	}
	if !IsMatchaPackageKind(v.Kind) {
		return &ValidationError{Field: "kind", Message: "kind must be agent or skill"}
	}
	if v.Runtime == "" {
		return &ValidationError{Field: "runtime", Message: "runtime is required"}
	}
	if !isSHA256(v.SHA256) {
		return &ValidationError{Field: "sha256", Message: "sha256 must be 64 hex characters"}
	}
	if v.SizeBytes <= 0 {
		return &ValidationError{Field: "size_bytes", Message: "size_bytes must be positive"}
	}
	if v.ObjectKey == "" {
		return &ValidationError{Field: "object_key", Message: "object_key is required"}
	}
	if !isMatchaPackageVersionStatus(v.Status) {
		return &ValidationError{Field: "status", Message: "invalid version status"}
	}
	if v.PriceAmount.IsNegative() {
		return &ValidationError{Field: "price_amount", Message: "price_amount must be >= 0"}
	}
	return nil
}

func IsMatchaPackageKind(kind string) bool {
	switch kind {
	case MatchaPackageKindAgent, MatchaPackageKindSkill:
		return true
	default:
		return false
	}
}

func isMatchaPackageStatus(status string) bool {
	switch status {
	case MatchaPackageStatusDraft, MatchaPackageStatusActive, MatchaPackageStatusDisabled, MatchaPackageStatusArchived:
		return true
	default:
		return false
	}
}

func isMatchaPackageVersionStatus(status string) bool {
	switch status {
	case MatchaPackageVersionStatusDraft, MatchaPackageVersionStatusPublished, MatchaPackageVersionStatusDisabled, MatchaPackageVersionStatusYanked:
		return true
	default:
		return false
	}
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
