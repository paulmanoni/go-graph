package graph

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// ValidationRule represents a single validation rule that can be applied to GraphQL queries
type ValidationRule interface {
	// Name returns a unique identifier for this rule
	Name() string

	// Validate executes the rule against the parsed query
	// Returns nil if valid, error if validation fails
	Validate(ctx *ValidationContext) error

	// Enabled checks if this rule should be executed
	Enabled() bool

	// Enable enables the rule
	Enable()

	// Disable disables the rule
	Disable()
}

// BaseRule provides common functionality for all validation rules
// All custom rules should embed this struct
type BaseRule struct {
	name    string
	enabled bool
}

// NewBaseRule creates a new base rule with the given name
func NewBaseRule(name string) BaseRule {
	return BaseRule{
		name:    name,
		enabled: true,
	}
}

func (r *BaseRule) Name() string    { return r.name }
func (r *BaseRule) Enabled() bool   { return r.enabled }
func (r *BaseRule) Enable()         { r.enabled = true }
func (r *BaseRule) Disable()        { r.enabled = false }

// SetEnabled sets the enabled state
func (r *BaseRule) SetEnabled(enabled bool) {
	r.enabled = enabled
}

// NewValidationError creates a validation error for this rule
func (r *BaseRule) NewError(message string) *ValidationError {
	return &ValidationError{
		Rule:    r.name,
		Message: message,
	}
}

// NewErrorf creates a validation error with formatted message
func (r *BaseRule) NewErrorf(format string, args ...interface{}) *ValidationError {
	return &ValidationError{
		Rule:    r.name,
		Message: fmt.Sprintf(format, args...),
	}
}

// ValidationContext provides all necessary information for validation
type ValidationContext struct {
	// GraphQL query components
	Query     string
	Document  *ast.Document
	Schema    *graphql.Schema
	Variables map[string]interface{}

	// Request context
	Request *http.Request

	// User details from UserDetailsFn (can be nil if not authenticated)
	// Validation rules can type-assert this to whatever structure they need
	UserDetails interface{}
}

// ValidationError provides detailed error information
type ValidationError struct {
	Rule     string
	Message  string
	Location *ast.Location
	Path     []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Rule, e.Message)
}

// MultiValidationError combines multiple validation errors
type MultiValidationError struct {
	Errors []error
}

func NewMultiValidationError(errors []error) *MultiValidationError {
	return &MultiValidationError{Errors: errors}
}

func (e *MultiValidationError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("validation failed:\n  - %s", strings.Join(msgs, "\n  - "))
}

// ValidationOptions configures validation behavior
type ValidationOptions struct {
	// StopOnFirstError stops validation after first error
	StopOnFirstError bool

	// SkipInDebug skips validation when DEBUG=true
	SkipInDebug bool
}

// ASTVisitor allows traversing the AST with hooks
type ASTVisitor struct {
	EnterField     func(field *ast.Field, ctx *ValidationContext) error
	LeaveField     func(field *ast.Field, ctx *ValidationContext) error
	EnterOperation func(op *ast.OperationDefinition, ctx *ValidationContext) error
	LeaveOperation func(op *ast.OperationDefinition, ctx *ValidationContext) error
	EnterFragment  func(frag *ast.FragmentDefinition, ctx *ValidationContext) error
	LeaveFragment  func(frag *ast.FragmentDefinition, ctx *ValidationContext) error
}

// traverseAST walks the AST and calls visitor hooks
func traverseAST(node ast.Node, visitor *ASTVisitor, ctx *ValidationContext) error {
	switch n := node.(type) {
	case *ast.Document:
		for _, def := range n.Definitions {
			if err := traverseAST(def, visitor, ctx); err != nil {
				return err
			}
		}

	case *ast.OperationDefinition:
		if visitor.EnterOperation != nil {
			if err := visitor.EnterOperation(n, ctx); err != nil {
				return err
			}
		}
		if n.SelectionSet != nil {
			if err := traverseSelectionSet(n.SelectionSet, visitor, ctx); err != nil {
				return err
			}
		}
		if visitor.LeaveOperation != nil {
			if err := visitor.LeaveOperation(n, ctx); err != nil {
				return err
			}
		}

	case *ast.FragmentDefinition:
		if visitor.EnterFragment != nil {
			if err := visitor.EnterFragment(n, ctx); err != nil {
				return err
			}
		}
		if n.SelectionSet != nil {
			if err := traverseSelectionSet(n.SelectionSet, visitor, ctx); err != nil {
				return err
			}
		}
		if visitor.LeaveFragment != nil {
			if err := visitor.LeaveFragment(n, ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func traverseSelectionSet(selectionSet *ast.SelectionSet, visitor *ASTVisitor, ctx *ValidationContext) error {
	for _, selection := range selectionSet.Selections {
		switch sel := selection.(type) {
		case *ast.Field:
			if visitor.EnterField != nil {
				if err := visitor.EnterField(sel, ctx); err != nil {
					return err
				}
			}
			if sel.SelectionSet != nil {
				if err := traverseSelectionSet(sel.SelectionSet, visitor, ctx); err != nil {
					return err
				}
			}
			if visitor.LeaveField != nil {
				if err := visitor.LeaveField(sel, ctx); err != nil {
					return err
				}
			}

		case *ast.InlineFragment:
			if sel.SelectionSet != nil {
				if err := traverseSelectionSet(sel.SelectionSet, visitor, ctx); err != nil {
					return err
				}
			}
		}
	}
	return nil
}