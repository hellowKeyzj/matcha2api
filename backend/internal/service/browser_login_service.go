package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const BrowserLoginTicketTTL = 60 * time.Second

var ErrInvalidBrowserLoginTicket = infraerrors.Unauthorized("INVALID_BROWSER_LOGIN_TICKET", "Invalid or expired browser login ticket")

type BrowserLoginTicketData struct {
	UserID       int64 `json:"user_id"`
	TokenVersion int64 `json:"token_version"`
}

// BrowserLoginTicketStore persists only ticket hashes and consumes each ticket once.
type BrowserLoginTicketStore interface {
	Store(ctx context.Context, ticketHash string, data *BrowserLoginTicketData) error
	Consume(ctx context.Context, ticketHash string) (*BrowserLoginTicketData, error)
}

type BrowserLoginTicket struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
}

type BrowserLoginService struct {
	tickets        BrowserLoginTicketStore
	userRepo       UserRepository
	authService    *AuthService
	settingService *SettingService
}

func NewBrowserLoginService(tickets BrowserLoginTicketStore, userRepo UserRepository, authService *AuthService, settingService *SettingService) *BrowserLoginService {
	return &BrowserLoginService{tickets: tickets, userRepo: userRepo, authService: authService, settingService: settingService}
}

func (s *BrowserLoginService) Issue(ctx context.Context, userID int64) (*BrowserLoginTicket, error) {
	user, err := s.getActiveUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, infraerrors.InternalServer("BROWSER_LOGIN_TICKET_FAILED", "Failed to create browser login ticket").WithCause(fmt.Errorf("generate browser login ticket randomness: %w", err))
	}
	ticket := base64.RawURLEncoding.EncodeToString(random)
	data := &BrowserLoginTicketData{UserID: user.ID, TokenVersion: resolvedTokenVersion(user)}
	if err := s.tickets.Store(ctx, hashToken(ticket), data); err != nil {
		return nil, ErrServiceUnavailable.WithCause(fmt.Errorf("store browser login ticket: %w", err))
	}
	return &BrowserLoginTicket{Ticket: ticket, ExpiresIn: int(BrowserLoginTicketTTL / time.Second)}, nil
}

func (s *BrowserLoginService) Exchange(ctx context.Context, ticket string) (*TokenPair, *User, error) {
	if len(ticket) != base64.RawURLEncoding.EncodedLen(32) {
		return nil, nil, ErrInvalidBrowserLoginTicket
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(ticket)
	if err != nil || len(decoded) != 32 {
		return nil, nil, ErrInvalidBrowserLoginTicket
	}
	data, err := s.tickets.Consume(ctx, hashToken(ticket))
	if err != nil {
		if errors.Is(err, ErrInvalidBrowserLoginTicket) {
			return nil, nil, ErrInvalidBrowserLoginTicket
		}
		return nil, nil, ErrServiceUnavailable.WithCause(fmt.Errorf("consume browser login ticket: %w", err))
	}
	user, err := s.getActiveUser(ctx, data.UserID)
	if err != nil {
		return nil, nil, err
	}
	if resolvedTokenVersion(user) != data.TokenVersion {
		return nil, nil, ErrInvalidBrowserLoginTicket
	}

	// The redeeming browser supplies the binding; the desktop session is never copied.
	pair, err := s.authService.GenerateTokenPair(ctx, user, "")
	if err != nil {
		return nil, nil, ErrServiceUnavailable.WithCause(fmt.Errorf("generate browser login token pair: %w", err))
	}
	s.authService.RecordSuccessfulLogin(ctx, user.ID)
	return pair, user, nil
}

func (s *BrowserLoginService) getActiveUser(ctx context.Context, userID int64) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidBrowserLoginTicket
		}
		return nil, ErrServiceUnavailable.WithCause(fmt.Errorf("get browser login user: %w", err))
	}
	if user == nil || !user.IsActive() {
		return nil, ErrInvalidBrowserLoginTicket
	}
	settings, err := s.settingService.GetPublicSettings(ctx)
	if err != nil {
		return nil, ErrServiceUnavailable.WithCause(fmt.Errorf("get browser login public settings: %w", err))
	}
	if settings.BackendModeEnabled && !user.IsAdmin() {
		return nil, infraerrors.Forbidden("BACKEND_MODE_ADMIN_ONLY", "Backend mode is active. Only admin login is allowed.")
	}
	return user, nil
}
