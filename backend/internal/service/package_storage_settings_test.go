//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type recordingPackageObjectStore struct{}

func (recordingPackageObjectStore) Upload(context.Context, string, io.Reader, string) (int64, error) {
	return 0, nil
}
func (recordingPackageObjectStore) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}
func (recordingPackageObjectStore) Delete(context.Context, string) error { return nil }
func (recordingPackageObjectStore) PresignURL(context.Context, string) (string, error) {
	return "", nil
}

func newMatchaPackageStorageFixture(t *testing.T, encryptionKeyConfigured bool) (*MatchaPackageStorageSettingService, *stubSettingRepo, *[]MatchaPackageStorageConfig) {
	t.Helper()
	repo := newStubSettingRepo()
	encryptor := reversibleEncryptor{}
	backup := NewBackupService(repo, &config.Config{
		Totp: config.TotpConfig{EncryptionKeyConfigured: encryptionKeyConfigured},
	}, encryptor, nil, nil)
	var built []MatchaPackageStorageConfig
	factory := func(_ context.Context, cfg *MatchaPackageStorageConfig) (MatchaPackageObjectStore, error) {
		built = append(built, *cfg)
		return recordingPackageObjectStore{}, nil
	}
	return NewMatchaPackageStorageSettingService(repo, encryptor, backup, factory), repo, &built
}

func TestMatchaPackageStorageDefaultsToLocalAndCachesResolver(t *testing.T) {
	svc, _, built := newMatchaPackageStorageFixture(t, true)
	ctx := context.Background()

	store, err := svc.Resolver()(ctx)
	require.NoError(t, err)
	require.NotNil(t, store)
	_, err = svc.Resolver()(ctx)
	require.NoError(t, err)

	require.Len(t, *built, 1)
	require.Equal(t, "local", (*built)[0].Driver)
	require.Equal(t, DefaultMatchaPackageStorageSettings().BasePath, (*built)[0].BasePath)
}

func TestMatchaPackageStorageS3CredentialsAreEncryptedMaskedAndReusable(t *testing.T) {
	svc, repo, built := newMatchaPackageStorageFixture(t, true)
	ctx := context.Background()

	saved, err := svc.Update(ctx, MatchaPackageStorageSettings{
		Driver:          "s3",
		Endpoint:        "https://acct.r2.cloudflarestorage.com",
		Region:          "wnam",
		Bucket:          "packages",
		AccessKeyID:     "ak",
		SecretAccessKey: "super-secret",
		Prefix:          "matcha-packages",
	}, nil)
	require.NoError(t, err)
	require.Empty(t, saved.SecretAccessKey)
	require.True(t, saved.SecretAccessKeyConfigured)

	raw, err := repo.GetValue(ctx, settingKeyMatchaPackageStorageSettings)
	require.NoError(t, err)
	require.NotContains(t, raw, `"secret_access_key":"super-secret"`)
	require.Contains(t, raw, "enc:super-secret")

	fetched, err := svc.Get(ctx)
	require.NoError(t, err)
	require.Empty(t, fetched.SecretAccessKey)
	require.True(t, fetched.SecretAccessKeyConfigured)

	_, err = svc.Resolver()(ctx)
	require.NoError(t, err)
	require.Len(t, *built, 1)
	require.Equal(t, "super-secret", (*built)[0].SecretAccessKey)
	require.Equal(t, "matcha-packages", (*built)[0].Prefix)

	_, err = svc.Update(ctx, MatchaPackageStorageSettings{
		Driver:      "s3",
		Endpoint:    "https://acct.r2.cloudflarestorage.com",
		Region:      "wnam",
		Bucket:      "packages",
		AccessKeyID: "ak",
	}, nil)
	require.NoError(t, err)
	_, err = svc.Resolver()(ctx)
	require.NoError(t, err)
	require.Len(t, *built, 2)
	require.Equal(t, "super-secret", (*built)[1].SecretAccessKey)
}

func TestMatchaPackageStorageReuseBackupS3Credentials(t *testing.T) {
	svc, repo, built := newMatchaPackageStorageFixture(t, true)
	ctx := context.Background()
	seedBackupS3(t, repo, BackupS3Config{
		Endpoint:        "https://backup.r2.cloudflarestorage.com",
		Region:          "auto",
		Bucket:          "backup-bucket",
		AccessKeyID:     "backup-ak",
		SecretAccessKey: "backup-sk",
		Prefix:          "backups/",
		ForcePathStyle:  true,
	})

	_, err := svc.Update(ctx, MatchaPackageStorageSettings{Driver: "s3", ReuseBackupS3: true, Prefix: "packages"}, nil)
	require.NoError(t, err)

	raw, err := repo.GetValue(ctx, settingKeyMatchaPackageStorageSettings)
	require.NoError(t, err)
	var stored MatchaPackageStorageSettings
	require.NoError(t, json.Unmarshal([]byte(raw), &stored))
	require.Empty(t, stored.SecretAccessKey)
	require.Empty(t, stored.AccessKeyID)

	_, err = svc.Resolver()(ctx)
	require.NoError(t, err)
	require.Len(t, *built, 1)
	got := (*built)[0]
	require.Equal(t, "https://backup.r2.cloudflarestorage.com", got.Endpoint)
	require.Equal(t, "backup-ak", got.AccessKeyID)
	require.Equal(t, "backup-sk", got.SecretAccessKey)
	require.Equal(t, "backup-bucket", got.Bucket)
	require.True(t, got.ForcePathStyle)
	require.Equal(t, "packages", got.Prefix)
}

func TestMatchaPackageStorageRejectsOwnSecretWithEphemeralKey(t *testing.T) {
	svc, repo, built := newMatchaPackageStorageFixture(t, false)
	ctx := context.Background()

	_, err := svc.Update(ctx, MatchaPackageStorageSettings{
		Driver:          "s3",
		Endpoint:        "https://acct.r2.cloudflarestorage.com",
		Bucket:          "packages",
		AccessKeyID:     "ak",
		SecretAccessKey: "super-secret",
	}, nil)
	require.ErrorIs(t, err, ErrSecretEncryptionKeyNotConfigured)
	raw, _ := repo.GetValue(ctx, settingKeyMatchaPackageStorageSettings)
	require.Empty(t, raw)
	require.Empty(t, *built)

	seedBackupS3(t, repo, BackupS3Config{
		Endpoint:        "https://backup.r2.cloudflarestorage.com",
		Bucket:          "backup-bucket",
		AccessKeyID:     "backup-ak",
		SecretAccessKey: "backup-sk",
	})
	_, err = svc.Update(ctx, MatchaPackageStorageSettings{Driver: "s3", ReuseBackupS3: true}, nil)
	require.NoError(t, err)
}
