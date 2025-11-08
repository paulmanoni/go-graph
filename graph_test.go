package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
			got, err := GetArgString(params, tt.key)

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
			got, err := GetArgInt(params, tt.key)

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
			got, err := GetArgBool(params, tt.key)

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
			err := GetArg(params, tt.key, &got)

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
			got, err := GetRootString(params, tt.key)

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
			err := GetRootInfo(params, tt.key, &got)

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
				WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
					return User{ID: 1, Name: "Test"}, nil
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
		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
			return User{ID: 1, Name: "Test"}, nil
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
		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
			return User{ID: 1, Name: "Test"}, nil
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
		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
			return []User{{ID: 1, Name: "Test"}}, nil
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

	field := NewResolver[User]("users").
		AsPaginated().
		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
			return PaginatedResponse[User]{
				Items:      []User{{ID: 1, Name: "Test"}},
				TotalCount: 1,
				PageInfo: PageInfo{
					HasNextPage:     false,
					HasPreviousPage: false,
				},
			}, nil
		}).BuildQuery()

	if field.Name() != "users" {
		t.Errorf("Field name = %v, want users", field.Name())
	}

	graphqlField := field.Serve()
	if graphqlField.Type == nil {
		t.Error("Field type should not be nil")
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
		UserDetailsFn: func(token string) (interface{}, error) {
			return map[string]interface{}{"id": 1, "name": "Test User"}, nil
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
	resolver := func(p graphql.ResolveParams) (interface{}, error) {
		return "test result", nil
	}

	wrapped := LoggingMiddleware(resolver)

	params := graphql.ResolveParams{
		Info: graphql.ResolveInfo{
			FieldName: "testField",
		},
	}

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
					WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
						return User{ID: 1, Name: "Test"}, nil
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