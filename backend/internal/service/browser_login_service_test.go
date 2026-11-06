//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type browserLoginTicketStoreStub struct {
	hash       string
	data       *BrowserLoginTicketData
	storeErr   error
	consumeErr error
}

func (s *browserLoginTicketStoreStub) Store(_ context.Context, hash string, data *BrowserLoginTicketData) error {
	if s.storeErr != nil {
		return s.storeErr
	}
	s.hash, s.data = hash, data
	return nil
}

func (s *browserLoginTicketStoreStub) Consume(_ context.Context, hash string) (*BrowserLoginTicketData, error) {
	if s.consumeErr != nil {
		return nil, s.consumeErr
	}
	if hash != s.hash || s.data == nil {
		return nil, ErrInvalidBrowserLoginTicket
	}
	data := s.data
	s.data = nil
	return data, nil
}

func newBrowserLoginTestService() (*BrowserLoginService, *userRepoStub, *settingPublicRepoStub, *browserLoginTicketStoreStub) {
	users := &userRepoStub{user: &User{
		ID: 42, Email: "user@example.com", PasswordHash: "original-hash", Status: StatusActive, TokenVersion: 7,
	}}
	settings := &settingPublicRepoStub{}
	tickets := &browserLoginTicketStoreStub{}
	// Rejected exchanges must never reach token issuance.
	svc := NewBrowserLoginService(tickets, users, nil, NewSettingService(settings, &config.Config{}))
	return svc, users, settings, tickets
}

func TestBrowserLoginIssue_RejectsDisabledUser(t *testing.T) {
	svc, users, _, tickets := newBrowserLoginTestService()
	users.user.Status = StatusDisabled
	ticket, err := svc.Issue(context.Background(), users.user.ID)
	require.ErrorIs(t, err, ErrInvalidBrowserLoginTicket)
	require.Equal(t, http.StatusUnauthorized, infraerrors.Code(err))
	require.Nil(t, ticket)
	require.Nil(t, tickets.data)
}

func TestBrowserLoginExchange_RejectsChangedUser(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*User)
	}{
		{name: "disabled_after_issue", change: func(u *User) { u.Status = StatusDisabled }},
		{name: "token_version_changed", change: func(u *User) { u.TokenVersion++ }},
		{name: "password_fingerprint_changed", change: func(u *User) { u.PasswordHash = "new-hash" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, users, _, tickets := newBrowserLoginTestService()
			ctx := context.Background()
			ticket, err := svc.Issue(ctx, users.user.ID)
			require.NoError(t, err)
			require.Equal(t, hashToken(ticket.Ticket), tickets.hash)
			require.Equal(t, users.user.ID, tickets.data.UserID)
			require.Equal(t, resolvedTokenVersion(users.user), tickets.data.TokenVersion)

			fresh := *users.user
			tc.change(&fresh)
			users.user = &fresh
			pair, user, err := svc.Exchange(ctx, ticket.Ticket)
			require.ErrorIs(t, err, ErrInvalidBrowserLoginTicket)
			require.Equal(t, http.StatusUnauthorized, infraerrors.Code(err))
			require.Nil(t, pair)
			require.Nil(t, user)
			require.Nil(t, tickets.data, "even a rejected redemption consumes the ticket")
		})
	}
}

func TestBrowserLogin_DependencyErrorsRemainServiceUnavailable(t *testing.T) {
	for _, phase := range []string{"issue", "exchange"} {
		for _, dependency := range []string{"user_repository", "settings_repository", "ticket_store"} {
			t.Run(phase+"/"+dependency, func(t *testing.T) {
				svc, users, settings, tickets := newBrowserLoginTestService()
				ctx := context.Background()
				var ticket *BrowserLoginTicket
				var err error
				if phase == "exchange" {
					ticket, err = svc.Issue(ctx, users.user.ID)
					require.NoError(t, err)
				}
				cause := errors.New("repository unavailable")
				switch dependency {
				case "user_repository":
					users.getErr = cause
				case "settings_repository":
					settings.err = cause
				case "ticket_store":
					tickets.storeErr, tickets.consumeErr = cause, cause
				}

				if phase == "issue" {
					ticket, err = svc.Issue(ctx, users.user.ID)
					require.Nil(t, ticket)
				} else {
					var pair *TokenPair
					var user *User
					pair, user, err = svc.Exchange(ctx, ticket.Ticket)
					require.Nil(t, pair)
					require.Nil(t, user)
				}
				require.ErrorIs(t, err, ErrServiceUnavailable)
				require.ErrorIs(t, err, cause)
				require.Equal(t, http.StatusServiceUnavailable, infraerrors.Code(err))
			})
		}
	}
}

func TestBrowserLogin_MissingUserIsUnauthorized(t *testing.T) {
	for _, phase := range []string{"issue", "exchange"} {
		t.Run(phase, func(t *testing.T) {
			svc, users, _, _ := newBrowserLoginTestService()
			ctx := context.Background()
			ticket, err := svc.Issue(ctx, users.user.ID)
			require.NoError(t, err)
			users.getErr = ErrUserNotFound
			if phase == "issue" {
				ticket, err = svc.Issue(ctx, users.user.ID)
				require.Nil(t, ticket)
			} else {
				var pair *TokenPair
				var user *User
				pair, user, err = svc.Exchange(ctx, ticket.Ticket)
				require.Nil(t, pair)
				require.Nil(t, user)
			}
			require.ErrorIs(t, err, ErrInvalidBrowserLoginTicket)
			require.Equal(t, http.StatusUnauthorized, infraerrors.Code(err))
		})
	}
}
