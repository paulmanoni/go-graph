package graph

import (
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
	middlewareInfos        []MiddlewareInfo  // parallel to resolverMiddlewares; "anonymous" for WithMiddleware
	argValidators          map[string][]Validator
	deprecated             bool
	deprecationReason      string
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

	// Remove package paths - handle multiple segments
	// e.g., "graph.TestPageableResponse[graph.Interview]" -> "TestPageableResponse[Interview]"
	// First, handle the generic parameter if present
	if bracketIdx := strings.Index(fullName, "["); bracketIdx >= 0 {
		basePart := fullName[:bracketIdx]
		paramPart := fullName[bracketIdx:]

		// Remove package path from base
		if idx := strings.LastIndex(basePart, "."); idx >= 0 {
			basePart = basePart[idx+1:]
		}

		// Remove package paths from generic parameters
		// Handle patterns like "[graph.Interview]" or "[[]graph.Interview]"
		paramPart = strings.ReplaceAll(paramPart, "[", " [ ")
		paramPart = strings.ReplaceAll(paramPart, "]", " ] ")
		parts := strings.Fields(paramPart)
		for i, part := range parts {
			if part != "[" && part != "]" {
				if idx := strings.LastIndex(part, "."); idx >= 0 {
					parts[i] = part[idx+1:]
				}
			}
		}
		paramPart = strings.Join(parts, "")

		fullName = basePart + paramPart
	} else {
		// No generics, just remove package path
		if idx := strings.LastIndex(fullName, "."); idx >= 0 {
			fullName = fullName[idx+1:]
		}
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

	// Sanitize to remove Go runtime identifiers (e.g., "Interview·91" -> "Interview")
	// This also handles cases like "TestPageableResponse_Interview93" -> "TestPageableResponse_Interview"
	fullName = sanitizeTypeName(fullName)

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

	// Get the type name and sanitize it
	typeName := sanitizeTypeName(t.Name())

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

	// Final sanitization
	fullName = sanitizeTypeName(fullName)

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
// This provides better type safety and eliminates the need for type assertions or casts.
//
// WithResolver requires the function signature:
//   - func(p ResolveParams) (*T, error)
//
// Access arguments using type-safe getter functions with ArgsMap:
//   - Get[T](ArgsMap(p.Args), "key") - returns zero value if not found
//   - GetE[T](ArgsMap(p.Args), "key") - returns error if not found
//   - GetOr[T](ArgsMap(p.Args), "key", defaultVal) - returns default if not found
//   - MustGet[T](ArgsMap(p.Args), "key") - panics if not found
//
// Example usage:
//
//	NewResolver[User]("user").
//		WithArg("id", graph.String).
//		WithResolver(func(p graph.ResolveParams) (*User, error) {
//			id := graph.Get[string](graph.ArgsMap(p.Args), "id")
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
func (r *UnifiedResolver[T]) WithResolver(resolver func(ResolveParams) (*T, error)) *UnifiedResolver[T] {
	r.resolver = func(p graphql.ResolveParams) (interface{}, error) {
		result, err := resolver(ResolveParams(p))
		// Handle typed nil - return untyped nil for interface compatibility
		if result == nil {
			return nil, err
		}
		return result, err
	}
	return r
}

// extractResolverResults handles the return values from a resolver function
func extractResolverResults(results []reflect.Value) (interface{}, error) {
	if len(results) != 2 {
		panic("resolver must return exactly 2 values: (*T, error)")
	}

	// First value is the result (could be nil)
	var result interface{}
	if !results[0].IsNil() {
		result = results[0].Interface()
	}

	// Second value is the error
	if results[1].IsNil() {
		return result, nil
	}
	return result, results[1].Interface().(error)
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
	r.middlewareInfos = append(r.middlewareInfos, MiddlewareInfo{Name: "anonymous"})
	return r
}

// WithNamedMiddleware adds a middleware with a display name and description.
// Prefer this over WithMiddleware when the middleware's identity matters to
// tools that render the resolver — dashboards, documentation generators, or
// a lint step that enforces "every mutation must have an 'auth' middleware".
//
// The name shows up verbatim in FieldInfo.Middlewares[i].Name, so keep it
// kebab-case and stable ("auth", "rate-limit", "permission:admin").
//
//	r.WithNamedMiddleware("auth", "Bearer token validation", AuthMiddleware)
//	r.WithNamedMiddleware("permission:admin", "Admin role required",
//	    PermissionMiddleware([]string{"admin"}))
func (r *UnifiedResolver[T]) WithNamedMiddleware(name, description string, middleware FieldMiddleware) *UnifiedResolver[T] {
	r.resolverMiddlewares = append(r.resolverMiddlewares, middleware)
	r.middlewareInfos = append(r.middlewareInfos, MiddlewareInfo{Name: name, Description: description})
	return r
}

// WithArgValidator attaches one or more validators to a named argument.
// Validators run after graphql-go parses input but before the resolver
// fires; the first failing validator aborts with its error. Metadata
// surfaces on FieldInfo.Args[i].Validators for dashboards.
//
// Use the built-in constructors (Required, StringLength, StringMatch,
// OneOf, IntRange, Custom) rather than populating Validator by hand.
//
//	r.WithArgValidator("title", graph.Required(), graph.StringLength(1, 100))
//	r.WithArgValidator("email", graph.StringMatch(`^\S+@\S+$`, "must be an email"))
func (r *UnifiedResolver[T]) WithArgValidator(argName string, validators ...Validator) *UnifiedResolver[T] {
	if r.argValidators == nil {
		r.argValidators = map[string][]Validator{}
	}
	r.argValidators[argName] = append(r.argValidators[argName], validators...)
	return r
}

// WithDeprecated marks this field deprecated. The reason propagates to the
// generated *graphql.Field and to FieldInfo.DeprecationReason so both
// GraphQL clients (via introspection) and tools see it.
func (r *UnifiedResolver[T]) WithDeprecated(reason string) *UnifiedResolver[T] {
	r.deprecated = true
	r.deprecationReason = reason
	return r
}

// --- Introspection ----------------------------------------------------------

// FieldInfo returns a serializable snapshot of this resolver's metadata.
// Calling this is cheap — it reads already-stored fields and runs Serve()
// once for the return type. Safe to call multiple times.
func (r *UnifiedResolver[T]) FieldInfo() FieldInfo {
	kind := FieldKindQuery
	if r.isMutation {
		kind = FieldKindMutation
	}

	mws := make([]MiddlewareInfo, 0, len(r.resolverMiddlewares))
	for i := range r.resolverMiddlewares {
		if i < len(r.middlewareInfos) {
			mws = append(mws, r.middlewareInfos[i])
		} else {
			mws = append(mws, MiddlewareInfo{Name: "anonymous"})
		}
	}

	info := FieldInfo{
		Name:              r.name,
		Description:       r.description,
		Kind:              kind,
		List:              r.isList,
		Paginated:         r.isPaginated,
		Deprecated:        r.deprecated,
		DeprecationReason: r.deprecationReason,
		Args:              argsToInfo(r.args, r.argValidators),
		Middlewares:       mws,
		InputObject:       inputObjectInfo(r.inputType, r.nullableInput, r.useInputObject),
	}

	// Return type requires the concrete graphql.Field.
	if field := r.Serve(); field != nil {
		info.ReturnType = typeString(field.Type)
	}

	return info
}

// GetArgsDefinition returns the underlying graphql.FieldConfigArgument.
// Useful for tools that want the raw graphql-go types (e.g. to fabricate
// their own introspection query programmatically).
func (r *UnifiedResolver[T]) GetArgsDefinition() graphql.FieldConfigArgument {
	return r.args
}

// IsMutation reports whether BuildMutation or AsMutation was used.
func (r *UnifiedResolver[T]) IsMutation() bool { return r.isMutation }

// IsList reports whether AsList was used (or the return type is a slice).
func (r *UnifiedResolver[T]) IsList() bool { return r.isList }

// IsPaginated reports whether AsPaginated was used.
func (r *UnifiedResolver[T]) IsPaginated() bool { return r.isPaginated }

// IsDeprecated reports whether WithDeprecated was applied.
func (r *UnifiedResolver[T]) IsDeprecated() bool { return r.deprecated }

// GetMiddlewareCount returns the number of middlewares on the main resolver.
// For richer information use FieldInfo().Middlewares.
func (r *UnifiedResolver[T]) GetMiddlewareCount() int { return len(r.resolverMiddlewares) }

// GetMiddlewareInfos returns the MiddlewareInfo slice in application order.
// Entries for middlewares added via plain WithMiddleware have Name == "anonymous".
func (r *UnifiedResolver[T]) GetMiddlewareInfos() []MiddlewareInfo {
	out := make([]MiddlewareInfo, len(r.middlewareInfos))
	copy(out, r.middlewareInfos)
	return out
}

// ArgsGetter is an interface for types that can provide arguments by name.
// This allows Get[T], GetE[T], GetOr[T], and MustGet[T] to work with both:
//   - graph.Args (from WithArg chainable API)
//   - graph.ArgsMap (from graphql.FieldConfigArgument / p.Args)
type ArgsGetter interface {
	GetArg(name string) (interface{}, bool)
}

// Args provides type-safe access to GraphQL arguments
type Args struct {
	raw map[string]interface{}
}

// NewArgs creates an Args instance from a map
func NewArgs(raw map[string]interface{}) Args {
	return Args{raw: raw}
}

// Raw returns the underlying map
func (a Args) Raw() map[string]interface{} {
	return a.raw
}

// GetArg implements ArgsGetter interface
func (a Args) GetArg(name string) (interface{}, bool) {
	if a.raw == nil {
		return nil, false
	}
	val, exists := a.raw[name]
	return val, exists
}

// Has checks if an argument exists
func (a Args) Has(name string) bool {
	_, exists := a.raw[name]
	return exists
}

// ArgsMap wraps map[string]interface{} to implement ArgsGetter.
// This allows using p.Args directly with Get[T], GetE[T], etc.
//
// Usage:
//
//	// In subscription resolvers with graphql.FieldConfigArgument
//	channelID := graph.Get[string](graph.ArgsMap(p.Args), "channelID")
type ArgsMap map[string]interface{}

// GetArg implements ArgsGetter interface
func (m ArgsMap) GetArg(name string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	val, exists := m[name]
	return val, exists
}

// RootInfoGetter is an interface for types that can provide root value data by name.
// This allows GetRoot[T], GetRootE[T], GetRootOr[T], and MustGetRoot[T] to work with
// root value data extracted from ResolveParams.
type RootInfoGetter interface {
	GetRootValue(name string) (interface{}, bool)
}

// RootInfo wraps map[string]interface{} to implement RootInfoGetter.
// This allows using p.Info.RootValue with type-safe generic functions.
//
// Usage:
//
//	// Extract user details from root value
//	user := graph.GetRoot[UserDetails](graph.NewRootInfo(p), "details")
//
//	// With error handling
//	user, err := graph.GetRootE[UserDetails](graph.NewRootInfo(p), "details")
//
//	// With default value
//	token := graph.GetRootOr[string](graph.NewRootInfo(p), "token", "")
type RootInfo map[string]interface{}

// GetRootValue implements RootInfoGetter interface
func (r RootInfo) GetRootValue(name string) (interface{}, bool) {
	if r == nil {
		return nil, false
	}
	val, exists := r[name]
	return val, exists
}

// NewRootInfo extracts RootInfo from ResolveParams.
// Returns nil if RootValue is nil or not a map[string]interface{}.
//
// Usage:
//
//	rootInfo := graph.NewRootInfo(p)
//	user := graph.GetRoot[UserDetails](rootInfo, "details")
func NewRootInfo(p ResolveParams) RootInfo {
	if p.Info.RootValue == nil {
		return nil
	}
	if rootMap, ok := p.Info.RootValue.(map[string]interface{}); ok {
		return RootInfo(rootMap)
	}
	return nil
}

// GetRoot retrieves a value from root info by name with type safety.
// Returns zero value if key is missing or conversion fails.
// Use GetRootE for explicit error handling.
//
// Usage:
//
//	user := graph.GetRoot[UserDetails](graph.NewRootInfo(p), "details")
//	token := graph.GetRoot[string](graph.NewRootInfo(p), "token")
func GetRoot[T any](r RootInfoGetter, name string) T {
	val, _ := GetRootE[T](r, name)
	return val
}

// GetRootE retrieves a value from root info by name with type safety and error handling.
// Returns an error if the key is missing or type conversion fails.
//
// Usage:
//
//	user, err := graph.GetRootE[UserDetails](graph.NewRootInfo(p), "details")
//	if err != nil {
//	    return nil, fmt.Errorf("authentication required: %w", err)
//	}
func GetRootE[T any](r RootInfoGetter, name string) (T, error) {
	var zero T
	if r == nil {
		return zero, fmt.Errorf("root value %q not found: root info is nil", name)
	}
	val, exists := r.GetRootValue(name)
	if !exists {
		return zero, fmt.Errorf("root value %q not found", name)
	}
	if val == nil {
		return zero, fmt.Errorf("root value %q is nil", name)
	}
	if typed, ok := val.(T); ok {
		return typed, nil
	}
	result, err := convertArgE[T](val, name)
	if err != nil {
		return zero, fmt.Errorf("root value %q: %w", name, err)
	}
	return result, nil
}

// MustGetRoot retrieves a value from root info by name with type safety.
// Panics if the key is missing or conversion fails.
// Use only when you're certain the value exists and is valid.
//
// Usage:
//
//	user := graph.MustGetRoot[UserDetails](graph.NewRootInfo(p), "details")
func MustGetRoot[T any](r RootInfoGetter, name string) T {
	val, err := GetRootE[T](r, name)
	if err != nil {
		panic(fmt.Sprintf("MustGetRoot failed: %v", err))
	}
	return val
}

// GetRootOr retrieves a value from root info by name with a default value if not found.
//
// Usage:
//
//	token := graph.GetRootOr[string](graph.NewRootInfo(p), "token", "anonymous")
//	userID := graph.GetRootOr[int](graph.NewRootInfo(p), "userID", 0)
func GetRootOr[T any](r RootInfoGetter, name string, defaultVal T) T {
	if r == nil {
		return defaultVal
	}
	val, exists := r.GetRootValue(name)
	if !exists {
		return defaultVal
	}
	if val == nil {
		return defaultVal
	}
	if typed, ok := val.(T); ok {
		return typed
	}
	result, err := convertArgE[T](val, name)
	if err != nil {
		return defaultVal
	}
	return result
}

// Get retrieves an argument by name with type safety.
// Returns zero value if key is missing or conversion fails.
// Use GetE for explicit error handling.
//
// Works with both graph.Args (from WithArg) and graph.ArgsMap (from p.Args):
//
//	// With graph.Args (from WithArg chainable API)
//	id := graph.Get[string](args, "id")
//
//	// With p.Args (from graphql.FieldConfigArgument)
//	channelID := graph.Get[string](graph.ArgsMap(p.Args), "channelID")
func Get[T any](a ArgsGetter, name string) T {
	val, _ := GetE[T](a, name)
	return val
}

// GetE retrieves an argument by name with type safety and error handling.
// Returns an error if the key is missing or type conversion fails.
//
// Works with both graph.Args and graph.ArgsMap (p.Args):
//
//	// With graph.Args
//	input, err := graph.GetE[UserInput](args, "input")
//
//	// With p.Args
//	channelID, err := graph.GetE[string](graph.ArgsMap(p.Args), "channelID")
func GetE[T any](a ArgsGetter, name string) (T, error) {
	var zero T
	if a == nil {
		return zero, fmt.Errorf("argument %q not found: args is nil", name)
	}
	val, exists := a.GetArg(name)
	if !exists {
		return zero, fmt.Errorf("argument %q not found", name)
	}
	if val == nil {
		return zero, fmt.Errorf("argument %q is nil", name)
	}
	if typed, ok := val.(T); ok {
		return typed, nil
	}
	result, err := convertArgE[T](val, name)
	if err != nil {
		return zero, err
	}
	return result, nil
}

// MustGet retrieves an argument by name with type safety.
// Panics if the key is missing or conversion fails.
// Use only when you're certain the argument exists and is valid.
//
// Works with both graph.Args and graph.ArgsMap (p.Args).
func MustGet[T any](a ArgsGetter, name string) T {
	val, err := GetE[T](a, name)
	if err != nil {
		panic(fmt.Sprintf("MustGet failed: %v", err))
	}
	return val
}

// GetOr retrieves an argument by name with a default value if not found.
//
// Works with both graph.Args and graph.ArgsMap (p.Args):
//
//	// With graph.Args
//	limit := graph.GetOr[int](args, "limit", 10)
//
//	// With p.Args
//	limit := graph.GetOr[int](graph.ArgsMap(p.Args), "limit", 10)
func GetOr[T any](a ArgsGetter, name string, defaultVal T) T {
	if a == nil {
		return defaultVal
	}
	val, exists := a.GetArg(name)
	if !exists {
		return defaultVal
	}
	if val == nil {
		return defaultVal
	}
	if typed, ok := val.(T); ok {
		return typed
	}
	result, err := convertArgE[T](val, name)
	if err != nil {
		return defaultVal
	}
	return result
}

// convertArg handles type conversions for GraphQL arguments (returns zero value on error)
func convertArg[T any](val interface{}) T {
	result, _ := convertArgE[T](val, "")
	return result
}

// convertArgE handles type conversions for GraphQL arguments with error handling
func convertArgE[T any](val interface{}, argName string) (T, error) {
	var zero T
	targetType := reflect.TypeOf(zero)
	if targetType == nil {
		return zero, fmt.Errorf("cannot determine target type for argument %q", argName)
	}

	valReflect := reflect.ValueOf(val)
	if !valReflect.IsValid() {
		return zero, fmt.Errorf("argument %q has invalid value", argName)
	}

	// Direct conversion if possible
	if valReflect.Type().ConvertibleTo(targetType) {
		return valReflect.Convert(targetType).Interface().(T), nil
	}

	// Handle int/float conversions (GraphQL often sends numbers as float64)
	switch targetType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := val.(type) {
		case float64:
			return reflect.ValueOf(int64(v)).Convert(targetType).Interface().(T), nil
		case int:
			return reflect.ValueOf(int64(v)).Convert(targetType).Interface().(T), nil
		}
	case reflect.Float32, reflect.Float64:
		switch v := val.(type) {
		case int:
			return reflect.ValueOf(float64(v)).Convert(targetType).Interface().(T), nil
		case int64:
			return reflect.ValueOf(float64(v)).Convert(targetType).Interface().(T), nil
		}
	case reflect.Struct:
		// Handle map[string]interface{} to struct conversion
		if mapVal, ok := val.(map[string]interface{}); ok {
			var result T
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				TagName: "json",
				Result:  &result,
			})
			if err != nil {
				return zero, fmt.Errorf("failed to create decoder for argument %q: %w", argName, err)
			}
			if err := decoder.Decode(mapVal); err != nil {
				return zero, fmt.Errorf("failed to decode argument %q to %s: %w", argName, targetType.Name(), err)
			}
			return result, nil
		}
		return zero, fmt.Errorf("argument %q: expected map[string]interface{} for struct conversion, got %T", argName, val)
	}

	return zero, fmt.Errorf("cannot convert argument %q from %T to %s", argName, val, targetType.Name())
}

// GraphQL scalar type constants for use with WithArg
// These mirror Go's type names for intuitive usage:
//
//	WithArg("id", graph.String)   // like using 'string'
//	WithArg("limit", graph.Int)   // like using 'int'
var (
	String  = graphql.String
	Int     = graphql.Int
	Float   = graphql.Float
	Boolean = graphql.Boolean
	ID      = graphql.ID
)

// WithArg adds an argument to the resolver. Supports:
// - Go primitive types: string, int, int64, float64, bool (pass zero value or instance)
// - GraphQL types: graph.String, graph.Int, graph.Float, graph.Boolean, graph.ID
// - Struct types: any struct will be converted to GraphQL InputObject (supports deeply nested structs)
// - Slices: []Type will be converted to [Type] GraphQL list
//
// Usage:
//
//	// With Go primitive types (recommended)
//	NewResolver[User]("user").
//		WithArg("id", "").           // string
//		WithArg("limit", 0).         // int
//		WithArg("active", false).    // bool
//		WithResolver(func(p ResolveParams) (*User, error) {
//			id := Get[string](ArgsMap(p.Args), "id")
//			return userService.GetByID(id)
//		})
//
//	// With struct type (deeply nested supported)
//	type AddressInput struct {
//		Street  string `json:"street"`
//		City    string `json:"city"`
//	}
//	type UserInput struct {
//		Name    string       `json:"name"`
//		Address AddressInput `json:"address"`
//	}
//
//	NewResolver[User]("createUser").
//		WithArg("input", UserInput{}).
//		WithResolver(func(p ResolveParams) (*User, error) {
//			input := Get[UserInput](ArgsMap(p.Args), "input")
//			return userService.Create(input)
//		})
func (r *UnifiedResolver[T]) WithArg(name string, argType interface{}) *UnifiedResolver[T] {
	if r.args == nil {
		r.args = graphql.FieldConfigArgument{}
	}

	graphqlType := resolveInputType(argType, name)
	if graphqlType != nil {
		r.args[name] = &graphql.ArgumentConfig{
			Type: graphqlType,
		}
	}
	return r
}

// WithArgRequired adds a required argument to the resolver
func (r *UnifiedResolver[T]) WithArgRequired(name string, argType interface{}) *UnifiedResolver[T] {
	if r.args == nil {
		r.args = graphql.FieldConfigArgument{}
	}

	graphqlType := resolveInputType(argType, name)
	if graphqlType != nil {
		r.args[name] = &graphql.ArgumentConfig{
			Type: graphql.NewNonNull(graphqlType),
		}
	}
	return r
}

// WithArgDefault adds an argument with a default value
func (r *UnifiedResolver[T]) WithArgDefault(name string, argType interface{}, defaultValue interface{}) *UnifiedResolver[T] {
	if r.args == nil {
		r.args = graphql.FieldConfigArgument{}
	}

	graphqlType := resolveInputType(argType, name)
	if graphqlType != nil {
		r.args[name] = &graphql.ArgumentConfig{
			Type:         graphqlType,
			DefaultValue: defaultValue,
		}
	}
	return r
}

// resolveInputType converts Go types to GraphQL input types
func resolveInputType(argType interface{}, argName string) graphql.Input {
	// If it's already a GraphQL type, use it directly
	if gqlType, ok := argType.(graphql.Input); ok {
		return gqlType
	}

	// Get reflect type
	t := reflect.TypeOf(argType)
	if t == nil {
		return nil
	}

	// Handle pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check for primitive types first
	if primitive := getPrimitiveGraphQLType(t); primitive != nil {
		return primitive
	}

	// Handle struct types - generate InputObject
	if t.Kind() == reflect.Struct {
		typeName := t.Name()
		if typeName == "" {
			typeName = strings.Title(argName)
		}
		if !strings.HasSuffix(typeName, "Input") {
			typeName = typeName + "Input"
		}
		typeName = sanitizeTypeName(typeName)

		// Check if already registered
		inputTypeRegistryMu.RLock()
		if existing, exists := inputTypeRegistry[typeName]; exists {
			inputTypeRegistryMu.RUnlock()
			return existing
		}
		inputTypeRegistryMu.RUnlock()

		// Generate using FieldGenerator (supports deeply nested structs)
		gen := NewFieldGenerator[any]()
		fields := gen.generateInputFields(t)

		inputTypeRegistryMu.Lock()
		defer inputTypeRegistryMu.Unlock()

		// Double check after acquiring lock
		if existing, exists := inputTypeRegistry[typeName]; exists {
			return existing
		}

		inputObject := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   typeName,
			Fields: fields,
		})
		inputTypeRegistry[typeName] = inputObject
		return inputObject
	}

	// Handle slice types
	if t.Kind() == reflect.Slice {
		elemType := t.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		// Create a zero value instance of element type
		elemInstance := reflect.New(elemType).Elem().Interface()
		elemGraphQL := resolveInputType(elemInstance, argName+"Item")
		if elemGraphQL != nil {
			return graphql.NewList(elemGraphQL)
		}
	}

	return nil
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

	// Apply arg validators as an outermost pre-resolve step. Failing any
	// validator aborts with a user-facing error before the resolver runs.
	if len(r.argValidators) > 0 {
		inner := resolver
		validators := r.argValidators
		resolver = func(p graphql.ResolveParams) (interface{}, error) {
			for argName, vs := range validators {
				val, exists := p.Args[argName]
				for _, v := range vs {
					// Treat nil/missing as a "required" check opt-in.
					if !exists || val == nil {
						if v.Info.Kind == "required" {
							return nil, fmt.Errorf("%s: %s", argName, v.Info.Message)
						}
						continue
					}
					if err := v.Fn(val); err != nil {
						return nil, fmt.Errorf("%s: %w", argName, err)
					}
				}
			}
			return inner(p)
		}
	}

	// Convert and apply middlewares if any exist
	if len(r.resolverMiddlewares) > 0 {
		// Wrap graphql.FieldResolveFn to our FieldResolveFn
		wrappedResolver := wrapGraphQLResolver(resolver)

		// Apply resolver middlewares in order (first added = outermost layer)
		wrappedResolver = applyMiddlewares(wrappedResolver, r.resolverMiddlewares)

		// Convert back to graphql.FieldResolveFn
		resolver = unwrapGraphQLResolver(wrappedResolver)
	}

	field := &graphql.Field{
		Type:        outputType,
		Description: r.description,
		Args:        r.args,
		Resolve:     resolver,
	}
	if r.deprecated {
		field.DeprecationReason = r.deprecationReason
	}
	return field
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

	// Create new type - we need to register it BEFORE generating fields
	// to avoid deadlocks with recursive types
	typeRegistryMu.Lock()

	// Double-check in case another goroutine created it
	if existingType, exists := typeRegistry[r.objectName]; exists {
		typeRegistryMu.Unlock()
		return existingType
	}

	gen := NewFieldGenerator[T]()
	var instance T
	typeToUse := reflect.TypeOf(instance)

	// If T is a slice type, extract the element type for field generation
	if typeToUse != nil && typeToUse.Kind() == reflect.Slice {
		typeToUse = typeToUse.Elem()
	}

	// Capture variables for the closure
	capturedTypeToUse := typeToUse
	capturedObjectName := r.objectName
	capturedFieldOverrides := r.fieldOverrides
	capturedFieldMiddleware := r.fieldMiddleware
	capturedCustomFields := r.customFields

	// Create the object type with a FieldsThunk for lazy field generation
	// This avoids deadlock by releasing the lock before fields are generated
	newType := graphql.NewObject(graphql.ObjectConfig{
		Name: r.objectName,
		Fields: (graphql.FieldsThunk)(func() graphql.Fields {
			var baseFields graphql.Fields

			// Check if this is a wrapper type and handle it specially
			if capturedTypeToUse != nil && gen.isWrapperType(capturedTypeToUse) {
				// Create a wrapper object that handles the Data field safely
				wrapperObj := gen.createWrapperObject(capturedTypeToUse, capturedObjectName)
				// Use the wrapper object directly since we just need its field definitions
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
				baseFields = gen.generateFields(capturedTypeToUse)
			}

			// Apply field resolver overrides
			for fieldName, override := range capturedFieldOverrides {
				if field, exists := baseFields[fieldName]; exists {
					originalResolve := field.Resolve

					// Apply middleware if any
					finalResolve := override
					if middlewares, hasMiddleware := capturedFieldMiddleware[fieldName]; hasMiddleware {
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
			for fieldName, customField := range capturedCustomFields {
				baseFields[fieldName] = customField
			}

			return baseFields
		}),
	})

	// Register the type
	typeRegistry[r.objectName] = newType
	typeRegistryMu.Unlock()

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
