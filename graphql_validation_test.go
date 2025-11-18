package graph

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

// MockUser implements minimal interfaces for testing
type MockUser struct {
	id          string
	roles       []string
	permissions []string
}

func (u *MockUser) GetID() string {
	return u.id
}

func (u *MockUser) HasRole(role string) bool {
	for _, r := range u.roles {
		if r == role {
			return true
		}
	}
	return false
}

func (u *MockUser) HasPermission(permission string) bool {
	for _, p := range u.permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// createTestSchema creates a simple schema for testing
func createTestSchema() *graphql.Schema {
	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":    &graphql.Field{Type: graphql.String},
			"name":  &graphql.Field{Type: graphql.String},
			"email": &graphql.Field{Type: graphql.String},
		},
	})

	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"user": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return map[string]string{"id": "1", "name": "Test", "email": "test@example.com"}, nil
				},
			},
			"deleteUser": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "deleted", nil
				},
			},
			"sensitiveData": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "sensitive", nil
				},
			},
		},
	})

	rootMutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createUser": &graphql.Field{
				Type: userType,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return map[string]string{"id": "2", "name": "New User", "email": "new@example.com"}, nil
				},
			},
		},
	})

	schema, _ := graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: rootMutation,
	})

	return &schema
}

// TestMaxDepthRule tests the MaxDepthRule validation
func TestMaxDepthRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		maxDepth    int
		shouldError bool
	}{
		{
			name:        "Depth within limit",
			query:       `{ user { id name } }`,
			maxDepth:    5,
			shouldError: false,
		},
		{
			name:        "Depth exceeds limit",
			query:       `{ user { id name email } }`,
			maxDepth:    1,
			shouldError: true,
		},
		{
			name:        "Complex nested query",
			query:       `{ user { id } }`,
			maxDepth:    10,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewMaxDepthRule(tt.maxDepth)}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestMaxComplexityRule tests the MaxComplexityRule validation
func TestMaxComplexityRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name          string
		query         string
		maxComplexity int
		shouldError   bool
	}{
		{
			name:          "Complexity within limit",
			query:         `{ user { id } }`,
			maxComplexity: 100,
			shouldError:   false,
		},
		{
			name:          "Complexity exceeds limit",
			query:         `{ user { id name email } }`,
			maxComplexity: 1,
			shouldError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewMaxComplexityRule(tt.maxComplexity)}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestMaxAliasesRule tests the MaxAliasesRule validation
func TestMaxAliasesRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		maxAliases  int
		shouldError bool
	}{
		{
			name:        "No aliases",
			query:       `{ user { id name } }`,
			maxAliases:  4,
			shouldError: false,
		},
		{
			name:        "Aliases within limit",
			query:       `{ u1: user { id } u2: user { name } }`,
			maxAliases:  4,
			shouldError: false,
		},
		{
			name:        "Aliases exceed limit",
			query:       `{ u1: user { id } u2: user { name } u3: user { email } }`,
			maxAliases:  2,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewMaxAliasesRule(tt.maxAliases)}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestNoIntrospectionRule tests the NoIntrospectionRule validation
func TestNoIntrospectionRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		shouldError bool
	}{
		{
			name:        "Normal query",
			query:       `{ user { id } }`,
			shouldError: false,
		},
		{
			name:        "Schema introspection",
			query:       `{ __schema { types { name } } }`,
			shouldError: true,
		},
		{
			name:        "Type introspection",
			query:       `{ __type(name: "User") { name } }`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewNoIntrospectionRule()}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestMaxTokensRule tests the MaxTokensRule validation
func TestMaxTokensRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		maxTokens   int
		shouldError bool
	}{
		{
			name:        "Tokens within limit",
			query:       `{ user { id } }`,
			maxTokens:   100,
			shouldError: false,
		},
		{
			name:        "Tokens exceed limit",
			query:       `{ user { id name email } }`,
			maxTokens:   5,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewMaxTokensRule(tt.maxTokens)}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestRequireAuthRule tests the RequireAuthRule validation
func TestRequireAuthRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		userDetails interface{}
		targets     []string
		shouldError bool
	}{
		{
			name:        "Authenticated query",
			query:       `query { user { id } }`,
			userDetails: &MockUser{id: "1"},
			targets:     []string{"query"},
			shouldError: false,
		},
		{
			name:        "Unauthenticated query requiring auth",
			query:       `query { user { id } }`,
			userDetails: nil,
			targets:     []string{"query"},
			shouldError: true,
		},
		{
			name:        "Authenticated mutation",
			query:       `mutation { createUser { id } }`,
			userDetails: &MockUser{id: "1"},
			targets:     []string{"mutation"},
			shouldError: false,
		},
		{
			name:        "Unauthenticated mutation requiring auth",
			query:       `mutation { createUser { id } }`,
			userDetails: nil,
			targets:     []string{"mutation"},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewRequireAuthRule(tt.targets...)}
			err := ExecuteValidationRules(tt.query, schema, rules, tt.userDetails, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestRoleRule tests the RoleRule validation
func TestRoleRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		userDetails interface{}
		field       string
		roles       []string
		shouldError bool
	}{
		{
			name:        "User with required role",
			query:       `{ deleteUser }`,
			userDetails: &MockUser{id: "1", roles: []string{"admin"}},
			field:       "deleteUser",
			roles:       []string{"admin"},
			shouldError: false,
		},
		{
			name:        "User without required role",
			query:       `{ deleteUser }`,
			userDetails: &MockUser{id: "1", roles: []string{"user"}},
			field:       "deleteUser",
			roles:       []string{"admin"},
			shouldError: true,
		},
		{
			name:        "User with one of multiple required roles",
			query:       `{ deleteUser }`,
			userDetails: &MockUser{id: "1", roles: []string{"manager"}},
			field:       "deleteUser",
			roles:       []string{"admin", "manager"},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewRoleRule(tt.field, tt.roles...)}
			err := ExecuteValidationRules(tt.query, schema, rules, tt.userDetails, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestPermissionRule tests the PermissionRule validation
func TestPermissionRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		userDetails interface{}
		field       string
		permissions []string
		shouldError bool
	}{
		{
			name:        "User with required permission",
			query:       `{ sensitiveData }`,
			userDetails: &MockUser{id: "1", permissions: []string{"read:sensitive"}},
			field:       "sensitiveData",
			permissions: []string{"read:sensitive"},
			shouldError: false,
		},
		{
			name:        "User without required permission",
			query:       `{ sensitiveData }`,
			userDetails: &MockUser{id: "1", permissions: []string{"read:public"}},
			field:       "sensitiveData",
			permissions: []string{"read:sensitive"},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewPermissionRule(tt.field, tt.permissions...)}
			err := ExecuteValidationRules(tt.query, schema, rules, tt.userDetails, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestBlockedFieldsRule tests the BlockedFieldsRule validation
func TestBlockedFieldsRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		fields      []string
		shouldError bool
	}{
		{
			name:        "Query allowed field",
			query:       `{ user { id } }`,
			fields:      []string{"sensitiveData"},
			shouldError: false,
		},
		{
			name:        "Query blocked field",
			query:       `{ sensitiveData }`,
			fields:      []string{"sensitiveData"},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{NewBlockedFieldsRule(tt.fields...)}
			err := ExecuteValidationRules(tt.query, schema, rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestRateLimitRule tests the RateLimitRule validation
func TestRateLimitRule(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		query       string
		userDetails interface{}
		budget      int
		shouldError bool
	}{
		{
			name:        "Query within budget",
			query:       `{ user { id } }`,
			userDetails: &MockUser{id: "1"},
			budget:      1000,
			shouldError: false,
		},
		{
			name:        "Query exceeds budget",
			query:       `{ user { id name email } }`,
			userDetails: &MockUser{id: "1"},
			budget:      1,
			shouldError: true,
		},
		{
			name:        "Admin bypasses rate limit",
			query:       `{ user { id name email } }`,
			userDetails: &MockUser{id: "1", roles: []string{"admin"}},
			budget:      1,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []ValidationRule{
				NewRateLimitRule(
					WithBudgetFunc(SimpleBudgetFunc(tt.budget)),
					WithBypassRoles("admin"),
				),
			}
			err := ExecuteValidationRules(tt.query, schema, rules, tt.userDetails, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestMultipleValidationErrors tests that multiple errors are collected
func TestMultipleValidationErrors(t *testing.T) {
	schema := createTestSchema()

	rules := []ValidationRule{
		NewMaxDepthRule(1),
		NewMaxAliasesRule(0),
	}

	query := `{ u1: user { id name } u2: user { email } }`
	err := ExecuteValidationRules(query, schema, rules, nil, nil)

	if err == nil {
		t.Fatal("Expected error but got none")
	}

	multiErr, ok := err.(*MultiValidationError)
	if !ok {
		t.Fatalf("Expected MultiValidationError but got %T", err)
	}

	if len(multiErr.Errors) != 2 {
		t.Errorf("Expected 2 errors but got %d", len(multiErr.Errors))
	}
}

// TestStopOnFirstError tests that validation stops on first error when configured
func TestStopOnFirstError(t *testing.T) {
	schema := createTestSchema()

	rules := []ValidationRule{
		NewMaxDepthRule(1),
		NewMaxAliasesRule(0),
	}

	query := `{ u1: user { id name } u2: user { email } }`
	options := &ValidationOptions{StopOnFirstError: true}
	err := ExecuteValidationRules(query, schema, rules, nil, options)

	if err == nil {
		t.Fatal("Expected error but got none")
	}

	// Should be a single ValidationError, not MultiValidationError
	if _, ok := err.(*MultiValidationError); ok {
		t.Error("Expected single error but got multiple")
	}
}

// TestPresetRules tests the preset rule collections
func TestPresetRules(t *testing.T) {
	schema := createTestSchema()

	tests := []struct {
		name        string
		rules       []ValidationRule
		query       string
		shouldError bool
	}{
		{
			name:        "SecurityRules allows normal query",
			rules:       SecurityRules,
			query:       `{ user { id } }`,
			shouldError: false,
		},
		{
			name:        "SecurityRules blocks introspection",
			rules:       SecurityRules,
			query:       `{ __schema { types { name } } }`,
			shouldError: true,
		},
		{
			name:        "StrictSecurityRules enforces stricter limits",
			rules:       StrictSecurityRules,
			query:       `{ user { id } }`,
			shouldError: false,
		},
		{
			name:        "DevelopmentRules is lenient",
			rules:       DevelopmentRules,
			query:       `{ user { id name email } }`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteValidationRules(tt.query, schema, tt.rules, nil, nil)

			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestCombineRules tests combining multiple rule sets
func TestCombineRules(t *testing.T) {
	schema := createTestSchema()

	customRules := []ValidationRule{
		NewRequireAuthRule("mutation"),
	}

	combined := CombineRules(SecurityRules, customRules)

	// Should have rules from both sets
	if len(combined) != len(SecurityRules)+len(customRules) {
		t.Errorf("Expected %d combined rules but got %d", len(SecurityRules)+len(customRules), len(combined))
	}

	// Test that combined rules work
	var userDetails interface{}
	err := ExecuteValidationRules(`mutation { createUser { id } }`, schema, combined, userDetails, nil)

	if err == nil {
		t.Error("Expected authentication error but got none")
	}
}

// TestHandlerIntegration tests the validation integration in NewHTTP handler
func TestHandlerIntegration(t *testing.T) {
	tests := []struct {
		name           string
		graphCtx       *GraphContext
		query          string
		expectedStatus int
	}{
		{
			name: "DEBUG mode bypasses validation",
			graphCtx: &GraphContext{
				DEBUG:            true,
				EnableValidation: true,
			},
			query:          `{ __schema { types { name } } }`,
			expectedStatus: http.StatusOK,
		},
		{
			name: "EnableValidation blocks introspection",
			graphCtx: &GraphContext{
				DEBUG:            false,
				EnableValidation: true,
			},
			query:          `{ __schema { types { name } } }`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Custom ValidationRules",
			graphCtx: &GraphContext{
				DEBUG: false,
				ValidationRules: []ValidationRule{
					NewMaxDepthRule(1),
				},
			},
			query:          `{ user { id } }`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "No validation rules passes",
			graphCtx: &GraphContext{
				DEBUG: false,
			},
			query:          `{ user { id } }`,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the test schema directly instead of trying to build with SchemaParams
			schema := createTestSchema()
			tt.graphCtx.Schema = schema

			handler := NewHTTP(tt.graphCtx)

			// Create request
			body := strings.NewReader(`{"query":"` + tt.query + `"}`)
			req := httptest.NewRequest(http.MethodPost, "/graphql", body)
			req.Header.Set("Content-Type", "application/json")

			// Record response
			rec := httptest.NewRecorder()
			handler(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d but got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestDisableRule tests that disabled rules are skipped
func TestDisableRule(t *testing.T) {
	schema := createTestSchema()

	rule := NewMaxDepthRule(1)
	rule.Disable()

	rules := []ValidationRule{rule}
	query := `{ user { id name email } }`

	err := ExecuteValidationRules(query, schema, rules, nil, nil)
	if err != nil {
		t.Errorf("Expected no error from disabled rule but got: %v", err)
	}

	// Re-enable and test again
	rule.Enable()
	err = ExecuteValidationRules(query, schema, rules, nil, nil)
	if err == nil {
		t.Error("Expected error from enabled rule but got none")
	}
}