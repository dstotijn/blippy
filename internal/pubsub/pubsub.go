package pubsub

import "sync"

// Broker manages per-topic event subscriptions.
type Broker struct {
	mu   sync.RWMutex
	subs map[string]map[*Subscription]struct{}
	busy map[string]struct{}
}

// Subscription receives events for a single conversation.
type Subscription struct {
	conversationID string
	C              <-chan any
	ch             chan any
}

// New creates a new Broker.
func New() *Broker {
	return &Broker{
		subs: make(map[string]map[*Subscription]struct{}),
		busy: make(map[string]struct{}),
	}
}

// Subscribe returns a Subscription that receives events for the given conversation.
func (b *Broker) Subscribe(conversationID string) *Subscription {
	ch := make(chan any, 256)
	sub := &Subscription{
		conversationID: conversationID,
		C:              ch,
		ch:             ch,
	}

	b.mu.Lock()
	if b.subs[conversationID] == nil {
		b.subs[conversationID] = make(map[*Subscription]struct{})
	}
	b.subs[conversationID][sub] = struct{}{}
	b.mu.Unlock()

	return sub
}

// Unsubscribe removes a subscription and closes its channel.
func (b *Broker) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	if subs, ok := b.subs[sub.conversationID]; ok {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(b.subs, sub.conversationID)
		}
	}
	b.mu.Unlock()

	close(sub.ch)
}

// Publish sends an event to all subscribers of the conversation.
// Non-blocking: drops the event for slow subscribers.
func (b *Broker) Publish(conversationID string, event any) {
	b.mu.RLock()
	for sub := range b.subs[conversationID] {
		select {
		case sub.ch <- event:
		default:
		}
	}
	b.mu.RUnlock()
}

// SetBusy marks a conversation as having an active turn.
// Returns false if the conversation is already busy.
func (b *Broker) SetBusy(conversationID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.busy[conversationID]; ok {
		return false
	}
	b.busy[conversationID] = struct{}{}
	return true
}

// ClearBusy unmarks a conversation as busy.
func (b *Broker) ClearBusy(conversationID string) {
	b.mu.Lock()
	delete(b.busy, conversationID)
	b.mu.Unlock()
}

// IsBusy checks if a conversation has an active turn.
func (b *Broker) IsBusy(conversationID string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.busy[conversationID]
	return ok
}
