package graph

// This file contains examples of how to create custom validation rules using BaseRule

/*
Example 1: Simple custom rule

type MyCustomRule struct {
	BaseRule
	maxValue int
}

func NewMyCustomRule(maxValue int) ValidationRule {
	return &MyCustomRule{
		BaseRule: NewBaseRule("MyCustomRule"),
		maxValue: maxValue,
	}
}

func (r *MyCustomRule) Validate(ctx *ValidationContext) error {
	// Your validation logic here
	if someCondition {
		return r.NewError("simple error message")
	}

	if anotherCondition {
		return r.NewErrorf("formatted error: value=%d, max=%d", value, r.maxValue)
	}

	return nil
}

Example 2: Rule that uses AST visitor

type BlockMutationRule struct {
	BaseRule
	blockedMutations map[string]bool
}

func NewBlockMutationRule(mutations ...string) ValidationRule {
	rule := &BlockMutationRule{
		BaseRule:         NewBaseRule("BlockMutationRule"),
		blockedMutations: make(map[string]bool),
	}
	for _, m := range mutations {
		rule.blockedMutations[m] = true
	}
	return rule
}

func (r *BlockMutationRule) Validate(ctx *ValidationContext) error {
	visitor := &ASTVisitor{
		EnterField: func(field *ast.Field, vctx *ValidationContext) error {
			if field.Name != nil && r.blockedMutations[field.Name.Value] {
				return r.NewErrorf("mutation '%s' is blocked", field.Name.Value)
			}
			return nil
		},
	}
	return traverseAST(ctx.Document, visitor, ctx)
}

Example 3: Rule that checks authentication context

type RequireEmailVerifiedRule struct {
	BaseRule
}

func NewRequireEmailVerifiedRule() ValidationRule {
	return &RequireEmailVerifiedRule{
		BaseRule: NewBaseRule("RequireEmailVerifiedRule"),
	}
}

func (r *RequireEmailVerifiedRule) Validate(ctx *ValidationContext) error {
	if ctx.Auth == nil || !ctx.Auth.Authenticated {
		return r.NewError("authentication required")
	}

	// Check custom claim
	emailVerified, ok := ctx.Auth.Claims["email_verified"].(bool)
	if !ok || !emailVerified {
		return r.NewError("email verification required")
	}

	return nil
}

Example 4: Conditional rule

type BusinessHoursOnlyRule struct {
	BaseRule
	startHour int
	endHour   int
}

func NewBusinessHoursOnlyRule(start, end int) ValidationRule {
	return &BusinessHoursOnlyRule{
		BaseRule:  NewBaseRule("BusinessHoursOnlyRule"),
		startHour: start,
		endHour:   end,
	}
}

func (r *BusinessHoursOnlyRule) Validate(ctx *ValidationContext) error {
	now := time.Now()
	hour := now.Hour()

	if hour < r.startHour || hour >= r.endHour {
		return r.NewErrorf("API only available during business hours (%d:00-%d:00)", r.startHour, r.endHour)
	}

	return nil
}

Example 5: Using Enable/Disable

rule := NewMyCustomRule(100)

// Disable rule conditionally
if env == "development" {
	rule.Disable()
}

// Re-enable later
rule.Enable()

// Check if enabled
if rule.Enabled() {
	// Will be validated
}
*/