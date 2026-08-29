package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestGetLiveCallForIdentityRechecksShardScope(t *testing.T) {
	lookupErr := errors.New("scope unavailable")
	for _, tc := range []struct {
		name    string
		ids     map[int64]struct{}
		err     error
		wantErr error
	}{
		{name: "owner allowed", ids: map[int64]struct{}{11: {}}},
		{name: "owner withdrawn", ids: map[int64]struct{}{12: {}}, wantErr: ErrLiveIdentityMismatch},
		{name: "empty effective scope", ids: map[int64]struct{}{}, wantErr: ErrLiveIdentityMismatch},
		{name: "scope lookup fails closed", err: lookupErr, wantErr: lookupErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(44)
			record := &LiveCallRecord{
				CallID: "call_scope", CallHash: hashLiveCallID("call_scope"), AccountID: 11,
				APIKeyID: 22, UserID: 33, GroupID: groupID, LeaseID: "existing-lease",
				Controller: LiveControllerObserver, ControllerOwner: "existing-observer",
			}
			store := &liveTestStore{}
			require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
			shards := &openAISchedulerShardRepoStub{
				scope: &GroupShardAccountScope{Enabled: true, ShardID: 4, AccountIDs: tc.ids}, err: tc.err,
			}
			svc := &OpenAIGatewayService{cache: store, groupShardRepo: shards}
			ctx := context.WithValue(context.Background(), ctxkey.Group, &Group{
				ID: groupID, Hydrated: true, Platform: PlatformOpenAI, Status: StatusActive, ShardEnabled: true,
			})
			loaded, err := svc.GetLiveCallForIdentity(ctx, record.CallID, LiveCallIdentity{
				APIKeyID: record.APIKeyID, UserID: record.UserID, GroupID: &groupID,
			})
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Nil(t, loaded)
			} else {
				require.NoError(t, err)
				require.Equal(t, record.AccountID, loaded.AccountID)
			}
			require.Equal(t, 1, shards.listScopeCalls)
			require.Equal(t, record.UserID, shards.lastUserID, "identity supplies the user, not ctxkey.UserID")
			require.Equal(t, groupID, shards.lastGroupID)
			require.Equal(t, PlatformOpenAI, shards.lastPlatform)
			retained, err := store.GetLiveCall(ctx, record.CallHash)
			require.NoError(t, err)
			require.Equal(t, record, retained, "preflight must not claim a controller, release a lease, or delete the binding")
		})
	}
}
