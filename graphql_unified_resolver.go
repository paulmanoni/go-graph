package graph

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/mitchellh/mapstructure"
)

// Global type registry to prevent duplicate type creation
var (
	typeRegistry        = make(map[string]*graphql.Object)
	typeRegistryMu      sync.RWMutex
	inputTypeRegistry   = make(map[string]*graphql.InputObject)
	inputTypeRegistryMu sync.RWMutex
)

// RegisterObjectType registers a GraphQL object type in the global registry
// Returns existing type if already registered, otherwise creates and registers new type
func RegisterObjectType(name string, typeFactory func() *graphql.Object) *graphql.Object {
	typeRegistryMu.RLock()
	if existingType, exists := typeRegistry[name]; exists {
		typeRegistryMu.RUnlock()
		return existingType
	}
	typeRegistryMu.RUnlock()

	// Create new type
	typeRegistryMu.Lock()
	defer typeRegistryMu.Unlock()

	// Double-check in case another goroutine created it
	if existingType, exists := typeRegistry[name]; exists {
		return existingType
	}

	newType := typeFactory()
	typeRegistry[name] = newType
	return newType
}

// PaginatedResponse represents a paginated response structure
type PaginatedResponse[T any] struct {
	Items      []T      `json:"items" description:"List of items"`
	TotalCount int      `json:"totalCount" description:"Total number of items"`
	PageInfo   PageInfo `json:"pageInfo" description:"Pagination information"`
}

// PageInfo contains pagination information
type PageInfo struct {
	HasNextPage     bool   `json:"hasNextPage" description:"Whether there are more pages"`
	HasPreviousPage bool   `json:"hasPreviousPage" description:"Whether there are previous pages"`
	StartCursor     string `json:"startCursor" description:"Cursor for the first item"`
	EndCursor       string `json:"endCursor" description:"Cursor for the last item"`
}

// PaginationArgs contains pagination arguments
type PaginationArgs struct {
	First  *int    `json:"first" description:"Number of items to fetch"`
	After  *string `json:"after" description:"Cursor to start after"`
	Last   *int    `json:"last" description:"Number of items to fetch from end"`
	Before *string `json:"before" description:"Cursor to start before"`
}

// UnifiedResolver handles all GraphQL resolver scenarios with field-level customization
type UnifiedResolver[T any] struct {
	name                   string
	description            string
	args                   graphql.FieldConfigArgument
	resolver               graphql.FieldResolveFn
	objectName             string
	isList                 bool
	isListManuallyAssigned bool
	isPaginated            bool
	isMutation             bool
	fieldOverrides         map[string]graphql.FieldResolveFn
	fieldMiddleware        map[string][]FieldMiddleware
	customFields           graphql.Fields
	inputType              interface{}
	useInputObject         bool
	nullableInput          bool
	inputName              string
	resolverMiddlewares    []FieldMiddleware // Middleware stack applied to the main resolver
}

// FieldMiddleware wraps a field resolver with additional functionality (auth, logging, caching, etc.)
type FieldMiddleware func(next FieldResolveFn) FieldResolveFn

// NewResolver creates a unified resolver for all GraphQL operations (queries, mutations, lists, pagination).
// This is the main entry point for creating GraphQL resolvers with extensive customization capabilities.
//
// Type Parameters:
//   - T: The Go struct type that will be converted to GraphQL type
//
// Parameters:
//   - name: The GraphQL field name (e.g., "user", "users", "createUser")
//   - objectName: The GraphQL type name (e.g., "User", "Product")
//
// Basic Usage Examples:
//
//	// Single item query
//	NewResolver[User]("user", "User").
//		WithArgs(graphql.FieldConfigArgument{
//			"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
//		}).
//		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//			return userService.GetByID(p.Args["id"].(int))
//		}).
//		BuildQuery()
//
//	// List query
//	NewResolver[User]("users", "User").
//		AsList().
//		WithArgsFromStruct(UserFilter{}).
//		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//			return userService.List(extractFilters(p.Args))
//		}).
//		BuildQuery()
//
//	// Paginated query
//	NewResolver[User]("users", "User").
//		AsPaginated().
//		WithArgsFromStruct(struct{ PaginationArgs; UserFilter }{}).
//		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//			return userService.ListPaginated(extractArgs(p.Args))
//		}).
//		BuildQuery()
//
//	// Mutation with input object
//	NewResolver[User]("createUser", "User").
//		AsMutation().
//		WithInputObject(CreateUserInput{}).
//		WithResolver(func(p graphql.ResolveParams) (interface{}, error) {
//			input := p.Args["input"].(map[string]interface{})
//			return userService.Create(parseInput(input))
//		}).
//		BuildMutation()
//
// Field-Level Customization Examples:
//
//	// Override specific field resolvers
//	NewResolver[Product]("product", "Product").
//		WithFieldResolver("price", func(p graphql.ResolveParams) (interface{}, error) {
//			product := p.Source.(Product)
//			// Apply user-specific discount
//			if userRole := p.Context.Value("userRole").(string); userRole == "premium" {
//				return product.Price * 0.9, nil
//			}
//			return product.Price, nil
//		}).
//		BuildQuery()
//
//	// Add computed fields that don't exist in struct
//	NewResolver[Product]("product", "Product").
//		WithComputedField("isAvailable", graphql.Boolean, func(p graphql.ResolveParams) (interface{}, error) {
//			product := p.Source.(Product)
//			return product.Stock > 0, nil
//		}).
//		BuildQuery()
//
//	// Add field middleware for cross-cutting concerns
//	NewResolver[Product]("product", "Product").
//		WithFieldMiddleware("price", LoggingMiddleware).
//		WithFieldMiddleware("price", AuthMiddleware("user")).
//		BuildQuery()
//
//	// Performance optimizations
//	NewResolver[Product]("product", "Product").
//		WithLazyField("reviews", func(source interface{}) (interface{}, error) {
//			product := source.(Product)
//			return loadReviews(product.ID) // Only loads when requested
//		}).
//		WithCachedField("category", func(p graphql.ResolveParams) string {
//			product := p.Source.(Product)
//			return fmt.Sprintf("cat_%s", product.CategoryID)
//		}, func(p graphql.ResolveParams) (interface{}, error) {
//			return loadExpensiveCategory(product.CategoryID)
//		}).
//		WithAsyncField("recommendations", func(p graphql.ResolveParams) (interface{}, error) {
//			return loadRecommendationsAsync(product.ID) // Loads in background
//		}).
//		BuildQuery()
//
// Method Chaining:
// The resolver uses a fluent builder pattern. You can chain multiple configuration methods:
//
//	NewResolver[T](name, objectName).
//		AsList().                              // Configure as list
//		WithDescription("Description").        // Add description
//		WithArgsFromStruct(FilterStruct{}).    // Auto-generate args from struct
//		WithFieldResolver("field", resolver).  // Override field resolver
//		WithComputedField("computed", type, resolver). // Add computed field
//		WithFieldMiddleware("field", middleware).      // Add field middleware
//		WithResolver(mainResolver).            // Set main resolver
//		BuildQuery()                           // Build and return QueryField
//
// Available Configuration Methods:
//   - AsList() - Configure as list query (returns []T)
//   - AsPaginated() - Configure as paginated query (returns PaginatedResponse[T])
//   - AsMutation() - Configure as mutation
//   - WithDescription(string) - Add field description
//   - WithArgs(graphql.FieldConfigArgument) - Set custom arguments
//   - WithArgsFromStruct(interface{}) - Auto-generate args from struct
//   - WithResolver(graphql.FieldResolveFn) - Set main resolver function
//   - WithTypedResolver(interface{}) - Set typed resolver with direct struct parameters
//   - WithFieldResolver(fieldName, resolver) - Override specific field resolver
//   - WithFieldResolvers(map[string]graphql.FieldResolveFn) - Override multiple fields
//   - WithFieldMiddleware(fieldName, middleware) - Add field middleware
//   - WithCustomField(name, *graphql.Field) - Add completely custom field
//   - WithComputedField(name, type, resolver) - Add computed field
//   - WithLazyField(fieldName, loader) - Add lazy-loaded field
//   - WithCachedField(fieldName, keyFunc, resolver) - Add cached field
//   - WithAsyncField(fieldName, resolver) - Add async field
//   - WithInputObject(interface{}) - For mutations: auto-generate input type
//
// Build Methods:
//   - BuildQuery() - Returns QueryField interface for queries
//   - BuildMutation() - Returns MutationField interface for mutations
//   - Build() - Auto-detects and returns appropriate interface
//
// The resolver automatically:
//   - Generates GraphQL types from Go structs (including nested structs)
//   - Handles type conversions and validations
//   - Supports all Go primitive types, slices, maps, and custom types
//   - Respects struct tags (json, graphql, description, default)
//   - Provides type safety through Go generics
//   - Integrates with fx dependency injection

// GenericTypeInfo holds information about a generic type
type GenericTypeInfo struct {
	IsGeneric     bool
	IsWrapper     bool
	BaseTypeName  string
	ElementType   reflect.Type
	WrapperFields map[string]reflect.Type
}

// detectGenericType analyzes a type and returns information about its generic nature
func detectGenericType(v interface{}) GenericTypeInfo {
	t := reflect.TypeOf(v)
	info := GenericTypeInfo{
		WrapperFields: make(map[string]reflect.Type),
	}

	if t == nil {
		return info
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	info.BaseTypeName = t.Name()

	// Check if it's a struct
	if t.Kind() == reflect.Struct {
		// Check if this looks like a generic wrapper (Response[T], Result[T], etc.)
		info.IsWrapper = isGenericWrapper(t)
		if info.IsWrapper {
			info.IsGeneric = true
			// Analyze wrapper fields to understand the structure
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				info.WrapperFields[field.Name] = field.Type

				// If we find a Data field, check if it's the generic element
				if field.Name == "Data" {
					info.ElementType = field.Type
				}
			}
		}
	}

	return info
}

// isGenericWrapper determines if a struct type is a generic wrapper
func isGenericWrapper(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}

	// Check for common wrapper patterns
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

	// If it has Status, Code, and Data fields, it's likely a wrapper
	return hasStatus && hasData && hasCode
}

func detectGenericStruct(v interface{}) bool {
	info := detectGenericType(v)

	fmt.Printf("Type Analysis:\n")
	fmt.Printf("  Name: %s\n", info.BaseTypeName)
	fmt.Printf("  IsGeneric: %v\n", info.IsGeneric)
	fmt.Printf("  IsWrapper: %v\n", info.IsWrapper)
	if info.ElementType != nil {
		fmt.Printf("  ElementType: %v\n", info.ElementType)
	}
	fmt.Printf("  WrapperFields: %v\n", len(info.WrapperFields))
	fmt.Println("---")

	return info.IsGeneric
}

func GetTypeName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)

	if t == nil {
		return "interface"
	}

	fullName := t.String()

	// Check if it contains a slice inside generic brackets
	hasSlice := strings.Contains(fullName, "[[]")

	// Remove package paths
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		fullName = fullName[idx+1:]
	}

	// Remove slice markers and pointers
	fullName = strings.ReplaceAll(fullName, "[]", "")
	fullName = strings.ReplaceAll(fullName, "*", "")

	// Convert brackets to underscore: Response[User] -> Response_User
	fullName = strings.ReplaceAll(fullName, "[", "_")
	fullName = strings.ReplaceAll(fullName, "]", "")
	fullName = strings.ReplaceAll(fullName, " ", "_")

	// Add List prefix if it was a slice
	if hasSlice {
		fullName = "List" + fullName
	}

	return fullName
}

// getInputTypeName generates a clean GraphQL input type name from a reflect.Type
// Handles anonymous structs by generating meaningful names based on field context
func getInputTypeName(t reflect.Type, fieldName string) string {
	if t == nil {
		return "Input"
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Get the type name
	typeName := t.Name()

	// If it's an anonymous struct (no name), generate one from field name
	if typeName == "" && t.Kind() == reflect.Struct {
		if fieldName != "" {
			// Capitalize first letter of field name
			runes := []rune(fieldName)
			if len(runes) > 0 {
				runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
				typeName = string(runes)

				// Append "Input" suffix if not already present
				if !strings.HasSuffix(typeName, "Input") {
					typeName = typeName + "Input"
				}
			} else {
				typeName = "AnonymousInput"
			}
		} else {
			typeName = "AnonymousInput"
		}
	}

	fullName := typeName

	// If still empty, use the string representation
	if fullName == "" {
		fullName = t.String()
	}

	// Remove package paths
	if idx := strings.LastIndex(fullName, "."); idx >= 0 {
		fullName = fullName[idx+1:]
	}

	// Clean up the name
	fullName = strings.ReplaceAll(fullName, "[]", "")
	fullName = strings.ReplaceAll(fullName, "*", "")
	fullName = strings.ReplaceAll(fullName, "[", "_")
	fullName = strings.ReplaceAll(fullName, "]", "")
	fullName = strings.ReplaceAll(fullName, " ", "_")
	fullName = strings.ReplaceAll(fullName, "{", "")
	fullName = strings.ReplaceAll(fullName, "}", "")

	return fullName
}

func NewResolver[T any](name string) *UnifiedResolver[T] {
	resolver := &UnifiedResolver[T]{
		name:            name,
		objectName:      GetTypeName[T](),
		fieldOverrides:  make(map[string]graphql.FieldResolveFn),
		fieldMiddleware: make(map[string][]FieldMiddleware),
		customFields:    make(graphql.Fields),
	}

	// Auto-detect type characteristics
	var instance T
	t := reflect.TypeOf(instance)

	// Analyze the generic type
	typeInfo := detectGenericType(instance)

	if typeInfo.ElementType != nil {
		// If it's a wrapper with a slice element type, treat as list
		if typeInfo.ElementType.Kind() == reflect.Slice {
			resolver.isList = true
		}
	} else {
		if t.Kind() == reflect.Slice {
			resolver.isList = true
			resolver.isListManuallyAssigned = true
		}
	}
	return resolver
}

// Query Configuration
func (r *UnifiedResolver[T]) AsList() *UnifiedResolver[T] {
	r.isList = true
	r.isListManuallyAssigned = true
	return r
}

func (r *UnifiedResolver[T]) AsPaginated() *UnifiedResolver[T] {
	r.isPaginated = true
	r.isList = false // Paginated overrides list
	return r
}

// Mutation Configuration
func (r *UnifiedResolver[T]) AsMutation() *UnifiedResolver[T] {
	r.isMutation = true
	return r
}

func (r *UnifiedResolver[T]) WithInputObjectFieldName(name string) *UnifiedResolver[T] {
	r.inputName = name
	return r
}

func (r *UnifiedResolver[T]) WithInputObjectNullable() *UnifiedResolver[T] {
	r.nullableInput = true
	return r
}

func (r *UnifiedResolver[T]) WithInputObject(inputType interface{}) *UnifiedResolver[T] {
	r.inputType = inputType
	r.useInputObject = true

	// Generate input type name from the input struct
	t := reflect.TypeOf(inputType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	inputName := t.Name() + "Input"

	fieldName := "input"
	if r.inputName != "" {
		fieldName = r.inputName
	}

	inputGraphQLType := r.generateInputObject(inputType, inputName)
	if r.nullableInput {
		r.args = graphql.FieldConfigArgument{
			fieldName: &graphql.ArgumentConfig{
				Type:        inputGraphQLType,
				Description: "Input data",
			},
		}
	} else {
		r.args = graphql.FieldConfigArgument{
			fieldName: &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(inputGraphQLType),
				Description: "Input data",
			},
		}
	}
	return r
}

// Basic Configuration
func (r *UnifiedResolver[T]) WithDescription(desc string) *UnifiedResolver[T] {
	r.description = desc
	return r
}

func (r *UnifiedResolver[T]) WithArgs(args graphql.FieldConfigArgument) *UnifiedResolver[T] {
	r.args = args
	return r
}

func (r *UnifiedResolver[T]) WithArgsFromStruct(structType interface{}) *UnifiedResolver[T] {
	t := reflect.TypeOf(structType)
	r.args = generateArgsFromType(t)
	return r
}

// generateArgsFromType creates GraphQL arguments from a struct type
func generateArgsFromType(t reflect.Type) graphql.FieldConfigArgument {
	return generateArgsFromTypeWithContext(t, "")
}

// generateArgsFromTypeWithContext creates GraphQL arguments from a struct type with parent context
func generateArgsFromTypeWithContext(t reflect.Type, parentTypeName string) graphql.FieldConfigArgument {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return graphql.FieldConfigArgument{}
	}

	// Use parent type name if provided, otherwise use the struct's own name
	if parentTypeName == "" {
		parentTypeName = t.Name()
	}

	args := graphql.FieldConfigArgument{}
	gen := NewFieldGenerator[any]()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.PkgPath != "" {
			continue
		}

		fieldName := gen.getFieldName(field)
		if fieldName == "-" {
			continue
		}

		graphqlType := gen.getInputTypeWithContext(field.Type, field, parentTypeName)
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

// createPageInfoType creates the PageInfo GraphQL type
func createPageInfoType() *graphql.Object {
	// Check if PageInfo type already exists
	typeRegistryMu.RLock()
	if existingType, exists := typeRegistry["PageInfo"]; exists {
		typeRegistryMu.RUnlock()
		return existingType
	}
	typeRegistryMu.RUnlock()

	// Create new PageInfo type
	typeRegistryMu.Lock()
	defer typeRegistryMu.Unlock()

	// Double-check in case another goroutine created it
	if existingType, exists := typeRegistry["PageInfo"]; exists {
		return existingType
	}

	pageInfoType := graphql.NewObject(graphql.ObjectConfig{
		Name: "PageInfo",
		Fields: graphql.Fields{
			"hasNextPage": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if pageInfo, ok := p.Source.(PageInfo); ok {
						return pageInfo.HasNextPage, nil
					}
					return false, nil
				},
			},
			"hasPreviousPage": &graphql.Field{
				Type: graphql.Boolean,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if pageInfo, ok := p.Source.(PageInfo); ok {
						return pageInfo.HasPreviousPage, nil
					}
					return false, nil
				},
			},
			"startCursor": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if pageInfo, ok := p.Source.(PageInfo); ok {
						return pageInfo.StartCursor, nil
					}
					return "", nil
				},
			},
			"endCursor": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if pageInfo, ok := p.Source.(PageInfo); ok {
						return pageInfo.EndCursor, nil
					}
					return "", nil
				},
			},
		},
	})

	// Register the type
	typeRegistry["PageInfo"] = pageInfoType
	return pageInfoType
}

// WithResolver sets a type-safe resolver function that returns *T instead of interface{}
// This provides better type safety and eliminates the need for type assertions or casts
//
// Example usage:
//
//	NewResolver[User]("user").
//		WithResolver(func(p graph.ResolveParams) (*User, error) {
//			id, _ := GetArgInt(p, "id")
//			return userService.GetByID(id)
//		}).BuildQuery()
//
//	NewResolver[User]("users").
//		AsList().
//		WithResolver(func(p graph.ResolveParams) (*[]User, error) {
//			users := userService.List()
//			return &users, nil
//		}).BuildQuery()
//
//	NewResolver[string]("hello").
//		WithResolver(func(p graph.ResolveParams) (*string, error) {
//			msg := "Hello, World!"
//			return &msg, nil
//		}).BuildQuery()
func (r *UnifiedResolver[T]) WithResolver(resolver func(p ResolveParams) (*T, error)) *UnifiedResolver[T] {
	r.resolver = func(p graphql.ResolveParams) (interface{}, error) {
		return resolver(ResolveParams(p))
	}
	return r
}

// WithMiddleware adds middleware to the main resolver.
// Middleware functions are applied in the order they are added (first added = outermost layer).
// This is the foundation for all resolver-level middleware (auth, logging, caching, etc.).
//
// Example usage:
//
//	NewResolver[User]("user").
//		WithMiddleware(LoggingMiddleware).
//		WithMiddleware(AuthMiddleware("admin")).
//		WithResolver(func(p ResolveParams) (*User, error) {
//			return userService.GetByID(p.Args["id"].(int))
//		}).
//		BuildQuery()
func (r *UnifiedResolver[T]) WithMiddleware(middleware FieldMiddleware) *UnifiedResolver[T] {
	r.resolverMiddlewares = append(r.resolverMiddlewares, middleware)
	return r
}

// TypedArgsResolver provides type-safe argument handling
type TypedArgsResolver[T any, A any] struct {
	base     *UnifiedResolver[T]
	argName  []string
	argType  reflect.Type
	isScalar bool
}

// NewTypedResolver creates a resolver with type-safe arguments
// This allows you to specify both the return type and argument type
//
// For struct arguments (auto-generates fields from struct):
//
//	type GetUserArgs struct {
//		ID int `graphql:"id,required"`
//	}
//
//	NewArgsResolver[User, GetUserArgs]("user").
//		WithResolver(func(ctx context.Context, args GetUserArgs) (*User, error) {
//			return userService.GetByID(args.ID)
//		}).BuildQuery()
//
// For primitive arguments (requires argName parameter):
//
//	NewArgsResolver[string, string]("echo", "message").
//		WithResolver(func(ctx context.Context, message string) (*string, error) {
//			return &message, nil
//		}).BuildMutation()
//
//	NewArgsResolver[User, int]("user", "id").
//		WithResolver(func(ctx context.Context, id int) (*User, error) {
//			return userService.GetByID(id)
//		}).BuildQuery()

func NewArgsResolver[T any, A any](name string, argName ...string) *TypedArgsResolver[T, A] {
	base := NewResolver[T](name)

	// Get the type of A
	var argsInstance A
	argsType := reflect.TypeOf(argsInstance)

	// Check if A is a struct or a primitive type
	if argsType != nil && argsType.Kind() == reflect.Struct {
		// Struct type - auto-generate args from struct fields
		// Pass the parent type name for anonymous struct naming
		parentTypeName := argsType.Name()
		base.args = generateArgsFromTypeWithContext(argsType, parentTypeName)
	} else {
		// Primitive type (string, int, bool, etc.) - create single argument
		fieldName := "input"
		if len(argName) > 0 && argName[0] != "" {
			fieldName = argName[0]
		}

		// Get the GraphQL type for the primitive
		graphqlType := getPrimitiveGraphQLType(argsType)
		if graphqlType != nil {
			base.args = graphql.FieldConfigArgument{
				fieldName: &graphql.ArgumentConfig{
					Type: graphqlType,
				},
			}
		}
	}

	return &TypedArgsResolver[T, A]{
		base:     base,
		argName:  argName,
		argType:  argsType,
		isScalar: argsType != nil && argsType.Kind() != reflect.Struct,
	}
}

// getPrimitiveGraphQLType returns the GraphQL type for Go primitive types
func getPrimitiveGraphQLType(t reflect.Type) graphql.Input {
	if t == nil {
		return nil
	}

	switch t.Kind() {
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
	default:
		return nil
	}
}

// WithResolver sets a type-safe resolver with typed arguments and context support
//
// Example usage:
//
//	type GetPostArgs struct {
//		ID int `graphql:"id,required"`
//	}
//
//	resolver.WithArgs[GetPostArgs]().
//		WithResolver(func(ctx context.Context, args GetPostArgs) (*Post, error) {
//			return postService.GetByID(args.ID)
//		})
func (r *TypedArgsResolver[T, A]) WithResolver(resolver func(ctx context.Context, p ResolveParams, args A) (*T, error)) *TypedArgsResolver[T, A] {
	r.base.resolver = func(p graphql.ResolveParams) (interface{}, error) {
		// Extract context from ResolveParams
		ctx := p.Context
		if ctx == nil {
			ctx = context.Background()
		}

		var args A

		// Check if A is a scalar type (primitive)
		if r.isScalar {
			// For primitives, extract the single argument value directly
			fieldName := "input"
			if len(r.argName) > 0 && r.argName[0] != "" {
				fieldName = r.argName[0]
			}

			if argValue, exists := p.Args[fieldName]; exists {
				// Convert to type A
				argsValue := reflect.ValueOf(&args).Elem()
				if err := setFieldValue(argsValue, argValue); err != nil {
					return nil, fmt.Errorf("failed to parse argument %s: %w", fieldName, err)
				}
			}
		} else {
			// For structs, map all args to struct fields
			if err := mapArgsToStruct(p.Args, &args); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
		}

		// Call the typed resolver
		return resolver(ctx, ResolveParams(p), args)
	}
	return r
}

// BuildQuery builds and returns a QueryField
func (r *TypedArgsResolver[T, A]) BuildQuery() QueryField {
	return r.base.BuildQuery()
}

// BuildMutation builds and returns a MutationField
func (r *TypedArgsResolver[T, A]) BuildMutation() MutationField {
	return r.base.BuildMutation()
}

// AsList configures the resolver to return a list of items
func (r *TypedArgsResolver[T, A]) AsList() *TypedArgsResolver[T, A] {
	r.base.AsList()
	return r
}

// AsPaginated configures the resolver to return paginated results
func (r *TypedArgsResolver[T, A]) AsPaginated() *TypedArgsResolver[T, A] {
	r.base.AsPaginated()
	return r
}

// WithDescription sets the field description
func (r *TypedArgsResolver[T, A]) WithDescription(desc string) *TypedArgsResolver[T, A] {
	r.base.WithDescription(desc)
	return r
}

// Typed Resolver Support - allows direct struct parameters instead of graphql.ResolveParams
//
// Example usage:
//
//	func resolveUser(args GetUserArgs) (*User, error) {
//	    return &User{ID: args.ID, Name: "User"}, nil
//	}
//
//	NewResolver[User]("user", "User").
//	    WithTypedResolver(resolveUser).
//	    BuildQuery()
func (r *UnifiedResolver[T]) WithTypedResolver(typedResolver interface{}) *UnifiedResolver[T] {
	r.resolver = r.wrapTypedResolver(typedResolver)
	return r
}

// wrapTypedResolver converts a typed resolver function to a standard GraphQL resolver
func (r *UnifiedResolver[T]) wrapTypedResolver(typedResolver interface{}) graphql.FieldResolveFn {
	resolverValue := reflect.ValueOf(typedResolver)
	resolverType := resolverValue.Type()

	if resolverType.Kind() != reflect.Func {
		panic("typedResolver must be a function")
	}

	return func(p graphql.ResolveParams) (interface{}, error) {
		numIn := resolverType.NumIn()
		args := make([]reflect.Value, numIn)

		for i := 0; i < numIn; i++ {
			paramType := resolverType.In(i)

			// Create new instance of the parameter type
			paramValue := reflect.New(paramType)
			paramInterface := paramValue.Interface()

			// Try to map GraphQL args to this parameter
			var err error
			inputFieldName := "input"
			if r.inputName != "" {
				inputFieldName = r.inputName
			}
			if inputData, exists := p.Args[inputFieldName]; exists && i == 0 {
				// First parameter from input argument (mutations)
				err = mapstructure.Decode(inputData, paramInterface)
			} else if i == 0 && numIn == 1 {
				// Single parameter - try to map all args to it (queries)
				err = mapArgsToStruct(p.Args, paramInterface)
			} else {
				// Try to find matching argument by parameter name or position
				if fieldName := getParameterName(resolverType, i); fieldName != "" {
					if argData, exists := p.Args[fieldName]; exists {
						err = mapstructure.Decode(argData, paramInterface)
					}
				}
			}

			if err != nil {
				return nil, fmt.Errorf("failed to map parameter %d: %w", i, err)
			}

			args[i] = paramValue.Elem()
		}

		// Call the typed resolver
		results := resolverValue.Call(args)

		// Handle return values
		if len(results) == 0 {
			return nil, nil
		}

		if len(results) == 1 {
			result := results[0]
			if result.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
				// Single error return
				if result.IsNil() {
					return nil, nil
				}
				return nil, result.Interface().(error)
			}
			// Single value return
			return result.Interface(), nil
		}

		if len(results) == 2 {
			// (value, error) pattern
			value := results[0].Interface()
			errResult := results[1]

			if errResult.IsNil() {
				return value, nil
			}
			return value, errResult.Interface().(error)
		}

		return nil, fmt.Errorf("unsupported return pattern: %d values", len(results))
	}
}

// getParameterName attempts to get parameter name from function signature
func getParameterName(funcType reflect.Type, index int) string {
	// This is a basic implementation - in practice you might want to use
	// build tags or other methods to extract parameter names
	// For now, we'll use common patterns
	if index == 0 {
		return "input"
	}
	return fmt.Sprintf("arg%d", index)
}

// Field-Level Customization
func (r *UnifiedResolver[T]) WithFieldResolver(fieldName string, resolver graphql.FieldResolveFn) *UnifiedResolver[T] {
	r.fieldOverrides[fieldName] = resolver
	return r
}

func (r *UnifiedResolver[T]) WithFieldResolvers(overrides map[string]graphql.FieldResolveFn) *UnifiedResolver[T] {
	for fieldName, resolver := range overrides {
		r.fieldOverrides[fieldName] = resolver
	}
	return r
}

func (r *UnifiedResolver[T]) WithFieldMiddleware(fieldName string, middleware FieldMiddleware) *UnifiedResolver[T] {
	r.fieldMiddleware[fieldName] = append(r.fieldMiddleware[fieldName], middleware)
	return r
}

// WithPermission adds permission middleware to the resolver (similar to Python @permission_classes decorator)
// This is now just a convenience wrapper around WithMiddleware for backwards compatibility
func (r *UnifiedResolver[T]) WithPermission(middleware FieldMiddleware) *UnifiedResolver[T] {
	return r.WithMiddleware(middleware)
}

func (r *UnifiedResolver[T]) WithCustomField(name string, field *graphql.Field) *UnifiedResolver[T] {
	r.customFields[name] = field
	return r
}

func (r *UnifiedResolver[T]) WithComputedField(name string, fieldType graphql.Output, resolver graphql.FieldResolveFn) *UnifiedResolver[T] {
	r.customFields[name] = &graphql.Field{
		Type:    fieldType,
		Resolve: resolver,
	}
	return r
}

// Utility Methods for Field Configuration
func (r *UnifiedResolver[T]) WithLazyField(fieldName string, loader func(interface{}) (interface{}, error)) *UnifiedResolver[T] {
	r.fieldOverrides[fieldName] = LazyFieldResolver(fieldName, loader)
	return r
}

func (r *UnifiedResolver[T]) WithCachedField(fieldName string, cacheKeyFunc func(graphql.ResolveParams) string, resolver graphql.FieldResolveFn) *UnifiedResolver[T] {
	r.fieldOverrides[fieldName] = CachedFieldResolver(cacheKeyFunc, resolver)
	return r
}

func (r *UnifiedResolver[T]) WithAsyncField(fieldName string, resolver graphql.FieldResolveFn) *UnifiedResolver[T] {
	r.fieldOverrides[fieldName] = AsyncFieldResolver(resolver)
	return r
}

// Build Methods
func (r *UnifiedResolver[T]) BuildQuery() QueryField {
	return r
}

func (r *UnifiedResolver[T]) BuildMutation() MutationField {
	r.isMutation = true
	return r
}

func (r *UnifiedResolver[T]) Build() interface{} {
	if r.isMutation {
		return r.BuildMutation()
	}
	return r.BuildQuery()
}

// Interface Implementation
func (r *UnifiedResolver[T]) Name() string {
	return r.name
}

func (r *UnifiedResolver[T]) Serve() *graphql.Field {
	var outputType graphql.Output

	if r.isPaginated {
		outputType = r.generatePaginatedType()
	} else if r.isList && r.isListManuallyAssigned {
		// Check if the element type is a scalar
		var instance T
		t := reflect.TypeOf(instance)

		// For slice types, get the element type
		var elementType reflect.Type
		if t != nil && t.Kind() == reflect.Slice {
			elementType = t.Elem()
		}

		// Check if element type is scalar
		elementScalarType := r.getScalarType(elementType)
		if elementScalarType != nil {
			// List of scalars
			outputType = graphql.NewList(elementScalarType)
		} else {
			// List of objects
			outputType = graphql.NewList(r.generateObjectTypeWithOverrides())
		}
	} else {
		// Check if T is a primitive/scalar type
		var instance T
		t := reflect.TypeOf(instance)
		scalarType := r.getScalarType(t)

		if scalarType != nil {
			// Use scalar type directly for primitives
			outputType = scalarType
		} else {
			// Generate object type for struct types
			outputType = r.generateObjectTypeWithOverrides()
		}
	}

	// Apply middleware stack to the resolver
	resolver := r.resolver

	// Convert and apply middlewares if any exist
	if len(r.resolverMiddlewares) > 0 {
		// Wrap graphql.FieldResolveFn to our FieldResolveFn
		wrappedResolver := wrapGraphQLResolver(resolver)

		// Apply resolver middlewares in order (first added = outermost layer)
		wrappedResolver = applyMiddlewares(wrappedResolver, r.resolverMiddlewares)

		// Convert back to graphql.FieldResolveFn
		resolver = unwrapGraphQLResolver(wrappedResolver)
	}

	return &graphql.Field{
		Type:        outputType,
		Description: r.description,
		Args:        r.args,
		Resolve:     resolver,
	}
}

// getScalarType returns the GraphQL scalar type for primitive Go types
func (r *UnifiedResolver[T]) getScalarType(t reflect.Type) graphql.Output {
	if t == nil {
		return nil
	}

	switch t.Kind() {
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
	default:
		return nil
	}
}

// Internal Generation Methods
func (r *UnifiedResolver[T]) generateObjectTypeWithOverrides() *graphql.Object {
	// Check if type already exists in registry

	typeRegistryMu.RLock()
	if existingType, exists := typeRegistry[r.objectName]; exists {
		typeRegistryMu.RUnlock()
		return existingType
	}
	typeRegistryMu.RUnlock()

	// Create new type
	typeRegistryMu.Lock()
	defer typeRegistryMu.Unlock()

	// Double-check in case another goroutine created it
	if existingType, exists := typeRegistry[r.objectName]; exists {
		return existingType
	}

	gen := NewFieldGenerator[T]()
	var instance T
	typeToUse := reflect.TypeOf(instance)

	// If T is a slice type, extract the element type for field generation
	if typeToUse != nil && typeToUse.Kind() == reflect.Slice {
		typeToUse = typeToUse.Elem()
	}

	// Check if this is a wrapper type and handle it specially
	var baseFields graphql.Fields
	if typeToUse != nil && gen.isWrapperType(typeToUse) {
		// Create a wrapper object that handles the Data field safely
		wrapperObj := gen.createWrapperObject(typeToUse, r.objectName)
		// Use the wrapper object directly since we just need its field definitions
		// We'll create our own type later but use the wrapper's field definitions
		fieldDefinitionMap := wrapperObj.Fields()
		baseFields = make(graphql.Fields)
		for name, fieldDef := range fieldDefinitionMap {
			baseFields[name] = &graphql.Field{
				Type:        fieldDef.Type,
				Description: fieldDef.Description,
				Resolve:     fieldDef.Resolve,
			}
		}
	} else {

		baseFields = gen.generateFields(typeToUse)
	}

	// Apply field resolver overrides
	for fieldName, override := range r.fieldOverrides {
		if field, exists := baseFields[fieldName]; exists {
			originalResolve := field.Resolve

			// Apply middleware if any
			finalResolve := override
			if middlewares, hasMiddleware := r.fieldMiddleware[fieldName]; hasMiddleware {
				// Wrap, apply middleware, then unwrap
				wrapped := wrapGraphQLResolver(override)
				wrapped = applyMiddlewares(wrapped, middlewares)
				finalResolve = unwrapGraphQLResolver(wrapped)
			}

			// Set up fallback to original resolver if needed
			if originalResolve != nil {
				field.Resolve = func(p graphql.ResolveParams) (interface{}, error) {
					result, err := finalResolve(p)
					if err != nil {
						// Fallback to original resolver
						return originalResolve(p)
					}
					return result, nil
				}
			} else {
				field.Resolve = finalResolve
			}
		}
	}

	// Add custom fields
	for fieldName, customField := range r.customFields {
		baseFields[fieldName] = customField
	}

	newType := graphql.NewObject(graphql.ObjectConfig{
		Name:   r.objectName,
		Fields: baseFields,
	})

	// Register the type
	typeRegistry[r.objectName] = newType
	return newType
}

func (r *UnifiedResolver[T]) generatePaginatedType() *graphql.Object {
	itemType := r.generateObjectTypeWithOverrides()

	return graphql.NewObject(graphql.ObjectConfig{
		Name: r.objectName + "Connection",
		Fields: graphql.Fields{
			"items": &graphql.Field{
				Type: graphql.NewList(itemType),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if paginated, ok := p.Source.(PaginatedResponse[T]); ok {
						return paginated.Items, nil
					}
					return nil, nil
				},
			},
			"totalCount": &graphql.Field{
				Type: graphql.Int,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if paginated, ok := p.Source.(PaginatedResponse[T]); ok {
						return paginated.TotalCount, nil
					}
					return 0, nil
				},
			},
			"pageInfo": &graphql.Field{
				Type: createPageInfoType(),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					if paginated, ok := p.Source.(PaginatedResponse[T]); ok {
						return paginated.PageInfo, nil
					}
					return PageInfo{}, nil
				},
			},
		},
	})
}

func (r *UnifiedResolver[T]) generateInputObject(inputType interface{}, name string) *graphql.InputObject {
	// Check if input type already exists in registry
	inputTypeRegistryMu.RLock()
	if existingType, exists := inputTypeRegistry[name]; exists {
		inputTypeRegistryMu.RUnlock()
		return existingType
	}
	inputTypeRegistryMu.RUnlock()

	// Create new input type
	inputTypeRegistryMu.Lock()
	defer inputTypeRegistryMu.Unlock()

	// Double-check in case another goroutine created it
	if existingType, exists := inputTypeRegistry[name]; exists {
		return existingType
	}

	t := reflect.TypeOf(inputType)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	gen := NewFieldGenerator[any]()
	fields := gen.generateInputFields(t)

	newInputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name:   name,
		Fields: fields,
	})

	// Register the input type
	inputTypeRegistry[name] = newInputType
	return newInputType
}

// Utility Functions for Middleware and Resolvers

// wrapGraphQLResolver converts graphql.FieldResolveFn to our custom FieldResolveFn
func wrapGraphQLResolver(resolver graphql.FieldResolveFn) FieldResolveFn {
	return func(p ResolveParams) (interface{}, error) {
		// Convert ResolveParams to graphql.ResolveParams
		return resolver(graphql.ResolveParams(p))
	}
}

// unwrapGraphQLResolver converts our custom FieldResolveFn to graphql.FieldResolveFn
func unwrapGraphQLResolver(resolver FieldResolveFn) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		// Convert graphql.ResolveParams to ResolveParams
		return resolver(ResolveParams(p))
	}
}

func applyMiddlewares(resolver FieldResolveFn, middlewares []FieldMiddleware) FieldResolveFn {
	for i := len(middlewares) - 1; i >= 0; i-- {
		resolver = middlewares[i](resolver)
	}
	return resolver
}

// Common Middleware Functions

// LoggingMiddleware logs field resolution time
func LoggingMiddleware(next FieldResolveFn) FieldResolveFn {
	return func(p ResolveParams) (interface{}, error) {
		start := time.Now()
		result, err := next(p)
		fmt.Printf("Field %s resolved in %v\n", p.Info.FieldName, time.Since(start))
		return result, err
	}
}

// AuthMiddleware requires a specific user role
func AuthMiddleware(requiredRole string) FieldMiddleware {
	return func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			if userRole, exists := p.Context.Value("userRole").(string); exists {
				if userRole != requiredRole {
					return nil, fmt.Errorf("insufficient permissions")
				}
			}
			return next(p)
		}
	}
}

// CacheMiddleware caches field results based on a key function
func CacheMiddleware(cacheKey func(ResolveParams) string) FieldMiddleware {
	cache := make(map[string]interface{})
	return func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) {
			key := cacheKey(p)
			if cached, exists := cache[key]; exists {
				return cached, nil
			}
			result, err := next(p)
			if err == nil {
				cache[key] = result
			}
			return result, err
		}
	}
}

// Helper Functions for Common Resolvers

// AsyncFieldResolver executes a resolver asynchronously
func AsyncFieldResolver(resolver graphql.FieldResolveFn) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		type result struct {
			data interface{}
			err  error
		}

		ch := make(chan result, 1)
		go func() {
			data, err := resolver(p)
			ch <- result{data, err}
		}()

		r := <-ch
		return r.data, r.err
	}
}

// CachedFieldResolver caches field results with a key function
func CachedFieldResolver(cacheKey func(graphql.ResolveParams) string, resolver graphql.FieldResolveFn) graphql.FieldResolveFn {
	cache := make(map[string]interface{})

	return func(p graphql.ResolveParams) (interface{}, error) {
		key := cacheKey(p)
		if cached, exists := cache[key]; exists {
			return cached, nil
		}

		result, err := resolver(p)
		if err == nil {
			cache[key] = result
		}
		return result, err
	}
}

// LazyFieldResolver loads a field only when requested
func LazyFieldResolver(fieldName string, loader func(interface{}) (interface{}, error)) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		source := reflect.ValueOf(p.Source)
		if source.Kind() == reflect.Ptr {
			source = source.Elem()
		}

		field := source.FieldByName(fieldName)
		if field.IsValid() && !field.IsZero() {
			return field.Interface(), nil
		}

		return loader(p.Source)
	}
}

// Convenience Functions

// DataTransformResolver applies a transformation to a field value
func DataTransformResolver(transform func(interface{}) interface{}) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		source := reflect.ValueOf(p.Source)
		if source.Kind() == reflect.Ptr {
			source = source.Elem()
		}

		field := source.FieldByName(strings.Title(p.Info.FieldName))
		if field.IsValid() {
			return transform(field.Interface()), nil
		}
		return nil, nil
	}
}

// ConditionalResolver resolves based on a condition
func ConditionalResolver(condition func(graphql.ResolveParams) bool, ifTrue, ifFalse graphql.FieldResolveFn) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		if condition(p) {
			return ifTrue(p)
		}
		return ifFalse(p)
	}
}

// mapArgsToStruct maps GraphQL arguments directly to a struct
func mapArgsToStruct(args map[string]interface{}, output interface{}) error {
	// Use reflection to map arguments to struct fields
	outputValue := reflect.ValueOf(output)
	if outputValue.Kind() != reflect.Ptr || outputValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("output must be a pointer to a struct")
	}

	outputValue = outputValue.Elem()
	outputType := outputValue.Type()

	for i := 0; i < outputType.NumField(); i++ {
		field := outputType.Field(i)
		fieldValue := outputValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get field name from json tag or use field name
		fieldName := getFieldName(field)
		if fieldName == "-" {
			continue
		}

		if argValue, exists := args[fieldName]; exists && argValue != nil {
			if err := setFieldValue(fieldValue, argValue); err != nil {
				return fmt.Errorf("failed to set field %s: %w", fieldName, err)
			}
		}
	}

	return nil
}

// getFieldName extracts the field name from struct tags
func getFieldName(field reflect.StructField) string {
	// Check json tag first
	if jsonTag := field.Tag.Get("json"); jsonTag != "" {
		parts := strings.Split(jsonTag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}

	// Check graphql tag
	if graphqlTag := field.Tag.Get("graphql"); graphqlTag != "" {
		parts := strings.Split(graphqlTag, ",")
		for _, part := range parts {
			if !strings.Contains(part, "=") && part != "required" {
				return part
			}
		}
	}

	// Convert field name to camelCase
	return toCamelCase(field.Name)
}

// toCamelCase converts PascalCase to camelCase
func toCamelCase(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
	return string(runes)
}

// setFieldValue sets a reflect.Value with the appropriate type conversion
func setFieldValue(fieldValue reflect.Value, argValue interface{}) error {
	argReflectValue := reflect.ValueOf(argValue)

	// Handle pointer fields
	if fieldValue.Kind() == reflect.Ptr {
		if argValue == nil {
			return nil // Leave as nil
		}
		// Create new instance of the pointer type
		newValue := reflect.New(fieldValue.Type().Elem())
		if err := setFieldValue(newValue.Elem(), argValue); err != nil {
			return err
		}
		fieldValue.Set(newValue)
		return nil
	}

	// Handle type conversion
	if argReflectValue.Type().ConvertibleTo(fieldValue.Type()) {
		fieldValue.Set(argReflectValue.Convert(fieldValue.Type()))
		return nil
	}

	// Handle special cases
	switch fieldValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if argReflectValue.Kind() == reflect.Float64 {
			fieldValue.SetInt(int64(argReflectValue.Float()))
			return nil
		}
	case reflect.Float32, reflect.Float64:
		if argReflectValue.Kind() == reflect.Int {
			fieldValue.SetFloat(float64(argReflectValue.Int()))
			return nil
		}
	case reflect.Struct:
		// Handle nested structs - convert map[string]interface{} to struct
		if argMap, ok := argValue.(map[string]interface{}); ok {
			return mapArgsToStruct(argMap, fieldValue.Addr().Interface())
		}
	case reflect.Slice:
		if argReflectValue.Kind() == reflect.Slice {
			// Handle slice conversion
			newSlice := reflect.MakeSlice(fieldValue.Type(), argReflectValue.Len(), argReflectValue.Cap())
			for i := 0; i < argReflectValue.Len(); i++ {
				if err := setFieldValue(newSlice.Index(i), argReflectValue.Index(i).Interface()); err != nil {
					return err
				}
			}
			fieldValue.Set(newSlice)
			return nil
		}
	}

	return fmt.Errorf("cannot convert %v (%s) to %s", argValue, argReflectValue.Type(), fieldValue.Type())
}
