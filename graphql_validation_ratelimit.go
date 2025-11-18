package graph

// RateLimitRule implements per-user rate limiting based on query complexity
type RateLimitRule struct {
	BaseRule
	costPerUnit int
	getBudget   func(userID string) (int, error)
	bypassRoles []string
}

// RateLimitOption configures rate limiting behavior
type RateLimitOption func(*RateLimitRule)

// WithCostPerUnit sets the cost multiplier per complexity unit (default: 1)
func WithCostPerUnit(cost int) RateLimitOption {
	return func(r *RateLimitRule) {
		r.costPerUnit = cost
	}
}

// WithBudgetFunc sets the function to get user's remaining budget
func WithBudgetFunc(fn func(userID string) (int, error)) RateLimitOption {
	return func(r *RateLimitRule) {
		r.getBudget = fn
	}
}

// WithBypassRoles sets roles that bypass rate limiting (e.g., "admin", "service")
func WithBypassRoles(roles ...string) RateLimitOption {
	return func(r *RateLimitRule) {
		r.bypassRoles = roles
	}
}

// NewRateLimitRule creates a new rate limiting rule with optional configuration
//
// Example:
//   NewRateLimitRule(
//       WithBudgetFunc(getBudgetFromRedis),
//       WithCostPerUnit(2),
//       WithBypassRoles("admin", "service"),
//   )
func NewRateLimitRule(opts ...RateLimitOption) ValidationRule {
	rule := &RateLimitRule{
		BaseRule:    NewBaseRule("RateLimitRule"),
		costPerUnit: 1,
		bypassRoles: []string{},
	}

	for _, opt := range opts {
		opt(rule)
	}

	return rule
}

// HasIDInterface - implement this on your user struct for rate limiting
type HasIDInterface interface {
	GetID() string
}

func (r *RateLimitRule) Validate(ctx *ValidationContext) error {
	// Skip if no budget function configured
	if r.getBudget == nil {
		return nil
	}

	// Skip if user not authenticated
	if ctx.UserDetails == nil {
		return nil
	}

	// Get user ID - try to type assert to HasIDInterface
	userWithID, ok := ctx.UserDetails.(HasIDInterface)
	if !ok {
		return r.NewError("rate limiting requires user to implement GetID() method")
	}

	// Check if user has bypass role
	if len(r.bypassRoles) > 0 {
		if userWithRoles, ok := ctx.UserDetails.(HasRolesInterface); ok {
			for _, bypassRole := range r.bypassRoles {
				if userWithRoles.HasRole(bypassRole) {
					return nil
				}
			}
		}
	}

	// Get user's budget
	budget, err := r.getBudget(userWithID.GetID())
	if err != nil {
		return r.NewErrorf("failed to check rate limit: %v", err)
	}

	// Calculate query cost
	complexity := calculateQueryComplexity(ctx.Document, 1)
	cost := complexity * r.costPerUnit

	// Check if cost exceeds budget
	if cost > budget {
		return r.NewErrorf("query cost %d exceeds available budget %d", cost, budget)
	}

	return nil
}

// SimpleBudgetFunc creates a simple budget function that returns a fixed budget
// Useful for testing or simple rate limiting scenarios
func SimpleBudgetFunc(budget int) func(string) (int, error) {
	return func(userID string) (int, error) {
		return budget, nil
	}
}

// PerUserBudgetFunc creates a budget function with per-user budgets
// Useful for different tier users or testing
func PerUserBudgetFunc(budgets map[string]int, defaultBudget int) func(string) (int, error) {
	return func(userID string) (int, error) {
		if budget, ok := budgets[userID]; ok {
			return budget, nil
		}
		return defaultBudget, nil
	}
}