package graph

import (
	"testing"
)

// Benchmark validation rules against a test schema

func BenchmarkMaxDepthRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewMaxDepthRule(10)
	rules := []ValidationRule{rule}
	query := `{ user { id name email } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, nil, nil)
	}
}

func BenchmarkMaxComplexityRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewMaxComplexityRule(200)
	rules := []ValidationRule{rule}
	query := `{ user { id name email } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, nil, nil)
	}
}

func BenchmarkMaxAliasesRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewMaxAliasesRule(4)
	rules := []ValidationRule{rule}
	query := `{ u1: user { id } u2: user { name } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, nil, nil)
	}
}

func BenchmarkNoIntrospectionRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewNoIntrospectionRule()
	rules := []ValidationRule{rule}
	query := `{ user { id name } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, nil, nil)
	}
}

func BenchmarkRequireAuthRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewRequireAuthRule("mutation")
	rules := []ValidationRule{rule}
	query := `mutation { createUser { id } }`
	user := &MockUser{id: "1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, user, nil)
	}
}

func BenchmarkRoleRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewRoleRule("deleteUser", "admin")
	rules := []ValidationRule{rule}
	query := `{ deleteUser }`
	user := &MockUser{id: "1", roles: []string{"admin"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, user, nil)
	}
}

func BenchmarkPermissionRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewPermissionRule("sensitiveData", "read:sensitive")
	rules := []ValidationRule{rule}
	query := `{ sensitiveData }`
	user := &MockUser{id: "1", permissions: []string{"read:sensitive"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, user, nil)
	}
}

func BenchmarkRateLimitRule(b *testing.B) {
	schema := createTestSchema()
	rule := NewRateLimitRule(
		WithBudgetFunc(SimpleBudgetFunc(1000)),
	)
	rules := []ValidationRule{rule}
	query := `{ user { id name } }`
	user := &MockUser{id: "1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, user, nil)
	}
}

func BenchmarkSecurityRules(b *testing.B) {
	schema := createTestSchema()
	query := `{ user { id name email } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, SecurityRules, nil, nil)
	}
}

func BenchmarkStrictSecurityRules(b *testing.B) {
	schema := createTestSchema()
	query := `{ user { id name } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, StrictSecurityRules, nil, nil)
	}
}

func BenchmarkCombinedRules(b *testing.B) {
	schema := createTestSchema()
	customRules := []ValidationRule{
		NewRequireAuthRule("mutation"),
		NewRoleRule("deleteUser", "admin"),
	}
	combined := CombineRules(SecurityRules, customRules)
	query := `{ user { id } }`
	user := &MockUser{id: "1", roles: []string{"admin"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, combined, user, nil)
	}
}

// Benchmark complex queries
func BenchmarkComplexQuery(b *testing.B) {
	schema := createTestSchema()
	query := `{
		u1: user { id name email }
		u2: user { id name }
		u3: user { id }
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, SecurityRules, nil, nil)
	}
}

// Benchmark multiple validation errors
func BenchmarkMultipleErrors(b *testing.B) {
	schema := createTestSchema()
	rules := []ValidationRule{
		NewMaxDepthRule(1),
		NewMaxAliasesRule(0),
	}
	query := `{ u1: user { id name } u2: user { email } }`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ExecuteValidationRules(query, schema, rules, nil, nil)
	}
}