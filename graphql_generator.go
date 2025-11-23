package graph

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
)

// Global processing types to prevent infinite recursion across all generators
var (
	globalProcessingTypes   = make(map[reflect.Type]bool)
	globalProcessingTypesMu sync.RWMutex
	// Object type registry to avoid duplicate type creation
	objectTypeRegistry   = make(map[string]*graphql.Object)
	objectTypeRegistryMu sync.RWMutex
)

type FieldGenerator[T any] struct {
	typeCache       map[reflect.Type]graphql.Output
	processingTypes map[reflect.Type]bool
	objectTypeName  *string
}

func NewFieldGenerator[T any]() *FieldGenerator[T] {
	return &FieldGenerator[T]{
		typeCache:       make(map[reflect.Type]graphql.Output),
		processingTypes: make(map[reflect.Type]bool),
	}
}

func GenerateGraphQLFields[T any]() graphql.Fields {
	gen := NewFieldGenerator[T]()
	var instance T
	return gen.generateFields(reflect.TypeOf(instance))
}

func GenerateGraphQLObject[T any](name string) *graphql.Object {
	gen := NewFieldGenerator[T]()
	var instance T
	fields := gen.generateFields(reflect.TypeOf(instance))

	return graphql.NewObject(graphql.ObjectConfig{
		Name:   name,
		Fields: fields,
	})
}

func (g *FieldGenerator[T]) generateFields(t reflect.Type) graphql.Fields {

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return graphql.Fields{}
	}

	fields := graphql.Fields{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded (anonymous) fields by flattening them
		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			// Recursively get fields from embedded struct
			embeddedFields := g.generateFields(embeddedType)
			for name, embeddedField := range embeddedFields {
				// Only add if not already present (child fields take precedence)
				if _, exists := fields[name]; !exists {
					fields[name] = embeddedField
				}
			}
			continue
		}

		if field.PkgPath != "" {
			continue
		}

		fieldName := g.getFieldName(field)
		if fieldName == "-" {
			continue
		}
		graphqlType := g.getGraphQLType(field.Type, field)
		if graphqlType == nil {
			continue
		}

		description := field.Tag.Get("description")
		fields[fieldName] = &graphql.Field{
			Type:        graphqlType,
			Description: description,
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				source := reflect.ValueOf(p.Source)
				if source.Kind() == reflect.Ptr {
					source = source.Elem()
				}

				if source.Kind() != reflect.Struct {
					return nil, fmt.Errorf("expected struct, got %v", source.Kind())
				}

				fieldValue := source.FieldByName(field.Name)
				if !fieldValue.IsValid() {
					return nil, nil
				}

				return fieldValue.Interface(), nil
			},
		}
	}

	return fields
}

func (g *FieldGenerator[T]) getGraphQLType(t reflect.Type, field reflect.StructField) graphql.Output {
	isRequired := strings.Contains(field.Tag.Get("graphql"), "required")

	baseType := g.getBaseGraphQLType(t, g.objectTypeName)

	if baseType == nil {
		return nil
	}

	if isRequired {
		return graphql.NewNonNull(baseType)
	}

	return baseType
}

func (g *FieldGenerator[T]) getBaseGraphQLType(t reflect.Type, objectTypeName *string) graphql.Output {
	g.objectTypeName = objectTypeName
	switch t.Kind() {
	case reflect.Ptr:
		return g.getBaseGraphQLType(t.Elem(), objectTypeName)

	case reflect.String:
		return graphql.String

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return graphql.Int

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return graphql.Int

	case reflect.Float32, reflect.Float64:
		return graphql.Float

	case reflect.Bool:
		return graphql.Boolean

	case reflect.Slice, reflect.Array:
		elemType := g.getBaseGraphQLType(t.Elem(), objectTypeName)
		if elemType == nil {
			return nil
		}
		return graphql.NewList(elemType)

	case reflect.Map:
		return graphql.NewScalar(graphql.ScalarConfig{
			Name: fmt.Sprintf("Map_%s", t.String()),
			Serialize: func(value interface{}) interface{} {
				return value
			},
		})

	case reflect.Struct:
		if t == reflect.TypeOf(time.Time{}) {
			return DateTime
		} else if t == reflect.TypeOf(JSONTime{}) {
			return DateTime
		}
		nameObject := ""
		if g.objectTypeName != nil {
			nameObject = fmt.Sprintf("%s_%s", *g.objectTypeName, t.Name())
		} else {
			nameObject = t.Name()
		}
		if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			elemType := g.getBaseGraphQLType(t.Elem(), objectTypeName)
			if elemType == nil {
				return nil
			}
			return graphql.NewList(elemType)
		} else {
			// Check if object type already exists in the registry
			objectTypeRegistryMu.RLock()
			if existingType, exists := objectTypeRegistry[nameObject]; exists {
				objectTypeRegistryMu.RUnlock()
				return existingType
			}
			objectTypeRegistryMu.RUnlock()

			// Create new object type
			objectTypeRegistryMu.Lock()
			defer objectTypeRegistryMu.Unlock()

			// Double-check in case another goroutine created it
			if existingType, exists := objectTypeRegistry[nameObject]; exists {
				return existingType
			}

			newObjectType := graphql.NewObject(graphql.ObjectConfig{
				Name: nameObject,
				Fields: (graphql.FieldsThunk)(func() graphql.Fields {
					fields := g.generateFields(t)
					if len(fields) == 0 {
						// Add a placeholder field if no fields generated
						fields = graphql.Fields{
							"id": &graphql.Field{
								Type:        graphql.String,
								Description: "Placeholder field for " + nameObject,
							},
						}
					}
					return fields
				}),
			})

			// Register the new object type
			objectTypeRegistry[nameObject] = newObjectType
			return newObjectType
		}
	case reflect.Interface:
		return graphql.NewScalar(graphql.ScalarConfig{
			Name: "Interface",
			Serialize: func(value interface{}) interface{} {
				return value
			},
		})

	default:
		return nil
	}
}

func (g *FieldGenerator[T]) getFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}

	graphqlTag := field.Tag.Get("graphql")
	if graphqlTag != "" {
		parts := strings.Split(graphqlTag, ",")
		for _, part := range parts {
			if !strings.Contains(part, "=") && part != "required" {
				return part
			}
		}
	}

	return g.toGraphQLFieldName(field.Name)
}

func (g *FieldGenerator[T]) toGraphQLFieldName(name string) string {
	if name == "" {
		return ""
	}

	runes := []rune(name)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

func GenerateInputObject[T any](name string) *graphql.InputObject {
	gen := NewFieldGenerator[T]()
	var instance T
	fields := gen.generateInputFields(reflect.TypeOf(instance))

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   name,
		Fields: fields,
	})
}

func (g *FieldGenerator[T]) generateInputFields(t reflect.Type) graphql.InputObjectConfigFieldMap {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return graphql.InputObjectConfigFieldMap{}
	}

	fields := graphql.InputObjectConfigFieldMap{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded (anonymous) fields by flattening them
		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			// Recursively get fields from embedded struct
			embeddedFields := g.generateInputFields(embeddedType)
			for name, embeddedField := range embeddedFields {
				// Only add if not already present (child fields take precedence)
				if _, exists := fields[name]; !exists {
					fields[name] = embeddedField
				}
			}
			continue
		}

		if field.PkgPath != "" {
			continue
		}

		fieldName := g.getFieldName(field)
		if fieldName == "-" {
			continue
		}

		graphqlType := g.getInputType(field.Type, field)
		if graphqlType == nil {
			continue
		}

		description := field.Tag.Get("description")
		defaultValue := field.Tag.Get("default")

		fieldConfig := &graphql.InputObjectFieldConfig{
			Type:        graphqlType,
			Description: description,
		}

		if defaultValue != "" {
			fieldConfig.DefaultValue = defaultValue
		}

		fields[fieldName] = fieldConfig
	}

	return fields
}

func (g *FieldGenerator[T]) getInputType(t reflect.Type, field reflect.StructField) graphql.Input {
	isRequired := strings.Contains(field.Tag.Get("graphql"), "required")

	baseType := g.getBaseInputType(t, field.Name)

	if baseType == nil {
		return nil
	}

	if isRequired {
		return graphql.NewNonNull(baseType)
	}

	return baseType
}

func (g *FieldGenerator[T]) getInputTypeWithContext(t reflect.Type, field reflect.StructField, parentTypeName string) graphql.Input {
	isRequired := strings.Contains(field.Tag.Get("graphql"), "required")

	baseType := g.getBaseInputTypeWithContext(t, field.Name, parentTypeName)

	if baseType == nil {
		return nil
	}

	if isRequired {
		return graphql.NewNonNull(baseType)
	}

	return baseType
}

func (g *FieldGenerator[T]) getBaseInputType(t reflect.Type, fieldName string) graphql.Input {
	return g.getBaseInputTypeWithContext(t, fieldName, "")
}

func (g *FieldGenerator[T]) getBaseInputTypeWithContext(t reflect.Type, fieldName string, parentTypeName string) graphql.Input {
	switch t.Kind() {
	case reflect.Ptr:
		return g.getBaseInputTypeWithContext(t.Elem(), fieldName, parentTypeName)

	case reflect.String:
		return graphql.String

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return graphql.Int

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return graphql.Int

	case reflect.Float32, reflect.Float64:
		return graphql.Float

	case reflect.Bool:
		return graphql.Boolean

	case reflect.Slice, reflect.Array:
		elemType := g.getBaseInputTypeWithContext(t.Elem(), fieldName, parentTypeName)
		if elemType == nil {
			return nil
		}
		return graphql.NewList(elemType)

	case reflect.Struct:
		// Use parent type name for anonymous structs, otherwise use the field name
		var inputTypeName string
		if t.Name() == "" && parentTypeName != "" {
			// Anonymous struct - use parent type name
			inputTypeName = parentTypeName + "Input"
		} else {
			// Named struct - use getInputTypeName
			inputTypeName = getInputTypeName(t, fieldName)
		}

		// Check if input type already exists in the global registry (from unified resolver)
		inputTypeRegistryMu.RLock()
		if existingType, exists := inputTypeRegistry[inputTypeName]; exists {
			inputTypeRegistryMu.RUnlock()
			return existingType
		}
		inputTypeRegistryMu.RUnlock()

		// Create new input type
		inputTypeRegistryMu.Lock()
		defer inputTypeRegistryMu.Unlock()

		// Double-check in case another goroutine created it
		if existingType, exists := inputTypeRegistry[inputTypeName]; exists {
			return existingType
		}

		newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
			Name: inputTypeName,
			Fields: (graphql.InputObjectConfigFieldMapThunk)(func() graphql.InputObjectConfigFieldMap {
				return g.generateInputFields(t)
			}),
		})

		// Register the new input type
		inputTypeRegistry[inputTypeName] = newInputType

		return newInputType

	default:
		return nil
	}
}

type FieldConfig struct {
	Resolver          graphql.FieldResolveFn
	Description       string
	Args              graphql.FieldConfigArgument
	DeprecationReason string
}

func GenerateArgsFromStruct[T any]() graphql.FieldConfigArgument {
	gen := NewFieldGenerator[T]()
	var instance T
	t := reflect.TypeOf(instance)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return graphql.FieldConfigArgument{}
	}

	args := graphql.FieldConfigArgument{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded (anonymous) fields by flattening them
		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			// Recursively process embedded struct fields
			embeddedGen := NewFieldGenerator[T]()
			embeddedArgs := processStructArgs(embeddedGen, embeddedType)
			for name, embeddedArg := range embeddedArgs {
				// Only add if not already present (child fields take precedence)
				if _, exists := args[name]; !exists {
					args[name] = embeddedArg
				}
			}
			continue
		}

		if field.PkgPath != "" {
			continue
		}

		fieldName := gen.getFieldName(field)
		if fieldName == "-" {
			continue
		}

		graphqlType := gen.getInputType(field.Type, field)
		if graphqlType == nil {
			continue
		}

		description := field.Tag.Get("description")
		defaultValue := field.Tag.Get("default")

		argConfig := &graphql.ArgumentConfig{
			Type:        graphqlType,
			Description: description,
		}

		if defaultValue != "" {
			argConfig.DefaultValue = defaultValue
		}

		args[fieldName] = argConfig
	}

	return args
}

// Helper function to process struct fields for args
func processStructArgs[T any](gen *FieldGenerator[T], t reflect.Type) graphql.FieldConfigArgument {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return graphql.FieldConfigArgument{}
	}

	args := graphql.FieldConfigArgument{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded fields recursively
		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}

			embeddedArgs := processStructArgs(gen, embeddedType)
			for name, embeddedArg := range embeddedArgs {
				if _, exists := args[name]; !exists {
					args[name] = embeddedArg
				}
			}
			continue
		}

		if field.PkgPath != "" {
			continue
		}

		fieldName := gen.getFieldName(field)
		if fieldName == "-" {
			continue
		}

		graphqlType := gen.getInputType(field.Type, field)
		if graphqlType == nil {
			continue
		}

		description := field.Tag.Get("description")
		defaultValue := field.Tag.Get("default")

		argConfig := &graphql.ArgumentConfig{
			Type:        graphqlType,
			Description: description,
		}

		if defaultValue != "" {
			argConfig.DefaultValue = defaultValue
		}

		args[fieldName] = argConfig
	}

	return args
}

// isWrapperType detects if a type is a wrapper like Response[T] that should be handled specially
func (g *FieldGenerator[T]) isWrapperType(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	// Check for Response-like pattern: has Status, Code, Data fields
	hasStatus := false
	hasData := false
	hasCode := false

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		switch field.Name {
		case "Status":
			hasStatus = true
		case "Code":
			hasCode = true
		case "Data":
			hasData = true
		}
	}

	return hasStatus && hasData && hasCode
}

// createWrapperObject creates a GraphQL object for wrapper types with safe field handling
func (g *FieldGenerator[T]) createWrapperObject(t reflect.Type, typeName string) *graphql.Object {
	return graphql.NewObject(graphql.ObjectConfig{
		Name: typeName,
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			fields := graphql.Fields{}

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)

				if field.PkgPath != "" { // Skip unexported fields
					continue
				}

				fieldName := g.getFieldName(field)
				if fieldName == "-" {
					continue
				}

				// Handle wrapper fields differently to prevent deep recursion
				var graphqlType graphql.Output
				dataType := field.Type
				graphqlType = g.getBaseGraphQLType(dataType, &typeName)

				description := field.Tag.Get("description")
				fields[fieldName] = &graphql.Field{
					Type:        graphqlType,
					Description: description,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						source := reflect.ValueOf(p.Source)
						if source.Kind() == reflect.Ptr {
							source = source.Elem()
						}

						if source.Kind() == reflect.Struct {
							fieldValue := source.FieldByName(field.Name)
							if fieldValue.IsValid() && fieldValue.CanInterface() {
								return fieldValue.Interface(), nil
							}
						}
						return nil, nil
					},
				}
			}

			return fields
		}),
	})
}
