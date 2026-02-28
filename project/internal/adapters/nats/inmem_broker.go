package nats

import (
	"context"
	"strings"
	"sync"

	"diplom.com/m/internal/domain"
)

type InMemoryBroker struct {
	mu   sync.RWMutex
	subs []subscription
}

type subscription struct {
	subject string
	h       func(domain.Event) error
}

func NewInMemoryBroker() *InMemoryBroker { return &InMemoryBroker{} }

func (b *InMemoryBroker) Publish(ctx context.Context, subject string, evt domain.Event) error {
	return b.dispatch(ctx, subject, evt)
}

func (b *InMemoryBroker) PublishUI(ctx context.Context, subject string, evt domain.Event) error {
	return b.dispatch(ctx, subject, evt)
}

func (b *InMemoryBroker) Subscribe(ctx context.Context, subject, durable, queue string, handler func(domain.Event) error) error {
	b.mu.Lock()
	b.subs = append(b.subs, subscription{subject: subject, h: handler})
	b.mu.Unlock()
	<-ctx.Done()
	return nil
}

func (b *InMemoryBroker) dispatch(ctx context.Context, subject string, evt domain.Event) error {
	b.mu.RLock()
	subs := append([]subscription(nil), b.subs...)
	b.mu.RUnlock()
	for _, s := range subs {
		if subjectMatch(s.subject, subject) {
			_ = s.h(evt)
		}
	}
	return nil
}

func subjectMatch(pattern, subject string) bool {
	if pattern == subject {
		return true
	}
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(subject, prefix)
	}
	if strings.HasSuffix(pattern, ".>") {
		prefix := strings.TrimSuffix(pattern, ">")
		return strings.HasPrefix(subject, prefix)
	}
	return false
}
