package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const browserLoginTicketPrefix = "browser_login:"

type browserLoginTicketStore struct {
	redis *redis.Client
}

func NewBrowserLoginTicketStore(redisClient *redis.Client) service.BrowserLoginTicketStore {
	return &browserLoginTicketStore{redis: redisClient}
}

func (s *browserLoginTicketStore) Store(ctx context.Context, ticketHash string, data *service.BrowserLoginTicketData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.redis.Set(ctx, browserLoginTicketPrefix+ticketHash, payload, service.BrowserLoginTicketTTL).Err()
}

func (s *browserLoginTicketStore) Consume(ctx context.Context, ticketHash string) (*service.BrowserLoginTicketData, error) {
	payload, err := s.redis.GetDel(ctx, browserLoginTicketPrefix+ticketHash).Bytes()
	if err == redis.Nil {
		return nil, service.ErrInvalidBrowserLoginTicket
	}
	if err != nil {
		return nil, err
	}
	var data service.BrowserLoginTicketData
	if err := json.Unmarshal(payload, &data); err != nil || data.UserID <= 0 {
		return nil, service.ErrInvalidBrowserLoginTicket
	}
	return &data, nil
}
