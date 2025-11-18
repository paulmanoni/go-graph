package graph

import (
	"github.com/graphql-go/graphql/language/ast"
)

// HasRolesInterface - implement this on your user struct for role-based rules
type HasRolesInterface interface {
	HasRole(role string) bool
}

// HasPermissionsInterface - implement this on your user struct for permission-based rules
type HasPermissionsInterface interface {
	HasPermission(permission string) bool
}

// RequireAuthRule requires authentication for specific operations or fields
// Simply checks if ctx.UserDetails != nil
type RequireAuthRule struct {
	BaseRule
	operations map[string]bool // "mutation", "subscription", "query"
	fields     map[string]bool // specific field names
}

// NewRequireAuthRule creates a new require authentication rule
// Targets can be operation types ("mutation", "subscription", "query") or field names
//
// Example:
//   NewRequireAuthRule("mutation", "subscription")  // Require auth for all mutations and subscriptions
//   NewRequireAuthRule("deleteUser", "updateUser")  // Require auth for specific fields
func NewRequireAuthRule(targets ...string) ValidationRule {
	rule := &RequireAuthRule{
		BaseRule:   NewBaseRule("RequireAuthRule"),
		operations: make(map[string]bool),
		fields:     make(map[string]bool),
	}

	for _, target := range targets {
		switch target {
		case "mutation", "query", "subscription":
			rule.operations[target] = true
		default:
			rule.fields[target] = true
		}
	}

	return rule
}

func (r *RequireAuthRule) Validate(ctx *ValidationContext) error {
	// Check if user is authenticated (UserDetails is not nil)
	if ctx.UserDetails != nil {
		return nil // User is authenticated, all good
	}

	// User is not authenticated, check if query requires auth

	// Check operations
	for _, def := range ctx.Document.Definitions {
		if op, ok := def.(*ast.OperationDefinition); ok {
			if r.operations[op.Operation] {
				return r.NewErrorf("%s operations require authentication", op.Operation)
			}
		}
	}

	// Check specific fields
	if len(r.fields) > 0 {
		visitor := &ASTVisitor{
			EnterField: func(field *ast.Field, vctx *ValidationContext) error {
				if field.Name != nil && r.fields[field.Name.Value] {
					return r.NewErrorf("field '%s' requires authentication", field.Name.Value)
				}
				return nil
			},
		}
		return traverseAST(ctx.Document, visitor, ctx)
	}

	return nil
}

// RoleRule validates a single field requires specific roles
type RoleRule struct {
	BaseRule
	field string
	roles []string
}

// NewRoleRule creates a new role validation rule for a single field
//
// Example:
//   NewRoleRule("deleteUser", "admin")
//   NewRoleRule("viewAuditLog", "admin", "auditor")
func NewRoleRule(field string, roles ...string) ValidationRule {
	return &RoleRule{
		BaseRule: NewBaseRule("RoleRule:" + field),
		field:    field,
		roles:    roles,
	}
}

func (r *RoleRule) Validate(ctx *ValidationContext) error {
	if ctx.UserDetails == nil {
		return nil // RequireAuthRule should handle authentication check
	}

	// Check if user implements HasRolesInterface
	userWithRoles, ok := ctx.UserDetails.(HasRolesInterface)
	if !ok {
		return nil // User doesn't implement role checking
	}

	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name != nil && field.Name.Value == r.field {
				for _, role := range r.roles {
					if userWithRoles.HasRole(role) {
						return nil
					}
				}
				return r.NewErrorf("field '%s' requires one of roles: %v", r.field, r.roles)
			}
			return nil
		},
	}

	return traverseAST(ctx.Document, visitor, ctx)
}

// RoleRules validates multiple fields with role requirements
type RoleRules struct {
	BaseRule
	config map[string][]string
}

// NewRoleRules creates a batch role validation rule
// config maps field names to required roles
//
// Example:
//   NewRoleRules(map[string][]string{
//       "deleteUser":    {"admin"},
//       "viewAuditLog":  {"admin", "auditor"},
//       "approveOrder":  {"admin", "manager"},
//   })
func NewRoleRules(config map[string][]string) ValidationRule {
	return &RoleRules{
		BaseRule: NewBaseRule("RoleRules"),
		config:   config,
	}
}

func (r *RoleRules) Validate(ctx *ValidationContext) error {
	if ctx.UserDetails == nil {
		return nil
	}

	// Check if user implements HasRolesInterface
	userWithRoles, ok := ctx.UserDetails.(HasRolesInterface)
	if !ok {
		return nil // User doesn't implement role checking
	}

	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name == nil {
				return nil
			}

			requiredRoles, ok := r.config[field.Name.Value]
			if !ok {
				return nil
			}

			for _, role := range requiredRoles {
				if userWithRoles.HasRole(role) {
					return nil
				}
			}

			return r.NewErrorf("field '%s' requires one of: %v", field.Name.Value, requiredRoles)
		},
	}

	return traverseAST(ctx.Document, visitor, ctx)
}

// PermissionRule validates a single field requires specific permissions
type PermissionRule struct {
	BaseRule
	field       string
	permissions []string
}

// NewPermissionRule creates a new permission validation rule for a single field
//
// Example:
//   NewPermissionRule("sensitiveData", "read:sensitive")
//   NewPermissionRule("exportData", "export:data", "admin:all")
func NewPermissionRule(field string, permissions ...string) ValidationRule {
	return &PermissionRule{
		BaseRule:    NewBaseRule("PermissionRule:" + field),
		field:       field,
		permissions: permissions,
	}
}

func (r *PermissionRule) Validate(ctx *ValidationContext) error {
	if ctx.UserDetails == nil {
		return nil
	}

	// Check if user implements HasPermissionsInterface
	userWithPerms, ok := ctx.UserDetails.(HasPermissionsInterface)
	if !ok {
		return nil // User doesn't implement permission checking
	}

	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name != nil && field.Name.Value == r.field {
				for _, perm := range r.permissions {
					if userWithPerms.HasPermission(perm) {
						return nil
					}
				}
				return r.NewErrorf("field '%s' requires one of permissions: %v", r.field, r.permissions)
			}
			return nil
		},
	}

	return traverseAST(ctx.Document, visitor, ctx)
}

// PermissionRules validates multiple fields with permission requirements
type PermissionRules struct {
	BaseRule
	config map[string][]string
}

// NewPermissionRules creates a batch permission validation rule
// config maps field names to required permissions
//
// Example:
//   NewPermissionRules(map[string][]string{
//       "sensitiveData": {"read:sensitive"},
//       "exportData":    {"export:data"},
//       "adminPanel":    {"admin:access"},
//   })
func NewPermissionRules(config map[string][]string) ValidationRule {
	return &PermissionRules{
		BaseRule: NewBaseRule("PermissionRules"),
		config:   config,
	}
}

func (r *PermissionRules) Validate(ctx *ValidationContext) error {
	if ctx.UserDetails == nil {
		return nil
	}

	// Check if user implements HasPermissionsInterface
	userWithPerms, ok := ctx.UserDetails.(HasPermissionsInterface)
	if !ok {
		return nil // User doesn't implement permission checking
	}

	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name == nil {
				return nil
			}

			requiredPerms, ok := r.config[field.Name.Value]
			if !ok {
				return nil
			}

			for _, perm := range requiredPerms {
				if userWithPerms.HasPermission(perm) {
					return nil
				}
			}

			return r.NewErrorf("field '%s' requires one of: %v", field.Name.Value, requiredPerms)
		},
	}

	return traverseAST(ctx.Document, visitor, ctx)
}

// BlockedFieldsRule blocks specific fields from being queried
type BlockedFieldsRule struct {
	BaseRule
	blockedFields map[string]string // field -> reason
}

// NewBlockedFieldsRule creates a new blocked fields rule
//
// Example:
//   NewBlockedFieldsRule("internalUsers", "deprecatedField")
func NewBlockedFieldsRule(fields ...string) ValidationRule {
	rule := &BlockedFieldsRule{
		BaseRule:      NewBaseRule("BlockedFieldsRule"),
		blockedFields: make(map[string]string),
	}
	for _, field := range fields {
		rule.blockedFields[field] = ""
	}
	return rule
}

// BlockField adds a field to the blocked list with an optional reason
func (r *BlockedFieldsRule) BlockField(field string, reason string) *BlockedFieldsRule {
	r.blockedFields[field] = reason
	return r
}

func (r *BlockedFieldsRule) Validate(ctx *ValidationContext) error {
	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name == nil {
				return nil
			}

			if reason, blocked := r.blockedFields[field.Name.Value]; blocked {
				msg := "field '" + field.Name.Value + "' is blocked"
				if reason != "" {
					msg += ": " + reason
				}
				return r.NewError(msg)
			}

			return nil
		},
	}

	return traverseAST(ctx.Document, visitor, ctx)
}