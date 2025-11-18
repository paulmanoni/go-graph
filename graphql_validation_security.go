package graph

import (
	"strings"
)

// MaxDepthRule validates maximum query depth
type MaxDepthRule struct {
	BaseRule
	maxDepth int
}

// NewMaxDepthRule creates a new max depth validation rule
func NewMaxDepthRule(maxDepth int) ValidationRule {
	return &MaxDepthRule{
		BaseRule: NewBaseRule("MaxDepthRule"),
		maxDepth: maxDepth,
	}
}

func (r *MaxDepthRule) Validate(ctx *ValidationContext) error {
	depth := calculateQueryDepth(ctx.Document, 0)
	if depth > r.maxDepth {
		return r.NewErrorf("query depth %d exceeds maximum %d", depth, r.maxDepth)
	}
	return nil
}

// MaxComplexityRule validates query complexity
type MaxComplexityRule struct {
	BaseRule
	maxComplexity int
}

// NewMaxComplexityRule creates a new max complexity validation rule
func NewMaxComplexityRule(maxComplexity int) ValidationRule {
	return &MaxComplexityRule{
		BaseRule:      NewBaseRule("MaxComplexityRule"),
		maxComplexity: maxComplexity,
	}
}

func (r *MaxComplexityRule) Validate(ctx *ValidationContext) error {
	complexity := calculateQueryComplexity(ctx.Document, 1)
	if complexity > r.maxComplexity {
		return r.NewErrorf("query complexity %d exceeds maximum %d", complexity, r.maxComplexity)
	}
	return nil
}

// MaxAliasesRule validates number of aliases
type MaxAliasesRule struct {
	BaseRule
	maxAliases int
}

// NewMaxAliasesRule creates a new max aliases validation rule
func NewMaxAliasesRule(maxAliases int) ValidationRule {
	return &MaxAliasesRule{
		BaseRule:   NewBaseRule("MaxAliasesRule"),
		maxAliases: maxAliases,
	}
}

func (r *MaxAliasesRule) Validate(ctx *ValidationContext) error {
	count := countAliases(ctx.Document)
	if count > r.maxAliases {
		return r.NewErrorf("query contains %d aliases, maximum %d allowed", count, r.maxAliases)
	}
	return nil
}

// NoIntrospectionRule blocks introspection queries
type NoIntrospectionRule struct {
	BaseRule
}

// NewNoIntrospectionRule creates a new no introspection validation rule
func NewNoIntrospectionRule() ValidationRule {
	return &NoIntrospectionRule{
		BaseRule: NewBaseRule("NoIntrospectionRule"),
	}
}

func (r *NoIntrospectionRule) Validate(ctx *ValidationContext) error {
	if hasIntrospection(ctx.Document) {
		return r.NewError("GraphQL introspection is disabled")
	}
	return nil
}

// MaxTokensRule limits query size by token count
type MaxTokensRule struct {
	BaseRule
	maxTokens int
}

// NewMaxTokensRule creates a new max tokens validation rule
func NewMaxTokensRule(maxTokens int) ValidationRule {
	return &MaxTokensRule{
		BaseRule:  NewBaseRule("MaxTokensRule"),
		maxTokens: maxTokens,
	}
}

func (r *MaxTokensRule) Validate(ctx *ValidationContext) error {
	tokens := len(strings.Fields(ctx.Query))
	if tokens > r.maxTokens {
		return r.NewErrorf("query contains %d tokens, maximum %d allowed", tokens, r.maxTokens)
	}
	return nil
}