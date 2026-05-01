package agent

import (
	"context"
	"time"

	"github.com/marang/franz-agent/internal/message"
)

type messageUpdateThrottler struct {
	messages message.Service
	interval time.Duration
	last     time.Time
}

func newMessageUpdateThrottler(messages message.Service, interval time.Duration) *messageUpdateThrottler {
	return &messageUpdateThrottler{
		messages: messages,
		interval: interval,
	}
}

func (u *messageUpdateThrottler) Update(ctx context.Context, msg message.Message) error {
	if u.last.IsZero() || time.Since(u.last) >= u.interval {
		return u.Flush(ctx, msg)
	}
	return nil
}

func (u *messageUpdateThrottler) Flush(ctx context.Context, msg message.Message) error {
	u.last = time.Now()
	return u.messages.Update(ctx, msg)
}
