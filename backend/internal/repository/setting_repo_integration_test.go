//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SettingRepoSuite struct {
	suite.Suite
	ctx  context.Context
	repo *settingRepository
}

func (s *SettingRepoSuite) SetupTest() {
	s.ctx = context.Background()
	tx := testEntTx(s.T())
	s.repo = NewSettingRepository(tx.Client()).(*settingRepository)
}

func TestSettingRepoSuite(t *testing.T) {
	suite.Run(t, new(SettingRepoSuite))
}

func (s *SettingRepoSuite) TestSetAndGetValue() {
	s.Require().NoError(s.repo.Set(s.ctx, "k1", "v1"), "Set")
	got, err := s.repo.GetValue(s.ctx, "k1")
	s.Require().NoError(err, "GetValue")
	s.Require().Equal("v1", got, "GetValue mismatch")
}

func (s *SettingRepoSuite) TestMatchaPackageCloudKeyEncryptionSurvivesRestart() {
	cfg := &config.Config{Totp: config.TotpConfig{
		EncryptionKey:           strings.Repeat("42", 32),
		EncryptionKeyConfigured: true,
	}}
	encryptor, err := NewAESEncryptor(cfg)
	s.Require().NoError(err)
	legacyCfg := *cfg
	legacyPEM, err := os.ReadFile("../service/testdata/sealed-v1/cloud-test.pem")
	s.Require().NoError(err)
	legacyCfg.SealedPackage.CloudKeyID = "legacy-config-key"
	legacyCfg.SealedPackage.CloudPrivateKeyPEM = string(legacyPEM)
	legacyReader := service.NewMatchaPackageCloudKeyService(s.repo, encryptor, &legacyCfg)
	legacyKey, err := legacyReader.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal("legacy-config-key", legacyKey.KeyID)

	keys := service.NewMatchaPackageCloudKeyService(s.repo, encryptor, cfg)
	status, err := keys.Generate(s.ctx, nil)
	s.Require().NoError(err)
	key, err := keys.Get(s.ctx)
	s.Require().NoError(err)

	raw, err := s.repo.GetValue(s.ctx, "matcha_package_sealed_cloud_key")
	s.Require().NoError(err)
	s.Require().NotContains(raw, "RSA PRIVATE KEY")
	var stored struct {
		EncryptedPrivateKeyPEM string `json:"encrypted_private_key_pem"`
	}
	s.Require().NoError(json.Unmarshal([]byte(raw), &stored))
	s.Require().NotEmpty(stored.EncryptedPrivateKeyPEM)
	plaintext, err := encryptor.Decrypt(stored.EncryptedPrivateKeyPEM)
	s.Require().NoError(err)
	s.Require().Equal(key.PrivateKeyPEM, strings.TrimSpace(plaintext))

	restartedEncryptor, err := NewAESEncryptor(cfg)
	s.Require().NoError(err)
	restarted := service.NewMatchaPackageCloudKeyService(s.repo, restartedEncryptor, cfg)
	reloaded, err := restarted.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(key.PrivateKeyPEM, reloaded.PrivateKeyPEM)
	s.Require().Equal(status.KeyID, reloaded.KeyID)
	current, err := legacyReader.Get(s.ctx)
	s.Require().NoError(err)
	s.Require().Equal(status.KeyID, current.KeyID)
	legacyRestarted := service.NewMatchaPackageCloudKeyService(s.repo, restartedEncryptor, &legacyCfg)
	for _, reader := range []*service.MatchaPackageCloudKeyService{legacyReader, legacyRestarted} {
		old, err := reader.Resolve(s.ctx, legacyKey.KeyID)
		s.Require().NoError(err)
		s.Require().Equal(legacyKey.PublicKeySPKIPEM, old.PublicKeySPKIPEM)
		stored, err := reader.Resolve(s.ctx, status.KeyID)
		s.Require().NoError(err)
		s.Require().Equal(key.PublicKeySPKIPEM, stored.PublicKeySPKIPEM)
		_, err = reader.Resolve(s.ctx, "unknown-key")
		s.Require().ErrorIs(err, service.ErrSealedPackageCloudKeyUnavailable)
	}
	_, err = restarted.Generate(s.ctx, nil)
	s.Require().ErrorIs(err, service.ErrSealedPackageCloudKeyExists)
	after, err := s.repo.GetValue(s.ctx, "matcha_package_sealed_cloud_key")
	s.Require().NoError(err)
	s.Require().Equal(raw, after)
}

func TestMatchaPackageCloudKeyRepositoryCreateIfAbsentConcurrent(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	key := t.Name()
	t.Cleanup(func() { require.NoError(t, NewSettingRepository(client).Delete(ctx, key)) })
	repositories := []service.MatchaPackageCloudKeyRepository{
		NewMatchaPackageCloudKeyRepository(client),
		NewMatchaPackageCloudKeyRepository(client),
	}
	type result struct {
		value   string
		created bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(repositories))
	for i, repo := range repositories {
		value := []string{"first", "second"}[i]
		go func() {
			<-start
			created, err := repo.CreateIfAbsent(ctx, key, value)
			results <- result{value, created, err}
		}()
	}
	close(start)
	winner := ""
	for range repositories {
		got := <-results
		require.NoError(t, got.err)
		if got.created {
			require.Empty(t, winner, "only one SQL insert may succeed")
			winner = got.value
		}
	}
	require.NotEmpty(t, winner)
	settings := NewSettingRepository(client)
	before, err := settings.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, winner, before.Value)
	created, err := repositories[0].CreateIfAbsent(ctx, key, "replacement")
	require.NoError(t, err)
	require.False(t, created)
	after, err := settings.Get(ctx, key)
	require.NoError(t, err)
	require.Equal(t, before, after, "DO NOTHING must preserve value and timestamp")
}

func (s *SettingRepoSuite) TestSet_Upsert() {
	s.Require().NoError(s.repo.Set(s.ctx, "k1", "v1"), "Set")
	s.Require().NoError(s.repo.Set(s.ctx, "k1", "v2"), "Set upsert")
	got, err := s.repo.GetValue(s.ctx, "k1")
	s.Require().NoError(err, "GetValue after upsert")
	s.Require().Equal("v2", got, "upsert mismatch")
}

func (s *SettingRepoSuite) TestGetValue_Missing() {
	_, err := s.repo.GetValue(s.ctx, "nonexistent")
	s.Require().Error(err, "expected error for missing key")
	s.Require().ErrorIs(err, service.ErrSettingNotFound)
}

func (s *SettingRepoSuite) TestSetMultiple_AndGetMultiple() {
	s.Require().NoError(s.repo.SetMultiple(s.ctx, map[string]string{"k2": "v2", "k3": "v3"}), "SetMultiple")
	m, err := s.repo.GetMultiple(s.ctx, []string{"k2", "k3"})
	s.Require().NoError(err, "GetMultiple")
	s.Require().Equal("v2", m["k2"])
	s.Require().Equal("v3", m["k3"])
}

func (s *SettingRepoSuite) TestGetMultiple_EmptyKeys() {
	m, err := s.repo.GetMultiple(s.ctx, []string{})
	s.Require().NoError(err, "GetMultiple with empty keys")
	s.Require().Empty(m, "expected empty map")
}

func (s *SettingRepoSuite) TestGetMultiple_Subset() {
	s.Require().NoError(s.repo.SetMultiple(s.ctx, map[string]string{"a": "1", "b": "2", "c": "3"}))
	m, err := s.repo.GetMultiple(s.ctx, []string{"a", "c", "nonexistent"})
	s.Require().NoError(err, "GetMultiple subset")
	s.Require().Equal("1", m["a"])
	s.Require().Equal("3", m["c"])
	_, exists := m["nonexistent"]
	s.Require().False(exists, "nonexistent key should not be in map")
}

func (s *SettingRepoSuite) TestGetAll() {
	s.Require().NoError(s.repo.SetMultiple(s.ctx, map[string]string{"x": "1", "y": "2"}))
	all, err := s.repo.GetAll(s.ctx)
	s.Require().NoError(err, "GetAll")
	s.Require().GreaterOrEqual(len(all), 2, "expected at least 2 settings")
	s.Require().Equal("1", all["x"])
	s.Require().Equal("2", all["y"])
}

func (s *SettingRepoSuite) TestDelete() {
	s.Require().NoError(s.repo.Set(s.ctx, "todelete", "val"))
	s.Require().NoError(s.repo.Delete(s.ctx, "todelete"), "Delete")
	_, err := s.repo.GetValue(s.ctx, "todelete")
	s.Require().Error(err, "expected missing key error after Delete")
	s.Require().ErrorIs(err, service.ErrSettingNotFound)
}

func (s *SettingRepoSuite) TestDelete_Idempotent() {
	// Delete a key that doesn't exist should not error
	s.Require().NoError(s.repo.Delete(s.ctx, "nonexistent_delete"), "Delete nonexistent should be idempotent")
}

func (s *SettingRepoSuite) TestSetMultiple_Upsert() {
	s.Require().NoError(s.repo.Set(s.ctx, "upsert_key", "old_value"))
	s.Require().NoError(s.repo.SetMultiple(s.ctx, map[string]string{"upsert_key": "new_value", "new_key": "new_val"}))

	got, err := s.repo.GetValue(s.ctx, "upsert_key")
	s.Require().NoError(err)
	s.Require().Equal("new_value", got, "SetMultiple should upsert existing key")

	got2, err := s.repo.GetValue(s.ctx, "new_key")
	s.Require().NoError(err)
	s.Require().Equal("new_val", got2)
}

// TestSet_EmptyValue 测试保存空字符串值
// 这是一个回归测试，确保可选设置（如站点Logo、API端点地址等）可以保存为空字符串
func (s *SettingRepoSuite) TestSet_EmptyValue() {
	// 测试 Set 方法保存空值
	s.Require().NoError(s.repo.Set(s.ctx, "empty_key", ""), "Set with empty value should succeed")

	got, err := s.repo.GetValue(s.ctx, "empty_key")
	s.Require().NoError(err, "GetValue for empty value")
	s.Require().Equal("", got, "empty value should be preserved")
}

// TestSetMultiple_WithEmptyValues 测试批量保存包含空字符串的设置
// 模拟用户保存站点设置时部分字段为空的场景
func (s *SettingRepoSuite) TestSetMultiple_WithEmptyValues() {
	// 模拟保存站点设置，部分字段有值，部分字段为空
	settings := map[string]string{
		"site_name":     "Sub2api",
		"site_subtitle": "Subscription to API",
		"site_logo":     "", // 用户未上传Logo
		"api_base_url":  "", // 用户未设置API地址
		"contact_info":  "", // 用户未设置联系方式
		"doc_url":       "", // 用户未设置文档链接
	}

	s.Require().NoError(s.repo.SetMultiple(s.ctx, settings), "SetMultiple with empty values should succeed")

	// 验证所有值都正确保存
	result, err := s.repo.GetMultiple(s.ctx, []string{"site_name", "site_subtitle", "site_logo", "api_base_url", "contact_info", "doc_url"})
	s.Require().NoError(err, "GetMultiple after SetMultiple with empty values")

	s.Require().Equal("Sub2api", result["site_name"])
	s.Require().Equal("Subscription to API", result["site_subtitle"])
	s.Require().Equal("", result["site_logo"], "empty site_logo should be preserved")
	s.Require().Equal("", result["api_base_url"], "empty api_base_url should be preserved")
	s.Require().Equal("", result["contact_info"], "empty contact_info should be preserved")
	s.Require().Equal("", result["doc_url"], "empty doc_url should be preserved")
}

// TestSetMultiple_UpdateToEmpty 测试将已有值更新为空字符串
// 确保用户可以清空之前设置的值
func (s *SettingRepoSuite) TestSetMultiple_UpdateToEmpty() {
	// 先设置非空值
	s.Require().NoError(s.repo.Set(s.ctx, "clearable_key", "initial_value"))

	got, err := s.repo.GetValue(s.ctx, "clearable_key")
	s.Require().NoError(err)
	s.Require().Equal("initial_value", got)

	// 更新为空值
	s.Require().NoError(s.repo.SetMultiple(s.ctx, map[string]string{"clearable_key": ""}), "Update to empty should succeed")

	got, err = s.repo.GetValue(s.ctx, "clearable_key")
	s.Require().NoError(err)
	s.Require().Equal("", got, "value should be updated to empty string")
}
