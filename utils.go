package graph

import (
	"encoding/json"
	"fmt"

	"github.com/graphql-go/graphql"
)

// QueryField represents a GraphQL query field with its configuration.
// Implementations must provide both the field configuration and its name.
//
// Use NewResolver to create QueryField instances:
//
//	query := graph.NewResolver[User]("user").
//	    WithArgs(...).
//	    WithResolver(...).
//	    BuildQuery()
type QueryField interface {
	// Serve returns the GraphQL field configuration
	Serve() *graphql.Field

	// Name returns the field name used in the GraphQL schema
	Name() string
}

// MutationField represents a GraphQL mutation field with its configuration.
// Implementations must provide both the field configuration and its name.
//
// Use NewResolver to create MutationField instances:
//
//	mutation := graph.NewResolver[User]("createUser").
//	    WithInputObject(CreateUserInput{}).
//	    WithResolver(...).
//	    BuildMutation()
type MutationField interface {
	// Serve returns the GraphQL field configuration
	Serve() *graphql.Field

	// Name returns the field name used in the GraphQL schema
	Name() string
}

// GetRootInfo safely extracts a value from p.Info.RootValue and unmarshals it into the target.
// This is commonly used to retrieve user details set by UserDetailsFn in the GraphContext.
//
// The function handles:
//   - Primitive types (string, int) with optimized direct assignment
//   - Complex types using JSON marshaling/unmarshaling for type conversion
//   - Type mismatches with descriptive error messages
//
// Returns an error if:
//   - Root value is nil or not a map
//   - The key doesn't exist in the root value
//   - Type conversion fails
//
// Example:
//
//	// In your resolver
//	var user UserDetails
//	if err := graph.GetRootInfo(p, "details", &user); err != nil {
//	    return nil, fmt.Errorf("authentication required")
//	}
//	// Use user.ID, user.Email, etc.
func GetRootInfo(p ResolveParams, key string, target interface{}) error {
	if p.Info.RootValue == nil {
		return fmt.Errorf("root value is nil")
	}

	rootMap, ok := p.Info.RootValue.(map[string]interface{})
	if !ok {
		return fmt.Errorf("root value is not a map")
	}

	value, exists := rootMap[key]
	if !exists {
		return fmt.Errorf("key '%s' not found in root value", key)
	}

	// If the target is a pointer to a string and value is already a string
	if strPtr, ok := target.(*string); ok {
		if str, ok := value.(string); ok {
			*strPtr = str
			return nil
		}
	}

	// If the target is a pointer to an int and value is already an int
	if intPtr, ok := target.(*int); ok {
		if i, ok := value.(int); ok {
			*intPtr = i
			return nil
		}
		if f, ok := value.(float64); ok {
			*intPtr = int(f)
			return nil
		}
	}

	// For complex types, use JSON marshaling/unmarshaling
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal value into target: %w", err)
	}

	return nil
}

// GetRootString safely extracts a string value from p.Info.RootValue.
// This is commonly used to retrieve the authentication token.
//
// Returns an error if:
//   - Root value is nil or not a map
//   - The key doesn't exist in the root value
//   - The value is not a string
//
// Example:
//
//	// Get authentication token
//	token, err := graph.GetRootString(p, "token")
//	if err != nil {
//	    return nil, fmt.Errorf("authentication required")
//	}
//	// Validate token...
func GetRootString(p ResolveParams, key string) (string, error) {
	if p.Info.RootValue == nil {
		return "", fmt.Errorf("root value is nil")
	}

	rootMap, ok := p.Info.RootValue.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("root value is not a map")
	}

	value, exists := rootMap[key]
	if !exists {
		return "", fmt.Errorf("key '%s' not found in root value", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value for key '%s' is not a string", key)
	}

	return str, nil
}

// GetArg safely extracts a value from p.Args and unmarshals it into the target.
// This is useful for extracting complex types like structs, slices, or maps.
//
// The function handles:
//   - Primitive types (string, int, bool) with optimized direct assignment
//   - Complex types using JSON marshaling/unmarshaling for type conversion
//   - Type mismatches with descriptive error messages
//
// Returns an error if:
//   - The argument key doesn't exist
//   - Type conversion fails
//
// Example:
//
//	var input CreateUserInput
//	if err := graph.GetArg(p, "input", &input); err != nil {
//	    return nil, err
//	}
//	// Use input.Name, input.Email, etc.
func GetArg(p ResolveParams, key string, target interface{}) error {
	value, exists := p.Args[key]
	if !exists {
		return fmt.Errorf("argument '%s' not found", key)
	}

	// If the target is a pointer to a string and value is already a string
	if strPtr, ok := target.(*string); ok {
		if str, ok := value.(string); ok {
			*strPtr = str
			return nil
		}
	}

	// If the target is a pointer to an int and value is already an int
	if intPtr, ok := target.(*int); ok {
		if i, ok := value.(int); ok {
			*intPtr = i
			return nil
		}
		if f, ok := value.(float64); ok {
			*intPtr = int(f)
			return nil
		}
	}

	// If the target is a pointer to a bool and value is already a bool
	if boolPtr, ok := target.(*bool); ok {
		if b, ok := value.(bool); ok {
			*boolPtr = b
			return nil
		}
	}

	// For complex types, use JSON marshaling/unmarshaling
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal argument: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal argument into target: %w", err)
	}

	return nil
}

// GetArgString safely extracts a string argument from p.Args.
// Returns an error if the argument doesn't exist or is not a string.
//
// Example:
//
//	name, err := graph.GetArgString(p, "name")
func GetArgString(p ResolveParams, key string) (string, error) {
	value, exists := p.Args[key]
	if !exists {
		return "", fmt.Errorf("argument '%s' not found", key)
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("argument '%s' is not a string", key)
	}

	return str, nil
}

// GetArgInt safely extracts an int argument from p.Args.
// Handles both int and float64 types (JSON numbers are parsed as float64).
// Returns an error if the argument doesn't exist or is not a number.
//
// Example:
//
//	age, err := graph.GetArgInt(p, "age")
func GetArgInt(p ResolveParams, key string) (int, error) {
	value, exists := p.Args[key]
	if !exists {
		return 0, fmt.Errorf("argument '%s' not found", key)
	}

	// Handle both int and float64 (JSON numbers are parsed as float64)
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("argument '%s' is not a number", key)
	}
}

// GetArgBool safely extracts a bool argument from p.Args.
// Returns an error if the argument doesn't exist or is not a boolean.
//
// Example:
//
//	active, err := graph.GetArgBool(p, "active")
func GetArgBool(p ResolveParams, key string) (bool, error) {
	value, exists := p.Args[key]
	if !exists {
		return false, fmt.Errorf("argument '%s' not found", key)
	}

	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("argument '%s' is not a boolean", key)
	}

	return b, nil
}
