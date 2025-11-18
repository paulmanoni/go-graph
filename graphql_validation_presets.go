package graph

// Preset rule collections for common scenarios

var (
	// SecurityRules provides standard security validation
	// - Max depth: 10
	// - Max complexity: 200
	// - Max aliases: 4
	// - No introspection
	SecurityRules = []ValidationRule{
		NewMaxDepthRule(10),
		NewMaxComplexityRule(200),
		NewMaxAliasesRule(4),
		NewNoIntrospectionRule(),
	}

	// StrictSecurityRules provides strict security for production
	// - Max depth: 8
	// - Max complexity: 150
	// - Max aliases: 3
	// - Max tokens: 500
	// - No introspection
	StrictSecurityRules = []ValidationRule{
		NewMaxDepthRule(8),
		NewMaxComplexityRule(150),
		NewMaxAliasesRule(3),
		NewMaxTokensRule(500),
		NewNoIntrospectionRule(),
	}

	// DevelopmentRules provides lenient rules for development
	// - Max depth: 20
	// - Max complexity: 500
	DevelopmentRules = []ValidationRule{
		NewMaxDepthRule(20),
		NewMaxComplexityRule(500),
	}
)

// CombineRules combines multiple rule sets into one
//
// Example:
//   rules := CombineRules(
//       SecurityRules,
//       []ValidationRule{NewRequireAuthRule("mutation")},
//   )
func CombineRules(ruleSets ...[]ValidationRule) []ValidationRule {
	var combined []ValidationRule
	for _, rules := range ruleSets {
		combined = append(combined, rules...)
	}
	return combined
}

// DefaultValidationRules returns the default validation rules
// Equivalent to EnableValidation=true
func DefaultValidationRules() []ValidationRule {
	return SecurityRules
}

// ProductionValidationRules returns recommended production rules
func ProductionValidationRules() []ValidationRule {
	return StrictSecurityRules
}

// DevelopmentValidationRules returns lenient development rules
func DevelopmentValidationRules() []ValidationRule {
	return DevelopmentRules
}

// Common role configurations for convenience
var (
	// AdminOnlyFields - fields that require admin role
	AdminOnlyFields = map[string][]string{
		"deleteUser":     {"admin"},
		"deleteAccount":  {"admin"},
		"viewAuditLog":   {"admin"},
		"systemSettings": {"admin"},
		"manageRoles":    {"admin"},
	}

	// ManagerFields - fields that require admin or manager role
	ManagerFields = map[string][]string{
		"approveOrder":  {"admin", "manager"},
		"viewReports":   {"admin", "manager"},
		"manageTeam":    {"admin", "manager"},
		"bulkOperations": {"admin", "manager"},
	}

	// AuditorFields - fields that require admin or auditor role
	AuditorFields = map[string][]string{
		"viewAuditLog": {"admin", "auditor"},
		"exportLogs":   {"admin", "auditor"},
		"viewAnalytics": {"admin", "auditor"},
	}
)

// MergeRoleConfigs combines multiple role configurations
//
// Example:
//   allRoles := MergeRoleConfigs(AdminOnlyFields, ManagerFields, AuditorFields)
func MergeRoleConfigs(configs ...map[string][]string) map[string][]string {
	merged := make(map[string][]string)
	for _, config := range configs {
		for field, roles := range config {
			if existing, ok := merged[field]; ok {
				// Merge roles, removing duplicates
				roleSet := make(map[string]bool)
				for _, r := range existing {
					roleSet[r] = true
				}
				for _, r := range roles {
					roleSet[r] = true
				}

				// Convert back to slice
				var uniqueRoles []string
				for r := range roleSet {
					uniqueRoles = append(uniqueRoles, r)
				}
				merged[field] = uniqueRoles
			} else {
				merged[field] = roles
			}
		}
	}
	return merged
}