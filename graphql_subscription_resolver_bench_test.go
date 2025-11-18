package graph

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
)

// Benchmark NewSubscription creation
func BenchmarkNewSubscription_Simple(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events")
	}
}

// Benchmark BuildSubscription
func BenchmarkSubscription_Build(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events").
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event)
				close(ch)
				return ch, nil
			}).
			BuildSubscription()
	}
}

// Benchmark BuildSubscription with complex type
func BenchmarkSubscription_Build_ComplexType(b *testing.B) {
	type ComplexEvent struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"createdAt"`
		AuthorID  string    `json:"authorID"`
		Tags      []string  `json:"tags"`
		Metadata  map[string]interface{} `json:"metadata"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[ComplexEvent]("complexEvent").
			WithDescription("Complex event subscription").
			WithArgs(graphql.FieldConfigArgument{
				"filter": &graphql.ArgumentConfig{Type: graphql.String},
			}).
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *ComplexEvent, error) {
				ch := make(chan *ComplexEvent)
				close(ch)
				return ch, nil
			}).
			BuildSubscription()
	}
}

// Benchmark subscription execution
func BenchmarkSubscription_Execute(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 1)
			ch <- &Event{ID: "1", Message: "test"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Subscribe(graphql.ResolveParams{
			Context: context.Background(),
		})
	}
}

// Benchmark subscription with filter
func BenchmarkSubscription_WithFilter(b *testing.B) {
	type Event struct {
		ID     string `json:"id"`
		UserID string `json:"userID"`
	}

	events := make([]*Event, 100)
	for i := 0; i < 100; i++ {
		userID := "user1"
		if i%2 == 0 {
			userID = "user2"
		}
		events[i] = &Event{ID: string(rune(i)), UserID: userID}
	}

	sub := NewSubscription[Event]("events").
		WithArgs(graphql.FieldConfigArgument{
			"userID": &graphql.ArgumentConfig{Type: graphql.String},
		}).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, len(events))
			for _, event := range events {
				ch <- event
			}
			close(ch)
			return ch, nil
		}).
		WithFilter(func(ctx context.Context, data *Event, p ResolveParams) bool {
			userID, _ := GetArgString(p, "userID")
			return data.UserID == userID
		}).
		BuildSubscription()

	field := sub.Serve()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, _ := field.Subscribe(graphql.ResolveParams{
			Args:    map[string]interface{}{"userID": "user1"},
			Context: context.Background(),
		})
		if ch, ok := result.(<-chan interface{}); ok {
			// Drain channel
			for range ch {
			}
		}
	}
}

// Benchmark subscription with middleware
func BenchmarkSubscription_WithMiddleware(b *testing.B) {
	type Event struct {
		ID string `json:"id"`
	}

	testMiddleware := func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			return next(p)
		}
	}

	sub := NewSubscription[Event]("events").
		WithMiddleware(testMiddleware).
		WithMiddleware(testMiddleware).
		WithMiddleware(testMiddleware).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 1)
			ch <- &Event{ID: "1"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Subscribe(graphql.ResolveParams{
			Context: context.Background(),
		})
	}
}

// Benchmark UnmarshalSubscriptionMessage
func BenchmarkUnmarshalSubscriptionMessage(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		UserID  string `json:"userID"`
	}

	data, _ := json.Marshal(Event{
		ID:      "123",
		Message: "test message",
		UserID:  "user456",
	})

	msg := &Message{
		Topic: "test",
		Data:  data,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = UnmarshalSubscriptionMessage[Event](msg)
	}
}

// Benchmark subscription with PubSub
func BenchmarkSubscription_WithPubSub(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	pubsub := NewInMemoryPubSub()
	defer pubsub.Close()

	sub := NewSubscription[Event]("events").
		WithArgs(graphql.FieldConfigArgument{
			"channelID": &graphql.ArgumentConfig{Type: graphql.String},
		}).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			channelID, _ := GetArgString(p, "channelID")
			events := make(chan *Event, 10)

			subscription := pubsub.Subscribe(ctx, "events:"+channelID)

			go func() {
				defer close(events)
				for msg := range subscription {
					var event Event
					if err := json.Unmarshal(msg.Data, &event); err == nil {
						events <- &event
					}
				}
			}()

			return events, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Subscribe(graphql.ResolveParams{
			Args:    map[string]interface{}{"channelID": "test"},
			Context: context.Background(),
		})
	}
}

// Benchmark event throughput
func BenchmarkSubscription_EventThroughput(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub := NewSubscription[Event]("events").
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event, 1000)
				go func() {
					defer close(ch)
					for j := 0; j < 1000; j++ {
						ch <- &Event{ID: string(rune(j)), Message: "test"}
					}
				}()
				return ch, nil
			}).
			BuildSubscription()

		field := sub.Serve()
		result, _ := field.Subscribe(graphql.ResolveParams{
			Context: context.Background(),
		})

		if ch, ok := result.(<-chan interface{}); ok {
			// Drain channel
			for range ch {
			}
		}
	}
}

// Benchmark Resolve function
func BenchmarkSubscription_Resolve(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()
	event := Event{ID: "1", Message: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Resolve(graphql.ResolveParams{
			Source: event,
		})
	}
}

// Benchmark subscription with field resolver
func BenchmarkSubscription_WithFieldResolver(b *testing.B) {
	type Event struct {
		ID       string `json:"id"`
		AuthorID string `json:"authorID"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events").
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event, 1)
				ch <- &Event{ID: "1", AuthorID: "author1"}
				close(ch)
				return ch, nil
			}).
			WithFieldResolver("author", func(p graphql.ResolveParams) (interface{}, error) {
				event := p.Source.(Event)
				return map[string]interface{}{
					"id":   event.AuthorID,
					"name": "Author Name",
				}, nil
			}).
			BuildSubscription()
	}
}

// Benchmark subscription with field middleware
func BenchmarkSubscription_WithFieldMiddleware(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}

	testMiddleware := func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			return next(p)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events").
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event, 1)
				ch <- &Event{ID: "1", Content: "test"}
				close(ch)
				return ch, nil
			}).
			WithFieldMiddleware("content", testMiddleware).
			BuildSubscription()
	}
}

// Benchmark schema build with subscriptions
func BenchmarkSchemaBuilder_WithSubscriptions(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	params := SchemaBuilderParams{
		QueryFields:        []QueryField{getDefaultHelloQuery()},
		MutationFields:     []MutationField{getDefaultEchoMutation()},
		SubscriptionFields: []SubscriptionField{sub},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewSchemaBuilder(params).Build()
	}
}

// Benchmark multiple subscriptions
func BenchmarkSchemaBuilder_MultipleSubscriptions(b *testing.B) {
	type Event1 struct {
		ID string `json:"id"`
	}

	type Event2 struct {
		Name string `json:"name"`
	}

	type Event3 struct {
		Title string `json:"title"`
	}

	sub1 := NewSubscription[Event1]("events1").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event1, error) {
			ch := make(chan *Event1)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	sub2 := NewSubscription[Event2]("events2").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event2, error) {
			ch := make(chan *Event2)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	sub3 := NewSubscription[Event3]("events3").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event3, error) {
			ch := make(chan *Event3)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	params := SchemaBuilderParams{
		QueryFields:        []QueryField{getDefaultHelloQuery()},
		SubscriptionFields: []SubscriptionField{sub1, sub2, sub3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewSchemaBuilder(params).Build()
	}
}

// Benchmark subscription with arguments
func BenchmarkSubscription_WithArgs(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events").
			WithArgs(graphql.FieldConfigArgument{
				"channelID": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.String),
					Description: "Channel ID",
				},
				"filter": &graphql.ArgumentConfig{
					Type:        graphql.String,
					Description: "Filter pattern",
				},
			}).
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event)
				close(ch)
				return ch, nil
			}).
			BuildSubscription()
	}
}

// Benchmark concurrent subscription execution
func BenchmarkSubscription_Concurrent(b *testing.B) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 10)
			go func() {
				defer close(ch)
				for i := 0; i < 10; i++ {
					ch <- &Event{ID: string(rune(i)), Message: "test"}
				}
			}()
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, _ := field.Subscribe(graphql.ResolveParams{
				Context: context.Background(),
			})
			if ch, ok := result.(<-chan interface{}); ok {
				// Drain channel
				for range ch {
				}
			}
		}
	})
}

// Benchmark type generation for subscriptions
func BenchmarkSubscription_TypeGeneration(b *testing.B) {
	type ComplexEvent struct {
		ID        string                 `json:"id"`
		Title     string                 `json:"title"`
		Content   string                 `json:"content"`
		CreatedAt time.Time              `json:"createdAt"`
		UpdatedAt time.Time              `json:"updatedAt"`
		AuthorID  string                 `json:"authorID"`
		Tags      []string               `json:"tags"`
		Metadata  map[string]interface{} `json:"metadata"`
		IsActive  bool                   `json:"isActive"`
		ViewCount int                    `json:"viewCount"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub := NewSubscription[ComplexEvent]("complexEvent").
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *ComplexEvent, error) {
				ch := make(chan *ComplexEvent)
				close(ch)
				return ch, nil
			})

		_ = sub.BuildSubscription()
	}
}

// Benchmark subscription with all features
func BenchmarkSubscription_AllFeatures(b *testing.B) {
	type Event struct {
		ID       string `json:"id"`
		Message  string `json:"message"`
		UserID   string `json:"userID"`
		AuthorID string `json:"authorID"`
	}

	testMiddleware := func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			return next(p)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSubscription[Event]("events").
			WithDescription("Complete event subscription").
			WithArgs(graphql.FieldConfigArgument{
				"channelID": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.String),
				},
				"userID": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			}).
			WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
				ch := make(chan *Event, 10)
				go func() {
					defer close(ch)
					for j := 0; j < 10; j++ {
						ch <- &Event{
							ID:       string(rune(j)),
							Message:  "test",
							UserID:   "user1",
							AuthorID: "author1",
						}
					}
				}()
				return ch, nil
			}).
			WithFilter(func(ctx context.Context, data *Event, p ResolveParams) bool {
				userID, _ := GetArgString(p, "userID")
				if userID != "" {
					return data.UserID == userID
				}
				return true
			}).
			WithMiddleware(testMiddleware).
			WithFieldResolver("author", func(p graphql.ResolveParams) (interface{}, error) {
				return map[string]interface{}{"id": "1", "name": "Author"}, nil
			}).
			WithFieldMiddleware("message", testMiddleware).
			BuildSubscription()
	}
}