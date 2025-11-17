package graph

import (
	"context"
	"encoding/json"
	"sync"
)

// PubSub defines the interface for publishing and subscribing to events.
// Implementations can use in-memory channels, Redis, Kafka, or other message brokers.
//
// Example Usage:
//
//	pubsub := graph.NewInMemoryPubSub()
//	defer pubsub.Close()
//
//	// Subscribe to a topic
//	ctx := context.Background()
//	subscription := pubsub.Subscribe(ctx, "messages:channel1")
//
//	// Publish an event
//	pubsub.Publish(ctx, "messages:channel1", map[string]string{"text": "Hello"})
//
//	// Receive events
//	for msg := range subscription {
//	    fmt.Println("Received:", string(msg.Data))
//	}
type PubSub interface {
	// Publish sends data to all subscribers of a topic.
	// The data will be JSON-marshaled automatically.
	//
	// Returns an error if:
	//   - JSON marshaling fails
	//   - Context is canceled
	//   - PubSub is closed
	Publish(ctx context.Context, topic string, data interface{}) error

	// Subscribe creates a new subscription to a topic.
	// Returns a channel that receives messages published to the topic.
	//
	// The subscription remains active until:
	//   - The context is canceled
	//   - Unsubscribe is called with the subscription ID
	//   - The PubSub is closed
	//
	// The returned channel will be closed when the subscription ends.
	Subscribe(ctx context.Context, topic string) <-chan *Message

	// Unsubscribe removes a subscription by its ID.
	// The subscription's message channel will be closed.
	Unsubscribe(ctx context.Context, subscriptionID string) error

	// Close shuts down the PubSub system and closes all active subscriptions.
	Close() error
}

// Message represents a published message with its topic and data payload.
type Message struct {
	// Topic is the channel/topic name where this message was published
	Topic string

	// Data is the JSON-encoded payload
	Data []byte
}

// InMemoryPubSub is a simple in-memory implementation of PubSub.
// It's suitable for development, testing, and single-instance deployments.
// For production multi-instance deployments, use RedisPubSub or similar.
type InMemoryPubSub struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]chan *Message // topic -> subscriptionID -> channel
	nextSubID     int
	closed        bool
}

// NewInMemoryPubSub creates a new in-memory PubSub implementation.
//
// Example:
//
//	pubsub := graph.NewInMemoryPubSub()
//	defer pubsub.Close()
//
//	ctx := context.Background()
//	sub := pubsub.Subscribe(ctx, "events")
//
//	go func() {
//	    for msg := range sub {
//	        fmt.Println("Event:", string(msg.Data))
//	    }
//	}()
//
//	pubsub.Publish(ctx, "events", map[string]string{"type": "user_created"})
func NewInMemoryPubSub() *InMemoryPubSub {
	return &InMemoryPubSub{
		subscriptions: make(map[string]map[string]chan *Message),
	}
}

// Publish sends data to all subscribers of the topic.
// Slow subscribers are skipped to prevent blocking.
func (p *InMemoryPubSub) Publish(ctx context.Context, topic string, data interface{}) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return ErrPubSubClosed
	}

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	msg := &Message{
		Topic: topic,
		Data:  jsonData,
	}

	// Send to all subscribers of this topic
	subs, exists := p.subscriptions[topic]
	if !exists {
		return nil // No subscribers
	}

	for _, ch := range subs {
		select {
		case ch <- msg:
			// Message sent successfully
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Skip slow consumers (non-blocking)
		}
	}

	return nil
}

// Subscribe creates a subscription to a topic.
// The subscription is automatically cleaned up when the context is canceled.
func (p *InMemoryPubSub) Subscribe(ctx context.Context, topic string) <-chan *Message {
	p.mu.Lock()

	// Generate unique subscription ID
	p.nextSubID++
	subID := string(rune(p.nextSubID))

	// Create buffered channel to prevent blocking publishers
	ch := make(chan *Message, 100)

	// Initialize topic map if needed
	if p.subscriptions[topic] == nil {
		p.subscriptions[topic] = make(map[string]chan *Message)
	}

	// Store subscription
	p.subscriptions[topic][subID] = ch
	p.mu.Unlock()

	// Clean up subscription when context is done
	go func() {
		<-ctx.Done()
		p.mu.Lock()
		defer p.mu.Unlock()

		// Remove subscription
		if subs, exists := p.subscriptions[topic]; exists {
			if _, exists := subs[subID]; exists {
				delete(subs, subID)
				close(ch)

				// Clean up empty topic maps
				if len(subs) == 0 {
					delete(p.subscriptions, topic)
				}
			}
		}
	}()

	return ch
}

// Unsubscribe removes a subscription by ID (not commonly used with context-based cleanup).
func (p *InMemoryPubSub) Unsubscribe(ctx context.Context, subscriptionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPubSubClosed
	}

	// Find and remove the subscription
	for topic, subs := range p.subscriptions {
		if ch, exists := subs[subscriptionID]; exists {
			delete(subs, subscriptionID)
			close(ch)

			// Clean up empty topic maps
			if len(subs) == 0 {
				delete(p.subscriptions, topic)
			}
			return nil
		}
	}

	return ErrSubscriptionNotFound
}

// Close shuts down the PubSub and closes all active subscriptions.
func (p *InMemoryPubSub) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrPubSubClosed
	}

	p.closed = true

	// Close all subscription channels
	for _, subs := range p.subscriptions {
		for _, ch := range subs {
			close(ch)
		}
	}

	// Clear subscriptions map
	p.subscriptions = make(map[string]map[string]chan *Message)

	return nil
}

// Common errors
var (
	ErrPubSubClosed          = newError("pubsub is closed")
	ErrSubscriptionNotFound  = newError("subscription not found")
)

type pubsubError struct {
	msg string
}

func newError(msg string) error {
	return &pubsubError{msg: msg}
}

func (e *pubsubError) Error() string {
	return e.msg
}