package dto

import "time"

type PackageListQuery struct {
	Page        int
	PageSize    int
	Search      string
	PackageType string
}

type PackageListResult struct {
	Items []PackageVersion
	Total int64
}

type PackageVersion struct {
	PackageID         string                  `json:"packageId"`
	PackageVersionID  string                  `json:"packageVersionId"`
	Name              string                  `json:"name"`
	DisplayName       string                  `json:"displayName,omitempty"`
	PackageType       string                  `json:"packageType"`
	Version           string                  `json:"version"`
	Description       string                  `json:"description,omitempty"`
	Status            string                  `json:"status"`
	EntitlementStatus string                  `json:"entitlementStatus,omitempty"`
	Downloadable      bool                    `json:"downloadable"`
	MeteringBinding   *PackageMeteringBinding `json:"meteringBinding,omitempty"`
	DownloadCount     int64                   `json:"downloadCount,omitempty"`
	CreatedAt         *time.Time              `json:"createdAt,omitempty"`
	UpdatedAt         *time.Time              `json:"updatedAt,omitempty"`
}

type PackageMeteringBinding struct {
	ID       string  `json:"id,omitempty"`
	Type     string  `json:"type,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Amount   float64 `json:"amount,omitempty"`
	Currency string  `json:"currency,omitempty"`
}

type PackageAuthorizationRequest struct {
	PackageVersionID string `json:"packageVersionId,omitempty"`
	DevicePublicKey  string `json:"devicePublicKey"`
	ClientVersion    string `json:"clientVersion,omitempty"`
	InstallID        string `json:"installId,omitempty"`
	Source           string `json:"source,omitempty"`
}

type PackageEnvelope struct {
	KeyID            string `json:"keyId,omitempty"`
	Algorithm        string `json:"algorithm"`
	CiphertextBase64 string `json:"ciphertextBase64"`
}

type SealedCloudKeyResponse struct {
	Version   int    `json:"version"`
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	PublicKey string `json:"publicKey"`
}

type PackageAuthorizationResponse struct {
	PackageVersionID  string                  `json:"packageVersionId"`
	PackageType       string                  `json:"packageType"`
	MeteringBinding   *PackageMeteringBinding `json:"meteringBinding,omitempty"`
	EntitlementStatus string                  `json:"entitlementStatus,omitempty"`
	DeviceEnvelope    *PackageEnvelope        `json:"deviceEnvelope"`
	// LeaseExpiresAt is an authorization window clients must honor; the server cannot expire an issued content key.
	LeaseExpiresAt time.Time `json:"leaseExpiresAt"`
}
