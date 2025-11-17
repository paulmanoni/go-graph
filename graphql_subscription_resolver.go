package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/graphql-go/graphql"
)

// SubscriptionField represents a GraphQL subscription field that can be added to a schema.
// It follows the same interface pattern as QueryField and MutationField.
//
// Create subscription fields using NewSubscription:
//
//	sub := NewSubscription[MessageEvent]("messageAdded").
//	    WithArgs(...).
//	    WithResolver(...).
//	    BuildSubscription()
type SubscriptionField interface {
	// Serve returns the GraphQL field configuration
	Serve() *graphql.Field

	// Name returns the subscription field name
	Name() string
}

// subscriptionField is the concrete implementation of SubscriptionField
type subscriptionField struct {
	name  string
	field *graphql.Field
}

func (s *subscriptionField) Serve() *graphql.Field {
	return s.field
}

func (s *subscriptionField) Name() string {
	return s.name
}

// SubscriptionResolver builds type-safe subscription fields with extensive customization capabilities.
// It provides a fluent API similar to UnifiedResolver for building subscriptions.
//
// Type Parameters:
//   - T: The Go struct type that will be sent to subscribers
//
// The resolver function should return a channel that emits events of type T.
// When the channel is closed or the context is canceled, the subscription ends.
//
// Basic Usage:
//
//	type MessageEvent struct {
//	    ID        string    `json:"id"`
//	    Content   string    `json:"content"`
//	    Timestamp time.Time `json:"timestamp"`
//	}
//
//	sub := NewSubscription[MessageEvent]("messageAdded").
//	    WithDescription("Subscribe to new messages").
//	    WithArgs(graphql.FieldConfigArgument{
//	        "channelID": &graphql.ArgumentConfig{
//	            Type: graphql.NewNonNull(graphql.String),
//	        },
//	    }).
//	    WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *MessageEvent, error) {
//	        channelID, _ := GetArgString(p, "channelID")
//	        events := make(chan *MessageEvent)
//	        subscription := pubsub.Subscribe(ctx, "messages:"+channelID)
//
//	        go func() {
//	            defer close(events)
//	            for msg := range subscription {
//	                var event MessageEvent
//	                json.Unmarshal(msg.Data, &event)
//	                events <- &event
//	            }
//	        }()
//
//	        return events, nil
//	    }).
//	    WithFilter(func(ctx context.Context, data *MessageEvent, p ResolveParams) bool {
//	        // Optional: filter events before sending to client
//	        return true
//	    }).
//	    WithMiddleware(AuthMiddleware("user")).
//	    BuildSubscription()
//
// Advanced Features:
//
//	// Middleware support
//	sub := NewSubscription[Event]("events").
//	    WithMiddleware(LoggingMiddleware).
//	    WithMiddleware(AuthMiddleware("admin")).
//	    // ... rest of configuration
//
//	// Field-level customization (like UnifiedResolver)
//	sub := NewSubscription[ComplexEvent]("complexEvent").
//	    WithFieldResolver("computedField", func(p ResolveParams) (interface{}, error) {
//	        event := p.Source.(ComplexEvent)
//	        return computeValue(event), nil
//	    }).
//	    // ... rest of configuration
type SubscriptionResolver[T any] struct {
	name            string
	description     string
	args            graphql.FieldConfigArgument
	resolver        SubscriptionResolveFn[T]
	filterFn        SubscriptionFilterFn[T]
	middleware      []FieldMiddleware
	fieldMiddleware map[string][]FieldMiddleware
	fieldResolvers  map[string]graphql.FieldResolveFn
	generatedType   *graphql.Object
	objectName      string
}

// SubscriptionResolveFn is the resolver function for subscriptions.
// It returns a channel that emits events of type T.
//
// The resolver should:
//   - Create a buffered channel to prevent blocking
//   - Start a goroutine to handle event publishing
//   - Close the channel when done or context is canceled
//   - Handle errors by returning nil channel and an error
//
// Example:
//
//	func(ctx context.Context, p ResolveParams) (<-chan *MessageEvent, error) {
//	    channelID, _ := GetArgString(p, "channelID")
//	    events := make(chan *MessageEvent, 10)
//
//	    subscription := pubsub.Subscribe(ctx, "messages:"+channelID)
//
//	    go func() {
//	        defer close(events)
//	        for msg := range subscription {
//	            var event MessageEvent
//	            if err := json.Unmarshal(msg.Data, &event); err == nil {
//	                events <- &event
//	            }
//	        }
//	    }()
//
//	    return events, nil
//	}
type SubscriptionResolveFn[T any] func(ctx context.Context, p ResolveParams) (<-chan *T, error)

// SubscriptionFilterFn filters events before sending them to clients.
// Return true to send the event, false to skip it.
//
// This is useful for:
//   - User-specific filtering based on permissions
//   - Content-based filtering based on subscription arguments
//   - Rate limiting or throttling
//
// Example:
//
//	func(ctx context.Context, data *MessageEvent, p ResolveParams) bool {
//	    userID, _ := GetRootString(p, "userID")
//	    return canUserViewMessage(userID, data.ID)
//	}
type SubscriptionFilterFn[T any] func(ctx context.Context, data *T, p ResolveParams) bool

// NewSubscription creates a new subscription resolver with the specified name.
// The type parameter T determines the event type that will be sent to subscribers.
//
// Example:
//
//	type UserStatusEvent struct {
//	    UserID string `json:"userID"`
//	    Status string `json:"status"`
//	    Timestamp time.Time `json:"timestamp"`
//	}
//
//	sub := NewSubscription[UserStatusEvent]("userStatusChanged").
//	    WithArgs(graphql.FieldConfigArgument{
//	        "userID": &graphql.ArgumentConfig{Type: graphql.String},
//	    }).
//	    WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *UserStatusEvent, error) {
//	        // Implementation
//	    }).
//	    BuildSubscription()
func NewSubscription[T any](name string) *SubscriptionResolver[T] {
	return &SubscriptionResolver[T]{
		name:            name,
		args:            make(graphql.FieldConfigArgument),
		fieldMiddleware: make(map[string][]FieldMiddleware),
		fieldResolvers:  make(map[string]graphql.FieldResolveFn),
		objectName:      fmt.Sprintf("%s%s", strings.ToUpper(name[:1]), name[1:]),
	}
}

// WithDescription adds a description to the subscription field.
func (s *SubscriptionResolver[T]) WithDescription(desc string) *SubscriptionResolver[T] {
	s.description = desc
	return s
}

// WithArgs sets custom arguments for the subscription.
//
// Example:
//
//	WithArgs(graphql.FieldConfigArgument{
//	    "channelID": &graphql.ArgumentConfig{
//	        Type:        graphql.NewNonNull(graphql.String),
//	        Description: "Channel to subscribe to",
//	    },
//	    "filter": &graphql.ArgumentConfig{
//	        Type:        graphql.String,
//	        Description: "Optional filter pattern",
//	    },
//	})
func (s *SubscriptionResolver[T]) WithArgs(args graphql.FieldConfigArgument) *SubscriptionResolver[T] {
	s.args = args
	return s
}

// WithResolver sets the subscription resolver function.
// The resolver should return a channel that emits events of type T.
//
// Example:
//
//	WithResolver(func(ctx context.Context, p ResolveParams) (<-chan *MessageEvent, error) {
//	    channelID, _ := GetArgString(p, "channelID")
//
//	    events := make(chan *MessageEvent, 10)
//	    subscription := pubsub.Subscribe(ctx, "messages:"+channelID)
//
//	    go func() {
//	        defer close(events)
//	        for msg := range subscription {
//	            var event MessageEvent
//	            if err := json.Unmarshal(msg.Data, &event); err == nil {
//	                events <- &event
//	            }
//	        }
//	    }()
//
//	    return events, nil
//	})
func (s *SubscriptionResolver[T]) WithResolver(fn SubscriptionResolveFn[T]) *SubscriptionResolver[T] {
	s.resolver = fn
	return s
}

// WithFilter adds a filter function to filter events before sending to clients.
// Only events that pass the filter (return true) will be sent.
//
// Example:
//
//	WithFilter(func(ctx context.Context, data *MessageEvent, p ResolveParams) bool {
//	    userID, _ := GetRootString(p, "userID")
//	    return data.AuthorID != userID // Don't send user's own messages
//	})
func (s *SubscriptionResolver[T]) WithFilter(fn SubscriptionFilterFn[T]) *SubscriptionResolver[T] {
	s.filterFn = fn
	return s
}

// WithMiddleware adds middleware to the subscription resolver.
// Middleware is executed in the order it's added.
//
// Example:
//
//	WithMiddleware(LoggingMiddleware).
//	WithMiddleware(AuthMiddleware("user"))
func (s *SubscriptionResolver[T]) WithMiddleware(middleware FieldMiddleware) *SubscriptionResolver[T] {
	s.middleware = append(s.middleware, middleware)
	return s
}

// WithFieldResolver overrides the resolver for a specific field in the event type.
// This allows customizing how specific fields are resolved.
//
// Example:
//
//	WithFieldResolver("author", func(p ResolveParams) (interface{}, error) {
//	    event := p.Source.(MessageEvent)
//	    return userService.GetByID(event.AuthorID), nil
//	})
func (s *SubscriptionResolver[T]) WithFieldResolver(fieldName string, resolver graphql.FieldResolveFn) *SubscriptionResolver[T] {
	s.fieldResolvers[fieldName] = resolver
	return s
}

// WithFieldMiddleware adds middleware to a specific field in the event type.
//
// Example:
//
//	WithFieldMiddleware("sensitiveData", AuthMiddleware("admin"))
func (s *SubscriptionResolver[T]) WithFieldMiddleware(fieldName string, middleware FieldMiddleware) *SubscriptionResolver[T] {
	s.fieldMiddleware[fieldName] = append(s.fieldMiddleware[fieldName], middleware)
	return s
}

// BuildSubscription builds and returns a SubscriptionField that can be added to the schema.
//
// This method:
//   - Auto-generates the GraphQL type from the Go struct T
//   - Applies field-level customizations and middleware
//   - Creates the subscription and resolve functions
//   - Registers the type in the global type registry
//
// Example:
//
//	sub := NewSubscription[MessageEvent]("messageAdded").
//	    WithArgs(...).
//	    WithResolver(...).
//	    BuildSubscription()
//
//	// Add to schema
//	schema := graph.NewSchemaBuilder(graph.SchemaBuilderParams{
//	    SubscriptionFields: []graph.SubscriptionField{sub},
//	})
func (s *SubscriptionResolver[T]) BuildSubscription() SubscriptionField {
	// Auto-generate GraphQL type from T
	s.generatedType = s.generateType()

	// Apply field-level customizations
	s.applyFieldCustomizations()

	// Build subscribe and resolve functions
	subscribeFn := s.buildSubscribeFn()
	resolveFn := s.buildResolveFn()

	return &subscriptionField{
		name: s.name,
		field: &graphql.Field{
			Type:        s.generatedType,
			Args:        s.args,
			Description: s.description,
			Subscribe:   subscribeFn,
			Resolve:     resolveFn,
		},
	}
}

// generateType creates a GraphQL type from the Go struct T
func (s *SubscriptionResolver[T]) generateType() *graphql.Object {
	var zero T
	t := reflect.TypeOf(zero)

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	typeName := s.objectName
	if t.Name() != "" {
		typeName = t.Name()
	}

	// Check if type already exists in registry
	return RegisterObjectType(typeName, func() *graphql.Object {
		return GenerateGraphQLObject[T](typeName)
	})
}

// applyFieldCustomizations applies field resolvers and middleware
func (s *SubscriptionResolver[T]) applyFieldCustomizations() {
	if s.generatedType == nil {
		return
	}

	fields := s.generatedType.Fields()

	// Apply field resolvers
	for fieldName, resolver := range s.fieldResolvers {
		if field, exists := fields[fieldName]; exists {
			field.Resolve = resolver
		}
	}

	// Apply field middleware
	for fieldName, middlewares := range s.fieldMiddleware {
		if field, exists := fields[fieldName]; exists {
			if field.Resolve != nil {
				originalResolve := field.Resolve

				// Wrap with middleware chain (apply in reverse order)
				wrapped := func(p ResolveParams) (interface{}, error) {
					return originalResolve(graphql.ResolveParams(p))
				}
				for i := len(middlewares) - 1; i >= 0; i-- {
					wrapped = middlewares[i](wrapped)
				}

				field.Resolve = func(p graphql.ResolveParams) (interface{}, error) {
					return wrapped(ResolveParams(p))
				}
			}
		}
	}
}

// buildSubscribeFn creates the subscribe function that returns the event channel
func (s *SubscriptionResolver[T]) buildSubscribeFn() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		if s.resolver == nil {
			return nil, fmt.Errorf("subscription resolver not configured for %s", s.name)
		}

		// Apply middleware to resolver if any
		wrappedResolver := s.wrapWithMiddleware()

		// Call the resolver to get the event channel
		ctx := p.Context
		eventChannel, err := wrappedResolver(ctx, ResolveParams(p))
		if err != nil {
			return nil, err
		}

		// Convert the typed channel to interface{} channel for graphql-go
		outputChannel := make(chan interface{}, 10)

		go func() {
			defer close(outputChannel)
			for event := range eventChannel {
				// Apply filter if defined
				if s.filterFn != nil && !s.filterFn(ctx, event, ResolveParams(p)) {
					continue
				}
				// Send the dereferenced event (graphql-go expects the actual struct, not pointer)
				if event != nil {
					outputChannel <- *event
				}
			}
		}()

		return outputChannel, nil
	}
}

// buildResolveFn creates the resolve function that processes each event
func (s *SubscriptionResolver[T]) buildResolveFn() graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		// The source is the event emitted from the channel
		// Return it as-is (it's already the dereferenced struct)
		return p.Source, nil
	}
}

// wrapWithMiddleware wraps the resolver with all middleware
func (s *SubscriptionResolver[T]) wrapWithMiddleware() SubscriptionResolveFn[T] {
	if len(s.middleware) == 0 {
		return s.resolver
	}

	// Convert SubscriptionResolveFn[T] to FieldResolveFn for middleware compatibility
	baseResolver := func(p ResolveParams) (interface{}, error) {
		return s.resolver(p.Context, p)
	}

	// Apply middleware chain (in reverse order so first added is outermost)
	wrapped := baseResolver
	for i := len(s.middleware) - 1; i >= 0; i-- {
		wrapped = s.middleware[i](wrapped)
	}

	// Convert back to SubscriptionResolveFn[T]
	return func(ctx context.Context, p ResolveParams) (<-chan *T, error) {
		result, err := wrapped(p)
		if err != nil {
			return nil, err
		}
		if channel, ok := result.(<-chan *T); ok {
			return channel, nil
		}
		return nil, fmt.Errorf("middleware returned invalid type, expected channel")
	}
}

// Helper function to unmarshal subscription messages
func UnmarshalSubscriptionMessage[T any](msg *Message) (*T, error) {
	var event T
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}