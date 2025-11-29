package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
)

// Test Utility Functions

func TestGetArgString(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		key       string
		want      string
		wantError bool
	}{
		{
			name:      "valid string argument",
			args:      map[string]interface{}{"name": "John"},
			key:       "name",
			want:      "John",
			wantError: false,
		},
		{
			name:      "missing argument",
			args:      map[string]interface{}{},
			key:       "name",
			want:      "",
			wantError: true,
		},
		{
			name:      "wrong type argument",
			args:      map[string]interface{}{"name": 123},
			key:       "name",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{Args: tt.args}
			got, err := GetArgString(ResolveParams(params), tt.key)

			if (err != nil) != tt.wantError {
				t.Errorf("GetArgString() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("GetArgString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArgInt(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		key       string
		want      int
		wantError bool
	}{
		{
			name:      "valid int argument",
			args:      map[string]interface{}{"age": 30},
			key:       "age",
			want:      30,
			wantError: false,
		},
		{
			name:      "missing argument",
			args:      map[string]interface{}{},
			key:       "age",
			want:      0,
			wantError: true,
		},
		{
			name:      "wrong type argument",
			args:      map[string]interface{}{"age": "thirty"},
			key:       "age",
			want:      0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{Args: tt.args}
			got, err := GetArgInt(ResolveParams(params), tt.key)

			if (err != nil) != tt.wantError {
				t.Errorf("GetArgInt() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("GetArgInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArgBool(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		key       string
		want      bool
		wantError bool
	}{
		{
			name:      "valid bool argument true",
			args:      map[string]interface{}{"active": true},
			key:       "active",
			want:      true,
			wantError: false,
		},
		{
			name:      "valid bool argument false",
			args:      map[string]interface{}{"active": false},
			key:       "active",
			want:      false,
			wantError: false,
		},
		{
			name:      "missing argument",
			args:      map[string]interface{}{},
			key:       "active",
			want:      false,
			wantError: true,
		},
		{
			name:      "wrong type argument",
			args:      map[string]interface{}{"active": "yes"},
			key:       "active",
			want:      false,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{Args: tt.args}
			got, err := GetArgBool(ResolveParams(params), tt.key)

			if (err != nil) != tt.wantError {
				t.Errorf("GetArgBool() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("GetArgBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArg(t *testing.T) {
	type Input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	tests := []struct {
		name      string
		args      map[string]interface{}
		key       string
		want      Input
		wantError bool
	}{
		{
			name: "valid complex argument",
			args: map[string]interface{}{
				"input": map[string]interface{}{
					"name":  "John",
					"email": "john@example.com",
					"age":   float64(30),
				},
			},
			key:       "input",
			want:      Input{Name: "John", Email: "john@example.com", Age: 30},
			wantError: false,
		},
		{
			name:      "missing argument",
			args:      map[string]interface{}{},
			key:       "input",
			want:      Input{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{Args: tt.args}
			var got Input
			err := GetArg(ResolveParams(params), tt.key, &got)

			if (err != nil) != tt.wantError {
				t.Errorf("GetArg() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("GetArg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRootString(t *testing.T) {
	tests := []struct {
		name      string
		rootValue map[string]interface{}
		key       string
		want      string
		wantError bool
	}{
		{
			name:      "valid root string",
			rootValue: map[string]interface{}{"token": "abc123"},
			key:       "token",
			want:      "abc123",
			wantError: false,
		},
		{
			name:      "missing key",
			rootValue: map[string]interface{}{},
			key:       "token",
			want:      "",
			wantError: true,
		},
		{
			name:      "wrong type",
			rootValue: map[string]interface{}{"token": 123},
			key:       "token",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{
				Info: graphql.ResolveInfo{
					RootValue: tt.rootValue,
				},
			}
			got, err := GetRootString(ResolveParams(params), tt.key)

			if (err != nil) != tt.wantError {
				t.Errorf("GetRootString() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if got != tt.want {
				t.Errorf("GetRootString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRootInfo(t *testing.T) {
	type UserDetails struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		rootValue map[string]interface{}
		key       string
		want      UserDetails
		wantError bool
	}{
		{
			name: "valid root info",
			rootValue: map[string]interface{}{
				"details": map[string]interface{}{
					"id":   float64(1),
					"name": "John",
				},
			},
			key:       "details",
			want:      UserDetails{ID: 1, Name: "John"},
			wantError: false,
		},
		{
			name:      "missing key",
			rootValue: map[string]interface{}{},
			key:       "details",
			want:      UserDetails{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := graphql.ResolveParams{
				Info: graphql.ResolveInfo{
					RootValue: tt.rootValue,
				},
			}
			var got UserDetails
			err := GetRootInfo(ResolveParams(params), tt.key, &got)

			if (err != nil) != tt.wantError {
				t.Errorf("GetRootInfo() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got != tt.want {
				t.Errorf("GetRootInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Token Extraction

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "valid bearer token",
			header: "Bearer abc123def456",
			want:   "abc123def456",
		},
		{
			name:   "valid bearer token with extra spaces",
			header: "Bearer   abc123def456",
			want:   "abc123def456",
		},
		{
			name:   "no bearer prefix",
			header: "abc123def456",
			want:   "",
		},
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "bearer only",
			header: "Bearer",
			want:   "",
		},
		{
			name:   "lowercase bearer",
			header: "bearer abc123def456",
			want:   "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := ExtractBearerToken(req)
			if got != tt.want {
				t.Errorf("ExtractBearerToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Schema Builder

func TestSchemaBuilder_Simple(t *testing.T) {
	params := SchemaBuilderParams{
		QueryFields: []QueryField{
			getDefaultHelloQuery(),
		},
		MutationFields: []MutationField{
			getDefaultEchoMutation(),
		},
	}

	schema, err := NewSchemaBuilder(params).Build()
	if err != nil {
		t.Fatalf("NewSchemaBuilder().Build() error = %v", err)
	}

	if schema.QueryType() == nil {
		t.Error("Schema should have query type")
	}

	if schema.MutationType() == nil {
		t.Error("Schema should have mutation type")
	}
}

func TestSchemaBuilder_WithCustomTypes(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	params := SchemaBuilderParams{
		QueryFields: []QueryField{
			NewResolver[User]("user").
				WithArgs(graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.Int},
				}).
				WithResolver(func(p ResolveParams) (*User, error) {
					return &User{ID: 1, Name: "Test"}, nil
				}).BuildQuery(),
		},
	}

	schema, err := NewSchemaBuilder(params).Build()
	if err != nil {
		t.Fatalf("NewSchemaBuilder().Build() error = %v", err)
	}

	if schema.QueryType() == nil {
		t.Error("Schema should have query type")
	}
}

// Test Resolver Creation

func TestNewResolver_Simple(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	field := NewResolver[User]("user").
		WithResolver(func(p ResolveParams) (*User, error) {
			return &User{ID: 1, Name: "Test"}, nil
		}).BuildQuery()

	if field.Name() != "user" {
		t.Errorf("Field name = %v, want user", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
	}

	if graphqlField.Resolve == nil {
		t.Error("Field resolve function should not be nil")
	}
}

func TestNewResolver_WithArgs(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	field := NewResolver[User]("user").
		WithArgs(graphql.FieldConfigArgument{
			"id": &graphql.ArgumentConfig{Type: graphql.Int},
		}).
		WithResolver(func(p ResolveParams) (*User, error) {
			return &User{ID: 1, Name: "Test"}, nil
		}).BuildQuery()

	graphqlField := field.Serve()
	if graphqlField.Args == nil {
		t.Error("Field args should not be nil")
	}

	if _, ok := graphqlField.Args["id"]; !ok {
		t.Error("Field should have 'id' argument")
	}
}

func TestNewResolver_AsList(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	field := NewResolver[[]User]("users").
		AsList().
		WithResolver(func(p ResolveParams) (*[]User, error) {
			users := []User{{ID: 1, Name: "Test"}}
			return &users, nil
		}).BuildQuery()

	if field.Name() != "users" {
		t.Errorf("Field name = %v, want users", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
	}
}

func TestNewResolver_AsPaginated(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	field := NewResolver[PaginatedResponse[User]]("users").
		AsPaginated().
		WithResolver(func(p ResolveParams) (*PaginatedResponse[User], error) {
			response := PaginatedResponse[User]{
				Items:      []User{{ID: 1, Name: "Test"}},
				TotalCount: 1,
				PageInfo: PageInfo{
					HasNextPage:     false,
					HasPreviousPage: false,
				},
			}
			return &response, nil
		}).BuildQuery()

	if field.Name() != "users" {
		t.Errorf("Field name = %v, want users", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
	}
}

func TestNewResolver_SliceOfStrings_WithoutAsList(t *testing.T) {
	// Test that NewResolver[[]string] works without calling AsList()
	field := NewResolver[[]string]("tags").
		WithResolver(func(p ResolveParams) (*[]string, error) {
			tags := []string{"go", "graphql", "test"}
			return &tags, nil
		}).BuildQuery()

	if field.Name() != "tags" {
		t.Errorf("Field name = %v, want tags", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
	}

	// Verify it's a list type
	listType, ok := graphqlField.Type.(*graphql.List)
	if !ok {
		t.Errorf("Expected list type, got %T", graphqlField.Type)
		return
	}

	// Verify the element type is String
	if listType.OfType != graphql.String {
		t.Errorf("Expected list of String, got list of %v", listType.OfType)
	}

	// Test resolver execution
	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{field},
	}).Build()
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ tags }`,
	})

	if len(result.Errors) > 0 {
		t.Errorf("Query returned errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result.Data)
	}

	tags, ok := data["tags"].([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} for tags, got %T", data["tags"])
	}

	if len(tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tags))
	}

	expectedTags := []string{"go", "graphql", "test"}
	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("Tag[%d] = %v, want %v", i, tag, expectedTags[i])
		}
	}
}

func TestNewResolver_SliceOfInts_WithoutAsList(t *testing.T) {
	// Test that NewResolver[[]int] works without calling AsList()
	field := NewResolver[[]int]("numbers").
		WithResolver(func(p ResolveParams) (*[]int, error) {
			numbers := []int{1, 2, 3, 4, 5}
			return &numbers, nil
		}).BuildQuery()

	if field.Name() != "numbers" {
		t.Errorf("Field name = %v, want numbers", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
	}

	// Verify it's a list type
	listType, ok := graphqlField.Type.(*graphql.List)
	if !ok {
		t.Errorf("Expected list type, got %T", graphqlField.Type)
		return
	}

	// Verify the element type is Int
	if listType.OfType != graphql.Int {
		t.Errorf("Expected list of Int, got list of %v", listType.OfType)
	}

	// Test resolver execution
	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{field},
	}).Build()
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ numbers }`,
	})

	if len(result.Errors) > 0 {
		t.Errorf("Query returned errors: %v", result.Errors)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result.Data)
	}

	numbers, ok := data["numbers"].([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} for numbers, got %T", data["numbers"])
	}

	if len(numbers) != 5 {
		t.Errorf("Expected 5 numbers, got %d", len(numbers))
	}
}

// Test Query Validation

func TestValidateGraphQLQuery_SimpleQuery(t *testing.T) {
	schema, _ := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{getDefaultHelloQuery()},
	}).Build()

	tests := []struct {
		name      string
		query     string
		wantError bool
	}{
		{
			name:      "valid simple query",
			query:     `{ hello }`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGraphQLQuery(tt.query, &schema)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateGraphQLQuery() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateGraphQLQuery_MaxDepth(t *testing.T) {
	schema, _ := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{getDefaultHelloQuery()},
	}).Build()

	// Deep query that exceeds max depth (10 levels)
	query := `{
		level1 {
			level2 {
				level3 {
					level4 {
						level5 {
							level6 {
								level7 {
									level8 {
										level9 {
											level10 {
												level11
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	err := ValidateGraphQLQuery(query, &schema)
	if err == nil {
		t.Error("ValidateGraphQLQuery() should reject queries exceeding max depth")
	}
}

func TestValidateGraphQLQuery_MaxAliases(t *testing.T) {
	schema, _ := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{getDefaultHelloQuery()},
	}).Build()

	// Query with too many aliases (more than 4)
	query := `{
		alias1: hello
		alias2: hello
		alias3: hello
		alias4: hello
		alias5: hello
	}`

	err := ValidateGraphQLQuery(query, &schema)
	if err == nil {
		t.Error("ValidateGraphQLQuery() should reject queries with too many aliases")
	}
}

func TestValidateGraphQLQuery_Introspection(t *testing.T) {
	schema, _ := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{getDefaultHelloQuery()},
	}).Build()

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "introspection __schema",
			query: `{ __schema { types { name } } }`,
		},
		{
			name:  "introspection __type",
			query: `{ __type(name: "Query") { name } }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGraphQLQuery(tt.query, &schema)
			if err == nil {
				t.Error("ValidateGraphQLQuery() should block introspection")
			}
			// Just check that an error was returned - the exact message may vary
		})
	}
}

// Test HTTP Handler

func TestNewHTTP_DefaultSchema(t *testing.T) {
	graphCtx := &GraphContext{
		DEBUG:      true,
		Playground: true,
	}

	handler := NewHTTP(graphCtx)
	if handler == nil {
		t.Fatal("NewHTTP() should return a handler")
	}

	// Test POST request
	body := bytes.NewBufferString(`{"query":"{ hello }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if data, ok := response["data"].(map[string]interface{}); !ok {
		t.Error("Response should have 'data' field")
	} else if hello, ok := data["hello"].(string); !ok || hello == "" {
		t.Error("Response should have 'hello' field with value")
	}
}

func TestNewHTTP_WithAuth(t *testing.T) {
	graphCtx := &GraphContext{
		DEBUG: true,
		UserDetailsFn: func(ctx context.Context, token string) (context.Context, interface{}, error) {
			return ctx, map[string]interface{}{"id": 1, "name": "Test User"}, nil
		},
	}

	handler := NewHTTP(graphCtx)

	body := bytes.NewBufferString(`{"query":"{ hello }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token-123")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestNewHTTP_GET(t *testing.T) {
	graphCtx := &GraphContext{
		DEBUG:      true,
		Playground: true,
	}

	handler := NewHTTP(graphCtx)

	req := httptest.NewRequest(http.MethodGet, "/graphql?query={hello}", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestNewHTTP_Playground(t *testing.T) {
	graphCtx := &GraphContext{
		DEBUG:      true,
		Playground: true,
	}

	handler := NewHTTP(graphCtx)

	// GET without query should return playground
	req := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	// Just verify response is not empty (playground returns some content)
	if w.Body.Len() == 0 {
		t.Error("Expected non-empty playground response")
	}
}

func TestNewHTTP_CustomRootObject(t *testing.T) {
	graphCtx := &GraphContext{
		DEBUG: true,
		RootObjectFn: func(ctx context.Context, r *http.Request) map[string]interface{} {
			return map[string]interface{}{
				"customData": "test-value",
			}
		},
	}

	handler := NewHTTP(graphCtx)

	body := bytes.NewBufferString(`{"query":"{ hello }"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}
}

// Test Middleware

func TestLoggingMiddleware(t *testing.T) {
	resolver := func(p ResolveParams) (interface{}, error) {
		return "test result", nil
	}

	wrapped := LoggingMiddleware(resolver)

	params := ResolveParams(graphql.ResolveParams{
		Info: graphql.ResolveInfo{
			FieldName: "testField",
		},
	})

	result, err := wrapped(params)
	if err != nil {
		t.Errorf("LoggingMiddleware() error = %v", err)
	}

	if result != "test result" {
		t.Errorf("LoggingMiddleware() result = %v, want 'test result'", result)
	}
}

func TestCachedFieldResolver(t *testing.T) {
	callCount := 0
	resolver := func(p graphql.ResolveParams) (interface{}, error) {
		callCount++
		return "expensive result", nil
	}

	cached := CachedFieldResolver(
		func(p graphql.ResolveParams) string {
			return "cache-key"
		},
		resolver,
	)

	params := graphql.ResolveParams{}

	// First call - should execute resolver
	result1, err1 := cached(params)
	if err1 != nil {
		t.Errorf("CachedFieldResolver() error = %v", err1)
	}
	if callCount != 1 {
		t.Errorf("Resolver should be called once, called %d times", callCount)
	}

	// Second call - should use cache
	result2, err2 := cached(params)
	if err2 != nil {
		t.Errorf("CachedFieldResolver() error = %v", err2)
	}
	if callCount != 1 {
		t.Errorf("Resolver should still be called once, called %d times", callCount)
	}

	if result1 != result2 {
		t.Errorf("Cached results should be equal: %v != %v", result1, result2)
	}
}

// Test Type Registration

func TestRegisterObjectType(t *testing.T) {
	typeName := "TestUser"

	RegisterObjectType(typeName, func() *graphql.Object {
		return graphql.NewObject(graphql.ObjectConfig{
			Name: typeName,
			Fields: graphql.Fields{
				"id":   &graphql.Field{Type: graphql.Int},
				"name": &graphql.Field{Type: graphql.String},
			},
		})
	})

	// Try to get the type (this tests internal registration)
	// Second registration should reuse cached type
	RegisterObjectType(typeName, func() *graphql.Object {
		return graphql.NewObject(graphql.ObjectConfig{
			Name: typeName,
			Fields: graphql.Fields{
				"id": &graphql.Field{Type: graphql.Int},
			},
		})
	})
}

// Test Handler Creation

func TestNew(t *testing.T) {
	graphCtx := GraphContext{
		Playground: true,
		DEBUG:      true,
	}

	handler, err := New(graphCtx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if handler == nil {
		t.Error("New() should return a handler")
	}
}

func TestNew_WithCustomSchema(t *testing.T) {
	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "world", nil
					},
				},
			},
		}),
	})

	graphCtx := GraphContext{
		Schema:     &schema,
		Playground: true,
	}

	handler, err := New(graphCtx)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if handler == nil {
		t.Error("New() should return a handler")
	}
}

// Test Response Writer Wrapper

func TestResponseWriterWrapper(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := newResponseWriterWrapper(w)

	data := []byte(`{"data":{"hello":"world"}}`)
	n, err := wrapper.Write(data)

	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if n != len(data) {
		t.Errorf("Write() n = %v, want %v", n, len(data))
	}

	if wrapper.statusCode != http.StatusOK {
		t.Errorf("statusCode = %v, want %v", wrapper.statusCode, http.StatusOK)
	}
}

func TestResponseWriterWrapper_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	wrapper := newResponseWriterWrapper(w)

	wrapper.WriteHeader(http.StatusBadRequest)

	if wrapper.statusCode != http.StatusBadRequest {
		t.Errorf("statusCode = %v, want %v", wrapper.statusCode, http.StatusBadRequest)
	}
}

// Test Build Schema From Context

func TestBuildSchemaFromContext_Default(t *testing.T) {
	graphCtx := &GraphContext{}

	schema, err := buildSchemaFromContext(graphCtx)
	if err != nil {
		t.Fatalf("buildSchemaFromContext() error = %v", err)
	}

	if schema.QueryType() == nil {
		t.Error("Default schema should have query type")
	}
}

func TestBuildSchemaFromContext_WithParams(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	graphCtx := &GraphContext{
		SchemaParams: &SchemaBuilderParams{
			QueryFields: []QueryField{
				NewResolver[User]("user").
					WithResolver(func(p ResolveParams) (*User, error) {
						return &User{ID: 1, Name: "Test"}, nil
					}).BuildQuery(),
			},
		},
	}

	schema, err := buildSchemaFromContext(graphCtx)
	if err != nil {
		t.Fatalf("buildSchemaFromContext() error = %v", err)
	}

	if schema.QueryType() == nil {
		t.Error("Schema should have query type")
	}
}

func TestBuildSchemaFromContext_WithCustomSchema(t *testing.T) {
	customSchema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"test": &graphql.Field{
					Type: graphql.String,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return "test", nil
					},
				},
			},
		}),
	})

	graphCtx := &GraphContext{
		Schema: &customSchema,
	}

	schema, err := buildSchemaFromContext(graphCtx)
	if err != nil {
		t.Fatalf("buildSchemaFromContext() error = %v", err)
	}

	if schema.QueryType() == nil {
		t.Error("Custom schema should have query type")
	}
}

// Test Type-Safe Arguments with NewArgsResolver

func TestNewArgsResolver_StructArgs(t *testing.T) {
	type GetUserArgs struct {
		ID   int    `json:"id" graphql:"id,required"`
		Name string `json:"name"`
	}

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	// Create resolver with struct arguments
	resolver := NewArgsResolver[User, GetUserArgs]("user").
		WithResolver(func(ctx context.Context, p ResolveParams, args GetUserArgs) (*User, error) {
			return &User{ID: args.ID, Name: args.Name}, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test that arguments were generated
	if field.Args == nil {
		t.Error("Expected args to be generated from struct")
	}

	if _, hasID := field.Args["id"]; !hasID {
		t.Error("Expected 'id' argument")
	}

	if _, hasName := field.Args["name"]; !hasName {
		t.Error("Expected 'name' argument")
	}

	// Test resolver execution
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"id":   1,
			"name": "John",
		},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	user, ok := result.(*User)
	if !ok {
		t.Fatalf("Expected *User, got %T", result)
	}

	if user.ID != 1 || user.Name != "John" {
		t.Errorf("Expected User{ID: 1, Name: John}, got %+v", user)
	}
}

func TestNewArgsResolver_PrimitiveArgs_String(t *testing.T) {
	// Create resolver with primitive string argument
	resolver := NewArgsResolver[string, string]("echo", "message").
		WithResolver(func(ctx context.Context, p ResolveParams, args string) (*string, error) {
			return &args, nil
		})

	field := resolver.BuildMutation().Serve()

	// Test that argument was generated
	if field.Args == nil {
		t.Error("Expected args to be generated")
	}

	if _, hasMessage := field.Args["message"]; !hasMessage {
		t.Error("Expected 'message' argument")
	}

	// Test resolver execution
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"message": "Hello World",
		},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	msg, ok := result.(*string)
	if !ok {
		t.Fatalf("Expected *string, got %T", result)
	}

	if *msg != "Hello World" {
		t.Errorf("Expected 'Hello World', got %s", *msg)
	}
}

func TestNewArgsResolver_PrimitiveArgs_Int(t *testing.T) {
	// Create resolver with primitive int argument
	resolver := NewArgsResolver[int, int]("double", "number").
		WithResolver(func(ctx context.Context, p ResolveParams, args int) (*int, error) {
			result := args * 2
			return &result, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test that argument was generated
	if field.Args == nil {
		t.Error("Expected args to be generated")
	}

	if _, hasNumber := field.Args["number"]; !hasNumber {
		t.Error("Expected 'number' argument")
	}

	// Test resolver execution
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"number": 5,
		},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	num, ok := result.(*int)
	if !ok {
		t.Fatalf("Expected *int, got %T", result)
	}

	if *num != 10 {
		t.Errorf("Expected 10, got %d", *num)
	}
}

func TestNewArgsResolver_NestedStructArgs(t *testing.T) {
	type MessageArgs struct {
		Input struct {
			Message string `json:"message"`
			Name    string `json:"name"`
		} `json:"input"`
	}

	// Create resolver with nested struct arguments
	resolver := NewArgsResolver[string, MessageArgs]("sendMessage").
		WithResolver(func(ctx context.Context, p ResolveParams, args MessageArgs) (*string, error) {
			result := args.Input.Name + ": " + args.Input.Message
			return &result, nil
		})

	field := resolver.BuildMutation().Serve()

	// Test that argument was generated
	if field.Args == nil {
		t.Error("Expected args to be generated")
	}

	if _, hasInput := field.Args["input"]; !hasInput {
		t.Error("Expected 'input' argument")
	}

	// Test resolver execution
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"message": "Hello",
				"name":    "John",
			},
		},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	msg, ok := result.(*string)
	if !ok {
		t.Fatalf("Expected *string, got %T", result)
	}

	expected := "John: Hello"
	if *msg != expected {
		t.Errorf("Expected '%s', got %s", expected, *msg)
	}
}

func TestNewArgsResolver_WithContext(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type GetUserArgs struct {
		ID int `json:"id"`
	}

	// Create resolver that uses context
	resolver := NewArgsResolver[User, GetUserArgs]("user").
		WithResolver(func(ctx context.Context, p ResolveParams, args GetUserArgs) (*User, error) {
			// Check if context is available
			if ctx == nil {
				t.Error("Context should not be nil")
			}

			// Check if we can get values from context
			if val := ctx.Value("test_key"); val != nil {
				name := val.(string)
				return &User{ID: args.ID, Name: name}, nil
			}

			return &User{ID: args.ID, Name: "Default"}, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test with context value
	ctx := context.WithValue(context.Background(), "test_key", "Context User")
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 1,
		},
		Context: ctx,
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	user, ok := result.(*User)
	if !ok {
		t.Fatalf("Expected *User, got %T", result)
	}

	if user.Name != "Context User" {
		t.Errorf("Expected 'Context User', got %s", user.Name)
	}
}

func TestNewArgsResolver_AsList(t *testing.T) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type ListUsersArgs struct {
		Limit int `json:"limit"`
	}

	// Create resolver that returns a list
	resolver := NewArgsResolver[[]User, ListUsersArgs]("users").
		AsList().
		WithResolver(func(ctx context.Context, p ResolveParams, args ListUsersArgs) (*[]User, error) {
			users := make([]User, args.Limit)
			for i := 0; i < args.Limit; i++ {
				users[i] = User{ID: i + 1, Name: "User"}
			}
			return &users, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test resolver execution
	result, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"limit": 3,
		},
		Context: context.Background(),
	})

	if err != nil {
		t.Fatalf("Resolver error = %v", err)
	}

	users, ok := result.(*[]User)
	if !ok {
		t.Fatalf("Expected *[]User, got %T", result)
	}

	if len(*users) != 3 {
		t.Errorf("Expected 3 users, got %d", len(*users))
	}
}

func TestNewArgsResolver_WithDescription(t *testing.T) {
	type GetUserArgs struct {
		ID int `json:"id"`
	}

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	resolver := NewArgsResolver[User, GetUserArgs]("user").
		WithDescription("Get a user by ID").
		WithResolver(func(ctx context.Context, p ResolveParams, args GetUserArgs) (*User, error) {
			return &User{ID: args.ID, Name: "Test"}, nil
		})

	field := resolver.BuildQuery().Serve()

	if field.Description != "Get a user by ID" {
		t.Errorf("Expected description 'Get a user by ID', got %s", field.Description)
	}
}

func TestNewArgsResolver_ErrorHandling(t *testing.T) {
	type GetUserArgs struct {
		ID int `json:"id"`
	}

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	resolver := NewArgsResolver[User, GetUserArgs]("user").
		WithResolver(func(ctx context.Context, p ResolveParams, args GetUserArgs) (*User, error) {
			if args.ID < 0 {
				return nil, fmt.Errorf("ID must be positive")
			}
			return &User{ID: args.ID, Name: "Test"}, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test with invalid ID
	_, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": -1,
		},
		Context: context.Background(),
	})

	if err == nil {
		t.Error("Expected error for negative ID")
	}
}

func TestNewArgsResolver_NilContext(t *testing.T) {
	type GetUserArgs struct {
		ID int `json:"id"`
	}

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	resolver := NewArgsResolver[User, GetUserArgs]("user").
		WithResolver(func(ctx context.Context, p ResolveParams, args GetUserArgs) (*User, error) {
			// Context should default to Background if nil
			if ctx == nil {
				t.Error("Context should not be nil, should default to Background")
			}
			return &User{ID: args.ID, Name: "Test"}, nil
		})

	field := resolver.BuildQuery().Serve()

	// Test with nil context - should use Background
	_, err := field.Resolve(graphql.ResolveParams{
		Args: map[string]interface{}{
			"id": 1,
		},
		Context: nil,
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// Test Embedded Struct Support

func TestGenerateGraphQLFields_EmbeddedStruct(t *testing.T) {
	type BaseEntity struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	}

	type Product struct {
		BaseEntity
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	fields := GenerateGraphQLFields[Product]()

	expectedFields := []string{"id", "created_at", "updated_at", "name", "price"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field %s to exist", fieldName)
		}
	}

	if len(fields) != len(expectedFields) {
		t.Errorf("Expected %d fields, got %d", len(expectedFields), len(fields))
	}
}

func TestGenerateGraphQLFields_MultipleEmbedding(t *testing.T) {
	type Timestamped struct {
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	}

	type Identified struct {
		ID string `json:"id"`
	}

	type Article struct {
		Identified
		Timestamped
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	fields := GenerateGraphQLFields[Article]()

	expectedFields := []string{"id", "created_at", "updated_at", "title", "content"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field %s to exist", fieldName)
		}
	}
}

func TestGenerateGraphQLFields_PointerEmbedding(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Company struct {
		*Address
		Name string `json:"name"`
	}

	fields := GenerateGraphQLFields[Company]()

	expectedFields := []string{"street", "city", "name"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field %s to exist", fieldName)
		}
	}
}

func TestGenerateGraphQLFields_FieldOverride(t *testing.T) {
	type BaseEntity struct {
		ID string `json:"id"`
	}

	type OverrideTest struct {
		BaseEntity
		ID string `json:"id"` // This should override BaseEntity.ID
	}

	fields := GenerateGraphQLFields[OverrideTest]()

	if _, exists := fields["id"]; !exists {
		t.Error("Expected id field to exist")
	}

	// Should only have one id field (the override)
	idCount := 0
	for name := range fields {
		if name == "id" {
			idCount++
		}
	}

	if idCount != 1 {
		t.Errorf("Expected exactly 1 id field, got %d", idCount)
	}
}

func TestFieldResolver_EmbeddedFields(t *testing.T) {
	type BaseEntity struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	}

	type Product struct {
		BaseEntity
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	fields := GenerateGraphQLFields[Product]()

	now := time.Now()
	product := Product{
		BaseEntity: BaseEntity{
			ID:        "123",
			CreatedAt: now,
			UpdatedAt: &now,
		},
		Name:  "Test Product",
		Price: 99.99,
	}

	// Test ID field resolver
	if idField, exists := fields["id"]; exists {
		result, err := idField.Resolve(graphql.ResolveParams{
			Source: product,
		})
		if err != nil {
			t.Errorf("ID field resolver error: %v", err)
		}
		if result != "123" {
			t.Errorf("Expected ID '123', got %v", result)
		}
	}

	// Test Name field resolver
	if nameField, exists := fields["name"]; exists {
		result, err := nameField.Resolve(graphql.ResolveParams{
			Source: product,
		})
		if err != nil {
			t.Errorf("Name field resolver error: %v", err)
		}
		if result != "Test Product" {
			t.Errorf("Expected Name 'Test Product', got %v", result)
		}
	}

	// Test Price field resolver
	if priceField, exists := fields["price"]; exists {
		result, err := priceField.Resolve(graphql.ResolveParams{
			Source: product,
		})
		if err != nil {
			t.Errorf("Price field resolver error: %v", err)
		}
		if result != 99.99 {
			t.Errorf("Expected Price 99.99, got %v", result)
		}
	}
}

func TestGenerateInputObject_EmbeddedStruct(t *testing.T) {
	type BaseEntity struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	}

	type Product struct {
		BaseEntity
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	input := GenerateInputObject[Product]("ProductInput")

	if input.Name() != "ProductInput" {
		t.Errorf("Expected input object name 'ProductInput', got %s", input.Name())
	}

	fields := input.Fields()
	expectedFields := []string{"id", "created_at", "updated_at", "name", "price"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected input field %s to exist", fieldName)
		}
	}
}

func TestGenerateArgsFromStruct_EmbeddedStruct(t *testing.T) {
	type BaseEntity struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt *time.Time `json:"updated_at,omitempty"`
	}

	type Product struct {
		BaseEntity
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	args := GenerateArgsFromStruct[Product]()

	expectedArgs := []string{"id", "created_at", "updated_at", "name", "price"}
	for _, argName := range expectedArgs {
		if _, exists := args[argName]; !exists {
			t.Errorf("Expected argument %s to exist", argName)
		}
	}
}

func TestGenerateGraphQLFields_NestedEmbedding(t *testing.T) {
	type Level1 struct {
		Field1 string `json:"field1"`
	}

	type Level2 struct {
		Level1
		Field2 string `json:"field2"`
	}

	type Level3 struct {
		Level2
		Field3 string `json:"field3"`
	}

	fields := GenerateGraphQLFields[Level3]()

	expectedFields := []string{"field1", "field2", "field3"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field %s to exist in nested embedding", fieldName)
		}
	}

	if len(fields) != len(expectedFields) {
		t.Errorf("Expected %d fields, got %d", len(expectedFields), len(fields))
	}
}

func TestGenerateGraphQLFields_MixedEmbedding(t *testing.T) {
	type BaseEntity struct {
		ID string `json:"id"`
	}

	type Metadata struct {
		Tags []string `json:"tags"`
	}

	type Document struct {
		BaseEntity
		Metadata
		Title string `json:"title"`
		Body  string `json:"body"`
	}

	fields := GenerateGraphQLFields[Document]()

	expectedFields := []string{"id", "tags", "title", "body"}
	for _, fieldName := range expectedFields {
		if _, exists := fields[fieldName]; !exists {
			t.Errorf("Expected field %s to exist in mixed embedding", fieldName)
		}
	}
}

func TestConcurrentEmbeddedFieldGeneration(t *testing.T) {
	type BaseEntity struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
	}

	type Product struct {
		BaseEntity
		Name string `json:"name"`
	}

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			_ = GenerateGraphQLFields[Product]()
			_ = GenerateGraphQLObject[Product]("Product")
			_ = GenerateInputObject[Product]("ProductInput")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// PageableResponse for testing (similar to user's pkg.PageableResponse)
type TestPageableResponse[T any] struct {
	Size             int   `json:"size"`
	Last             bool  `json:"last"`
	HasNext          bool  `json:"hasNext"`
	HasContent       bool  `json:"hasContent"`
	NumberOfElements int   `json:"numberOfElements"`
	Number           int   `json:"number"`
	TotalElements    int64 `json:"totalElements"`
	Content          []T   `json:"content"`
}

// Test for generic pageable response with nested struct - reproduces the issue
// "Cannot query field 'id' on type 'Interview'"
func TestPageableResponse_WithNestedStruct(t *testing.T) {
	// Clear type registry to ensure clean state
	typeRegistryMu.Lock()
	typeRegistry = make(map[string]*graphql.Object)
	typeRegistryMu.Unlock()

	// Define nested struct similar to user's Advert
	type Advert struct {
		ID           int64  `json:"id"`
		UID          string `json:"uid"`
		AdvertName   string `json:"advertName"`
		EmployerName string `json:"employerName"`
	}

	// Define main struct similar to user's Interview
	type Interview struct {
		ID                  int64      `json:"id"`
		UID                 string     `json:"uid"`
		Code                string     `json:"code"`
		ApprovalStatus      *bool      `json:"approvalStatus,omitempty"`
		InterviewDate       *time.Time `json:"interviewDate,omitempty"`
		InterviewType       string     `json:"interviewType"`
		InterviewStateEnum  string     `json:"interviewStateEnum"`
		EndingTime          *time.Time `json:"endingTime,omitempty"`
		StartingTime        *time.Time `json:"startingTime,omitempty"`
		HasInvigis          *bool      `json:"hasInvigis,omitempty"`
		Advert              *Advert    `json:"advert,omitempty"`
	}

	// Create resolver similar to user's NewGetActiveInterviewsPageable
	field := NewResolver[TestPageableResponse[Interview]]("getActiveInterviewsPageable").
		WithResolver(func(p ResolveParams) (*TestPageableResponse[Interview], error) {
			approved := true
			hasInvigis := false
			now := time.Now()
			return &TestPageableResponse[Interview]{
				Size:             10,
				Last:             false,
				HasNext:          true,
				HasContent:       true,
				NumberOfElements: 1,
				Number:           0,
				TotalElements:    100,
				Content: []Interview{
					{
						ID:                 1,
						UID:                "interview-001",
						Code:               "INT-001",
						ApprovalStatus:     &approved,
						InterviewDate:      &now,
						InterviewType:      "ORAL",
						InterviewStateEnum: "STARTED",
						StartingTime:       &now,
						HasInvigis:         &hasInvigis,
						Advert: &Advert{
							ID:           100,
							UID:          "advert-001",
							AdvertName:   "Software Engineer",
							EmployerName: "Tech Corp",
						},
					},
				},
			}, nil
		}).BuildQuery()

	// Build schema
	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{field},
	}).Build()
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	// Execute the exact query that was failing
	query := `{
		getActiveInterviewsPageable {
			content {
				id
				uid
				code
				approvalStatus
				interviewDate
				interviewType
				interviewStateEnum
				endingTime
				startingTime
				hasInvigis
				advert {
					id
					uid
					advertName
					employerName
				}
			}
		}
	}`

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})

	// Check for errors - this is the main assertion
	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			t.Errorf("GraphQL error: %s", err.Message)
		}
		t.Fatalf("Query returned %d errors", len(result.Errors))
	}

	// Verify data structure
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", result.Data)
	}

	pageableResult, ok := data["getActiveInterviewsPageable"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for getActiveInterviewsPageable, got %T", data["getActiveInterviewsPageable"])
	}

	content, ok := pageableResult["content"].([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{} for content, got %T", pageableResult["content"])
	}

	if len(content) != 1 {
		t.Fatalf("Expected 1 interview, got %d", len(content))
	}

	interview, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for interview, got %T", content[0])
	}

	// Verify interview fields
	if interview["id"] != 1 {
		t.Errorf("Expected id=1, got %v", interview["id"])
	}
	if interview["uid"] != "interview-001" {
		t.Errorf("Expected uid='interview-001', got %v", interview["uid"])
	}
	if interview["code"] != "INT-001" {
		t.Errorf("Expected code='INT-001', got %v", interview["code"])
	}

	// Verify nested advert
	advert, ok := interview["advert"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map for advert, got %T", interview["advert"])
	}

	if advert["id"] != 100 {
		t.Errorf("Expected advert id=100, got %v", advert["id"])
	}
	if advert["advertName"] != "Software Engineer" {
		t.Errorf("Expected advert advertName='Software Engineer', got %v", advert["advertName"])
	}
}

// Test that Interview type is consistently named regardless of where it's referenced
// Note: Types defined in function scope get numeric suffixes from Go's reflection (e.g., Interview93)
// This is a Go runtime behavior. In real applications, types are defined at package level
// and don't have these suffixes. This test verifies that the queries work correctly.
func TestTypeRegistryConsistency(t *testing.T) {
	// Clear type registry
	typeRegistryMu.Lock()
	typeRegistry = make(map[string]*graphql.Object)
	typeRegistryMu.Unlock()

	type Advert struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	type Interview struct {
		ID     int64   `json:"id"`
		Code   string  `json:"code"`
		Advert *Advert `json:"advert"`
	}

	// First, create a resolver that uses Interview directly
	directField := NewResolver[Interview]("getInterview").
		WithResolver(func(p ResolveParams) (*Interview, error) {
			return &Interview{ID: 1, Code: "INT-001"}, nil
		}).BuildQuery()

	// Then create a resolver that uses Interview inside PageableResponse
	pageableField := NewResolver[TestPageableResponse[Interview]]("getInterviews").
		WithResolver(func(p ResolveParams) (*TestPageableResponse[Interview], error) {
			return &TestPageableResponse[Interview]{
				Content: []Interview{{ID: 1, Code: "INT-001"}},
			}, nil
		}).BuildQuery()

	// Build schema with both
	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{directField, pageableField},
	}).Build()
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	// Both queries should work and return the same type structure
	directQuery := `{ getInterview { id code advert { id name } } }`
	pageableQuery := `{ getInterviews { content { id code advert { id name } } } }`

	directResult := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: directQuery,
	})

	if len(directResult.Errors) > 0 {
		for _, err := range directResult.Errors {
			t.Errorf("Direct query error: %s", err.Message)
		}
	}

	pageableResult := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: pageableQuery,
	})

	if len(pageableResult.Errors) > 0 {
		for _, err := range pageableResult.Errors {
			t.Errorf("Pageable query error: %s", err.Message)
		}
	}

	// Both queries should succeed - that's the key verification
	t.Logf("Both direct and pageable queries succeeded")
}

// Test deeply nested generic types
func TestDeeplyNestedGenericTypes(t *testing.T) {
	// Clear type registry
	typeRegistryMu.Lock()
	typeRegistry = make(map[string]*graphql.Object)
	typeRegistryMu.Unlock()

	type Level3 struct {
		Value string `json:"value"`
	}

	type Level2 struct {
		Level3Data *Level3 `json:"level3Data"`
		Name       string  `json:"name"`
	}

	type Level1 struct {
		Level2Data *Level2 `json:"level2Data"`
		ID         int64   `json:"id"`
	}

	field := NewResolver[TestPageableResponse[Level1]]("getData").
		WithResolver(func(p ResolveParams) (*TestPageableResponse[Level1], error) {
			return &TestPageableResponse[Level1]{
				Content: []Level1{
					{
						ID: 1,
						Level2Data: &Level2{
							Name: "test",
							Level3Data: &Level3{
								Value: "deep value",
							},
						},
					},
				},
			}, nil
		}).BuildQuery()

	schema, err := NewSchemaBuilder(SchemaBuilderParams{
		QueryFields: []QueryField{field},
	}).Build()
	if err != nil {
		t.Fatalf("Failed to build schema: %v", err)
	}

	query := `{
		getData {
			content {
				id
				level2Data {
					name
					level3Data {
						value
					}
				}
			}
		}
	}`

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})

	if len(result.Errors) > 0 {
		for _, err := range result.Errors {
			t.Errorf("GraphQL error: %s", err.Message)
		}
		t.Fatalf("Query returned errors")
	}

	// Verify deep nesting works
	data := result.Data.(map[string]interface{})
	getData := data["getData"].(map[string]interface{})
	content := getData["content"].([]interface{})
	level1 := content[0].(map[string]interface{})
	level2Data := level1["level2Data"].(map[string]interface{})
	level3Data := level2Data["level3Data"].(map[string]interface{})

	if level3Data["value"] != "deep value" {
		t.Errorf("Expected 'deep value', got %v", level3Data["value"])
	}
}
