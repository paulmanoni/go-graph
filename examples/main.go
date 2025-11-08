package main

import (
	"context"
	"log"
	"net/http"

	"github.com/paulmanoni/go-graph"
)

func main() {

	// Create GraphQL handler with all features
	handler := graph.NewHTTP(&graph.GraphContext{
		Playground: true,
		DEBUG:      true, // Set to true to disable validation/sanitization

		// Enable security features (ignored if DEBUG=true)
		EnableValidation:   true, // Validates depth, complexity, blocks introspection
		EnableSanitization: true, // Removes field suggestions from errors

		// Optional: Custom token extraction
		// If not provided, default Bearer token extraction is used
		TokenExtractorFn: func(r *http.Request) string {
			// You can customize this to extract from cookies, custom headers, etc.
			// For now, use the default Bearer token extraction
			return graph.ExtractBearerToken(r)
		},

		// Optional: Fetch user details based on token
		UserDetailsFn: func(token string) (interface{}, error) {
			// In production, validate JWT token, query database, etc.
			return nil, nil
		},

		// Optional: Custom root object setup
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			// You can add custom values to the root object here
			// The token and user details are already added automatically
			return nil
		},
	})

	// Setup routes
	http.HandleFunc("/graphql", handler)

	// Start server
	port := ":8080"
	log.Printf("GraphQL server starting on http://localhost%s/graphql", port)
	log.Printf("GraphQL Playground available at http://localhost%s/graphql", port)
	log.Println("\nExample queries:")
	log.Println("1. Public query (no auth):")
	log.Println("   { hello }")
	log.Println("\n2. Protected query (requires auth header: 'Authorization: Bearer token123'):")
	log.Println("   { me { id name email } }")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
