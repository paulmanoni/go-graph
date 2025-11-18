package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
)

// Test Subscription Field Interface
func TestSubscriptionField_Interface(t *testing.T) {
	type TestEvent struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	sub := NewSubscription[TestEvent]("testEvent").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *TestEvent, error) {
			ch := make(chan *TestEvent, 1)
			ch <- &TestEvent{ID: "1", Message: "test"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	if sub.Name() != "testEvent" {
		t.Errorf("Expected name 'testEvent', got %s", sub.Name())
	}

	field := sub.Serve()
	if field == nil {
		t.Fatal("Expected field to be non-nil")
	}

	if field.Subscribe == nil {
		t.Error("Expected Subscribe function to be set")
	}

	if field.Resolve == nil {
		t.Error("Expected Resolve function to be set")
	}
}

// Test NewSubscription Creation
func TestNewSubscription_Basic(t *testing.T) {
	type MessageEvent struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}

	resolver := NewSubscription[MessageEvent]("messageAdded")

	if resolver.name != "messageAdded" {
		t.Errorf("Expected name 'messageAdded', got %s", resolver.name)
	}

	if resolver.args == nil {
		t.Error("Args should be initialized")
	}

	if resolver.fieldMiddleware == nil {
		t.Error("Field middleware map should be initialized")
	}

	if resolver.fieldResolvers == nil {
		t.Error("Field resolvers map should be initialized")
	}
}

// Test WithDescription
func TestSubscription_WithDescription(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	desc := "Subscribe to events"
	sub := NewSubscription[Event]("events").
		WithDescription(desc).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()
	if field.Description != desc {
		t.Errorf("Expected description '%s', got '%s'", desc, field.Description)
	}
}

// Test WithArgs
func TestSubscription_WithArgs(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	args := graphql.FieldConfigArgument{
		"channelID": &graphql.ArgumentConfig{
			Type:        graphql.NewNonNull(graphql.String),
			Description: "Channel ID to subscribe to",
		},
	}

	sub := NewSubscription[Event]("events").
		WithArgs(args).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()
	if field.Args == nil {
		t.Fatal("Expected args to be set")
	}

	if _, hasChannelID := field.Args["channelID"]; !hasChannelID {
		t.Error("Expected 'channelID' argument")
	}
}

// Test WithResolver
func TestSubscription_WithResolver(t *testing.T) {
	type Event struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	events := []*Event{
		{ID: "1", Message: "first"},
		{ID: "2", Message: "second"},
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, len(events))
			for _, event := range events {
				ch <- event
			}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	// Execute subscribe function
	result, err := field.Subscribe(graphql.ResolveParams{
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	outputCh, ok := result.(<-chan interface{})
	if !ok {
		t.Fatalf("Expected channel, got %T", result)
	}

	// Collect all events
	var received []Event
	for event := range outputCh {
		received = append(received, event.(Event))
	}

	if len(received) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(received))
	}
}

// Test WithFilter
func TestSubscription_WithFilter(t *testing.T) {
	type Event struct {
		ID       string `json:"id"`
		UserID   string `json:"userID"`
		Message  string `json:"message"`
	}

	events := []*Event{
		{ID: "1", UserID: "user1", Message: "msg1"},
		{ID: "2", UserID: "user2", Message: "msg2"},
		{ID: "3", UserID: "user1", Message: "msg3"},
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

	// Execute with filter for user1
	result, err := field.Subscribe(graphql.ResolveParams{
		Args:    map[string]interface{}{"userID": "user1"},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	outputCh, ok := result.(<-chan interface{})
	if !ok {
		t.Fatalf("Expected channel, got %T", result)
	}

	// Collect filtered events
	var received []Event
	for event := range outputCh {
		received = append(received, event.(Event))
	}

	// Should only receive events for user1
	if len(received) != 2 {
		t.Errorf("Expected 2 events for user1, got %d", len(received))
	}

	for _, event := range received {
		if event.UserID != "user1" {
			t.Errorf("Expected UserID 'user1', got '%s'", event.UserID)
		}
	}
}

// Test WithMiddleware
func TestSubscription_WithMiddleware(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	middlewareCalled := false

	testMiddleware := func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			middlewareCalled = true
			return next(p)
		}
	}

	sub := NewSubscription[Event]("events").
		WithMiddleware(testMiddleware).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 1)
			ch <- &Event{ID: "1"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	// Execute subscribe
	_, err := field.Subscribe(graphql.ResolveParams{
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	if !middlewareCalled {
		t.Error("Middleware was not called")
	}
}

// Test WithFieldResolver
func TestSubscription_WithFieldResolver(t *testing.T) {
	type Event struct {
		ID       string `json:"id"`
		AuthorID string `json:"authorID"`
	}

	sub := NewSubscription[Event]("events").
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

	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	field := sub.Serve()
	if field == nil {
		t.Fatal("Expected field to be non-nil")
	}

	// Verify field resolver was registered
	sf := sub.(*subscriptionField)
	// We can't directly access the resolver's field resolvers, but we can verify the subscription built successfully
	if sf.name != "events" {
		t.Errorf("Expected name 'events', got %s", sf.name)
	}
}

// Test BuildSubscription type generation
func TestSubscription_BuildSubscription_TypeGeneration(t *testing.T) {
	type ComplexEvent struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"createdAt"`
		AuthorID  string    `json:"authorID"`
	}

	sub := NewSubscription[ComplexEvent]("complexEvent").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *ComplexEvent, error) {
			ch := make(chan *ComplexEvent)
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	// Verify type was generated
	if field.Type == nil {
		t.Fatal("Expected type to be generated")
	}

	obj, ok := field.Type.(*graphql.Object)
	if !ok {
		t.Fatalf("Expected Object type, got %T", field.Type)
	}

	// Verify fields exist
	fields := obj.Fields()
	expectedFields := []string{"id", "title", "content", "createdAt", "authorID"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field '%s' to exist", fieldName)
		}
	}
}

// Test error handling when resolver not configured
func TestSubscription_NoResolver_Error(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	sub := NewSubscription[Event]("events").
		BuildSubscription()

	field := sub.Serve()

	// Execute subscribe without resolver
	_, err := field.Subscribe(graphql.ResolveParams{
		Context: context.Background(),
	})

	if err == nil {
		t.Error("Expected error when resolver not configured")
	}

	expectedMsg := "subscription resolver not configured for events"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

// Test context cancellation
func TestSubscription_ContextCancellation(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	ctx, cancel := context.WithCancel(context.Background())

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 10)

			go func() {
				defer close(ch)
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()

				for i := 0; i < 100; i++ {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						ch <- &Event{ID: fmt.Sprintf("%d", i)}
					}
				}
			}()

			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	result, err := field.Subscribe(graphql.ResolveParams{
		Context: ctx,
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	outputCh, ok := result.(<-chan interface{})
	if !ok {
		t.Fatalf("Expected channel, got %T", result)
	}

	// Receive a few events
	count := 0
	for event := range outputCh {
		count++
		if count == 3 {
			cancel() // Cancel context
		}
		if count > 10 {
			t.Error("Expected subscription to stop after context cancellation")
			break
		}
		_ = event
	}

	if count > 10 {
		t.Errorf("Expected fewer than 10 events, got %d", count)
	}
}

// Test UnmarshalSubscriptionMessage helper
func TestUnmarshalSubscriptionMessage(t *testing.T) {
	type TestEvent struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	}

	tests := []struct {
		name      string
		message   *Message
		want      *TestEvent
		wantError bool
	}{
		{
			name: "valid message",
			message: &Message{
				Topic: "test",
				Data:  []byte(`{"id":"1","message":"hello"}`),
			},
			want:      &TestEvent{ID: "1", Message: "hello"},
			wantError: false,
		},
		{
			name: "invalid JSON",
			message: &Message{
				Topic: "test",
				Data:  []byte(`{invalid json}`),
			},
			want:      nil,
			wantError: true,
		},
		{
			name: "empty data",
			message: &Message{
				Topic: "test",
				Data:  []byte(`{}`),
			},
			want:      &TestEvent{},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalSubscriptionMessage[TestEvent](tt.message)

			if (err != nil) != tt.wantError {
				t.Errorf("UnmarshalSubscriptionMessage() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && (got.ID != tt.want.ID || got.Message != tt.want.Message) {
				t.Errorf("UnmarshalSubscriptionMessage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Test subscription with PubSub integration
func TestSubscription_WithPubSub(t *testing.T) {
	type MessageEvent struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}

	pubsub := NewInMemoryPubSub()
	defer pubsub.Close()

	sub := NewSubscription[MessageEvent]("messageAdded").
		WithArgs(graphql.FieldConfigArgument{
			"channelID": &graphql.ArgumentConfig{
				Type: graphql.NewNonNull(graphql.String),
			},
		}).
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *MessageEvent, error) {
			channelID, _ := GetArgString(p, "channelID")
			events := make(chan *MessageEvent, 10)

			subscription := pubsub.Subscribe(ctx, "messages:"+channelID)

			go func() {
				defer close(events)
				for msg := range subscription {
					var event MessageEvent
					if err := json.Unmarshal(msg.Data, &event); err == nil {
						events <- &event
					}
				}
			}()

			return events, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	// Subscribe to channel
	ctx := context.Background()
	result, err := field.Subscribe(graphql.ResolveParams{
		Args:    map[string]interface{}{"channelID": "general"},
		Context: ctx,
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	outputCh, ok := result.(<-chan interface{})
	if !ok {
		t.Fatalf("Expected channel, got %T", result)
	}

	// Publish some events
	go func() {
		time.Sleep(10 * time.Millisecond)
		pubsub.Publish(ctx, "messages:general", &MessageEvent{
			ID:      "1",
			Content: "Hello",
		})
		time.Sleep(10 * time.Millisecond)
		pubsub.Publish(ctx, "messages:general", &MessageEvent{
			ID:      "2",
			Content: "World",
		})
		time.Sleep(10 * time.Millisecond)
		pubsub.Close() // Close to end subscription
	}()

	// Collect events
	var received []MessageEvent
	timeout := time.After(1 * time.Second)
	for {
		select {
		case event, ok := <-outputCh:
			if !ok {
				goto done
			}
			received = append(received, event.(MessageEvent))
		case <-timeout:
			t.Fatal("Timeout waiting for events")
		}
	}

done:
	if len(received) != 2 {
		t.Errorf("Expected 2 events, got %d", len(received))
	}

	if len(received) >= 1 && received[0].Content != "Hello" {
		t.Errorf("Expected first event content 'Hello', got '%s'", received[0].Content)
	}

	if len(received) >= 2 && received[1].Content != "World" {
		t.Errorf("Expected second event content 'World', got '%s'", received[1].Content)
	}
}

// Test subscription with schema integration
func TestSubscription_SchemaIntegration(t *testing.T) {
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

	// Build schema with subscription
	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields:        []QueryField{getDefaultHelloQuery()},
		SubscriptionFields: []SubscriptionField{sub},
	}).Build()

	if err != nil {
		t.Fatalf("Schema build error: %v", err)
	}

	if schema.SubscriptionType() == nil {
		t.Fatal("Expected subscription type to be set")
	}

	fields := schema.SubscriptionType().Fields()
	if _, hasEvents := fields["events"]; !hasEvents {
		t.Error("Expected 'events' subscription field")
	}
}

// Test multiple subscriptions
func TestSubscription_MultipleSubscriptions(t *testing.T) {
	type Event1 struct {
		ID string `json:"id"`
	}

	type Event2 struct {
		Name string `json:"name"`
	}

	sub1 := NewSubscription[Event1]("events1").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event1, error) {
			ch := make(chan *Event1, 1)
			ch <- &Event1{ID: "1"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	sub2 := NewSubscription[Event2]("events2").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event2, error) {
			ch := make(chan *Event2, 1)
			ch <- &Event2{Name: "test"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields:        []QueryField{getDefaultHelloQuery()},
		SubscriptionFields: []SubscriptionField{sub1, sub2},
	}).Build()

	if err != nil {
		t.Fatalf("Schema build error: %v", err)
	}

	fields := schema.SubscriptionType().Fields()
	if len(fields) < 2 {
		t.Errorf("Expected at least 2 subscription fields, got %d", len(fields))
	}

	if _, hasEvents1 := fields["events1"]; !hasEvents1 {
		t.Error("Expected 'events1' subscription field")
	}

	if _, hasEvents2 := fields["events2"]; !hasEvents2 {
		t.Error("Expected 'events2' subscription field")
	}
}

// Test nil event handling
func TestSubscription_NilEventHandling(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}

	sub := NewSubscription[Event]("events").
		WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *Event, error) {
			ch := make(chan *Event, 3)
			ch <- &Event{ID: "1"}
			ch <- nil // Send nil event
			ch <- &Event{ID: "2"}
			close(ch)
			return ch, nil
		}).
		BuildSubscription()

	field := sub.Serve()

	result, err := field.Subscribe(graphql.ResolveParams{
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Subscribe error: %v", err)
	}

	outputCh, ok := result.(<-chan interface{})
	if !ok {
		t.Fatalf("Expected channel, got %T", result)
	}

	// Collect events
	var received []Event
	for event := range outputCh {
		received = append(received, event.(Event))
	}

	// Nil events should be filtered out
	if len(received) != 2 {
		t.Errorf("Expected 2 non-nil events, got %d", len(received))
	}
}

// Test subscription Resolve function
func TestSubscription_ResolveFunction(t *testing.T) {
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

	// Test Resolve function
	event := Event{ID: "1", Message: "test"}
	result, err := field.Resolve(graphql.ResolveParams{
		Source: event,
	})

	if err != nil {
		t.Fatalf("Resolve error: %v", err)
	}

	resultEvent, ok := result.(Event)
	if !ok {
		t.Fatalf("Expected Event, got %T", result)
	}

	if resultEvent.ID != event.ID || resultEvent.Message != event.Message {
		t.Errorf("Expected %+v, got %+v", event, resultEvent)
	}
}