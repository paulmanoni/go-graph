// Package graph provides a modern, secure GraphQL handler for Go with built-in
// authentication, validation, and an intuitive builder API.
//
// Built on top of graphql-go (github.com/graphql-go/graphql), this package simplifies
// GraphQL server development with sensible defaults while maintaining full flexibility.
//
// # Features
//
//   - Zero Config Start: Default hello world schema included
//   - Fluent Builder API: Clean, type-safe schema construction
//   - Built-in Authentication: Automatic Bearer token extraction
//   - Security First: Query depth, complexity, and introspection protection
//   - Response Sanitization: Remove field suggestions from errors
//   - Framework Agnostic: Works with net/http, Gin, Chi, or any HTTP framework
//
// # Quick Start
//
// Start immediately with the default schema:
//
//	import "github.com/paulmanoni/graph"
//
//	func main() {
//	    handler := graph.NewHTTP(&graph.GraphContext{
//	        Playground: true,
//	        DEBUG:      true,
//	    })
//	    http.Handle("/graphql", handler)
//	    http.ListenAndServe(":8080", nil)
//	}
//
// # Builder Pattern
//
// Use the fluent builder API for clean schema construction:
//
//	func getUser() graph.QueryField {
//	    return graph.NewResolver[User]("user").
//	        WithArgs(graphql.FieldConfigArgument{
//	            "id": &graphql.ArgumentConfig{Type: graphql.String},
//	        }).
//	        WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//	            id, _ := graph.GetArgString(p, "id")
//	            return User{ID: id, Name: "Alice"}, nil
//	        }).BuildQuery()
//	}
//
// # Authentication
//
// Automatic Bearer token extraction with optional user details fetching:
//
//	handler := graph.NewHTTP(&graph.GraphContext{
//	    SchemaParams: &graph.SchemaBuilderParams{
//	        QueryFields: []graph.QueryField{getProtectedQuery()},
//	    },
//	    UserDetailsFn: func(ctx context.Context, token string) (context.Context, interface{}, error) {
//	        user, err := validateAndGetUser(token)
//	        if err != nil {
//	            return ctx, nil, err
//	        }
//	        // Add values to context accessible via p.Context.Value() in resolvers
//	        ctx = context.WithValue(ctx, "userID", user.ID)
//	        return ctx, user, nil
//	    },
//	})
//
// Access token in resolvers:
//
//	func getProtectedQuery() graph.QueryField {
//	    return graph.NewResolver[User]("me").
//	        WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//	            token, err := graph.GetRootString(p, "token")
//	            if err != nil {
//	                return nil, fmt.Errorf("authentication required")
//	            }
//	            // Use token...
//	        }).BuildQuery()
//	}
//
// # Security
//
// Enable security features for production:
//
//	handler := graph.NewHTTP(&graph.GraphContext{
//	    SchemaParams:       &graph.SchemaBuilderParams{...},
//	    DEBUG:              false,  // Enable security features
//	    EnableValidation:   true,   // Max depth: 10, Max aliases: 4, Max complexity: 200
//	    EnableSanitization: true,   // Remove field suggestions from errors
//	    Playground:         false,  // Disable playground in production
//	})
//
// # Helper Functions
//
// Extract arguments safely:
//
//	name, err := graph.GetArgString(p, "name")
//	age, err := graph.GetArgInt(p, "age")
//	active, err := graph.GetArgBool(p, "active")
//
// Access root values:
//
//	token, err := graph.GetRootString(p, "token")
//	var user User
//	err := graph.GetRootInfo(p, "details", &user)
//
// For more information, see https://github.com/paulmanoni/graph
package graph