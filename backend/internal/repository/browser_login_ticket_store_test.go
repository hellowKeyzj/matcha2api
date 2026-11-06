package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestBrowserLoginTicketStore(t *testing.T) (service.BrowserLoginTicketStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return NewBrowserLoginTicketStore(client), mr
}

func TestBrowserLoginTicketStore_ConsumeOnceConcurrently(t *testing.T) {
	store, mr := newTestBrowserLoginTicketStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	want := &service.BrowserLoginTicketData{UserID: 42, TokenVersion: 7}
	require.NoError(t, store.Store(ctx, "ticket-hash", want))

	const consumers = 32
	type result struct {
		data *service.BrowserLoginTicketData
		err  error
	}
	results := make(chan result, consumers)
	start := make(chan struct{})
	for range consumers {
		go func() {
			<-start
			data, err := store.Consume(ctx, "ticket-hash")
			results <- result{data: data, err: err}
		}()
	}
	close(start)

	successes := 0
	for range consumers {
		got := <-results
		if got.err == nil {
			successes++
			require.Equal(t, want, got.data)
		} else {
			require.ErrorIs(t, got.err, service.ErrInvalidBrowserLoginTicket)
			require.Nil(t, got.data)
		}
	}
	require.Equal(t, 1, successes)
	require.False(t, mr.Exists(browserLoginTicketPrefix+"ticket-hash"))
	data, err := store.Consume(ctx, "ticket-hash")
	require.ErrorIs(t, err, service.ErrInvalidBrowserLoginTicket)
	require.Nil(t, data)
}

func TestBrowserLoginTicketStore_TTL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
		expired bool
	}{
		{name: "just_before_expiry", elapsed: service.BrowserLoginTicketTTL - time.Millisecond},
		{name: "at_expiry", elapsed: service.BrowserLoginTicketTTL, expired: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, mr := newTestBrowserLoginTicketStore(t)
			ctx := context.Background()
			want := &service.BrowserLoginTicketData{UserID: 42, TokenVersion: 7}
			require.NoError(t, store.Store(ctx, "ticket-hash", want))
			require.Equal(t, service.BrowserLoginTicketTTL, mr.TTL(browserLoginTicketPrefix+"ticket-hash"))
			mr.FastForward(tc.elapsed)

			data, err := store.Consume(ctx, "ticket-hash")
			if tc.expired {
				require.ErrorIs(t, err, service.ErrInvalidBrowserLoginTicket)
				require.Nil(t, data)
			} else {
				require.NoError(t, err)
				require.Equal(t, want, data)
			}
		})
	}
}
