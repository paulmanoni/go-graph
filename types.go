package graph

import (
	"context"
	"net/http"

	"github.com/graphql-go/graphql"
)

// GraphContext configures a GraphQL handler with schema, authentication, and security settings.
//
// Schema Configuration (choose one):
//   - Schema: Use a pre-built graphql.Schema
//   - SchemaParams: Use the builder pattern with QueryFields and MutationFields
//   - Neither: A default "hello world" schema will be created
//
// Security Modes:
//   - DEBUG mode (DEBUG: true): Disables all validation and sanitization for development
//   - Production mode (DEBUG: false): Enables validation and sanitization based on configuration flags
//
// Authentication:
//   - TokenExtractorFn: Extract tokens from requests (defaults to Bearer token extraction)
//   - UserDetailsFn: Fetch user details from the extracted token
//   - RootObjectFn: Custom root object setup for advanced use cases
//
// Example Development Setup:
//
//	ctx := &graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields: []graph.QueryField{getUserQuery()},
//	    },
//	    DEBUG:      true,
//	    Playground: true,
//	}
//
// Example Production Setup:
//
//	ctx := &graph.GraphContext{
//	    SchemaParams:       &graph.SchemaBuilderParams{...},
//	    DEBUG:              false,
//	    EnableValidation:   true,  // Max depth: 10, Max aliases: 4, Max complexity: 200
//	    EnableSanitization: true,  // Remove field suggestions from errors
//	    Playground:         false,
//	    UserDetailsFn: func(ctx context.Context, token string) (context.Context, interface{}, error) {
//	        user, err := validateJWT(token)
//	        if err != nil {
//	            return ctx, nil, err
//	        }
//	        ctx = context.WithValue(ctx, "userID", user.ID)
//	        return ctx, user, nil
//	    },
//	}
type GraphContext struct {
	// Schema: Provide either Schema OR SchemaParams (not both)
	// If both are nil, a default "hello world" schema will be created
	Schema *graphql.Schema

	// SchemaParams: Alternative to Schema - will be built automatically
	// If nil and Schema is also nil, defaults to hello world query/mutation
	SchemaParams *SchemaBuilderParams

	// PubSub: PubSub system for subscriptions (optional, only needed for subscriptions)
	// Use NewInMemoryPubSub() for development or RedisPubSub for production
	PubSub PubSub

	// EnableSubscriptions: Enable WebSocket support for GraphQL subscriptions
	// Default: false (subscriptions disabled)
	// Requires PubSub to be configured
	EnableSubscriptions bool

	// WebSocketPath: Path for WebSocket endpoint (default: same as HTTP endpoint)
	// If not set, WebSocket connections will be handled on the same path as HTTP
	WebSocketPath string

	// WebSocketCheckOrigin: Custom function to check WebSocket upgrade origin
	// If not provided, all origins are allowed (only use in development!)
	WebSocketCheckOrigin func(r *http.Request) bool

	// Pretty: Pretty-print JSON responses
	Pretty bool

	// GraphiQL: Enable GraphiQL interface (deprecated, use Playground instead)
	GraphiQL bool

	// Playground: Enable GraphQL Playground interface
	Playground bool

	// DEBUG mode skips validation and sanitization for easier development
	// Default: false (validation enabled)
	DEBUG bool

	// RootObjectFn: Custom function to set up root object for each request
	// Called before token extraction and user details fetching
	RootObjectFn func(ctx context.Context, r *http.Request) map[string]interface{}

	// TokenExtractorFn: Custom token extraction from request
	// If not provided, default Bearer token extraction will be used
	TokenExtractorFn func(*http.Request) string

	// UserDetailsFn: Custom user details fetching based on token
	// If not provided, user details will not be added to rootValue
	// The details are accessible in resolvers via GetRootInfo(p, "details", &user)
	//
	// The function receives the request context and token, and returns:
	//   - ctx: Updated context with custom values (accessible via p.Context.Value() in resolvers)
	//   - details: User details (accessible via GetRootInfo(p, "details", &user) in resolvers)
	//   - error: Any error during user details fetching
	//
	// Example:
	//
	//	UserDetailsFn: func(ctx context.Context, token string) (context.Context, interface{}, error) {
	//	    user, err := validateJWT(token)
	//	    if err != nil {
	//	        return ctx, nil, err
	//	    }
	//	    // Add user ID to context for access in resolvers via p.Context.Value("userID")
	//	    ctx = context.WithValue(ctx, "userID", user.ID)
	//	    return ctx, user, nil
	//	}
	UserDetailsFn func(ctx context.Context, token string) (context.Context, interface{}, error)

	// EnableValidation: Enable query validation (depth, complexity, introspection checks)
	// Default: false (validation disabled)
	// When enabled: Max depth=10, Max aliases=4, Max complexity=200, Introspection blocked
	// DEPRECATED: Use ValidationRules for more control
	EnableValidation bool

	// ValidationRules: Custom validation rules (takes precedence over EnableValidation)
	// Set to nil or empty slice to disable validation
	// Example:
	//   ValidationRules: []ValidationRule{
	//       NewMaxDepthRule(10),
	//       NewRequireAuthRule("mutation"),
	//       NewRoleRules(map[string][]string{
	//           "deleteUser": {"admin"},
	//       }),
	//   }
	ValidationRules []ValidationRule

	// ValidationOptions: Configure validation behavior (optional)
	// Default: StopOnFirstError=false, SkipInDebug=true
	ValidationOptions *ValidationOptions

	// EnableSanitization: Enable response sanitization (removes field suggestions from errors)
	// Default: false (sanitization disabled)
	// Prevents information disclosure by removing "Did you mean X?" suggestions
	EnableSanitization bool
}

type ResolveParams graphql.ResolveParams

type FieldResolveFn func(p ResolveParams) (interface{}, error)
