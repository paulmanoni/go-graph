package graph

import (
	"encoding/json"
	"fmt"

	"github.com/graphql-go/graphql"
)

type QueryField interface {
	Serve() *graphql.Field

	Name() string
}

type MutationField interface {
	Serve() *graphql.Field

	Name() string
}

// GetRootInfo safely extracts a value from p.Info.RootValue and unmarshals it into the target
func GetRootInfo(p graphql.ResolveParams, key string, target interface{}) error {
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

// GetRootString safely extracts a string value from p.Info.RootValue
func GetRootString(p graphql.ResolveParams, key string) (string, error) {
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

// GetArg safely extracts a value from p.Args and unmarshals it into the target
func GetArg(p graphql.ResolveParams, key string, target interface{}) error {
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

// GetArgString safely extracts a string argument from p.Args
func GetArgString(p graphql.ResolveParams, key string) (string, error) {
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

// GetArgInt safely extracts an int argument from p.Args
func GetArgInt(p graphql.ResolveParams, key string) (int, error) {
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

// GetArgBool safely extracts a bool argument from p.Args
func GetArgBool(p graphql.ResolveParams, key string) (bool, error) {
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
