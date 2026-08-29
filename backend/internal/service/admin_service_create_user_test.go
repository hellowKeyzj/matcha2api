//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAdminService_CreateUser_Success(t *testing.T) {
	repo := &userRepoStub{nextID: 10}
	svc := &adminServiceImpl{userRepo: repo}
	balance := 12.5

	input := &CreateUserInput{
		Email:         "user@test.com",
		Password:      "strong-pass",
		Username:      "tester",
		Notes:         "note",
		Balance:       &balance,
		Concurrency:   7,
		AllowedGroups: []int64{3, 5},
	}

	user, err := svc.CreateUser(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, int64(10), user.ID)
	require.Equal(t, input.Email, user.Email)
	require.Equal(t, input.Username, user.Username)
	require.Equal(t, input.Notes, user.Notes)
	require.Equal(t, balance, user.Balance)
	require.Equal(t, input.Concurrency, user.Concurrency)
	require.Equal(t, input.AllowedGroups, user.AllowedGroups)
	require.Equal(t, RoleUser, user.Role)
	require.Equal(t, StatusActive, user.Status)
	require.True(t, user.CheckPassword(input.Password))
	require.Len(t, repo.created, 1)
	require.Equal(t, user, repo.created[0])
}

func TestAdminService_CreateUser_UsesDefaultBalanceWhenBalanceOmitted(t *testing.T) {
	repo := &userRepoStub{nextID: 11}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserBalance: 0,
		},
	}
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDefaultBalance: "0.02",
	}}, cfg)
	svc := &adminServiceImpl{userRepo: repo, settingService: settingService}

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "default-balance@test.com",
		Password: "strong-pass",
	})

	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, 0.02, user.Balance)
	require.Len(t, repo.created, 1)
	require.Equal(t, 0.02, repo.created[0].Balance)
}

func TestAdminService_CreateUser_ExplicitZeroBalanceOverridesDefault(t *testing.T) {
	repo := &userRepoStub{nextID: 12}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserBalance: 0,
		},
	}
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDefaultBalance: "0.02",
	}}, cfg)
	svc := &adminServiceImpl{userRepo: repo, settingService: settingService}
	balance := 0.0

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "zero-balance@test.com",
		Password: "strong-pass",
		Balance:  &balance,
	})

	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, 0.0, user.Balance)
	require.Len(t, repo.created, 1)
	require.Equal(t, 0.0, repo.created[0].Balance)
}

func TestAdminService_CreateUser_EmailExists(t *testing.T) {
	repo := &userRepoStub{createErr: ErrEmailExists}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "dup@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, ErrEmailExists)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_CreateError(t *testing.T) {
	createErr := errors.New("db down")
	repo := &userRepoStub{createErr: createErr}
	svc := &adminServiceImpl{userRepo: repo}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "user@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, createErr)
	require.Empty(t, repo.created)
}

func TestAdminService_CreateUser_AssignsDefaultSubscriptions(t *testing.T) {
	repo := &userRepoStub{nextID: 21}
	assigner := &defaultSubscriptionAssignerStub{}
	cfg := &config.Config{
		Default: config.DefaultConfig{
			UserBalance:     0,
			UserConcurrency: 1,
		},
	}
	settingService := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDefaultSubscriptions: `[{"group_id":5,"validity_days":30}]`,
	}}, cfg)
	svc := &adminServiceImpl{
		userRepo:           repo,
		settingService:     settingService,
		defaultSubAssigner: assigner,
	}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "new-user@test.com",
		Password: "password",
	})
	require.NoError(t, err)
	require.Len(t, assigner.calls, 1)
	require.Equal(t, int64(21), assigner.calls[0].UserID)
	require.Equal(t, int64(5), assigner.calls[0].GroupID)
	require.Equal(t, 30, assigner.calls[0].ValidityDays)
}

type adminDefaultGatewayAccessStub struct {
	requireCalls int
	provisionIDs []int64
	requireErr   error
	provisionErr error
}

func (s *adminDefaultGatewayAccessStub) RequireDefaultShardGroup(context.Context) (*Group, error) {
	s.requireCalls++
	if s.requireErr != nil {
		return nil, s.requireErr
	}
	return &Group{ID: 5, Status: StatusActive, ShardEnabled: true, ShardMaxUsersPerShard: 2}, nil
}

func (s *adminDefaultGatewayAccessStub) ProvisionDefaultGatewayAccess(_ context.Context, userID int64) (*DefaultGatewayAccess, error) {
	s.provisionIDs = append(s.provisionIDs, userID)
	if s.provisionErr != nil {
		return nil, s.provisionErr
	}
	return &DefaultGatewayAccess{GroupID: 5, ShardID: 9, APIKeyID: 11}, nil
}

func TestAdminService_CreateUser_ProvisionsDefaultGatewayAccessForOrdinaryUser(t *testing.T) {
	repo := &userRepoStub{nextID: 31}
	access := &adminDefaultGatewayAccessStub{}
	svc := &adminServiceImpl{userRepo: repo, defaultGatewayAccess: access}

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "ordinary@test.com",
		Password: "password",
	})
	require.NoError(t, err)
	require.Equal(t, int64(31), user.ID)
	require.Equal(t, 1, access.requireCalls)
	require.Equal(t, []int64{31}, access.provisionIDs)
	require.Empty(t, repo.deletedIDs)
}

func TestAdminService_CreateUser_SkipsDefaultGatewayAccessForAdminUser(t *testing.T) {
	repo := &userRepoStub{nextID: 32}
	access := &adminDefaultGatewayAccessStub{}
	svc := &adminServiceImpl{userRepo: repo, defaultGatewayAccess: access}

	user, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "admin@test.com",
		Password: "password",
		Role:     RoleAdmin,
	})
	require.NoError(t, err)
	require.Equal(t, RoleAdmin, user.Role)
	require.Zero(t, access.requireCalls)
	require.Empty(t, access.provisionIDs)
}

func TestAdminService_CreateUser_RollsBackUserWhenDefaultGatewayProvisionFails(t *testing.T) {
	repo := &userRepoStub{nextID: 33}
	provisionErr := errors.New("provision failed")
	access := &adminDefaultGatewayAccessStub{provisionErr: provisionErr}
	svc := &adminServiceImpl{userRepo: repo, defaultGatewayAccess: access}

	_, err := svc.CreateUser(context.Background(), &CreateUserInput{
		Email:    "rollback@test.com",
		Password: "password",
	})
	require.ErrorIs(t, err, provisionErr)
	require.Equal(t, []int64{33}, repo.deletedIDs)
}
