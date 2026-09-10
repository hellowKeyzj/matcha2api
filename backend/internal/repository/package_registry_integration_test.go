//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMatchaPackagePublishDraftVersion(t *testing.T) {
	ctx := context.Background()
	repo := NewMatchaPackageRepository(integrationDB)
	pkg, err := repo.CreatePackage(ctx, &model.MatchaPackage{
		PackageKey: fmt.Sprintf("publish-test-%d", time.Now().UnixNano()),
		Name:       "Publish test", Status: model.MatchaPackageStatusActive,
		Metadata: map[string]any{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.DeletePackage(ctx, pkg.ID)) })

	for _, status := range []string{
		model.MatchaPackageVersionStatusDraft,
		model.MatchaPackageVersionStatusPublished,
		model.MatchaPackageVersionStatusDisabled,
		model.MatchaPackageVersionStatusYanked,
	} {
		t.Run(status, func(t *testing.T) {
			version, err := repo.CreateVersion(ctx, &model.MatchaPackageVersion{
				PackageID: pkg.ID, Version: status, Kind: model.MatchaPackageKindSkill,
				Runtime: "openclaw", SHA256: strings.Repeat("a", 64), SizeBytes: 1,
				ObjectKey: fmt.Sprintf("publish-test/%d/%s", pkg.ID, status),
				Status:    status, PriceCurrency: "USD", SurchargePolicy: map[string]any{}, Metadata: map[string]any{"adminNote": "retain"},
			})
			require.NoError(t, err)
			repaired, err := repo.MergeVersionMetadata(ctx, version.ID, map[string]any{
				"cloudEnvelope": map[string]any{"keyId": "test", "algorithm": "rsa-oaep-sha256", "ciphertextBase64": "YQ"},
			})
			require.NoError(t, err)
			require.Equal(t, status, repaired.Status)
			require.Equal(t, version.ObjectKey, repaired.ObjectKey)
			require.Equal(t, "retain", repaired.Metadata["adminNote"])
			require.Equal(t, "test", repaired.Metadata["cloudEnvelope"].(map[string]any)["keyId"])
			published, err := repo.PublishDraftVersion(ctx, version.ID)
			if status == model.MatchaPackageVersionStatusDraft || status == model.MatchaPackageVersionStatusPublished {
				require.NoError(t, err)
				require.Equal(t, model.MatchaPackageVersionStatusPublished, published.Status)
				require.NotNil(t, published.PublishedAt)
				again, err := repo.PublishDraftVersion(ctx, version.ID)
				require.NoError(t, err)
				require.True(t, published.PublishedAt.Equal(*again.PublishedAt))
				return
			}
			require.ErrorIs(t, err, service.ErrMatchaPackageVersionNotPublishable)
			unchanged, err := repo.GetVersion(ctx, version.ID)
			require.NoError(t, err)
			require.Equal(t, status, unchanged.Status)
		})
	}
	_, err = repo.PublishDraftVersion(ctx, -1)
	require.ErrorIs(t, err, service.ErrMatchaPackageVersionNotFound)
}
