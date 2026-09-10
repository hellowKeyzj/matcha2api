package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const settingKeyMatchaPackageStorageSettings = "matcha_package_storage_settings"

type MatchaPackageStorageSettings struct {
	Driver                    string     `json:"driver"`
	ReuseBackupS3             bool       `json:"reuse_backup_s3"`
	BasePath                  string     `json:"base_path"`
	PublicBaseURL             string     `json:"public_base_url"`
	Endpoint                  string     `json:"endpoint"`
	Region                    string     `json:"region"`
	Bucket                    string     `json:"bucket"`
	AccessKeyID               string     `json:"access_key_id"`
	SecretAccessKey           string     `json:"secret_access_key,omitempty"` //nolint:revive // field name follows AWS convention
	SecretAccessKeyConfigured bool       `json:"secret_access_key_configured,omitempty"`
	Prefix                    string     `json:"prefix"`
	PresignExpiryHours        int        `json:"presign_expiry_hours"`
	ForcePathStyle            bool       `json:"force_path_style"`
	UpdatedBy                 *int64     `json:"updated_by,omitempty"`
	UpdatedAt                 *time.Time `json:"updated_at,omitempty"`
}

type MatchaPackageStorageSettingService struct {
	settingRepo SettingRepository
	encryptor   SecretEncryptor
	backup      *BackupService
	factory     MatchaPackageObjectStoreFactory

	mu       sync.Mutex
	resolved bool
	store    MatchaPackageObjectStore
}

func NewMatchaPackageStorageSettingService(
	settingRepo SettingRepository,
	encryptor SecretEncryptor,
	backup *BackupService,
	factory MatchaPackageObjectStoreFactory,
) *MatchaPackageStorageSettingService {
	return &MatchaPackageStorageSettingService{
		settingRepo: settingRepo,
		encryptor:   encryptor,
		backup:      backup,
		factory:     factory,
	}
}

func DefaultMatchaPackageStorageSettings() *MatchaPackageStorageSettings {
	return &MatchaPackageStorageSettings{
		Driver:             "local",
		BasePath:           filepath.Join("data", "matcha-packages"),
		PresignExpiryHours: 24,
	}
}

func (s *MatchaPackageStorageSettingService) Resolver() MatchaPackageObjectStoreResolver {
	return func(ctx context.Context) (MatchaPackageObjectStore, error) {
		return s.resolve(ctx)
	}
}

func (s *MatchaPackageStorageSettingService) Get(ctx context.Context) (*MatchaPackageStorageSettings, error) {
	settings, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		settings = DefaultMatchaPackageStorageSettings()
	}
	settings.SecretAccessKeyConfigured = s.secretConfigured(ctx, settings)
	settings.SecretAccessKey = ""
	return settings, nil
}

func (s *MatchaPackageStorageSettingService) SecretConfigured(ctx context.Context) bool {
	settings, err := s.load(ctx)
	if err != nil || settings == nil {
		return false
	}
	return s.secretConfigured(ctx, settings)
}

func (s *MatchaPackageStorageSettingService) Update(ctx context.Context, in MatchaPackageStorageSettings, actorID *int64) (*MatchaPackageStorageSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, ErrSettingNotFound
	}
	old, _ := s.load(ctx)
	normalizeMatchaPackageStorageSettings(&in)

	if in.Driver != "s3" {
		in.ReuseBackupS3 = false
		in.Endpoint, in.Region, in.Bucket = "", "", ""
		in.AccessKeyID, in.SecretAccessKey, in.Prefix = "", "", ""
		in.ForcePathStyle = false
	} else if in.ReuseBackupS3 {
		in.Endpoint, in.Region, in.AccessKeyID, in.SecretAccessKey = "", "", "", ""
		in.ForcePathStyle = false
	} else {
		if in.SecretAccessKey == "" && old != nil {
			in.SecretAccessKey = old.SecretAccessKey
		} else if in.SecretAccessKey != "" {
			if s.backup == nil || !s.backup.EncryptionKeyConfigured() {
				return nil, ErrSecretEncryptionKeyNotConfigured
			}
			encrypted, err := s.encryptor.Encrypt(in.SecretAccessKey)
			if err != nil {
				return nil, fmt.Errorf("encrypt matcha package storage secret: %w", err)
			}
			in.SecretAccessKey = encrypted
		}
	}

	now := time.Now().UTC()
	in.UpdatedAt = &now
	in.UpdatedBy = actorID
	in.SecretAccessKeyConfigured = false
	data, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("marshal matcha package storage settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, settingKeyMatchaPackageStorageSettings, string(data)); err != nil {
		return nil, fmt.Errorf("save matcha package storage settings: %w", err)
	}
	s.Invalidate()

	out := in
	out.SecretAccessKeyConfigured = s.secretConfigured(ctx, &in)
	out.SecretAccessKey = ""
	return &out, nil
}

func (s *MatchaPackageStorageSettingService) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.resolved = false
	s.store = nil
	s.mu.Unlock()
}

func (s *MatchaPackageStorageSettingService) resolve(ctx context.Context) (MatchaPackageObjectStore, error) {
	if s == nil || s.factory == nil {
		return nil, ErrMatchaPackageStorageNotConfigured
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resolved {
		return s.store, nil
	}
	settings, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		settings = DefaultMatchaPackageStorageSettings()
	}
	cfg, err := s.toStorageConfig(ctx, settings)
	if err != nil {
		return nil, err
	}
	if !cfg.IsConfigured() {
		return nil, ErrMatchaPackageStorageNotConfigured
	}
	store, err := s.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	s.store = store
	s.resolved = true
	return store, nil
}

func (s *MatchaPackageStorageSettingService) toStorageConfig(ctx context.Context, in *MatchaPackageStorageSettings) (*MatchaPackageStorageConfig, error) {
	if in == nil {
		in = DefaultMatchaPackageStorageSettings()
	}
	cfg := &MatchaPackageStorageConfig{
		Driver:             in.Driver,
		BasePath:           in.BasePath,
		Endpoint:           in.Endpoint,
		Region:             in.Region,
		Bucket:             in.Bucket,
		AccessKeyID:        in.AccessKeyID,
		SecretAccessKey:    in.SecretAccessKey,
		Prefix:             in.Prefix,
		ForcePathStyle:     in.ForcePathStyle,
		PublicBaseURL:      in.PublicBaseURL,
		PresignExpiryHours: in.PresignExpiryHours,
	}
	if cfg.Driver == "s3" && in.ReuseBackupS3 {
		backupCfg, err := s.backupS3Config(ctx)
		if err != nil {
			return nil, err
		}
		if backupCfg == nil || !backupCfg.IsConfigured() {
			return nil, ErrMatchaPackageBackupS3NotConfigured
		}
		cfg.Endpoint = backupCfg.Endpoint
		cfg.Region = backupCfg.Region
		cfg.AccessKeyID = backupCfg.AccessKeyID
		cfg.SecretAccessKey = backupCfg.SecretAccessKey
		cfg.ForcePathStyle = backupCfg.ForcePathStyle
		if cfg.Bucket == "" {
			cfg.Bucket = backupCfg.Bucket
		}
	} else if cfg.Driver == "s3" && cfg.SecretAccessKey != "" && s.encryptor != nil {
		if decrypted, err := s.encryptor.Decrypt(cfg.SecretAccessKey); err == nil {
			cfg.SecretAccessKey = decrypted
		}
	}
	return cfg, nil
}

func (s *MatchaPackageStorageSettingService) backupS3Config(ctx context.Context) (*BackupS3Config, error) {
	if s == nil || s.backup == nil {
		return nil, ErrMatchaPackageBackupS3NotConfigured
	}
	return s.backup.loadS3Config(ctx)
}

func (s *MatchaPackageStorageSettingService) secretConfigured(ctx context.Context, settings *MatchaPackageStorageSettings) bool {
	if settings == nil || settings.Driver != "s3" {
		return false
	}
	if settings.ReuseBackupS3 {
		cfg, err := s.backupS3Config(ctx)
		return err == nil && cfg != nil && cfg.SecretAccessKey != ""
	}
	return settings.SecretAccessKey != ""
}

func (s *MatchaPackageStorageSettingService) load(ctx context.Context) (*MatchaPackageStorageSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, nil //nolint:nilnil // no repository means no stored settings
	}
	raw, err := s.settingRepo.GetValue(ctx, settingKeyMatchaPackageStorageSettings)
	if err != nil || strings.TrimSpace(raw) == "" {
		return nil, nil //nolint:nilnil // never configured is a valid state
	}
	var settings MatchaPackageStorageSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("parse matcha package storage settings: %w", err)
	}
	normalizeMatchaPackageStorageSettings(&settings)
	return &settings, nil
}

func normalizeMatchaPackageStorageSettings(in *MatchaPackageStorageSettings) {
	if in == nil {
		return
	}
	in.Driver = strings.ToLower(strings.TrimSpace(in.Driver))
	if in.Driver == "" {
		in.Driver = "local"
	}
	if in.Driver != "local" && in.Driver != "s3" {
		in.Driver = "local"
	}
	in.BasePath = strings.TrimSpace(in.BasePath)
	if in.BasePath == "" {
		in.BasePath = DefaultMatchaPackageStorageSettings().BasePath
	}
	in.PublicBaseURL = strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.Region = strings.TrimSpace(in.Region)
	if in.Region == "" {
		in.Region = "auto"
	}
	in.Bucket = strings.TrimSpace(in.Bucket)
	in.AccessKeyID = strings.TrimSpace(in.AccessKeyID)
	in.SecretAccessKey = strings.TrimSpace(in.SecretAccessKey)
	in.Prefix = strings.Trim(strings.TrimSpace(in.Prefix), "/")
	in.SecretAccessKeyConfigured = false
	if in.PresignExpiryHours <= 0 {
		in.PresignExpiryHours = 24
	}
	if in.Driver != "s3" {
		in.ReuseBackupS3 = false
	}
}
