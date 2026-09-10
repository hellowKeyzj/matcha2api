package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestMatchaPackageCloudKeyServiceGeneratePersistsEncryptedKey(t *testing.T) {
	ctx := context.Background()
	repo := newPackageCloudKeySettingRepo()
	encryptor := packageCloudKeyEncryptor{}
	svc := NewMatchaPackageCloudKeyService(repo, encryptor, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}})
	actorID := int64(7)

	status, err := svc.Generate(ctx, &actorID)
	require.NoError(t, err)
	require.True(t, status.Configured)
	require.Equal(t, "database", status.Source)
	require.NotEmpty(t, status.KeyID)
	require.NotEmpty(t, status.PublicKey)
	require.Equal(t, actorID, *status.UpdatedBy)

	raw, err := repo.GetValue(ctx, settingKeyMatchaPackageCloudKey)
	require.NoError(t, err)
	require.Contains(t, raw, "enc:")
	require.NotContains(t, raw, "RSA PRIVATE KEY")

	key, err := svc.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, sealedPackageCloudKeyBits, key.PrivateKey.N.BitLen())

	// A fresh instance must decrypt the same permanent key, not generate a replacement.
	restarted := NewMatchaPackageCloudKeyService(repo, encryptor, svc.cfg)
	reloaded, err := restarted.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, key.PrivateKeyPEM, reloaded.PrivateKeyPEM)
	reloadedStatus, err := restarted.Status(ctx)
	require.NoError(t, err)
	require.Equal(t, status, reloadedStatus)
	_, err = restarted.Generate(ctx, nil)
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyExists)
	after, err := repo.GetValue(ctx, settingKeyMatchaPackageCloudKey)
	require.NoError(t, err)
	require.Equal(t, raw, after)
}

func TestMatchaPackageCloudKeyServiceGenerateRequiresDurableSecretKey(t *testing.T) {
	repo := newPackageCloudKeySettingRepo()
	encryptor := packageCloudKeyEncryptor{}
	svc := NewMatchaPackageCloudKeyService(repo, encryptor, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: false}})

	_, err := svc.Generate(context.Background(), nil)
	require.ErrorIs(t, err, ErrSecretEncryptionKeyNotConfigured)

	raw, _ := repo.GetValue(context.Background(), settingKeyMatchaPackageCloudKey)
	require.Empty(t, raw)
}

func TestMatchaPackageCloudKeyServiceConcurrentGenerate(t *testing.T) {
	ctx := context.Background()
	repo := newPackageCloudKeySettingRepo()
	encryptor := packageCloudKeyEncryptor{}
	durableConfig := &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}}
	services := []*MatchaPackageCloudKeyService{
		NewMatchaPackageCloudKeyService(repo, encryptor, durableConfig),
		NewMatchaPackageCloudKeyService(repo, encryptor, durableConfig),
	}
	// Force both instances past the initial absence check before either can insert.
	var ready sync.WaitGroup
	ready.Add(len(services))
	repo.beforeCreate = func() { ready.Done(); ready.Wait() }
	type result struct {
		status *MatchaPackageCloudKeyStatus
		err    error
	}
	results := make(chan result, len(services))
	for _, svc := range services {
		go func() {
			status, err := svc.Generate(ctx, nil)
			results <- result{status, err}
		}()
	}
	var winner *MatchaPackageCloudKeyStatus
	conflicts := 0
	for range services {
		got := <-results
		if got.err != nil {
			require.ErrorIs(t, got.err, ErrSealedPackageCloudKeyExists)
			conflicts++
		} else {
			require.Nil(t, winner, "only one instance may create the key")
			winner = got.status
		}
	}
	require.NotNil(t, winner)
	require.Equal(t, 1, conflicts)
	for _, svc := range services {
		status, err := svc.Status(ctx)
		require.NoError(t, err)
		require.Equal(t, winner, status)
	}
}

func TestMatchaPackageCloudKeyServiceDoesNotCacheDatabaseAbsence(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKeyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	cfg := &config.Config{}
	cfg.SealedPackage.CloudPrivateKeyPEM = privateKeyPEM
	cfg.SealedPackage.CloudKeyID = "config-fallback"

	for _, tc := range []struct {
		name string
		cfg  *config.Config
	}{
		{name: "absent"},
		{name: "config", cfg: cfg},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := newPackageCloudKeySettingRepo()
			encryptor := packageCloudKeyEncryptor{}
			durableConfig := &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}}
			reader := NewMatchaPackageCloudKeyService(repo, encryptor, tc.cfg)
			initial, err := reader.Status(ctx)
			require.NoError(t, err)
			if tc.cfg == nil {
				require.False(t, initial.Configured)
				_, err = reader.Get(ctx)
				require.ErrorIs(t, err, ErrSealedPackageCloudKeyNotConfigured)
			} else {
				require.True(t, initial.Configured)
				require.Equal(t, "config", initial.Source)
				require.Equal(t, "config-fallback", initial.KeyID)
				_, err = reader.Generate(ctx, nil)
				require.ErrorIs(t, err, ErrSealedPackageCloudKeyExists)
			}

			writer := NewMatchaPackageCloudKeyService(repo, encryptor, durableConfig)
			stored, err := writer.Generate(ctx, nil)
			require.NoError(t, err)
			loaded, err := reader.Status(ctx)
			require.NoError(t, err)
			require.Equal(t, stored, loaded)
			require.Equal(t, "database", loaded.Source)
			reads := repo.getCalls
			_, err = reader.Get(ctx)
			require.NoError(t, err)
			require.Equal(t, reads, repo.getCalls, "immutable database key should be cached")
		})
	}
}

func packageCloudKeyServiceForTest(key *SealedPackageCloudKey) *MatchaPackageCloudKeyService {
	cfg := &config.Config{}
	cfg.SealedPackage.CloudKeyID = key.KeyID
	cfg.SealedPackage.CloudPrivateKeyPEM = key.PrivateKeyPEM
	return NewMatchaPackageCloudKeyService(nil, nil, cfg)
}

func TestMatchaPackageCloudKeyResolveRetainsConfigAndScopesConflicts(t *testing.T) {
	ctx := context.Background()
	k1 := sealedTestCloudKey(t)
	repo := newPackageCloudKeySettingRepo()
	reader := packageCloudKeyServiceForTest(k1)
	reader.settingRepo = repo
	reader.encryptor = packageCloudKeyEncryptor{}
	resolved, err := reader.Resolve(ctx, k1.KeyID)
	require.NoError(t, err)
	_, err = reader.Resolve(ctx, "unknown-before-database-key")
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))

	// Config material is parsed once, while database absence is checked again.
	reader.cfg.SealedPackage.CloudPrivateKeyPEM = "not a private key"
	writer := NewMatchaPackageCloudKeyService(repo, packageCloudKeyEncryptor{}, &config.Config{Totp: config.TotpConfig{EncryptionKeyConfigured: true}})
	status, err := writer.Generate(ctx, nil)
	require.NoError(t, err)
	current, err := reader.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, status.KeyID, current.KeyID)
	old, err := reader.Resolve(ctx, k1.KeyID)
	require.NoError(t, err)
	require.Same(t, resolved, old)
	stored, err := reader.Resolve(ctx, status.KeyID)
	require.NoError(t, err)
	require.Same(t, current, stored)

	// Same material under the same ID is fine, different SPKI blocks only that ID.
	matching := packageCloudKeyServiceForTest(current)
	matching.settingRepo, matching.encryptor = repo, packageCloudKeyEncryptor{}
	_, err = matching.Resolve(ctx, current.KeyID)
	require.NoError(t, err)
	conflicting := *k1
	conflicting.KeyID = current.KeyID
	ambiguous := packageCloudKeyServiceForTest(&conflicting)
	ambiguous.settingRepo, ambiguous.encryptor = repo, packageCloudKeyEncryptor{}
	_, err = ambiguous.Resolve(ctx, current.KeyID)
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyIDConflict)
	require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
	_, err = ambiguous.Resolve(ctx, "unrelated-key")
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable)
	_, err = (*MatchaPackageCloudKeyService)(nil).Resolve(ctx, "unknown")
	require.ErrorIs(t, err, ErrSealedPackageCloudKeyUnavailable)
}

func TestMatchaPackageCloudKeyResolveDoesNotHideOrCacheFailures(t *testing.T) {
	ctx := context.Background()
	key := sealedTestCloudKey(t)
	encryptor := packageCloudKeyEncryptor{}
	encrypted, err := encryptor.Encrypt(key.PrivateKeyPEM)
	require.NoError(t, err)
	good, err := json.Marshal(sealedPackageCloudKeySettings{KeyID: key.KeyID, EncryptedPrivateKeyPEM: encrypted})
	require.NoError(t, err)

	for _, tc := range []struct {
		name  string
		raw   string
		dbErr error
	}{
		{name: "database", dbErr: errors.New("database unavailable")},
		{name: "decrypt", raw: `{"key_id":"cloud-test","encrypted_private_key_pem":"not encrypted"}`},
		{name: "invalid private key", raw: `{"key_id":"cloud-test","encrypted_private_key_pem":"enc:aW52YWxpZA=="}`},
		{name: "invalid settings", raw: "{"},
		{name: "empty settings", raw: ""},
		{name: "missing encrypted key", raw: `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newPackageCloudKeySettingRepo()
			repo.getErr = tc.dbErr
			require.NoError(t, repo.Set(ctx, settingKeyMatchaPackageCloudKey, tc.raw))
			svc := packageCloudKeyServiceForTest(key)
			svc.settingRepo, svc.encryptor = repo, encryptor
			_, err := svc.Resolve(ctx, key.KeyID)
			require.Error(t, err, "must not silently fall back to config")
			if tc.dbErr != nil {
				require.ErrorIs(t, err, tc.dbErr)
			}
			_, err = svc.Get(ctx)
			require.Error(t, err)
			repo.getErr = nil
			require.NoError(t, repo.Set(ctx, settingKeyMatchaPackageCloudKey, string(good)))
			resolved, err := svc.Resolve(ctx, key.KeyID)
			require.NoError(t, err, "failed reads must be retried")
			require.Equal(t, key.PublicKeySPKIPEM, resolved.PublicKeySPKIPEM)
		})
	}
}

type packageCloudKeySettingRepo struct {
	mu           sync.Mutex
	values       map[string]string
	getCalls     int
	getErr       error
	beforeCreate func()
}

func newPackageCloudKeySettingRepo() *packageCloudKeySettingRepo {
	return &packageCloudKeySettingRepo{values: map[string]string{}}
}

func (r *packageCloudKeySettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getCalls++
	if r.getErr != nil {
		return nil, r.getErr
	}
	value, ok := r.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *packageCloudKeySettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *packageCloudKeySettingRepo) CreateIfAbsent(_ context.Context, key, value string) (bool, error) {
	if r.beforeCreate != nil {
		r.beforeCreate()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.values[key]; ok {
		return false, nil
	}
	r.values[key] = value
	return true, nil
}

func (r *packageCloudKeySettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *packageCloudKeySettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := map[string]string{}
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (r *packageCloudKeySettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *packageCloudKeySettingRepo) GetAll(context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := map[string]string{}
	for key, value := range r.values {
		values[key] = value
	}
	return values, nil
}

func (r *packageCloudKeySettingRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

type packageCloudKeyEncryptor struct{}

func (packageCloudKeyEncryptor) Encrypt(plaintext string) (string, error) {
	return "enc:" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (packageCloudKeyEncryptor) Decrypt(ciphertext string) (string, error) {
	value, ok := strings.CutPrefix(ciphertext, "enc:")
	if !ok {
		return "", fmt.Errorf("not encrypted")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
