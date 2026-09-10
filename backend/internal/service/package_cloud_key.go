package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	settingKeyMatchaPackageCloudKey = "matcha_package_sealed_cloud_key"
	sealedPackageCloudKeyBits       = 3072
)

var (
	ErrSealedPackageCloudKeyNotConfigured = infraerrors.ServiceUnavailable("MATCHA_PACKAGE_CLOUD_KEY_NOT_CONFIGURED", "sealed package cloud key is not configured")
	ErrSealedPackageCloudKeyUnavailable   = infraerrors.ServiceUnavailable("MATCHA_PACKAGE_CLOUD_KEY_UNAVAILABLE", "sealed package cloud key is unavailable")
	ErrSealedPackageCloudKeyIDConflict    = infraerrors.ServiceUnavailable("MATCHA_PACKAGE_CLOUD_KEY_ID_CONFLICT", "sealed package cloud key id refers to different keys")
	ErrSealedPackageCloudKeyExists        = infraerrors.Conflict("MATCHA_PACKAGE_CLOUD_KEY_EXISTS", "sealed package cloud key is already configured")
)

type MatchaPackageCloudKeyStatus struct {
	Configured bool       `json:"configured"`
	KeyID      string     `json:"key_id"`
	Algorithm  string     `json:"algorithm"`
	Source     string     `json:"source"`
	PublicKey  string     `json:"public_key"`
	UpdatedBy  *int64     `json:"updated_by,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
}

type sealedPackageCloudKeySettings struct {
	KeyID                  string     `json:"key_id"`
	EncryptedPrivateKeyPEM string     `json:"encrypted_private_key_pem"`
	UpdatedBy              *int64     `json:"updated_by,omitempty"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
}

// MatchaPackageCloudKeyRepository preserves the first stored key across instances.
type MatchaPackageCloudKeyRepository interface {
	GetValue(ctx context.Context, key string) (string, error)
	CreateIfAbsent(ctx context.Context, key, value string) (bool, error)
}

type MatchaPackageCloudKeyService struct {
	settingRepo MatchaPackageCloudKeyRepository
	encryptor   SecretEncryptor
	cfg         *config.Config

	configOnce sync.Once
	configKey  *SealedPackageCloudKey
	configErr  error

	mu           sync.Mutex
	cachedKey    *SealedPackageCloudKey
	cachedStatus MatchaPackageCloudKeyStatus
}

func NewMatchaPackageCloudKeyService(settingRepo MatchaPackageCloudKeyRepository, encryptor SecretEncryptor, cfg *config.Config) *MatchaPackageCloudKeyService {
	return &MatchaPackageCloudKeyService{settingRepo: settingRepo, encryptor: encryptor, cfg: cfg}
}

// Resolve selects only the envelope's key, retaining the configured key after a
// different instance creates the permanent database key.
func (s *MatchaPackageCloudKeyService) Resolve(ctx context.Context, keyID string) (*SealedPackageCloudKey, error) {
	current, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	configured, err := s.loadConfig()
	if err != nil {
		return nil, err
	}
	if current != nil && configured != nil && keyID == current.KeyID && current.KeyID == configured.KeyID && current.PublicKeySPKIPEM != configured.PublicKeySPKIPEM {
		return nil, ErrSealedPackageCloudKeyIDConflict
	}
	if current != nil && current.KeyID == keyID {
		return current, nil
	}
	if configured != nil && configured.KeyID == keyID {
		return configured, nil
	}
	return nil, ErrSealedPackageCloudKeyUnavailable
}

func (s *MatchaPackageCloudKeyService) Get(ctx context.Context) (*SealedPackageCloudKey, error) {
	key, _, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, ErrSealedPackageCloudKeyNotConfigured
	}
	return key, nil
}

func (s *MatchaPackageCloudKeyService) Status(ctx context.Context) (*MatchaPackageCloudKeyStatus, error) {
	_, status, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *MatchaPackageCloudKeyService) Generate(ctx context.Context, actorID *int64) (*MatchaPackageCloudKeyStatus, error) {
	if s == nil || s.settingRepo == nil {
		return nil, ErrSealedPackageCloudKeyNotConfigured
	}
	if key, _, err := s.load(ctx); err != nil {
		return nil, err
	} else if key != nil {
		return nil, ErrSealedPackageCloudKeyExists
	}
	if s.encryptor == nil || s.cfg == nil || !s.cfg.Totp.EncryptionKeyConfigured {
		return nil, ErrSecretEncryptionKeyNotConfigured
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, sealedPackageCloudKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate sealed package cloud key: %w", err)
	}
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	cloudKey, err := NewSealedPackageCloudKey("", privateKeyPEM)
	if err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("encrypt sealed package cloud key: %w", err)
	}
	now := time.Now().UTC()
	settings := sealedPackageCloudKeySettings{
		KeyID:                  cloudKey.KeyID,
		EncryptedPrivateKeyPEM: encrypted,
		UpdatedBy:              actorID,
		UpdatedAt:              &now,
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("marshal sealed package cloud key settings: %w", err)
	}
	created, err := s.settingRepo.CreateIfAbsent(ctx, settingKeyMatchaPackageCloudKey, string(data))
	if err != nil {
		return nil, fmt.Errorf("save sealed package cloud key settings: %w", err)
	}
	if !created {
		return nil, ErrSealedPackageCloudKeyExists
	}
	status := cloudKeyStatus(cloudKey, "database", actorID, &now)
	s.mu.Lock()
	s.cachedKey = cloudKey
	s.cachedStatus = status
	s.mu.Unlock()
	return &status, nil
}

func (s *MatchaPackageCloudKeyService) load(ctx context.Context) (*SealedPackageCloudKey, MatchaPackageCloudKeyStatus, error) {
	if s == nil {
		return nil, MatchaPackageCloudKeyStatus{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cachedKey != nil {
		return s.cachedKey, s.cachedStatus, nil
	}

	key, status, err := s.loadUncached(ctx)
	if err != nil {
		return nil, MatchaPackageCloudKeyStatus{}, err
	}
	// Only immutable keys are cached; another instance may create the database key.
	if key != nil && status.Source != "config" {
		s.cachedKey = key
		s.cachedStatus = status
	}
	return key, status, nil
}

func (s *MatchaPackageCloudKeyService) loadUncached(ctx context.Context) (*SealedPackageCloudKey, MatchaPackageCloudKeyStatus, error) {
	if key, status, err := s.loadStored(ctx); err != nil || key != nil {
		return key, status, err
	}
	key, err := s.loadConfig()
	return key, cloudKeyStatus(key, "config", nil, nil), err
}

func (s *MatchaPackageCloudKeyService) loadConfig() (*SealedPackageCloudKey, error) {
	if s == nil {
		return nil, nil
	}
	s.configOnce.Do(func() {
		if s.cfg != nil && strings.TrimSpace(s.cfg.SealedPackage.CloudPrivateKeyPEM) != "" {
			s.configKey, s.configErr = NewSealedPackageCloudKey(s.cfg.SealedPackage.CloudKeyID, s.cfg.SealedPackage.CloudPrivateKeyPEM)
		}
	})
	return s.configKey, s.configErr
}

func (s *MatchaPackageCloudKeyService) loadStored(ctx context.Context) (*SealedPackageCloudKey, MatchaPackageCloudKeyStatus, error) {
	if s.settingRepo == nil {
		return nil, MatchaPackageCloudKeyStatus{}, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyMatchaPackageCloudKey)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, MatchaPackageCloudKeyStatus{}, nil
		}
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("load sealed package cloud key settings: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("sealed package cloud key settings are empty")
	}
	var settings sealedPackageCloudKeySettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("parse sealed package cloud key settings: %w", err)
	}
	if strings.TrimSpace(settings.EncryptedPrivateKeyPEM) == "" {
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("sealed package cloud key settings have no encrypted private key")
	}
	if s.encryptor == nil {
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("sealed package cloud key decryptor is not configured")
	}
	privateKeyPEM, err := s.encryptor.Decrypt(settings.EncryptedPrivateKeyPEM)
	if err != nil {
		return nil, MatchaPackageCloudKeyStatus{}, fmt.Errorf("decrypt sealed package cloud key settings: %w", err)
	}
	key, err := NewSealedPackageCloudKey(settings.KeyID, privateKeyPEM)
	if err != nil {
		return nil, MatchaPackageCloudKeyStatus{}, err
	}
	return key, cloudKeyStatus(key, "database", settings.UpdatedBy, settings.UpdatedAt), nil
}

func cloudKeyStatus(key *SealedPackageCloudKey, source string, updatedBy *int64, updatedAt *time.Time) MatchaPackageCloudKeyStatus {
	if key == nil {
		return MatchaPackageCloudKeyStatus{}
	}
	return MatchaPackageCloudKeyStatus{
		Configured: true,
		KeyID:      key.KeyID,
		Algorithm:  key.Algorithm,
		Source:     source,
		PublicKey:  key.PublicKeySPKIPEM,
		UpdatedBy:  updatedBy,
		UpdatedAt:  updatedAt,
	}
}
