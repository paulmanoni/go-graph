package graph

import (
	"encoding/json"
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

// calculateQueryDepth recursively calculates the maximum depth of a query
func calculateQueryDepth(node ast.Node, currentDepth int) int {
	maxDepth := currentDepth

	switch n := node.(type) {
	case *ast.Document:
		for _, def := range n.Definitions {
			depth := calculateQueryDepth(def, currentDepth)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	case *ast.OperationDefinition:
		if n.SelectionSet != nil {
			depth := calculateSelectionSetDepth(n.SelectionSet, currentDepth)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	case *ast.FragmentDefinition:
		if n.SelectionSet != nil {
			depth := calculateSelectionSetDepth(n.SelectionSet, currentDepth)
			if depth > maxDepth {
				maxDepth = depth
			}
		}
	}

	return maxDepth
}

// calculateSelectionSetDepth calculates depth for a selection set
func calculateSelectionSetDepth(selectionSet *ast.SelectionSet, currentDepth int) int {
	maxDepth := currentDepth

	for _, selection := range selectionSet.Selections {
		var depth int
		switch sel := selection.(type) {
		case *ast.Field:
			if sel.SelectionSet != nil {
				depth = calculateSelectionSetDepth(sel.SelectionSet, currentDepth+1)
			} else {
				depth = currentDepth + 1
			}
		case *ast.InlineFragment:
			if sel.SelectionSet != nil {
				depth = calculateSelectionSetDepth(sel.SelectionSet, currentDepth)
			}
		case *ast.FragmentSpread:
			// Fragment spreads don't increase depth by themselves
			depth = currentDepth
		}

		if depth > maxDepth {
			maxDepth = depth
		}
	}

	return maxDepth
}

// countAliases recursively counts the number of field aliases in a query
func countAliases(node ast.Node) int {
	count := 0

	switch n := node.(type) {
	case *ast.Document:
		for _, def := range n.Definitions {
			count += countAliases(def)
		}
	case *ast.OperationDefinition:
		if n.SelectionSet != nil {
			count += countSelectionSetAliases(n.SelectionSet)
		}
	case *ast.FragmentDefinition:
		if n.SelectionSet != nil {
			count += countSelectionSetAliases(n.SelectionSet)
		}
	}

	return count
}

// countSelectionSetAliases counts aliases in a selection set
func countSelectionSetAliases(selectionSet *ast.SelectionSet) int {
	count := 0

	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			// If the field has an alias, count it
			if sel.Alias != nil && sel.Alias.Value != "" {
				count++
			}
			// Recursively count aliases in nested selections
			if sel.SelectionSet != nil {
				count += countSelectionSetAliases(sel.SelectionSet)
			}
		case *ast.InlineFragment:
			if sel.SelectionSet != nil {
				count += countSelectionSetAliases(sel.SelectionSet)
			}
		case *ast.FragmentSpread:
			// Fragment spreads themselves don't have aliases
			// The aliases will be counted when the fragment definition is processed
		}
	}

	return count
}

// calculateQueryComplexity calculates query complexity based on depth and field count
func calculateQueryComplexity(node ast.Node, multiplier int) int {
	complexity := 0

	switch n := node.(type) {
	case *ast.Document:
		for _, def := range n.Definitions {
			complexity += calculateQueryComplexity(def, multiplier)
		}
	case *ast.OperationDefinition:
		if n.SelectionSet != nil {
			complexity += calculateSelectionSetComplexity(n.SelectionSet, multiplier)
		}
	case *ast.FragmentDefinition:
		if n.SelectionSet != nil {
			complexity += calculateSelectionSetComplexity(n.SelectionSet, multiplier)
		}
	}

	return complexity
}

// calculateSelectionSetComplexity calculates complexity for a selection set
func calculateSelectionSetComplexity(selectionSet *ast.SelectionSet, multiplier int) int {
	complexity := 0

	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			// Base complexity for the field
			complexity += multiplier

			// If field has nested selections, multiply complexity
			if sel.SelectionSet != nil {
				nestedComplexity := calculateSelectionSetComplexity(sel.SelectionSet, multiplier*2)
				complexity += nestedComplexity
			}
		case *ast.InlineFragment:
			if sel.SelectionSet != nil {
				complexity += calculateSelectionSetComplexity(sel.SelectionSet, multiplier)
			}
		case *ast.FragmentSpread:
			// Fragment spreads add base complexity
			complexity += multiplier
		}
	}

	return complexity
}

// ValidateGraphQLQuery validates a GraphQL query against security rules.
// This function implements multiple layers of protection against malicious or expensive queries.
//
// Validation Rules:
//   - Max Query Depth: 10 levels (prevents deeply nested queries)
//   - Max Aliases: 4 per query (prevents alias-based DoS attacks)
//   - Max Complexity: 200 (prevents computationally expensive queries)
//   - Introspection: Blocked (__schema and __type queries are rejected)
//
// Returns an error if:
//   - Query depth exceeds 10 levels
//   - Query contains more than 4 aliases
//   - Query complexity exceeds 200
//   - Query contains __schema or __type introspection fields
//   - Query parsing fails (though parsing errors are allowed to pass through)
//
// Example usage:
//
//	if err := graph.ValidateGraphQLQuery(queryString, schema); err != nil {
//	    // Reject query with HTTP 400
//	    return fmt.Errorf("invalid query: %w", err)
//	}
//	// Query is safe to execute
//
// Enable this in production with GraphContext.EnableValidation = true.
func ValidateGraphQLQuery(queryString string, schema *graphql.Schema) error {
	// Handle empty query
	if queryString == "" {
		return nil
	}

	// Try to parse as JSON (for POST requests with JSON body)
	var queryData map[string]interface{}
	if err := json.Unmarshal([]byte(queryString), &queryData); err == nil {
		if query, ok := queryData["query"].(string); ok {
			queryString = query
		}
	}

	// Parse the query string into an AST
	src := source.NewSource(&source.Source{
		Body: []byte(queryString),
		Name: "GraphQL request",
	})

	doc, err := parser.Parse(parser.ParseParams{
		Source: src,
	})
	if err != nil {
		// If parsing fails, let the GraphQL handler deal with it
		return nil
	}

	// Check for introspection queries (matching Python's NoSchemaIntrospectionCustomRule)
	if hasIntrospection(doc) {
		return fmt.Errorf("GraphQL introspection is disabled")
	}

	// Apply validation rules
	// Limit query depth to 10 (matching Python's QueryDepthLimiter(max_depth=10))
	maxDepth := 10
	depth := calculateQueryDepth(doc, 0)
	if depth > maxDepth {
		return fmt.Errorf("query depth exceeds maximum allowed depth of %d (actual: %d)", maxDepth, depth)
	}

	// Limit max aliases to 10 (matching Python's MaxAliasesLimiter(max_alias_count=10))
	maxAliases := 4
	aliasCount := countAliases(doc)
	if aliasCount > maxAliases {
		return fmt.Errorf("query contains too many aliases. Maximum allowed: %d, found: %d", maxAliases, aliasCount)
	}

	// Optional: Limit query complexity
	maxComplexity := 200
	complexity := calculateQueryComplexity(doc, 1)
	if complexity > maxComplexity {
		return fmt.Errorf("query complexity exceeds maximum allowed complexity of %d (actual: %d)", maxComplexity, complexity)
	}

	return nil
}

// hasIntrospection checks if the query contains introspection fields
func hasIntrospection(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.Document:
		for _, def := range n.Definitions {
			if hasIntrospection(def) {
				return true
			}
		}
	case *ast.OperationDefinition:
		if n.SelectionSet != nil {
			return hasIntrospectionInSelectionSet(n.SelectionSet)
		}
	case *ast.FragmentDefinition:
		if n.SelectionSet != nil {
			return hasIntrospectionInSelectionSet(n.SelectionSet)
		}
	}
	return false
}

// hasIntrospectionInSelectionSet checks for introspection fields in a selection set
func hasIntrospectionInSelectionSet(selectionSet *ast.SelectionSet) bool {
	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			if sel.Name != nil {
				fieldName := sel.Name.Value
				// Block __schema and __type introspection queries
				if fieldName == "__schema" || fieldName == "__type" {
					return true
				}
			}
			// Check nested selections
			if sel.SelectionSet != nil {
				if hasIntrospectionInSelectionSet(sel.SelectionSet) {
					return true
				}
			}
		case *ast.InlineFragment:
			if sel.SelectionSet != nil {
				if hasIntrospectionInSelectionSet(sel.SelectionSet) {
					return true
				}
			}
		}
	}
	return false
}

// ExecuteValidationRules executes a set of validation rules against a GraphQL query.
// This is the modern validation system that supports custom rules.
//
// Parameters:
//   - queryString: The GraphQL query to validate
//   - schema: The GraphQL schema
//   - rules: The validation rules to execute
//   - authCtx: Authentication context (can be nil if not needed)
//   - options: Validation options (can be nil for defaults)
//
// Returns:
//   - nil if validation passes
//   - *ValidationError for single rule failure
//   - *MultiValidationError for multiple rule failures
//
// Example:
//
//	rules := []ValidationRule{
//	    NewMaxDepthRule(10),
//	    NewRequireAuthRule("mutation"),
//	    NewRoleRules(AdminOnlyFields),
//	}
//	err := ExecuteValidationRules(query, schema, rules, authCtx, nil)
func ExecuteValidationRules(
	queryString string,
	schema *graphql.Schema,
	rules []ValidationRule,
	userDetails interface{},
	options *ValidationOptions,
) error {
	// Handle empty query
	if queryString == "" {
		return nil
	}

	// Handle nil rules
	if len(rules) == 0 {
		return nil
	}

	// Set default options
	if options == nil {
		options = &ValidationOptions{
			StopOnFirstError: false,
			SkipInDebug:      true,
		}
	}

	// Parse the query string into an AST
	src := source.NewSource(&source.Source{
		Body: []byte(queryString),
		Name: "GraphQL request",
	})

	doc, err := parser.Parse(parser.ParseParams{
		Source: src,
	})
	if err != nil {
		// If parsing fails, let the GraphQL handler deal with it
		return nil
	}

	// Create validation context
	ctx := &ValidationContext{
		Document:    doc,
		Schema:      schema,
		UserDetails: userDetails,
	}

	// Execute all rules
	var errors []*ValidationError
	for _, rule := range rules {
		// Skip disabled rules
		if !rule.Enabled() {
			continue
		}

		// Execute rule
		if err := rule.Validate(ctx); err != nil {
			if validationErr, ok := err.(*ValidationError); ok {
				errors = append(errors, validationErr)
			} else {
				// Wrap non-ValidationError errors
				errors = append(errors, &ValidationError{
					Rule:    rule.Name(),
					Message: err.Error(),
				})
			}

			// Stop on first error if configured
			if options.StopOnFirstError {
				break
			}
		}
	}

	// Return appropriate error type
	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	} else {
		// Convert []*ValidationError to []error
		errs := make([]error, len(errors))
		for i, e := range errors {
			errs[i] = e
		}
		return &MultiValidationError{Errors: errs}
	}
}
