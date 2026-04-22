package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

// ExtractBearerToken extracts the Bearer token from the Authorization header.
// It performs case-insensitive matching for the "Bearer " prefix and trims whitespace.
//
// Returns an empty string if:
//   - The Authorization header is missing
//   - The header doesn't start with "Bearer " (case-insensitive)
//   - The token value is empty
//
// Example:
//
//	// Authorization: Bearer abc123xyz
//	token := graph.ExtractBearerToken(r) // Returns: "abc123xyz"
func ExtractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	// Check for Bearer prefix (case-insensitive)
	const bearerPrefix = "Bearer "
	if len(auth) > len(bearerPrefix) && strings.EqualFold(auth[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(auth[len(bearerPrefix):])
	}

	return ""
}

// extractToken extracts token using custom extractor or falls back to Bearer token extraction
func extractToken(r *http.Request, extractorFn func(*http.Request) string) string {
	if extractorFn != nil {
		return extractorFn(r)
	}
	return ExtractBearerToken(r)
}

// getDefaultHelloQuery creates a default hello world query
func getDefaultHelloQuery() QueryField {
	return NewResolver[string]("hello").
		WithResolver(func(p ResolveParams) (*string, error) {
			name := "Hello world"
			return &name, nil
		}).BuildQuery()
}

// EchoInput is the input for the default echo mutation.
type EchoInput struct {
	Message string `json:"message" graphql:"message,required" description:"Message to echo back"`
}

// getDefaultEchoMutation creates a default echo mutation.
func getDefaultEchoMutation() MutationField {
	return NewMutation[string, EchoInput]("echo").
		Action().
		WithResolver(func(ctx context.Context, in EchoInput) (*string, error) {
			return &in.Message, nil
		}).
		Build()
}

// userDetailsResult holds the result of calling UserDetailsFn
type userDetailsResult struct {
	ctx     context.Context
	details interface{}
	err     error
}

// callUserDetailsFn calls UserDetailsFn with the given context and token
// and returns the result. If UserDetailsFn is nil, returns the original context.
func callUserDetailsFn(graphCtx *GraphContext, ctx context.Context, token string) userDetailsResult {
	if graphCtx.UserDetailsFn == nil || token == "" {
		return userDetailsResult{ctx: ctx}
	}
	newCtx, details, err := graphCtx.UserDetailsFn(ctx, token)
	return userDetailsResult{ctx: newCtx, details: details, err: err}
}

// createWebSocketAuthFn creates an auth function for WebSocket connections
// that reuses the HTTP authentication logic from GraphContext
func createWebSocketAuthFn(graphCtx *GraphContext) func(r *http.Request) (interface{}, error) {
	if graphCtx.UserDetailsFn == nil {
		return nil
	}

	return func(r *http.Request) (interface{}, error) {
		// Use custom token extractor if provided, otherwise use default Bearer token extractor
		tokenExtractor := graphCtx.TokenExtractorFn
		if tokenExtractor == nil {
			tokenExtractor = ExtractBearerToken
		}

		token := tokenExtractor(r)
		if token == "" {
			return nil, nil // No token, no auth
		}

		_, details, err := graphCtx.UserDetailsFn(r.Context(), token)
		return details, err
	}
}

// buildSchemaFromContext builds a GraphQL schema from the GraphContext
// Priority: Schema > SchemaParams > Default hello world schema
func buildSchemaFromContext(graphCtx *GraphContext) (*graphql.Schema, error) {
	// If Schema is provided, use it
	if graphCtx.Schema != nil {
		return graphCtx.Schema, nil
	}

	// If SchemaParams is provided, build from it
	var params SchemaBuilderParams
	if graphCtx.SchemaParams != nil {
		params = *graphCtx.SchemaParams
	} else {
		// Use default hello world schema
		params = SchemaBuilderParams{
			QueryFields: []QueryField{
				getDefaultHelloQuery(),
			},
			MutationFields: []MutationField{
				getDefaultEchoMutation(),
			},
		}
	}

	// Build schema
	schema, err := NewSchemaBuilder(params).Build()
	if err != nil {
		return nil, err
	}

	return &schema, nil
}

var (
	didYouMeanRE = regexp.MustCompile(`Did you mean "[^"]+"\?`)
	whitespaceRE = regexp.MustCompile(`\s+`)
	errorsMarker = []byte(`"errors"`)
)

// bodyBufPool reuses *bytes.Buffer across requests for reading POST bodies.
// Buffers returned to the pool keep their underlying storage, so subsequent
// requests avoid growing a fresh buffer from zero.
var bodyBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// responseWrapperPool reuses *responseWriterWrapper (and its embedded
// *bytes.Buffer) across requests when sanitization is enabled.
var responseWrapperPool = sync.Pool{
	New: func() any {
		return &responseWriterWrapper{body: new(bytes.Buffer)}
	},
}

func acquireResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	rw := responseWrapperPool.Get().(*responseWriterWrapper)
	rw.ResponseWriter = w
	rw.body.Reset()
	rw.statusCode = http.StatusOK
	return rw
}

func releaseResponseWriterWrapper(rw *responseWriterWrapper) {
	rw.ResponseWriter = nil
	responseWrapperPool.Put(rw)
}

// gqlRequestBody is a minimal typed view of the GraphQL request envelope used
// to extract the query string without allocating a generic map[string]any.
type gqlRequestBody struct {
	Query string `json:"query"`
}

// gqlError / gqlErrorResponse mirror the GraphQL error response shape using
// typed structs to avoid nested map[string]any allocations on the error path.
type gqlError struct {
	Message string `json:"message"`
	Rule    string `json:"rule,omitempty"`
}

type gqlErrorResponse struct {
	Errors []gqlError `json:"errors"`
}

// responseWriterWrapper wraps http.ResponseWriter to capture and sanitize responses
type responseWriterWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// sanitizeAndWrite sanitizes the response body and writes it to the original writer
func (w *responseWriterWrapper) sanitizeAndWrite() {
	body := w.body.Bytes()

	// Skip parse/re-marshal entirely for responses without errors (the common case).
	if bytes.Contains(body, errorsMarker) {
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			if errors, ok := data["errors"].([]interface{}); ok {
				for _, errItem := range errors {
					if errMap, ok := errItem.(map[string]interface{}); ok {
						if message, ok := errMap["message"].(string); ok {
							sanitized := didYouMeanRE.ReplaceAllString(message, "")
							sanitized = whitespaceRE.ReplaceAllString(sanitized, " ")
							sanitized = strings.TrimSpace(sanitized)
							errMap["message"] = sanitized
						}
					}
				}
				if sanitizedBody, err := json.Marshal(data); err == nil {
					body = sanitizedBody
				}
			}
		}
	}

	// Write headers and body
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = w.ResponseWriter.Write(body)
}

// New creates a GraphQL handler from the provided GraphContext.
// It builds the schema and sets up authentication with token extraction and user details.
//
// The handler automatically:
//   - Extracts tokens using TokenExtractorFn (defaults to Bearer token extraction)
//   - Fetches user details using UserDetailsFn if provided
//   - Adds token and details to the root value for access in resolvers
//
// Returns an error if schema building fails.
//
// Example:
//
//	handler, err := graph.New(graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields: []graph.QueryField{getUserQuery()},
//	    },
//	    Playground: true,
//	})
func New(graphCtx GraphContext) (*handler.Handler, error) {
	// Build schema from context
	schema, err := buildSchemaFromContext(&graphCtx)
	if err != nil {
		return nil, err
	}

	h := handler.New(&handler.Config{
		Schema:     schema,
		Pretty:     graphCtx.Pretty,
		GraphiQL:   graphCtx.GraphiQL,
		Playground: graphCtx.Playground,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			if graphCtx.RootObjectFn != nil {
				graphCtx.RootObjectFn(ctx, r)
			}

			// Create root value with token for GraphQL resolvers
			rootValue := make(map[string]interface{})

			// Use custom token extractor if provided, otherwise use default Bearer token extractor
			tokenExtractor := graphCtx.TokenExtractorFn
			if tokenExtractor == nil {
				tokenExtractor = ExtractBearerToken
			}

			token := tokenExtractor(r)
			if token != "" {
				rootValue["token"] = token

				// Use custom user details fetcher if provided
				// Note: Context updates from UserDetailsFn are only accessible when using NewHTTP()
				// The New() function cannot modify the request context
				if graphCtx.UserDetailsFn != nil {
					_, details, err := graphCtx.UserDetailsFn(ctx, token)
					if err == nil {
						rootValue["details"] = details
					}
				}
			}

			return rootValue
		},
	})

	return h, nil
}

// NewHTTP creates a standard http.HandlerFunc with built-in validation and sanitization support.
// This is the recommended way to create a GraphQL handler for production use.
//
// The handler automatically detects WebSocket upgrade requests and handles them appropriately
// when subscriptions are enabled (EnableSubscriptions: true).
//
// The handler is fully compatible with net/http and any HTTP framework (Gin, Chi, Echo, etc.).
// If graphCtx is nil, defaults to DEBUG mode with Playground enabled.
//
// Behavior:
//   - In DEBUG mode (DEBUG: true): Skips all validation and sanitization for easier development
//   - In production (DEBUG: false): Enables validation and sanitization based on configuration
//   - Panics during initialization if schema building fails (fail-fast approach)
//   - WebSocket upgrade requests are handled when EnableSubscriptions: true
//
// Security Features (when DEBUG: false):
//   - EnableValidation: Validates query depth (max 10), aliases (max 4), complexity (max 200), and blocks introspection
//   - EnableSanitization: Removes field suggestions from error messages to prevent information disclosure
//
// Example without subscriptions:
//
//	// Development setup
//	handler := graph.NewHTTP(&graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields: []graph.QueryField{getUserQuery()},
//	    },
//	    DEBUG:      true,
//	    Playground: true,
//	})
//
//	// Production setup
//	handler := graph.NewHTTP(&graph.GraphContext{
//	    SchemaParams:       &graph.SchemaBuilderParams{...},
//	    DEBUG:              false,
//	    EnableValidation:   true,
//	    EnableSanitization: true,
//	    Playground:         false,
//	    UserDetailsFn: func(ctx context.Context, token string) (context.Context, interface{}, error) {
//	        user, err := validateToken(token)
//	        if err != nil {
//	            return ctx, nil, err
//	        }
//	        // Add user ID to context for access in resolvers via p.Context.Value("userID")
//	        ctx = context.WithValue(ctx, "userID", user.ID)
//	        return ctx, user, nil
//	    },
//	})
//
//	http.Handle("/graphql", handler)
//	http.ListenAndServe(":8080", nil)
//
// Example with subscriptions:
//
//	pubsub := graph.NewInMemoryPubSub()
//	defer pubsub.Close()
//
//	handler := graph.NewHTTP(&graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields:        []graph.QueryField{getUserQuery()},
//	        MutationFields:     []graph.MutationField{createUserMutation()},
//	        SubscriptionFields: []graph.SubscriptionField{userSubscription(pubsub)},
//	    },
//	    PubSub:              pubsub,
//	    EnableSubscriptions: true,
//	    DEBUG:               false,
//	})
//
//	http.Handle("/graphql", handler)
//	http.ListenAndServe(":8080", nil)
func NewHTTP(graphCtx *GraphContext) http.HandlerFunc {
	if graphCtx == nil {
		graphCtx = &GraphContext{DEBUG: true, Playground: true}
	}

	// Build handler (panic if schema building fails)
	h, err := New(*graphCtx)
	if err != nil {
		panic("failed to build GraphQL schema: " + err.Error())
	}

	// Get the built schema for validation
	schema, err := buildSchemaFromContext(graphCtx)
	if err != nil {
		panic("failed to build GraphQL schema: " + err.Error())
	}

	// Create WebSocket handler if subscriptions are enabled
	var wsHandler http.HandlerFunc
	if graphCtx.EnableSubscriptions {
		// Set up WebSocket handler
		wsParams := WebSocketParams{
			Schema:       schema,
			PubSub:       graphCtx.PubSub,
			AuthFn:       createWebSocketAuthFn(graphCtx),
			CheckOrigin:  graphCtx.WebSocketCheckOrigin,
			RootObjectFn: graphCtx.RootObjectFn,
		}
		wsHandler = NewWebSocketHandler(wsParams)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a WebSocket upgrade request
		if graphCtx.EnableSubscriptions && r.Header.Get("Upgrade") == "websocket" {
			if wsHandler != nil {
				wsHandler(w, r)
			} else {
				http.Error(w, "WebSocket subscriptions not configured", http.StatusServiceUnavailable)
			}
			return
		}

		// Call UserDetailsFn to potentially update context
		// This allows UserDetailsFn to add values to context accessible via p.Context.Value()
		token := extractToken(r, graphCtx.TokenExtractorFn)
		result := callUserDetailsFn(graphCtx, r.Context(), token)
		if result.ctx != r.Context() {
			// Context was updated by UserDetailsFn, update the request
			r = r.WithContext(result.ctx)
		}

		// Skip validation and sanitization in DEBUG mode
		if graphCtx.DEBUG {
			h.ServeHTTP(w, r)
			return
		}

		// Extract query for validation
		var query string
		if r.Method == http.MethodPost {
			// Read body into a pooled buffer. The buffer must outlive h.ServeHTTP
			// since r.Body wraps its bytes, so release via defer at closure scope.
			buf := bodyBufPool.Get().(*bytes.Buffer)
			buf.Reset()
			defer bodyBufPool.Put(buf)

			if _, err := buf.ReadFrom(r.Body); err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}
			bodyBytes := buf.Bytes()

			// Try to parse as form data
			if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				if err := r.ParseForm(); err == nil {
					query = r.PostForm.Get("query")
				}
			} else {
				// Try to parse as JSON
				var requestBody gqlRequestBody
				if err := json.Unmarshal(bodyBytes, &requestBody); err == nil {
					query = requestBody.Query
				}
			}

			// Restore body for GraphQL handler
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		} else if r.Method == http.MethodGet {
			query = r.URL.Query().Get("query")
		}

		// Validate query if enabled
		if query != "" {
			// Determine which validation rules to use
			var rules []ValidationRule
			if len(graphCtx.ValidationRules) > 0 {
				// Use custom validation rules (takes precedence)
				rules = graphCtx.ValidationRules
			} else if graphCtx.EnableValidation {
				// Fall back to default security rules for backward compatibility
				rules = SecurityRules
			}

			// Execute validation if rules are configured
			if len(rules) > 0 {
				// Use user details from earlier UserDetailsFn call
				userDetails := result.details

				// Execute validation rules
				if err := ExecuteValidationRules(query, schema, rules, userDetails, graphCtx.ValidationOptions); err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)

					var resp gqlErrorResponse
					switch e := err.(type) {
					case *MultiValidationError:
						resp.Errors = make([]gqlError, 0, len(e.Errors))
						for _, inner := range e.Errors {
							if validationErr, ok := inner.(*ValidationError); ok {
								resp.Errors = append(resp.Errors, gqlError{
									Message: validationErr.Error(),
									Rule:    validationErr.Rule,
								})
							} else {
								resp.Errors = append(resp.Errors, gqlError{Message: inner.Error()})
							}
						}
					case *ValidationError:
						resp.Errors = []gqlError{{Message: e.Message, Rule: e.Rule}}
					default:
						resp.Errors = []gqlError{{Message: err.Error()}}
					}

					_ = json.NewEncoder(w).Encode(resp)
					return
				}
			}
		}

		// Wrap response writer for sanitization if enabled
		if graphCtx.EnableSanitization {
			wrapper := acquireResponseWriterWrapper(w)
			h.ServeHTTP(wrapper, r)
			wrapper.sanitizeAndWrite()
			releaseResponseWriterWrapper(wrapper)
		} else {
			h.ServeHTTP(w, r)
		}
	}
}
