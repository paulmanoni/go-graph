package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/graphql-go/handler"
)

// ExtractBearerToken extracts the Bearer token from the Authorization header
// Returns empty string if no valid Bearer token is found
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

	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err == nil {
		// Sanitize error messages
		if errors, ok := data["errors"].([]interface{}); ok {
			for _, errItem := range errors {
				if errMap, ok := errItem.(map[string]interface{}); ok {
					if message, ok := errMap["message"].(string); ok {
						// Remove field suggestions using regex
						re := regexp.MustCompile(`Did you mean "[^"]+"\?`)
						sanitized := re.ReplaceAllString(message, "")
						// Clean up extra spaces
						sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, " ")
						sanitized = strings.TrimSpace(sanitized)
						errMap["message"] = sanitized
					}
				}
			}
			// Re-encode to JSON
			if sanitizedBody, err := json.Marshal(data); err == nil {
				body = sanitizedBody
			}
		}
	}

	// Write headers and body
	w.ResponseWriter.WriteHeader(w.statusCode)
	_, _ = w.ResponseWriter.Write(body)
}

func New(graphCtx GraphContext) *handler.Handler {
	return handler.New(&handler.Config{
		Schema:     graphCtx.Schema,
		Pretty:     graphCtx.Pretty,
		GraphiQL:   graphCtx.GraphiQL,
		Playground: graphCtx.Playground,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			graphCtx.RootObjectFn(ctx, r)

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
				if graphCtx.UserDetailsFn != nil {
					details, err := graphCtx.UserDetailsFn(token)
					if err == nil {
						rootValue["details"] = details
					}
				}
			}

			return rootValue
		},
	})
}

// NewHTTP creates a standard http.HandlerFunc with validation and sanitization support
// This handler is compatible with standard net/http without requiring Gin or other frameworks
func NewHTTP(graphCtx GraphContext) http.HandlerFunc {
	h := New(graphCtx)

	return func(w http.ResponseWriter, r *http.Request) {
		// Skip validation and sanitization in DEBUG mode
		if graphCtx.DEBUG {
			h.ServeHTTP(w, r)
			return
		}

		// Extract query for validation
		var query string
		if r.Method == http.MethodPost {
			// Read body
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read request body", http.StatusBadRequest)
				return
			}

			// Try to parse as form data
			if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				if err := r.ParseForm(); err == nil {
					query = r.PostForm.Get("query")
				}
			} else {
				// Try to parse as JSON
				var requestBody map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &requestBody); err == nil {
					if q, ok := requestBody["query"].(string); ok {
						query = q
					}
				}
			}

			// Restore body for GraphQL handler
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		} else if r.Method == http.MethodGet {
			query = r.URL.Query().Get("query")
		}

		// Validate query if enabled
		if graphCtx.EnableValidation && query != "" {
			if err := ValidateGraphQLQuery(query, graphCtx.Schema); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errors": []map[string]interface{}{
						{"message": err.Error()},
					},
				})
				return
			}
		}

		// Wrap response writer for sanitization if enabled
		if graphCtx.EnableSanitization {
			wrapper := newResponseWriterWrapper(w)
			h.ServeHTTP(wrapper, r)
			wrapper.sanitizeAndWrite()
		} else {
			h.ServeHTTP(w, r)
		}
	}
}
