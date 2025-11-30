package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

// Test types for WithArg tests
type TestUserInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type TestAddressInput struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type TestNestedInput struct {
	User    TestUserInput    `json:"user"`
	Address TestAddressInput `json:"address"`
}

type TestDeeplyNestedInput struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Value string `json:"value"`
			} `json:"level3"`
		} `json:"level2"`
	} `json:"level1"`
}

// TestArgs tests the Args type and Get functions
func TestArgs_Get(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]interface{}
		key      string
		wantStr  string
		wantInt  int
		wantBool bool
	}{
		{
			name:    "get string value",
			raw:     map[string]interface{}{"name": "Alice"},
			key:     "name",
			wantStr: "Alice",
		},
		{
			name:    "get int value",
			raw:     map[string]interface{}{"age": 25},
			key:     "age",
			wantInt: 25,
		},
		{
			name:    "get bool value",
			raw:     map[string]interface{}{"active": true},
			key:     "active",
			wantBool: true,
		},
		{
			name:    "get missing value returns zero",
			raw:     map[string]interface{}{},
			key:     "missing",
			wantStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := NewArgs(tt.raw)

			if tt.wantStr != "" || tt.key == "name" || tt.key == "missing" {
				got := Get[string](args, tt.key)
				if got != tt.wantStr {
					t.Errorf("Get[string]() = %v, want %v", got, tt.wantStr)
				}
			}
			if tt.wantInt != 0 {
				got := Get[int](args, tt.key)
				if got != tt.wantInt {
					t.Errorf("Get[int]() = %v, want %v", got, tt.wantInt)
				}
			}
			if tt.wantBool {
				got := Get[bool](args, tt.key)
				if got != tt.wantBool {
					t.Errorf("Get[bool]() = %v, want %v", got, tt.wantBool)
				}
			}
		})
	}
}

func TestArgs_GetOr(t *testing.T) {
	tests := []struct {
		name       string
		raw        map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "returns value when present",
			raw:        map[string]interface{}{"name": "Alice"},
			key:        "name",
			defaultVal: "default",
			want:       "Alice",
		},
		{
			name:       "returns default when missing",
			raw:        map[string]interface{}{},
			key:        "name",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "returns default when nil map",
			raw:        nil,
			key:        "name",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := NewArgs(tt.raw)
			got := GetOr[string](args, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetOr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestArgs_Has(t *testing.T) {
	args := NewArgs(map[string]interface{}{"name": "Alice"})

	if !args.Has("name") {
		t.Error("Has(name) should return true")
	}
	if args.Has("missing") {
		t.Error("Has(missing) should return false")
	}
}

func TestArgs_Raw(t *testing.T) {
	raw := map[string]interface{}{"name": "Alice"}
	args := NewArgs(raw)

	got := args.Raw()
	if got["name"] != "Alice" {
		t.Errorf("Raw() = %v, want %v", got, raw)
	}
}

// TestConvertArg tests type conversions
func TestConvertArg(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want interface{}
	}{
		{
			name: "float64 to int",
			val:  float64(42),
			want: 42,
		},
		{
			name: "int to float64",
			val:  42,
			want: float64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := NewArgs(map[string]interface{}{"val": tt.val})

			switch expected := tt.want.(type) {
			case int:
				got := Get[int](args, "val")
				if got != expected {
					t.Errorf("Get[int]() = %v, want %v", got, expected)
				}
			case float64:
				got := Get[float64](args, "val")
				if got != expected {
					t.Errorf("Get[float64]() = %v, want %v", got, expected)
				}
			}
		})
	}
}

// TestWithArg tests the WithArg method
func TestWithArg_ScalarTypes(t *testing.T) {
	tests := []struct {
		name     string
		argName  string
		argType  interface{}
		wantType graphql.Input
	}{
		{
			name:     "string type using graph.String",
			argName:  "name",
			argType:  String,
			wantType: graphql.String,
		},
		{
			name:     "int type using graph.Int",
			argName:  "age",
			argType:  Int,
			wantType: graphql.Int,
		},
		{
			name:     "float type using graph.Float",
			argName:  "price",
			argType:  Float,
			wantType: graphql.Float,
		},
		{
			name:     "bool type using graph.Boolean",
			argName:  "active",
			argType:  Boolean,
			wantType: graphql.Boolean,
		},
		{
			name:     "string type using zero value",
			argName:  "name",
			argType:  "",
			wantType: graphql.String,
		},
		{
			name:     "int type using zero value",
			argName:  "age",
			argType:  0,
			wantType: graphql.Int,
		},
		{
			name:     "bool type using zero value",
			argName:  "active",
			argType:  false,
			wantType: graphql.Boolean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TestResult struct {
				ID string `json:"id"`
			}

			resolver := NewResolver[TestResult]("test").
				WithArg(tt.argName, tt.argType)

			if resolver.args == nil {
				t.Fatal("args should not be nil")
			}

			argConfig, exists := resolver.args[tt.argName]
			if !exists {
				t.Fatalf("arg %s should exist", tt.argName)
			}

			if argConfig.Type != tt.wantType {
				t.Errorf("arg type = %v, want %v", argConfig.Type, tt.wantType)
			}
		})
	}
}

func TestWithArg_StructType(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	// Clear the registry before test
	inputTypeRegistryMu.Lock()
	delete(inputTypeRegistry, "TestUserInputInput")
	inputTypeRegistryMu.Unlock()

	resolver := NewResolver[TestResult]("test").
		WithArg("input", TestUserInput{})

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	argConfig, exists := resolver.args["input"]
	if !exists {
		t.Fatal("arg 'input' should exist")
	}

	inputObj, ok := argConfig.Type.(*graphql.InputObject)
	if !ok {
		t.Fatalf("arg type should be *graphql.InputObject, got %T", argConfig.Type)
	}

	// Check the input object has the expected fields
	fields := inputObj.Fields()
	if _, exists := fields["name"]; !exists {
		t.Error("input object should have 'name' field")
	}
	if _, exists := fields["email"]; !exists {
		t.Error("input object should have 'email' field")
	}
}

func TestWithArg_NestedStructType(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	// Clear the registry before test
	inputTypeRegistryMu.Lock()
	delete(inputTypeRegistry, "TestNestedInputInput")
	delete(inputTypeRegistry, "TestUserInputInput")
	delete(inputTypeRegistry, "TestAddressInputInput")
	inputTypeRegistryMu.Unlock()

	resolver := NewResolver[TestResult]("test").
		WithArg("input", TestNestedInput{})

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	argConfig, exists := resolver.args["input"]
	if !exists {
		t.Fatal("arg 'input' should exist")
	}

	inputObj, ok := argConfig.Type.(*graphql.InputObject)
	if !ok {
		t.Fatalf("arg type should be *graphql.InputObject, got %T", argConfig.Type)
	}

	// Check the input object has nested fields
	fields := inputObj.Fields()
	if _, exists := fields["user"]; !exists {
		t.Error("input object should have 'user' field")
	}
	if _, exists := fields["address"]; !exists {
		t.Error("input object should have 'address' field")
	}
}

func TestWithArg_DeeplyNestedStructType(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	resolver := NewResolver[TestResult]("test").
		WithArg("input", TestDeeplyNestedInput{})

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	argConfig, exists := resolver.args["input"]
	if !exists {
		t.Fatal("arg 'input' should exist")
	}

	_, ok := argConfig.Type.(*graphql.InputObject)
	if !ok {
		t.Fatalf("arg type should be *graphql.InputObject, got %T", argConfig.Type)
	}
}

func TestWithArgRequired(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	resolver := NewResolver[TestResult]("test").
		WithArgRequired("id", String)

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	argConfig, exists := resolver.args["id"]
	if !exists {
		t.Fatal("arg 'id' should exist")
	}

	nonNull, ok := argConfig.Type.(*graphql.NonNull)
	if !ok {
		t.Fatalf("arg type should be *graphql.NonNull, got %T", argConfig.Type)
	}

	if nonNull.OfType != graphql.String {
		t.Errorf("inner type should be graphql.String, got %v", nonNull.OfType)
	}
}

func TestWithArgDefault(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	resolver := NewResolver[TestResult]("test").
		WithArgDefault("limit", Int, 10)

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	argConfig, exists := resolver.args["limit"]
	if !exists {
		t.Fatal("arg 'limit' should exist")
	}

	if argConfig.DefaultValue != 10 {
		t.Errorf("default value = %v, want %v", argConfig.DefaultValue, 10)
	}
}

func TestWithArg_Chaining(t *testing.T) {
	type TestResult struct {
		ID string `json:"id"`
	}

	resolver := NewResolver[TestResult]("test").
		WithArg("id", String).
		WithArg("name", String).
		WithArg("limit", Int).
		WithArg("active", Boolean)

	if resolver.args == nil {
		t.Fatal("args should not be nil")
	}

	expectedArgs := []string{"id", "name", "limit", "active"}
	for _, arg := range expectedArgs {
		if _, exists := resolver.args[arg]; !exists {
			t.Errorf("arg '%s' should exist", arg)
		}
	}

	if len(resolver.args) != len(expectedArgs) {
		t.Errorf("expected %d args, got %d", len(expectedArgs), len(resolver.args))
	}
}

// TestWithResolverArgs tests the WithResolverArgs method
func TestWithResolverArgs(t *testing.T) {
	type TestUser struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	resolver := NewResolver[TestUser]("user").
		WithArg("id", String).
		WithArg("name", String).
		WithResolverArgs(func(p ResolveParams, args Args) (*TestUser, error) {
			id := Get[string](args, "id")
			name := Get[string](args, "name")
			return &TestUser{ID: id, Name: name}, nil
		})

	if resolver.resolver == nil {
		t.Fatal("resolver function should not be nil")
	}

	// Test execution
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"id":   "123",
			"name": "Alice",
		},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	user, ok := result.(*TestUser)
	if !ok {
		t.Fatalf("result should be *TestUser, got %T", result)
	}

	if user.ID != "123" {
		t.Errorf("user.ID = %v, want %v", user.ID, "123")
	}
	if user.Name != "Alice" {
		t.Errorf("user.Name = %v, want %v", user.Name, "Alice")
	}
}

func TestWithResolverArgs_WithStruct(t *testing.T) {
	type TestUser struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// Clear registry
	inputTypeRegistryMu.Lock()
	delete(inputTypeRegistry, "TestUserInputInput")
	inputTypeRegistryMu.Unlock()

	resolver := NewResolver[TestUser]("createUser").
		WithArg("input", TestUserInput{}).
		WithResolverArgs(func(p ResolveParams, args Args) (*TestUser, error) {
			// Use type-safe Get[T] for struct conversion
			input := Get[TestUserInput](args, "input")
			return &TestUser{
				ID:   "new-id",
				Name: input.Name,
			}, nil
		})

	if resolver.resolver == nil {
		t.Fatal("resolver function should not be nil")
	}

	// Test execution
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Bob",
				"email": "bob@example.com",
			},
		},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	user, ok := result.(*TestUser)
	if !ok {
		t.Fatalf("result should be *TestUser, got %T", result)
	}

	if user.Name != "Bob" {
		t.Errorf("user.Name = %v, want %v", user.Name, "Bob")
	}
}

// TestGetE tests GetE[T] with error handling
func TestGetE(t *testing.T) {
	t.Run("returns value for existing key", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"name": "Alice",
		})

		val, err := GetE[string](args, "name")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "Alice" {
			t.Errorf("val = %v, want %v", val, "Alice")
		}
	})

	t.Run("returns error for missing key", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{})

		_, err := GetE[string](args, "missing")
		if err == nil {
			t.Fatal("expected error for missing key")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error message should contain 'not found', got: %v", err)
		}
	})

	t.Run("returns error for nil value", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"name": nil,
		})

		_, err := GetE[string](args, "name")
		if err == nil {
			t.Fatal("expected error for nil value")
		}
		if !strings.Contains(err.Error(), "is nil") {
			t.Errorf("error message should contain 'is nil', got: %v", err)
		}
	})

	t.Run("returns error for nil args", func(t *testing.T) {
		args := NewArgs(nil)

		_, err := GetE[string](args, "name")
		if err == nil {
			t.Fatal("expected error for nil args")
		}
	})

	t.Run("returns error for type conversion failure", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"input": []int{1, 2, 3}, // slice cannot convert to struct
		})

		_, err := GetE[TestUserInput](args, "input")
		if err == nil {
			t.Fatal("expected error for type conversion failure")
		}
	})

	t.Run("struct conversion success", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Bob",
				"email": "bob@example.com",
			},
		})

		input, err := GetE[TestUserInput](args, "input")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if input.Name != "Bob" {
			t.Errorf("input.Name = %v, want %v", input.Name, "Bob")
		}
	})

	t.Run("struct conversion failure", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"input": "not a map", // wrong type
		})

		_, err := GetE[TestUserInput](args, "input")
		if err == nil {
			t.Fatal("expected error for invalid struct input")
		}
	})
}

// TestMustGet tests MustGet[T] panic behavior
func TestMustGet(t *testing.T) {
	t.Run("returns value for existing key", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"name": "Alice",
		})

		val := MustGet[string](args, "name")
		if val != "Alice" {
			t.Errorf("val = %v, want %v", val, "Alice")
		}
	})

	t.Run("panics for missing key", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for missing key")
			}
		}()

		args := NewArgs(map[string]interface{}{})
		_ = MustGet[string](args, "missing")
	})

	t.Run("panics for nil value", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic for nil value")
			}
		}()

		args := NewArgs(map[string]interface{}{
			"name": nil,
		})
		_ = MustGet[string](args, "name")
	})
}

// TestGet_StructConversion tests Get[T] for struct conversion from map
func TestGet_StructConversion(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type Person struct {
		Name    string  `json:"name"`
		Age     int     `json:"age"`
		Address Address `json:"address"`
	}

	t.Run("simple struct", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Alice",
				"email": "alice@example.com",
			},
		})

		input := Get[TestUserInput](args, "input")
		if input.Name != "Alice" {
			t.Errorf("input.Name = %v, want %v", input.Name, "Alice")
		}
		if input.Email != "alice@example.com" {
			t.Errorf("input.Email = %v, want %v", input.Email, "alice@example.com")
		}
	})

	t.Run("nested struct", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{
			"person": map[string]interface{}{
				"name": "Bob",
				"age":  30,
				"address": map[string]interface{}{
					"street": "123 Main St",
					"city":   "NYC",
				},
			},
		})

		person := Get[Person](args, "person")
		if person.Name != "Bob" {
			t.Errorf("person.Name = %v, want %v", person.Name, "Bob")
		}
		if person.Age != 30 {
			t.Errorf("person.Age = %v, want %v", person.Age, 30)
		}
		if person.Address.Street != "123 Main St" {
			t.Errorf("person.Address.Street = %v, want %v", person.Address.Street, "123 Main St")
		}
		if person.Address.City != "NYC" {
			t.Errorf("person.Address.City = %v, want %v", person.Address.City, "NYC")
		}
	})

	t.Run("missing key returns zero value", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{})

		input := Get[TestUserInput](args, "input")
		if input.Name != "" {
			t.Errorf("input.Name = %v, want empty string", input.Name)
		}
	})

	t.Run("direct struct value", func(t *testing.T) {
		// If the value is already the correct type, it should work
		expected := TestUserInput{Name: "Direct", Email: "direct@test.com"}
		args := NewArgs(map[string]interface{}{
			"input": expected,
		})

		input := Get[TestUserInput](args, "input")
		if input != expected {
			t.Errorf("input = %v, want %v", input, expected)
		}
	})
}

// TestWithResolver_OptionalArgs tests that WithResolver works without args
func TestWithResolver_OptionalArgs(t *testing.T) {
	type TestMessage struct {
		Message string `json:"message"`
	}

	resolver := NewResolver[TestMessage]("hello").
		WithResolver(func(p ResolveParams) (*TestMessage, error) {
			return &TestMessage{Message: "Hello, World!"}, nil
		})

	if resolver.resolver == nil {
		t.Fatal("resolver function should not be nil")
	}

	// Test execution without args
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	msg, ok := result.(*TestMessage)
	if !ok {
		t.Fatalf("result should be *TestMessage, got %T", result)
	}

	if msg.Message != "Hello, World!" {
		t.Errorf("msg.Message = %v, want %v", msg.Message, "Hello, World!")
	}
}

// TestResolveInputType tests the resolveInputType function
func TestResolveInputType(t *testing.T) {
	tests := []struct {
		name     string
		argType  interface{}
		argName  string
		wantNil  bool
		wantType string
	}{
		{
			name:     "graphql.String",
			argType:  graphql.String,
			argName:  "test",
			wantType: "String",
		},
		{
			name:     "empty string",
			argType:  "",
			argName:  "test",
			wantType: "String",
		},
		{
			name:     "zero int",
			argType:  0,
			argName:  "test",
			wantType: "Int",
		},
		{
			name:     "false bool",
			argType:  false,
			argName:  "test",
			wantType: "Boolean",
		},
		{
			name:     "float64 zero",
			argType:  float64(0),
			argName:  "test",
			wantType: "Float",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveInputType(tt.argType, tt.argName)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.Name() != tt.wantType {
				t.Errorf("type name = %v, want %v", result.Name(), tt.wantType)
			}
		})
	}
}

// TestIntegration_CompleteResolver tests a complete resolver setup
func TestIntegration_CompleteResolver(t *testing.T) {
	type User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// Build a complete resolver
	queryField := NewResolver[User]("user").
		WithArg("id", String).
		WithDescription("Get a user by ID").
		WithResolverArgs(func(p ResolveParams, args Args) (*User, error) {
			id := Get[string](args, "id")
			return &User{
				ID:    id,
				Name:  "Test User",
				Email: "test@example.com",
			}, nil
		}).
		BuildQuery()

	// Verify the field is built correctly
	if queryField.Name() != "user" {
		t.Errorf("name = %v, want %v", queryField.Name(), "user")
	}

	field := queryField.Serve()
	if field == nil {
		t.Fatal("Serve() returned nil")
	}

	if field.Description != "Get a user by ID" {
		t.Errorf("description = %v, want %v", field.Description, "Get a user by ID")
	}

	if field.Args == nil {
		t.Fatal("args should not be nil")
	}

	if _, exists := field.Args["id"]; !exists {
		t.Error("should have 'id' argument")
	}
}

func TestIntegration_MutationWithInput(t *testing.T) {
	type User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	type CreateUserInput struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// Clear registry
	inputTypeRegistryMu.Lock()
	delete(inputTypeRegistry, "CreateUserInputInput")
	inputTypeRegistryMu.Unlock()

	// Build a mutation resolver
	mutationField := NewResolver[User]("createUser").
		AsMutation().
		WithArg("input", CreateUserInput{}).
		WithDescription("Create a new user").
		WithResolverArgs(func(p ResolveParams, args Args) (*User, error) {
			raw := args.Raw()
			input := raw["input"].(map[string]interface{})
			return &User{
				ID:    "new-id",
				Name:  input["name"].(string),
				Email: input["email"].(string),
			}, nil
		}).
		BuildMutation()

	// Verify the field is built correctly
	if mutationField.Name() != "createUser" {
		t.Errorf("name = %v, want %v", mutationField.Name(), "createUser")
	}

	field := mutationField.Serve()
	if field == nil {
		t.Fatal("Serve() returned nil")
	}

	if _, exists := field.Args["input"]; !exists {
		t.Error("should have 'input' argument")
	}
}

// TestWithResolver_UnifiedSignature tests that WithResolver works with both signatures
func TestWithResolver_UnifiedSignature_WithoutArgs(t *testing.T) {
	type TestMessage struct {
		Message string `json:"message"`
	}

	// Test WithResolver without args signature: func(p ResolveParams) (*T, error)
	resolver := NewResolver[TestMessage]("hello").
		WithResolver(func(p ResolveParams) (*TestMessage, error) {
			return &TestMessage{Message: "Hello, World!"}, nil
		})

	if resolver.resolver == nil {
		t.Fatal("resolver function should not be nil")
	}

	// Test execution
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	msg, ok := result.(*TestMessage)
	if !ok {
		t.Fatalf("result should be *TestMessage, got %T", result)
	}

	if msg.Message != "Hello, World!" {
		t.Errorf("msg.Message = %v, want %v", msg.Message, "Hello, World!")
	}
}

func TestWithResolver_UnifiedSignature_WithArgs(t *testing.T) {
	type TestUser struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// Test WithResolver with args signature: func(p ResolveParams, args Args) (*T, error)
	resolver := NewResolver[TestUser]("user").
		WithArg("id", String).
		WithArg("name", String).
		WithResolver(func(p ResolveParams, args Args) (*TestUser, error) {
			id := Get[string](args, "id")
			name := Get[string](args, "name")
			return &TestUser{ID: id, Name: name}, nil
		})

	if resolver.resolver == nil {
		t.Fatal("resolver function should not be nil")
	}

	// Test execution
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"id":   "123",
			"name": "Alice",
		},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	user, ok := result.(*TestUser)
	if !ok {
		t.Fatalf("result should be *TestUser, got %T", result)
	}

	if user.ID != "123" {
		t.Errorf("user.ID = %v, want %v", user.ID, "123")
	}
	if user.Name != "Alice" {
		t.Errorf("user.Name = %v, want %v", user.Name, "Alice")
	}
}

// TestWithResolver_UnifiedSignature_Error tests error handling in unified WithResolver
func TestWithResolver_UnifiedSignature_Error(t *testing.T) {
	type TestResult struct {
		Value string `json:"value"`
	}

	expectedErr := "test error"

	// Test error return without args
	resolver := NewResolver[TestResult]("test").
		WithResolver(func(p ResolveParams) (*TestResult, error) {
			return nil, context.DeadlineExceeded
		})

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{},
	}

	result, err := resolver.resolver(params)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}

	// Test error return with args
	resolver2 := NewResolver[TestResult]("test2").
		WithArg("value", String).
		WithResolver(func(p ResolveParams, args Args) (*TestResult, error) {
			return nil, context.Canceled
		})

	result2, err2 := resolver2.resolver(params)
	if err2 == nil {
		t.Fatal("expected error, got nil")
	}
	if result2 != nil {
		t.Errorf("expected nil result, got %v", result2)
	}

	_ = expectedErr // silence unused variable warning
}

// TestWithResolver_UnifiedSignature_NilResult tests nil result handling
func TestWithResolver_UnifiedSignature_NilResult(t *testing.T) {
	type TestResult struct {
		Value string `json:"value"`
	}

	// Test nil return without error
	resolver := NewResolver[TestResult]("test").
		WithResolver(func(p ResolveParams) (*TestResult, error) {
			return nil, nil
		})

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{},
	}

	result, err := resolver.resolver(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

// TestWithResolver_Integration_CompleteFlow tests a complete flow using unified WithResolver
func TestWithResolver_Integration_CompleteFlow(t *testing.T) {
	type User struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	// Clear registry
	typeRegistryMu.Lock()
	delete(typeRegistry, "User")
	typeRegistryMu.Unlock()

	// Build a complete resolver using the unified WithResolver with args
	queryField := NewResolver[User]("user").
		WithArg("id", String).
		WithArg("includeEmail", Boolean).
		WithDescription("Get a user by ID").
		WithResolver(func(p ResolveParams, args Args) (*User, error) {
			id := Get[string](args, "id")
			includeEmail := GetOr[bool](args, "includeEmail", false)

			email := ""
			if includeEmail {
				email = "user@example.com"
			}

			return &User{
				ID:    id,
				Name:  "Test User",
				Email: email,
				Age:   25,
			}, nil
		}).
		BuildQuery()

	// Verify the field is built correctly
	if queryField.Name() != "user" {
		t.Errorf("name = %v, want %v", queryField.Name(), "user")
	}

	field := queryField.Serve()
	if field == nil {
		t.Fatal("Serve() returned nil")
	}

	if field.Description != "Get a user by ID" {
		t.Errorf("description = %v, want %v", field.Description, "Get a user by ID")
	}

	// Check args
	if _, exists := field.Args["id"]; !exists {
		t.Error("should have 'id' argument")
	}
	if _, exists := field.Args["includeEmail"]; !exists {
		t.Error("should have 'includeEmail' argument")
	}

	// Test resolver execution
	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"id":           "user-123",
			"includeEmail": true,
		},
	}

	result, err := field.Resolve(params)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}

	user, ok := result.(*User)
	if !ok {
		t.Fatalf("result should be *User, got %T", result)
	}

	if user.ID != "user-123" {
		t.Errorf("user.ID = %v, want %v", user.ID, "user-123")
	}
	if user.Email != "user@example.com" {
		t.Errorf("user.Email = %v, want %v", user.Email, "user@example.com")
	}
}

// =============================================================================
// BENCHMARKS
// =============================================================================

// BenchmarkArgs_Get benchmarks the Get function for Args
func BenchmarkArgs_Get(b *testing.B) {
	args := NewArgs(map[string]interface{}{
		"id":     "123",
		"name":   "Alice",
		"age":    25,
		"active": true,
		"score":  float64(95.5),
	})

	b.Run("string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[string](args, "name")
		}
	})

	b.Run("int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[int](args, "age")
		}
	})

	b.Run("bool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[bool](args, "active")
		}
	})

	b.Run("float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[float64](args, "score")
		}
	})

	b.Run("missing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[string](args, "missing")
		}
	})
}

// BenchmarkArgs_GetOr benchmarks the GetOr function
func BenchmarkArgs_GetOr(b *testing.B) {
	args := NewArgs(map[string]interface{}{
		"name": "Alice",
	})

	b.Run("existing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetOr[string](args, "name", "default")
		}
	})

	b.Run("missing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetOr[string](args, "missing", "default")
		}
	})
}

// BenchmarkArgs_TypeConversion benchmarks type conversions
func BenchmarkArgs_TypeConversion(b *testing.B) {
	args := NewArgs(map[string]interface{}{
		"float_as_int": float64(42),
		"int_as_float": 42,
	})

	b.Run("float64_to_int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[int](args, "float_as_int")
		}
	})

	b.Run("int_to_float64", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[float64](args, "int_as_float")
		}
	})
}

// BenchmarkWithArg benchmarks the WithArg method
func BenchmarkWithArg(b *testing.B) {
	type Result struct {
		ID string `json:"id"`
	}

	b.Run("scalar_type", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewResolver[Result]("test").
				WithArg("id", String)
		}
	})

	b.Run("multiple_scalars", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewResolver[Result]("test").
				WithArg("id", String).
				WithArg("name", String).
				WithArg("age", Int).
				WithArg("active", Boolean)
		}
	})
}

// BenchmarkWithResolver benchmarks the WithResolver method
func BenchmarkWithResolver(b *testing.B) {
	type Result struct {
		ID string `json:"id"`
	}

	b.Run("without_args", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewResolver[Result]("test").
				WithResolver(func(p ResolveParams) (*Result, error) {
					return &Result{ID: "123"}, nil
				})
		}
	})

	b.Run("with_args", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewResolver[Result]("test").
				WithArg("id", String).
				WithResolver(func(p ResolveParams, args Args) (*Result, error) {
					id := Get[string](args, "id")
					return &Result{ID: id}, nil
				})
		}
	})
}

// BenchmarkResolverExecution benchmarks the actual resolver execution
func BenchmarkResolverExecution(b *testing.B) {
	type User struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// Pre-build the resolver
	resolver := NewResolver[User]("user").
		WithArg("id", String).
		WithArg("name", String).
		WithResolver(func(p ResolveParams, args Args) (*User, error) {
			id := Get[string](args, "id")
			name := Get[string](args, "name")
			return &User{ID: id, Name: name}, nil
		})

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args: map[string]interface{}{
			"id":   "123",
			"name": "Alice",
		},
	}

	b.Run("execute_resolver", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = resolver.resolver(params)
		}
	})
}

// BenchmarkResolverExecution_WithoutArgs benchmarks resolver without args
func BenchmarkResolverExecution_WithoutArgs(b *testing.B) {
	type Message struct {
		Text string `json:"text"`
	}

	resolver := NewResolver[Message]("hello").
		WithResolver(func(p ResolveParams) (*Message, error) {
			return &Message{Text: "Hello, World!"}, nil
		})

	params := graphql.ResolveParams{
		Context: context.Background(),
		Args:    map[string]interface{}{},
	}

	b.Run("execute_resolver", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = resolver.resolver(params)
		}
	})
}

// BenchmarkNewArgs benchmarks Args creation
func BenchmarkNewArgs(b *testing.B) {
	raw := map[string]interface{}{
		"id":     "123",
		"name":   "Alice",
		"age":    25,
		"active": true,
	}

	b.Run("create_args", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewArgs(raw)
		}
	})
}

// BenchmarkResolveInputType benchmarks type resolution
func BenchmarkResolveInputType(b *testing.B) {
	b.Run("graphql_type", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resolveInputType(graphql.String, "test")
		}
	})

	b.Run("go_string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resolveInputType("", "test")
		}
	})

	b.Run("go_int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resolveInputType(0, "test")
		}
	})

	b.Run("go_bool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = resolveInputType(false, "test")
		}
	})
}

// ============================================================
// ArgsMap Tests - Testing unified ArgsGetter interface
// ============================================================

// TestArgsMap_Get tests Get[T] with ArgsMap (p.Args wrapper)
func TestArgsMap_Get(t *testing.T) {
	tests := []struct {
		name     string
		raw      map[string]interface{}
		key      string
		wantStr  string
		wantInt  int
		wantBool bool
	}{
		{
			name:    "get string value",
			raw:     map[string]interface{}{"name": "Alice"},
			key:     "name",
			wantStr: "Alice",
		},
		{
			name:    "get int value",
			raw:     map[string]interface{}{"age": 25},
			key:     "age",
			wantInt: 25,
		},
		{
			name:    "get bool value",
			raw:     map[string]interface{}{"active": true},
			key:     "active",
			wantBool: true,
		},
		{
			name:    "get missing value returns zero",
			raw:     map[string]interface{}{},
			key:     "missing",
			wantStr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsMap := ArgsMap(tt.raw)

			if tt.wantStr != "" || tt.key == "name" || tt.key == "missing" {
				got := Get[string](argsMap, tt.key)
				if got != tt.wantStr {
					t.Errorf("Get[string](ArgsMap) = %v, want %v", got, tt.wantStr)
				}
			}
			if tt.wantInt != 0 {
				got := Get[int](argsMap, tt.key)
				if got != tt.wantInt {
					t.Errorf("Get[int](ArgsMap) = %v, want %v", got, tt.wantInt)
				}
			}
			if tt.wantBool {
				got := Get[bool](argsMap, tt.key)
				if got != tt.wantBool {
					t.Errorf("Get[bool](ArgsMap) = %v, want %v", got, tt.wantBool)
				}
			}
		})
	}
}

// TestArgsMap_GetOr tests GetOr[T] with ArgsMap
func TestArgsMap_GetOr(t *testing.T) {
	tests := []struct {
		name       string
		raw        map[string]interface{}
		key        string
		defaultVal string
		want       string
	}{
		{
			name:       "returns value when present",
			raw:        map[string]interface{}{"name": "Alice"},
			key:        "name",
			defaultVal: "default",
			want:       "Alice",
		},
		{
			name:       "returns default when missing",
			raw:        map[string]interface{}{},
			key:        "name",
			defaultVal: "default",
			want:       "default",
		},
		{
			name:       "returns default when nil map",
			raw:        nil,
			key:        "name",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsMap := ArgsMap(tt.raw)
			got := GetOr[string](argsMap, tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetOr(ArgsMap) = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestArgsMap_GetE tests GetE[T] with ArgsMap
func TestArgsMap_GetE(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"name": "Alice"})
		val, err := GetE[string](argsMap, "name")
		if err != nil {
			t.Errorf("GetE(ArgsMap) unexpected error: %v", err)
		}
		if val != "Alice" {
			t.Errorf("GetE(ArgsMap) = %v, want Alice", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{})
		_, err := GetE[string](argsMap, "missing")
		if err == nil {
			t.Error("GetE(ArgsMap) expected error for missing key")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		// Use a type that truly cannot be converted (slice to string)
		argsMap := ArgsMap(map[string]interface{}{"name": []int{1, 2, 3}})
		_, err := GetE[string](argsMap, "name")
		if err == nil {
			t.Error("GetE(ArgsMap) expected error for wrong type")
		}
	})

	t.Run("nil map", func(t *testing.T) {
		argsMap := ArgsMap(nil)
		_, err := GetE[string](argsMap, "name")
		if err == nil {
			t.Error("GetE(ArgsMap) expected error for nil map")
		}
	})
}

// TestArgsMap_MustGet tests MustGet[T] with ArgsMap
func TestArgsMap_MustGet(t *testing.T) {
	t.Run("valid value", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"name": "Alice"})
		val := MustGet[string](argsMap, "name")
		if val != "Alice" {
			t.Errorf("MustGet(ArgsMap) = %v, want Alice", val)
		}
	})

	t.Run("missing key panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustGet(ArgsMap) expected panic for missing key")
			}
		}()
		argsMap := ArgsMap(map[string]interface{}{})
		_ = MustGet[string](argsMap, "missing")
	})

	t.Run("wrong type panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("MustGet(ArgsMap) expected panic for wrong type")
			}
		}()
		// Use a type that truly cannot be converted (slice to string)
		argsMap := ArgsMap(map[string]interface{}{"name": []int{1, 2, 3}})
		_ = MustGet[string](argsMap, "name")
	})
}

// TestArgsMap_GetArg tests GetArg method directly
func TestArgsMap_GetArg(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"id": "123"})
		val, exists := argsMap.GetArg("id")
		if !exists {
			t.Error("GetArg should return true for existing key")
		}
		if val != "123" {
			t.Errorf("GetArg = %v, want 123", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{})
		_, exists := argsMap.GetArg("missing")
		if exists {
			t.Error("GetArg should return false for missing key")
		}
	})

	t.Run("nil map", func(t *testing.T) {
		argsMap := ArgsMap(nil)
		_, exists := argsMap.GetArg("any")
		if exists {
			t.Error("GetArg should return false for nil map")
		}
	})
}

// TestArgsMap_StructConversion tests Get[T] struct conversion with ArgsMap
func TestArgsMap_StructConversion(t *testing.T) {
	t.Run("convert map to struct", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Alice",
				"email": "alice@example.com",
			},
		})
		input := Get[TestUserInput](argsMap, "input")
		if input.Name != "Alice" {
			t.Errorf("Get[TestUserInput](ArgsMap).Name = %v, want Alice", input.Name)
		}
		if input.Email != "alice@example.com" {
			t.Errorf("Get[TestUserInput](ArgsMap).Email = %v, want alice@example.com", input.Email)
		}
	})
}

// TestArgsMap_TypeConversion tests type conversions (float64 to int, etc.)
func TestArgsMap_TypeConversion(t *testing.T) {
	t.Run("float64 to int", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"num": float64(42)})
		got := Get[int](argsMap, "num")
		if got != 42 {
			t.Errorf("Get[int](ArgsMap) with float64 = %v, want 42", got)
		}
	})

	t.Run("int to float64", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"num": 42})
		got := Get[float64](argsMap, "num")
		if got != float64(42) {
			t.Errorf("Get[float64](ArgsMap) with int = %v, want 42.0", got)
		}
	})
}

// TestArgsGetter_Unified tests that both Args and ArgsMap work with same function
func TestArgsGetter_Unified(t *testing.T) {
	// Helper function that accepts ArgsGetter interface
	extractID := func(a ArgsGetter) string {
		return Get[string](a, "id")
	}

	t.Run("with Args", func(t *testing.T) {
		args := NewArgs(map[string]interface{}{"id": "123"})
		id := extractID(args)
		if id != "123" {
			t.Errorf("extractID(Args) = %v, want 123", id)
		}
	})

	t.Run("with ArgsMap", func(t *testing.T) {
		argsMap := ArgsMap(map[string]interface{}{"id": "456"})
		id := extractID(argsMap)
		if id != "456" {
			t.Errorf("extractID(ArgsMap) = %v, want 456", id)
		}
	})
}

// ============================================================
// ArgsMap Benchmarks
// ============================================================

// BenchmarkArgsMap_Get benchmarks Get[T] with ArgsMap
func BenchmarkArgsMap_Get(b *testing.B) {
	raw := map[string]interface{}{
		"id":     "123",
		"name":   "Alice",
		"age":    25,
		"active": true,
	}
	argsMap := ArgsMap(raw)

	b.Run("get_string", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[string](argsMap, "name")
		}
	})

	b.Run("get_int", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[int](argsMap, "age")
		}
	})

	b.Run("get_bool", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[bool](argsMap, "active")
		}
	})

	b.Run("get_missing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Get[string](argsMap, "missing")
		}
	})
}

// BenchmarkArgsMap_GetOr benchmarks GetOr[T] with ArgsMap
func BenchmarkArgsMap_GetOr(b *testing.B) {
	argsMap := ArgsMap(map[string]interface{}{"name": "Alice"})

	b.Run("existing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetOr[string](argsMap, "name", "default")
		}
	})

	b.Run("missing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = GetOr[string](argsMap, "missing", "default")
		}
	})
}

// BenchmarkArgsMap_GetE benchmarks GetE[T] with ArgsMap
func BenchmarkArgsMap_GetE(b *testing.B) {
	argsMap := ArgsMap(map[string]interface{}{"name": "Alice"})

	b.Run("valid_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = GetE[string](argsMap, "name")
		}
	})

	b.Run("missing_key", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = GetE[string](argsMap, "missing")
		}
	})
}

// BenchmarkArgsMap_Creation benchmarks ArgsMap creation
func BenchmarkArgsMap_Creation(b *testing.B) {
	raw := map[string]interface{}{
		"id":     "123",
		"name":   "Alice",
		"age":    25,
		"active": true,
	}

	b.Run("create_argsmap", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ArgsMap(raw)
		}
	})
}

// BenchmarkArgsGetter_Comparison compares Args vs ArgsMap performance
func BenchmarkArgsGetter_Comparison(b *testing.B) {
	raw := map[string]interface{}{
		"id":   "123",
		"name": "Alice",
	}

	b.Run("Args_Get", func(b *testing.B) {
		args := NewArgs(raw)
		for i := 0; i < b.N; i++ {
			_ = Get[string](args, "name")
		}
	})

	b.Run("ArgsMap_Get", func(b *testing.B) {
		argsMap := ArgsMap(raw)
		for i := 0; i < b.N; i++ {
			_ = Get[string](argsMap, "name")
		}
	})
}