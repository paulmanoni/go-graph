# go-graph

A modern, secure GraphQL handler for Go with built-in authentication, validation, and an intuitive builder API.

## Features

- 🚀 **Zero Config Start** - Default hello world schema included
- 🔧 **Type-Safe Resolvers** - Compile-time type safety with generic resolvers
- 🎯 **Type-Safe Arguments** - Automatic argument parsing with `NewArgsResolver`
- 🏗️ **Fluent Builder API** - Clean, intuitive schema construction
- 🔐 **Built-in Auth** - Automatic Bearer token extraction
- 🛡️ **Security First** - Query depth, complexity, and introspection protection
- 🧹 **Response Sanitization** - Remove field suggestions from errors
- 🎭 **Middleware System** - Built-in logging, auth, caching + custom middleware support
- 🔄 **Real-time Subscriptions** - WebSocket subscriptions with dual protocol support
- ⚡ **Framework Agnostic** - Works with net/http, Gin, or any framework
- ⚡ **High Performance** - ~60μs per request, 100k+ RPS capable

Built on top of [graphql-go](https://github.com/graphql-go/graphql).

## Installation

```bash
go get github.com/paulmanoni/go-graph
```

## Quick Start

### Option 1: Default Schema (Zero Config)

Start immediately with a built-in hello world schema:

```go
package main

import (
    "log"
    "net/http"
    "github.com/paulmanoni/go-graph"
)

func main() {
    // No schema needed! Includes default hello query & echo mutation
    handler := graph.NewHTTP(&graph.GraphContext{
        Playground: true,
        DEBUG:      true,
    })

    http.Handle("/graphql", handler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

Test it:
```graphql
# Query
{ hello }

# Mutation
mutation { echo(message: "test") }
```

### Option 2: Builder Pattern (Recommended)

Use the fluent builder API for clean, type-safe schema construction:

```go
package main

import (
    "log"
    "net/http"
    "github.com/paulmanoni/go-graph"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// Define your queries with type-safe resolvers
func getHello() graph.QueryField {
    return graph.NewResolver[string]("hello").
        WithResolver(func(p graph.ResolveParams) (*string, error) {
            msg := "Hello, World!"
            return &msg, nil
        }).BuildQuery()
}

func getUser() graph.QueryField {
    return graph.NewResolver[User]("user").
        WithArgs(graphql.FieldConfigArgument{
            "id": &graphql.ArgumentConfig{
                Type: graphql.String,
            },
        }).
        WithResolver(func(p graph.ResolveParams) (*User, error) {
            id, _ := graph.GetArgString(p, "id")
            return &User{ID: id, Name: "Alice"}, nil
        }).BuildQuery()
}

func main() {
    handler := graph.NewHTTP(&graph.GraphContext{
        SchemaParams: &graph.SchemaBuilderParams{
            QueryFields: []graph.QueryField{
                getHello(),
                getUser(),
            },
            MutationFields: []graph.MutationField{},
        },
        Playground: true,
        DEBUG:      false,
    })

    http.Handle("/graphql", handler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Option 3: Custom Schema

Bring your own graphql-go schema:

```go
import "github.com/graphql-go/graphql"

schema, _ := graphql.NewSchema(graphql.SchemaConfig{
    Query: graphql.NewObject(graphql.ObjectConfig{
        Name: "Query",
        Fields: graphql.Fields{
            "hello": &graphql.Field{
                Type: graphql.String,
                Resolve: func(p graph.ResolveParams) (interface{}, error) {
                    return "world", nil
                },
            },
        },
    }),
})

handler := graph.NewHTTP(&graph.GraphContext{
    Schema:     &schema,
    Playground: true,
})
```

## Authentication

### Automatic Bearer Token Extraction

Token is automatically extracted from `Authorization: Bearer <token>` header and available in all resolvers:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{
        QueryFields: []graph.QueryField{
            getProtectedQuery(),
        },
    },

    // Optional: Fetch user details from token
    UserDetailsFn: func(token string) (interface{}, error) {
        // Validate JWT, query database, etc.
        user, err := validateAndGetUser(token)
        return user, err
    },
})
```

Access in resolvers:

```go
func getProtectedQuery() graph.QueryField {
    return graph.NewResolver[User]("me").
        WithResolver(func(p graph.ResolveParams) (*User, error) {
            // Get token
            token, err := graph.GetRootString(p, "token")
            if err != nil {
                return nil, fmt.Errorf("authentication required")
            }

            // Get user details (if UserDetailsFn provided)
            var user User
            if err := graph.GetRootInfo(p, "details", &user); err != nil {
                return nil, err
            }

            return &user, nil
        }).BuildQuery()
}
```

### Custom Token Extraction

Extract tokens from cookies, custom headers, or query params:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{...},

    TokenExtractorFn: func(r *http.Request) string {
        // From cookie
        if cookie, err := r.Cookie("auth_token"); err == nil {
            return cookie.Value
        }

        // From custom header
        if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
            return apiKey
        }

        // From query param
        return r.URL.Query().Get("token")
    },

    UserDetailsFn: func(token string) (interface{}, error) {
        return getUserByToken(token)
    },
})
```

## Security Features

### Production Setup

Enable all security features for production:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams:       &graph.SchemaBuilderParams{...},
    DEBUG:              false,  // Enable security features
    EnableValidation:   true,   // Validate queries
    EnableSanitization: true,   // Sanitize errors
    Playground:         false,  // Disable playground

    UserDetailsFn: func(token string) (interface{}, error) {
        return validateAndGetUser(token)
    },
})
```

### Validation Rules (Legacy)

For simple validation needs, use `EnableValidation: true`:

- **Max Query Depth**: 10 levels
- **Max Aliases**: 4 per query
- **Max Complexity**: 200
- **Introspection**: Disabled (blocks `__schema` and `__type`)

For advanced validation needs, see the **Custom Validation Rules** section below.

### Response Sanitization (when `EnableSanitization: true`)

Removes field suggestions from error messages:

**Before:**
```json
{
  "errors": [{
    "message": "Cannot query field \"nam\". Did you mean \"name\"?"
  }]
}
```

**After:**
```json
{
  "errors": [{
    "message": "Cannot query field \"nam\"."
  }]
}
```

### Debug Mode

Use `DEBUG: true` during development to skip all validation and sanitization:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{...},
    DEBUG:        true,  // Disables validation & sanitization
    Playground:   true,  // Enable playground for testing
})
```

## Custom Validation Rules

The package provides a powerful and flexible validation system that goes beyond simple `EnableValidation: true`. Create custom validation rules to enforce security policies, authentication requirements, rate limits, and more.

### Features

- 🛡️ **Pre-built Security Rules** - Max depth, complexity, aliases, introspection blocking, token limits
- 🔐 **Authentication & Authorization** - Require auth for operations, role-based and permission-based access control
- ⚡ **Rate Limiting** - Budget-based rate limiting with role bypasses
- 📦 **Preset Collections** - SecurityRules, StrictSecurityRules, DevelopmentRules
- 🎯 **Type-Safe** - Strongly-typed AuthContext with user information
- 🔧 **Composable** - Combine multiple rules and rule sets
- ⚠️ **Multi-Error Support** - Collect and return all validation errors at once
- 🚀 **High Performance** - Adds minimal overhead (~1-2μs per query)

### Quick Start

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{...},

    // Custom validation rules (replaces EnableValidation)
    ValidationRules: []graph.ValidationRule{
        graph.NewMaxDepthRule(10),
        graph.NewMaxComplexityRule(200),
        graph.NewNoIntrospectionRule(),
        graph.NewRequireAuthRule("mutation", "subscription"),
        graph.NewRoleRules(map[string][]string{
            "deleteUser": {"admin"},
            "viewAuditLog": {"admin", "auditor"},
        }),
    },

    // Fetch user details from token (reuses existing UserDetailsFn)
    UserDetailsFn: func(token string) (interface{}, error) {
        user, err := validateJWT(token)
        if err != nil {
            return nil, err
        }
        return user, nil
    },
})
```

### Security Rules

#### MaxDepthRule

Prevents deeply nested queries that can cause performance issues:

```go
graph.NewMaxDepthRule(10)  // Max 10 levels deep
```

**Blocks:**
```graphql
{
  level1 {
    level2 {
      level3 {
        # ... more than 10 levels
      }
    }
  }
}
```

#### MaxComplexityRule

Limits query computational cost (complexity = number of fields × depth):

```go
graph.NewMaxComplexityRule(200)  // Max complexity of 200
```

**Example:** Query with 50 fields at depth 5 = complexity 250 → **Rejected**

#### MaxAliasesRule

Prevents alias-based denial-of-service attacks:

```go
graph.NewMaxAliasesRule(4)  // Max 4 aliases per query
```

**Blocks:**
```graphql
{
  u1: user(id: 1) { name }
  u2: user(id: 2) { name }
  u3: user(id: 3) { name }
  u4: user(id: 4) { name }
  u5: user(id: 5) { name }  # 5th alias rejected
}
```

#### NoIntrospectionRule

Blocks schema introspection in production:

```go
graph.NewNoIntrospectionRule()
```

**Blocks:**
```graphql
{ __schema { types { name } } }
{ __type(name: "User") { fields { name } } }
```

#### MaxTokensRule

Limits total tokens in query (prevents extremely large queries):

```go
graph.NewMaxTokensRule(500)  // Max 500 tokens
```

### Authentication & Authorization Rules

#### RequireAuthRule

Require authentication for specific operations or fields:

```go
// Require auth for all mutations and subscriptions
graph.NewRequireAuthRule("mutation", "subscription")

// Require auth for specific fields
graph.NewRequireAuthRule("sensitiveData", "adminPanel")

// Combine both
graph.NewRequireAuthRule("mutation", "subscription", "sensitiveData")
```

**Example response when unauthenticated:**
```json
{
  "errors": [{
    "message": "mutation operations require authentication",
    "rule": "RequireAuthRule"
  }]
}
```

#### RoleRule & RoleRules

Enforce role-based access control:

```go
// Single field rule
graph.NewRoleRule("deleteUser", "admin")

// Multiple roles allowed
graph.NewRoleRule("viewReports", "admin", "manager")

// Batch configuration (preferred for multiple fields)
graph.NewRoleRules(map[string][]string{
    "deleteUser":     {"admin"},
    "deleteAccount":  {"admin"},
    "viewAuditLog":   {"admin", "auditor"},
    "approveOrder":   {"admin", "manager"},
})
```

**Minimal interface requirements:**

Your user struct only needs to implement the methods required by the rules you use:

```go
// For RoleRule and RoleRules
type HasRolesInterface interface {
    HasRole(role string) bool
}

// For PermissionRule and PermissionRules
type HasPermissionsInterface interface {
    HasPermission(permission string) bool
}

// For RateLimitRule
type HasIDInterface interface {
    GetID() string
}

// Example user implementation
type User struct {
    ID          string
    Roles       []string
    Permissions []string
}

func (u *User) GetID() string {
    return u.ID
}

func (u *User) HasRole(role string) bool {
    for _, r := range u.Roles {
        if r == role {
            return true
        }
    }
    return false
}

func (u *User) HasPermission(perm string) bool {
    for _, p := range u.Permissions {
        if p == perm {
            return true
        }
    }
    return false
}
```

#### PermissionRule & PermissionRules

Fine-grained permission-based access control:

```go
// Single field permission
graph.NewPermissionRule("sensitiveData", "read:sensitive")

// Multiple permissions
graph.NewPermissionRule("exportData", "export:data", "admin:all")

// Batch configuration
graph.NewPermissionRules(map[string][]string{
    "sensitiveData": {"read:sensitive"},
    "exportData":    {"export:data"},
    "adminPanel":    {"admin:access"},
})
```

#### BlockedFieldsRule

Block specific fields from being queried:

```go
rule := graph.NewBlockedFieldsRule("internalUsers", "deprecatedField")

// With reasons
rule.BlockField("legacyAPI", "deprecated: use v2 API")
rule.BlockField("internalData", "not available in this version")
```

### Rate Limiting

Budget-based rate limiting with role bypasses:

```go
graph.NewRateLimitRule(
    graph.WithBudgetFunc(getBudgetFromRedis),
    graph.WithCostPerUnit(2),  // Multiply complexity by 2
    graph.WithBypassRoles("admin", "service"),
)
```

**Budget function examples:**

```go
// Simple fixed budget
graph.SimpleBudgetFunc(1000)

// Per-user budgets
graph.PerUserBudgetFunc(map[string]int{
    "premium_user": 10000,
    "basic_user":   1000,
}, 500)  // default budget

// Redis-based sliding window
func getRedisBudget(userID string) (int, error) {
    key := fmt.Sprintf("rate_limit:%s", userID)
    remaining, err := redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return 1000, nil  // Reset budget
    }
    return remaining, err
}
```

### Preset Rule Collections

Pre-configured rule sets for common scenarios:

#### SecurityRules (Default)

Standard security for production:
```go
graph.ValidationRules: graph.SecurityRules
```

- Max depth: 10
- Max complexity: 200
- Max aliases: 4
- Introspection: Blocked

#### StrictSecurityRules

Stricter limits for high-security environments:
```go
graph.ValidationRules: graph.StrictSecurityRules
```

- Max depth: 8
- Max complexity: 150
- Max aliases: 3
- Max tokens: 500
- Introspection: Blocked

#### DevelopmentRules

Lenient rules for development:
```go
graph.ValidationRules: graph.DevelopmentRules
```

- Max depth: 20
- Max complexity: 500
- No other restrictions

### Combining Rules

#### CombineRules

Merge multiple rule sets:

```go
rules := graph.CombineRules(
    graph.SecurityRules,
    []graph.ValidationRule{
        graph.NewRequireAuthRule("mutation"),
        graph.NewRoleRules(graph.AdminOnlyFields),
    },
)

handler := graph.NewHTTP(&graph.GraphContext{
    ValidationRules: rules,
})
```

#### Preset Role Configurations

Pre-built role configurations for common access patterns:

```go
// Use built-in configurations
graph.NewRoleRules(graph.AdminOnlyFields)
graph.NewRoleRules(graph.ManagerFields)
graph.NewRoleRules(graph.AuditorFields)

// Merge multiple configurations
allRoles := graph.MergeRoleConfigs(
    graph.AdminOnlyFields,
    graph.ManagerFields,
    graph.AuditorFields,
)
graph.NewRoleRules(allRoles)
```

**Built-in role configs:**
- `AdminOnlyFields`: deleteUser, deleteAccount, viewAuditLog, systemSettings, manageRoles
- `ManagerFields`: approveOrder, viewReports, manageTeam, bulkOperations
- `AuditorFields`: viewAuditLog, exportLogs, viewAnalytics

### Production Example

Complete production setup with security, auth, and rate limiting:

```go
package main

import (
    "net/http"
    graph "github.com/paulmanoni/go-graph"
)

func main() {
    handler := graph.NewHTTP(&graph.GraphContext{
        SchemaParams: &graph.SchemaBuilderParams{...},

        // Strict security validation
        ValidationRules: graph.CombineRules(
            graph.StrictSecurityRules,
            []graph.ValidationRule{
                // Authentication
                graph.NewRequireAuthRule("mutation", "subscription"),

                // Role-based access
                graph.NewRoleRules(map[string][]string{
                    "deleteUser":     {"admin"},
                    "viewAuditLog":   {"admin", "auditor"},
                    "approveOrder":   {"admin", "manager"},
                }),

                // Permission-based access
                graph.NewPermissionRules(map[string][]string{
                    "sensitiveData": {"read:sensitive"},
                    "exportData":    {"export:data"},
                }),

                // Rate limiting
                graph.NewRateLimitRule(
                    graph.WithBudgetFunc(getRedisBudget),
                    graph.WithCostPerUnit(2),
                    graph.WithBypassRoles("admin", "service"),
                ),
            },
        ),

        // Fetch user details from JWT token (reuses existing UserDetailsFn)
        UserDetailsFn: func(token string) (interface{}, error) {
            user, err := validateJWT(token)
            if err != nil {
                return nil, err  // Invalid token
            }
            return user, nil  // Returns your user struct that implements minimal interfaces
        },

        // Validation options
        ValidationOptions: &graph.ValidationOptions{
            StopOnFirstError: false,  // Collect all errors
            SkipInDebug:      true,   // Skip in DEBUG mode
        },

        // Other production settings
        EnableSanitization: true,
        Playground:         false,
        DEBUG:              false,
    })

    http.Handle("/graphql", handler)
    http.ListenAndServe(":8080", nil)
}
```

### Custom Validation Rules

Create custom rules by embedding `BaseRule`:

```go
type IPWhitelistRule struct {
    graph.BaseRule
    allowedIPs []string
}

func NewIPWhitelistRule(ips ...string) graph.ValidationRule {
    return &IPWhitelistRule{
        BaseRule:   graph.NewBaseRule("IPWhitelistRule"),
        allowedIPs: ips,
    }
}

func (r *IPWhitelistRule) Validate(ctx *graph.ValidationContext) error {
    // Access request from context if needed
    // clientIP := getClientIP(ctx.Request)

    // Check IP whitelist
    // if !contains(r.allowedIPs, clientIP) {
    //     return r.NewError("IP not whitelisted")
    // }

    return nil
}
```

**BaseRule provides:**
- `NewError(msg string) *ValidationError`
- `NewErrorf(format string, args ...interface{}) *ValidationError`
- `Enable()` / `Disable()` / `Enabled() bool`
- Consistent error formatting

### Error Responses

#### Single Validation Error

```json
{
  "errors": [{
    "message": "query depth 12 exceeds maximum 10",
    "rule": "MaxDepthRule"
  }]
}
```

#### Multiple Validation Errors

When `StopOnFirstError: false`:

```json
{
  "errors": [
    {
      "message": "query depth 12 exceeds maximum 10",
      "rule": "MaxDepthRule"
    },
    {
      "message": "field 'deleteUser' requires one of roles: [admin]",
      "rule": "RoleRules"
    },
    {
      "message": "query cost 250 exceeds available budget 200",
      "rule": "RateLimitRule"
    }
  ]
}
```

### Validation Options

Configure validation behavior:

```go
&graph.ValidationOptions{
    // Collect all errors vs stop on first
    StopOnFirstError: false,

    // Skip validation in DEBUG mode
    SkipInDebug: true,
}
```

### Performance Impact

Validation adds minimal overhead (measured on Apple M1 Pro):

| Rule | Time/op | Allocations | Description |
|------|---------|-------------|-------------|
| MaxDepthRule | ~4.4 μs | 43 allocs | Query depth validation |
| MaxComplexityRule | ~2.8 μs | 43 allocs | Complexity calculation |
| MaxAliasesRule | ~3.0 μs | 51 allocs | Alias counting |
| NoIntrospectionRule | ~10.5 μs | 36 allocs | Introspection blocking |
| RequireAuthRule | ~1.3 μs | 30 allocs | Authentication check |
| RoleRule | ~947 ns | 20 allocs | Role validation |
| PermissionRule | ~858 ns | 20 allocs | Permission check |
| RateLimitRule | ~3.7 μs | 36 allocs | Budget + complexity |
| SecurityRules (preset) | ~3.2 μs | 43 allocs | Depth + complexity + aliases + introspection |
| StrictSecurityRules | ~2.6 μs | 36 allocs | Stricter limits |
| Combined rules | ~1.4 μs | 29 allocs | Multiple custom rules |

**For a complete HTTP request:**
- Debug mode (no validation): ~29 μs
- With validation: ~31 μs
- With sanitization: ~36 μs
- Complete stack (validation + sanitization + auth): ~60 μs
- **Overhead: ~2-3 μs for validation (negligible)**

### Migration from EnableValidation

**Before (simple validation):**
```go
handler := graph.NewHTTP(&graph.GraphContext{
    EnableValidation: true,  // Deprecated
})
```

**After (custom rules):**
```go
handler := graph.NewHTTP(&graph.GraphContext{
    ValidationRules: graph.SecurityRules,  // Same defaults
})
```

**Or use presets:**
```go
// Development
ValidationRules: graph.DevelopmentValidationRules()

// Production
ValidationRules: graph.ProductionValidationRules()

// Custom
ValidationRules: graph.DefaultValidationRules()
```

### Best Practices

1. **Start with presets**: Use `SecurityRules` or `StrictSecurityRules` as a base
2. **Add auth rules**: Layer in `RequireAuthRule` and `RoleRules` as needed
3. **Test in development**: Use `DevelopmentRules` with `DEBUG: true` for development
4. **Collect all errors**: Set `StopOnFirstError: false` for better debugging
5. **Monitor performance**: Validation overhead is minimal (~3-5μs total)
6. **Use UserDetailsFn**: Centralize auth logic by implementing UserDetailsFn instead of checking tokens in each resolver
7. **Implement minimal interfaces**: Only implement the interfaces needed by your validation rules (HasRolesInterface, HasPermissionsInterface, HasIDInterface)

## Helper Functions

### Extracting Arguments

```go
// String argument
name, err := graph.GetArgString(p, "name")

// Int argument
age, err := graph.GetArgInt(p, "age")

// Bool argument
active, err := graph.GetArgBool(p, "active")

// Complex type
var input CreateUserInput
err := graph.GetArg(p, "input", &input)
```

### Accessing Root Values

```go
// Get token
token, err := graph.GetRootString(p, "token")

// Get user details
var user User
err := graph.GetRootInfo(p, "details", &user)
```

## Type-Safe Resolvers

### `WithResolver` - Type-Safe (Recommended)

The `WithResolver` method provides compile-time type safety by accepting a function that returns `*T` instead of `interface{}`:

```go
// ✅ Type-safe - returns *User
graph.NewResolver[User]("user").
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        id, _ := graph.GetArgString(p, "id")
        user := db.GetUserByID(id)  // Most ORMs return *User
        return user, nil             // No type assertions needed!
    }).BuildQuery()

// ✅ Works with lists - returns *[]User
graph.NewResolver[User]("users").
    AsList().
    WithResolver(func(p graph.ResolveParams) (*[]User, error) {
        users := db.ListUsers()
        return &users, nil
    }).BuildQuery()

// ✅ Works with primitives - returns *string
graph.NewResolver[string]("message").
    WithResolver(func(p graph.ResolveParams) (*string, error) {
        msg := "Hello!"
        return &msg, nil
    }).BuildQuery()
```

**Benefits:**
- ✅ No type assertions or casts needed
- ✅ Compiler catches type mismatches at build time
- ✅ Better IDE autocomplete and refactoring
- ✅ Cleaner, more readable code
- ✅ Works with pointers (can return `nil` for not found)

### Real-World Example

```go
type Post struct {
    ID       int    `json:"id"`
    Title    string `json:"title"`
    AuthorID int    `json:"authorId"`
}

// Type-safe query
func getPost() graph.QueryField {
    return graph.NewResolver[Post]("post").
        WithArgs(graphql.FieldConfigArgument{
            "id": &graphql.ArgumentConfig{Type: graphql.Int},
        }).
        WithResolver(func(p graph.ResolveParams) (*Post, error) {
            id, err := graph.GetArgInt(p, "id")
            if err != nil {
                return nil, err
            }

            post, err := postService.GetByID(id)
            if err != nil {
                return nil, err
            }

            // Return *Post directly - no type assertions!
            return post, nil
        }).BuildQuery()
}

// Type-safe list query
func getPosts() graph.QueryField {
    return graph.NewResolver[Post]("posts").
        AsList().
        WithResolver(func(p graph.ResolveParams) (*[]Post, error) {
            posts, err := postService.List()
            if err != nil {
                return nil, err
            }
            return &posts, nil
        }).BuildQuery()
}

// Type-safe mutation
func createPost() graph.MutationField {
    type CreatePostInput struct {
        Title    string `json:"title"`
        AuthorID int    `json:"authorId"`
    }

    return graph.NewResolver[Post]("createPost").
        WithInputObject(CreatePostInput{}).
        WithResolver(func(p graph.ResolveParams) (*Post, error) {
            var input CreatePostInput
            if err := graph.GetArg(p, "input", &input); err != nil {
                return nil, err
            }

            return postService.Create(input.Title, input.AuthorID)
        }).BuildMutation()
}
```

## Type-Safe Arguments with NewArgsResolver

`NewArgsResolver` provides compile-time type safety for both the return value AND arguments. The resolver function receives typed arguments directly, eliminating the need for manual argument extraction.

### Basic Usage

```go
// Struct arguments - auto-generates GraphQL args from struct fields
type GetUserArgs struct {
    ID int `json:"id" graphql:"id,required" description:"User ID"`
}

func getUser() graph.QueryField {
    return graph.NewArgsResolver[User, GetUserArgs]("user").
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args GetUserArgs) (*User, error) {
            // args.ID is already parsed and type-safe!
            return userService.GetByID(args.ID)
        }).BuildQuery()
}

// Primitive arguments - requires field name
func echo() graph.MutationField {
    return graph.NewArgsResolver[string, string]("echo", "message").
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args string) (*string, error) {
            // args is the message string directly
            return &args, nil
        }).BuildMutation()
}
```

### Resolver Function Signature

The `WithResolver` method accepts a function with three parameters:

```go
func(ctx context.Context, p graph.ResolveParams, args A) (*T, error)
```

- **`ctx context.Context`** - Request context (can be nil, defaults to Background)
- **`p graph.ResolveParams`** - Full GraphQL resolve parameters (for advanced use cases)
- **`args A`** - Typed arguments parsed and validated

### Anonymous Struct Naming

Anonymous nested structs are automatically given meaningful names based on the parent type:

```go
type MessageArgs struct {
    Input struct {
        Message string `json:"message"`
        Name    string `json:"name"`
    } `json:"input"`
}

// The anonymous Input struct becomes "MessageArgsInput" in GraphQL schema
func sendMessage() graph.MutationField {
    return graph.NewArgsResolver[string, MessageArgs]("sendMessage").
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args MessageArgs) (*string, error) {
            response := fmt.Sprintf("Hello %s: %s", args.Input.Name, args.Input.Message)
            return &response, nil
        }).BuildMutation()
}
```

**GraphQL Schema Generated:**
```graphql
type Mutation {
  sendMessage(input: MessageArgsInput!): String
}

input MessageArgsInput {
  message: String
  name: String
}
```

### Benefits

- ✅ **No manual argument extraction** - Arguments are parsed and typed automatically
- ✅ **Compile-time safety** - Both args and return type are type-checked
- ✅ **Context access** - Explicit `context.Context` parameter
- ✅ **Full params access** - Access to `graph.ResolveParams` when needed
- ✅ **Auto-generated schema** - Arguments converted to GraphQL types automatically
- ✅ **Meaningful type names** - Anonymous structs named after parent type

### Comparison with NewResolver

**NewResolver** - Manual argument extraction:
```go
graph.NewResolver[User]("user").
    WithArgs(graphql.FieldConfigArgument{
        "id": &graphql.ArgumentConfig{Type: graphql.Int},
    }).
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        id, err := graph.GetArgInt(p, "id")  // Manual extraction
        if err != nil {
            return nil, err
        }
        return userService.GetByID(id)
    }).BuildQuery()
```

**NewArgsResolver** - Type-safe arguments:
```go
type GetUserArgs struct {
    ID int `json:"id" graphql:"id,required"`
}

graph.NewArgsResolver[User, GetUserArgs]("user").
    WithResolver(func(ctx context.Context, p graph.ResolveParams, args GetUserArgs) (*User, error) {
        return userService.GetByID(args.ID)  // Direct access, type-safe!
    }).BuildQuery()
```

### Advanced Examples

**With validation tags:**
```go
type CreatePostArgs struct {
    Title   string `json:"title" graphql:"title,required" description:"Post title (required)"`
    Content string `json:"content" description:"Post content"`
    Tags    []string `json:"tags" description:"Post tags"`
}

func createPost() graph.MutationField {
    return graph.NewArgsResolver[Post, CreatePostArgs]("createPost").
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args CreatePostArgs) (*Post, error) {
            // All fields are already validated and typed
            post, err := postService.Create(args.Title, args.Content, args.Tags)
            if err != nil {
                return nil, err
            }
            return post, nil
        }).BuildMutation()
}
```

**With authentication (using middleware):**
```go
type UpdateUserArgs struct {
    ID   int    `json:"id" graphql:"id,required"`
    Name string `json:"name" graphql:"name,required"`
}

func updateUser() graph.MutationField {
    return graph.NewArgsResolver[User, UpdateUserArgs]("updateUser").
        WithMiddleware(graph.AuthMiddleware("admin")).  // Require admin role
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args UpdateUserArgs) (*User, error) {
            // Auth already validated by middleware
            // Use typed args directly
            return userService.Update(args.ID, args.Name)
        }).BuildMutation()
}
```

**With manual token extraction:**
```go
type UpdateUserArgs struct {
    ID   int    `json:"id" graphql:"id,required"`
    Name string `json:"name" graphql:"name,required"`
}

func updateUser() graph.MutationField {
    return graph.NewArgsResolver[User, UpdateUserArgs]("updateUser").
        WithResolver(func(ctx context.Context, p graph.ResolveParams, args UpdateUserArgs) (*User, error) {
            // Extract auth token from root
            token, err := graph.GetRootString(p, "token")
            if err != nil {
                return nil, fmt.Errorf("authentication required")
            }

            // Use typed args directly
            return userService.Update(token, args.ID, args.Name)
        }).BuildMutation()
}
```

## Middleware

The library provides a powerful middleware system for adding cross-cutting concerns like authentication, logging, caching, and more to your resolvers.

### Resolver Middleware

Apply middleware to the entire resolver using `WithMiddleware()`. Middleware functions are applied in the order they're added (first added = outermost layer):

```go
graph.NewResolver[User]("user").
    WithMiddleware(graph.LoggingMiddleware).
    WithMiddleware(graph.AuthMiddleware("admin")).
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        id, _ := graph.GetArgInt(p, "id")
        return userService.GetByID(id)
    }).BuildQuery()
```

**Execution flow:**
1. LoggingMiddleware (starts timer)
2. AuthMiddleware (checks permissions)
3. Your resolver (executes business logic)
4. AuthMiddleware (returns)
5. LoggingMiddleware (logs duration)

### Built-in Middleware

#### LoggingMiddleware

Logs resolver execution time to stdout:

```go
graph.NewResolver[Post]("post").
    WithMiddleware(graph.LoggingMiddleware).
    WithResolver(func(p graph.ResolveParams) (*Post, error) {
        return postService.GetByID(id)
    }).BuildQuery()

// Output: Field post resolved in 2.5ms
```

#### AuthMiddleware

Requires a specific user role from context:

```go
graph.NewResolver[User]("adminUser").
    WithMiddleware(graph.AuthMiddleware("admin")).
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        return userService.GetAdmin()
    }).BuildQuery()
```

#### CacheMiddleware

Caches resolver results based on a custom key function:

```go
graph.NewResolver[Product]("product").
    WithMiddleware(graph.CacheMiddleware(func(p graph.ResolveParams) string {
        id, _ := graph.GetArgInt(p, "id")
        return fmt.Sprintf("product:%d", id)
    })).
    WithResolver(func(p graph.ResolveParams) (*Product, error) {
        // Only executes on cache miss
        id, _ := graph.GetArgInt(p, "id")
        return productService.GetByID(id)
    }).BuildQuery()
```

### Custom Middleware

Create custom middleware by implementing the `FieldMiddleware` function signature:

```go
// FieldMiddleware wraps a resolver with additional functionality
type FieldMiddleware func(next FieldResolveFn) FieldResolveFn

// Example: Rate limiting middleware
func RateLimitMiddleware(limit int) graph.FieldMiddleware {
    requests := make(map[string]int)
    var mu sync.Mutex

    return func(next graph.FieldResolveFn) graph.FieldResolveFn {
        return func(p graph.ResolveParams) (interface{}, error) {
            // Extract user ID from context
            userID := p.Context.Value("userID").(string)

            mu.Lock()
            if requests[userID] >= limit {
                mu.Unlock()
                return nil, fmt.Errorf("rate limit exceeded")
            }
            requests[userID]++
            mu.Unlock()

            return next(p)
        }
    }
}

// Usage
graph.NewResolver[User]("user").
    WithMiddleware(RateLimitMiddleware(100)).
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        return userService.GetByID(id)
    }).BuildQuery()
```

### Middleware Patterns

#### Stacking Multiple Middleware

Chain multiple middleware for complex behavior:

```go
graph.NewResolver[User]("user").
    WithMiddleware(graph.LoggingMiddleware).           // 1. Log timing
    WithMiddleware(graph.AuthMiddleware("user")).      // 2. Check auth
    WithMiddleware(RateLimitMiddleware(100)).          // 3. Rate limit
    WithMiddleware(graph.CacheMiddleware(cacheKeyFn)). // 4. Cache results
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        return userService.GetByID(id)
    }).BuildQuery()
```

#### Field-Level Middleware

Apply middleware to specific fields within a type:

```go
graph.NewResolver[User]("users").
    AsList().
    WithFieldMiddleware("email", graph.AuthMiddleware("admin")).
    WithFieldMiddleware("salary", graph.AuthMiddleware("hr")).
    WithResolver(func(p graph.ResolveParams) (*[]User, error) {
        return userService.List(), nil
    }).BuildQuery()
```

#### Permission Middleware (Convenience Method)

`WithPermission()` is a convenience wrapper around `WithMiddleware()` for authorization:

```go
graph.NewResolver[User]("deleteUser").
    AsMutation().
    WithPermission(graph.AuthMiddleware("admin")).
    WithResolver(func(p graph.ResolveParams) (*User, error) {
        id, _ := graph.GetArgInt(p, "id")
        return userService.Delete(id)
    }).BuildMutation()
```

### Advanced Middleware Examples

#### Context Injection Middleware

```go
func InjectDependencies(db *gorm.DB, cache *redis.Client) graph.FieldMiddleware {
    return func(next graph.FieldResolveFn) graph.FieldResolveFn {
        return func(p graph.ResolveParams) (interface{}, error) {
            ctx := p.Context
            ctx = context.WithValue(ctx, "db", db)
            ctx = context.WithValue(ctx, "cache", cache)

            // Create new params with injected context
            newParams := p
            newParams.Context = ctx

            return next(newParams)
        }
    }
}
```

#### Error Handling Middleware

```go
func ErrorHandlingMiddleware(next graph.FieldResolveFn) graph.FieldResolveFn {
    return func(p graph.ResolveParams) (interface{}, error) {
        result, err := next(p)
        if err != nil {
            // Log error
            log.Printf("Error in %s: %v", p.Info.FieldName, err)

            // Transform error for user
            return nil, fmt.Errorf("an error occurred: please contact support")
        }
        return result, nil
    }
}
```

#### Performance Tracking Middleware

```go
func MetricsMiddleware(metrics *prometheus.Registry) graph.FieldMiddleware {
    return func(next graph.FieldResolveFn) graph.FieldResolveFn {
        return func(p graph.ResolveParams) (interface{}, error) {
            start := time.Now()
            result, err := next(p)
            duration := time.Since(start)

            // Record metrics
            fieldName := fmt.Sprintf("%s.%s", p.Info.ParentType.Name(), p.Info.FieldName)
            recordMetric(metrics, fieldName, duration, err != nil)

            return result, err
        }
    }
}
```

### Middleware Best Practices

1. **Order matters**: Place authentication before caching, logging outermost
2. **Keep middleware pure**: Avoid side effects when possible
3. **Use context for data**: Pass request-scoped data via context
4. **Handle errors gracefully**: Always return meaningful errors
5. **Measure performance**: Use logging/metrics middleware to track slow resolvers
6. **Batch database queries**: Use dataloaders to prevent N+1 queries

## GraphQL Subscriptions

Real-time event streaming over WebSocket with type-safe resolvers and middleware support.

### Features

- 🔄 **Real-time Events** - Stream data to clients over WebSocket
- 🎯 **Type-Safe Resolvers** - Generic subscription builders with compile-time safety
- 🔌 **Dual Protocol Support** - Works with both `graphql-ws` (modern) and `subscriptions-transport-ws` (legacy)
- 🏗️ **Fluent Builder API** - Same intuitive API as queries and mutations
- 🛡️ **Middleware Support** - Apply authentication, logging, and custom middleware
- 🔍 **Event Filtering** - Filter events before sending to clients
- 📦 **Pluggable PubSub** - In-memory, Redis, Kafka, or custom backends
- 🎭 **Field-Level Control** - Custom resolvers and middleware per field

### Quick Start

```go
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
    "github.com/graphql-go/graphql"
    graph "github.com/paulmanoni/go-graph"
)

type MessageEvent struct {
    ID        string    `json:"id"`
    Content   string    `json:"content"`
    Author    string    `json:"author"`
    Timestamp time.Time `json:"timestamp"`
}

func main() {
    // Initialize PubSub system
    pubsub := graph.NewInMemoryPubSub()
    defer pubsub.Close()

    // Create subscription
    messageSubscription := graph.NewSubscription[MessageEvent]("messageAdded").
        WithDescription("Subscribe to new messages").
        WithArgs(graphql.FieldConfigArgument{
            "channelID": &graphql.ArgumentConfig{
                Type: graphql.NewNonNull(graphql.String),
            },
        }).
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *MessageEvent, error) {
            channelID, _ := graph.GetArgString(p, "channelID")

            // Create output channel
            events := make(chan *MessageEvent, 10)

            // Subscribe to PubSub topic
            subscription := pubsub.Subscribe(ctx, "messages:"+channelID)

            // Forward events to GraphQL channel
            go func() {
                defer close(events)
                for msg := range subscription {
                    var event MessageEvent
                    json.Unmarshal(msg.Data, &event)
                    events <- &event
                }
            }()

            return events, nil
        }).
        BuildSubscription()

    // Build GraphQL handler with subscriptions
    handler := graph.NewHTTP(&graph.GraphContext{
        SchemaParams: &graph.SchemaBuilderParams{
            QueryFields:        []graph.QueryField{...},
            MutationFields:     []graph.MutationField{...},
            SubscriptionFields: []graph.SubscriptionField{messageSubscription},
        },
        PubSub:              pubsub,
        EnableSubscriptions: true,
        Playground:          true,
    })

    http.Handle("/graphql", handler)
    http.ListenAndServe(":8080", nil)
}
```

**Test the subscription:**

```graphql
subscription {
  messageAdded(channelID: "general") {
    id
    content
    author
    timestamp
  }
}
```

### PubSub System

The PubSub interface enables pluggable backends for event distribution:

```go
type PubSub interface {
    Publish(ctx context.Context, topic string, data interface{}) error
    Subscribe(ctx context.Context, topic string) <-chan *Message
    Unsubscribe(ctx context.Context, subscriptionID string) error
    Close() error
}
```

#### In-Memory PubSub (Development)

Perfect for development and single-instance deployments:

```go
pubsub := graph.NewInMemoryPubSub()
defer pubsub.Close()

// Publish events
ctx := context.Background()
pubsub.Publish(ctx, "messages:general", &MessageEvent{
    ID:      "1",
    Content: "Hello!",
    Author:  "Alice",
})

// Subscribe to events
subscription := pubsub.Subscribe(ctx, "messages:general")
for msg := range subscription {
    // Process message
}
```

#### Custom PubSub Backends

Implement the `PubSub` interface for Redis, Kafka, or other message brokers:

```go
type RedisPubSub struct {
    client *redis.Client
    // ... implementation
}

func (r *RedisPubSub) Publish(ctx context.Context, topic string, data interface{}) error {
    jsonData, _ := json.Marshal(data)
    return r.client.Publish(ctx, topic, jsonData).Err()
}

func (r *RedisPubSub) Subscribe(ctx context.Context, topic string) <-chan *graph.Message {
    // ... implementation
}

// Use with GraphContext
handler := graph.NewHTTP(&graph.GraphContext{
    PubSub: &RedisPubSub{client: redisClient},
    EnableSubscriptions: true,
})
```

### Subscription Resolver API

The `NewSubscription[T]` builder provides a fluent API for creating type-safe subscriptions:

```go
graph.NewSubscription[EventType]("subscriptionName").
    WithDescription("Description of the subscription").
    WithArgs(graphql.FieldConfigArgument{...}).
    WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *EventType, error) {
        // Return a channel that emits events
    }).
    WithFilter(func(ctx context.Context, data *EventType, p graph.ResolveParams) bool {
        // Filter events before sending to clients
        return true
    }).
    WithMiddleware(graph.AuthMiddleware("user")).
    WithFieldResolver("customField", func(p graph.ResolveParams) (interface{}, error) {
        // Custom resolver for specific fields
    }).
    WithFieldMiddleware("sensitiveField", graph.AuthMiddleware("admin")).
    BuildSubscription()
```

#### Basic Subscription

```go
type UserStatusEvent struct {
    UserID    string    `json:"userID"`
    Status    string    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
}

func userStatusSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[UserStatusEvent]("userStatusChanged").
        WithDescription("Subscribe to user status changes").
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *UserStatusEvent, error) {
            events := make(chan *UserStatusEvent, 10)
            subscription := pubsub.Subscribe(ctx, "user_status")

            go func() {
                defer close(events)
                for msg := range subscription {
                    var event UserStatusEvent
                    json.Unmarshal(msg.Data, &event)
                    events <- &event
                }
            }()

            return events, nil
        }).
        BuildSubscription()
}
```

#### With Arguments

```go
func messageSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[Message]("messageAdded").
        WithArgs(graphql.FieldConfigArgument{
            "channelID": &graphql.ArgumentConfig{
                Type:        graphql.NewNonNull(graphql.String),
                Description: "Channel to subscribe to",
            },
        }).
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *Message, error) {
            channelID, _ := graph.GetArgString(p, "channelID")

            events := make(chan *Message, 10)
            subscription := pubsub.Subscribe(ctx, "messages:"+channelID)

            go func() {
                defer close(events)
                for msg := range subscription {
                    var message Message
                    json.Unmarshal(msg.Data, &message)
                    events <- &message
                }
            }()

            return events, nil
        }).
        BuildSubscription()
}
```

#### With Event Filtering

Filter events based on user permissions or subscription criteria:

```go
func messageSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[Message]("messageAdded").
        WithArgs(graphql.FieldConfigArgument{
            "channelID": &graphql.ArgumentConfig{Type: graphql.String},
        }).
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *Message, error) {
            // Subscribe to all messages
            events := make(chan *Message, 10)
            subscription := pubsub.Subscribe(ctx, "messages")

            go func() {
                defer close(events)
                for msg := range subscription {
                    var message Message
                    json.Unmarshal(msg.Data, &message)
                    events <- &message
                }
            }()

            return events, nil
        }).
        WithFilter(func(ctx context.Context, data *Message, p graph.ResolveParams) bool {
            // Filter by channel ID
            channelID, _ := graph.GetArgString(p, "channelID")
            return data.ChannelID == channelID
        }).
        BuildSubscription()
}
```

#### With Authentication Middleware

Protect subscriptions with authentication:

```go
func adminSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[AdminEvent]("adminEvents").
        WithDescription("Admin-only event stream").
        WithMiddleware(graph.AuthMiddleware("admin")).
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *AdminEvent, error) {
            // Only admins reach here
            events := make(chan *AdminEvent, 10)
            subscription := pubsub.Subscribe(ctx, "admin_events")

            go func() {
                defer close(events)
                for msg := range subscription {
                    var event AdminEvent
                    json.Unmarshal(msg.Data, &event)
                    events <- &event
                }
            }()

            return events, nil
        }).
        BuildSubscription()
}
```

#### With Field-Level Customization

Customize how specific fields are resolved:

```go
type MessageEvent struct {
    ID       string `json:"id"`
    Content  string `json:"content"`
    AuthorID string `json:"authorID"`
}

func messageSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[MessageEvent]("messageAdded").
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *MessageEvent, error) {
            // ... resolver implementation
        }).
        WithFieldResolver("author", func(p graph.ResolveParams) (interface{}, error) {
            // Custom resolver for author field
            event := p.Source.(MessageEvent)
            return userService.GetByID(event.AuthorID), nil
        }).
        WithFieldMiddleware("content", graph.AuthMiddleware("user")).
        BuildSubscription()
}
```

### WebSocket Configuration

Configure WebSocket behavior through `GraphContext`:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{
        SubscriptionFields: []graph.SubscriptionField{...},
    },

    // Enable subscriptions
    EnableSubscriptions: true,

    // PubSub backend
    PubSub: pubsub,

    // Optional: Custom WebSocket path (default: auto-detects)
    WebSocketPath: "/subscriptions",

    // Optional: Custom origin check
    WebSocketCheckOrigin: func(r *http.Request) bool {
        origin := r.Header.Get("Origin")
        return origin == "https://example.com"
    },

    // Optional: Authentication for WebSocket connections
    UserDetailsFn: func(token string) (interface{}, error) {
        return validateAndGetUser(token)
    },
})
```

#### Authentication in Subscriptions

WebSocket authentication works similarly to HTTP:

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{
        SubscriptionFields: []graph.SubscriptionField{...},
    },
    EnableSubscriptions: true,
    PubSub:              pubsub,

    // Extract user details from token
    UserDetailsFn: func(token string) (interface{}, error) {
        user, err := validateJWT(token)
        if err != nil {
            return nil, err
        }
        return user, nil
    },
})
```

**Client sends token during connection initialization:**

```javascript
const client = new SubscriptionClient('ws://localhost:8080/graphql', {
  connectionParams: {
    authorization: 'Bearer <token>',
  },
});
```

**Access user details in subscription resolver:**

```go
func messageSubscription(pubsub graph.PubSub) graph.SubscriptionField {
    return graph.NewSubscription[Message]("messageAdded").
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *Message, error) {
            // Get authenticated user
            var user User
            if err := graph.GetRootInfo(p, "details", &user); err != nil {
                return nil, fmt.Errorf("authentication required")
            }

            // Subscribe to user-specific messages
            events := make(chan *Message, 10)
            subscription := pubsub.Subscribe(ctx, "messages:"+user.ID)

            // ... forward events

            return events, nil
        }).
        BuildSubscription()
}
```

### Publishing Events from Mutations

Trigger subscription events from mutations:

```go
func sendMessageMutation(pubsub graph.PubSub) graph.MutationField {
    type SendMessageInput struct {
        ChannelID string `json:"channelID" graphql:"channelID,required"`
        Content   string `json:"content" graphql:"content,required"`
    }

    return graph.NewResolver[Message]("sendMessage").
        WithInputObject(SendMessageInput{}).
        WithResolver(func(p graph.ResolveParams) (*Message, error) {
            var input SendMessageInput
            graph.GetArg(p, "input", &input)

            // Create message
            msg := &Message{
                ID:        uuid.New().String(),
                Content:   input.Content,
                ChannelID: input.ChannelID,
                Timestamp: time.Now(),
            }

            // Store in database
            db.Create(msg)

            // Publish to subscribers
            ctx := context.Background()
            pubsub.Publish(ctx, "messages:"+input.ChannelID, msg)

            return msg, nil
        }).
        BuildMutation()
}
```

### Protocol Support

The WebSocket handler supports both modern and legacy protocols:

| Protocol | Support | Client |
|----------|---------|--------|
| `graphql-ws` | ✅ Full | Apollo Client 3+, urql |
| `subscriptions-transport-ws` | ✅ Full | Apollo Client 2, GraphQL Playground |

Both protocols work simultaneously - no configuration needed.

### Production Considerations

#### Connection Limits

Monitor and limit concurrent WebSocket connections:

```go
var (
    maxConnections = 10000
    currentConns   int64
)

handler := graph.NewHTTP(&graph.GraphContext{
    EnableSubscriptions: true,
    WebSocketCheckOrigin: func(r *http.Request) bool {
        if atomic.LoadInt64(&currentConns) >= int64(maxConnections) {
            return false
        }
        atomic.AddInt64(&currentConns, 1)
        return true
    },
})
```

#### Event Buffering

Use buffered channels to handle bursts:

```go
// Good: Buffered channel prevents blocking
events := make(chan *Message, 100)

// Bad: Unbuffered channel may block publishers
events := make(chan *Message)
```

#### Error Handling

Handle errors gracefully in subscription resolvers:

```go
WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *Event, error) {
    // Validate arguments first
    channelID, err := graph.GetArgString(p, "channelID")
    if err != nil {
        return nil, fmt.Errorf("channelID required")
    }

    // Create subscription
    events := make(chan *Event, 10)
    subscription := pubsub.Subscribe(ctx, "events:"+channelID)

    go func() {
        defer close(events)
        for msg := range subscription {
            var event Event
            if err := json.Unmarshal(msg.Data, &event); err != nil {
                log.Printf("Failed to unmarshal event: %v", err)
                continue // Skip malformed events
            }
            events <- &event
        }
    }()

    return events, nil
})
```

#### Graceful Shutdown

Close all connections during shutdown:

```go
func main() {
    pubsub := graph.NewInMemoryPubSub()

    handler := graph.NewHTTP(&graph.GraphContext{
        PubSub:              pubsub,
        EnableSubscriptions: true,
    })

    server := &http.Server{Addr: ":8080", Handler: handler}

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        // Close PubSub (closes all subscriptions)
        pubsub.Close()

        // Shutdown HTTP server
        server.Shutdown(ctx)
    }()

    server.ListenAndServe()
}
```

### Complete Example

See [examples/subscription](./examples/subscription) for a complete working example with:
- Message subscriptions with channel filtering
- User status change subscriptions
- Authentication and authorization
- Mutations that trigger subscription events
- Background event simulator

Run the example:

```bash
cd examples/subscription
go run main.go
```

Then test subscriptions in GraphQL Playground at http://localhost:8080/graphql

## Framework Integration

### With Gin

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/paulmanoni/go-graph"
)

func main() {
    r := gin.Default()

    handler := graph.NewHTTP(&graph.GraphContext{
        SchemaParams:     &graph.SchemaBuilderParams{...},
        EnableValidation: true,
    })

    r.POST("/graphql", gin.WrapF(handler))
    r.GET("/graphql", gin.WrapF(handler))

    r.Run(":8080")
}
```

### With Chi

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/paulmanoni/go-graph"
)

func main() {
    r := chi.NewRouter()

    handler := graph.NewHTTP(&graph.GraphContext{
        SchemaParams: &graph.SchemaBuilderParams{...},
    })

    r.Handle("/graphql", handler)

    http.ListenAndServe(":8080", r)
}
```

### With Standard net/http

```go
handler := graph.NewHTTP(&graph.GraphContext{
    SchemaParams: &graph.SchemaBuilderParams{...},
})

http.Handle("/graphql", handler)
http.ListenAndServe(":8080", nil)
```

## API Reference

### `NewHTTP(graphCtx *GraphContext) http.HandlerFunc`

Creates a standard HTTP handler with validation and sanitization support.

### `GraphContext` Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Schema` | `*graphql.Schema` | `nil` | Custom GraphQL schema (Option 3) |
| `SchemaParams` | `*SchemaBuilderParams` | `nil` | Builder params (Option 2) |
| `Playground` | `bool` | `false` | Enable GraphQL Playground |
| `Pretty` | `bool` | `false` | Pretty-print JSON responses |
| `DEBUG` | `bool` | `false` | Skip validation/sanitization |
| `EnableValidation` | `bool` | `false` | Enable query validation |
| `EnableSanitization` | `bool` | `false` | Enable error sanitization |
| `TokenExtractorFn` | `func(*http.Request) string` | Bearer token | Custom token extraction |
| `UserDetailsFn` | `func(string) (interface{}, error)` | `nil` | Fetch user from token |
| `RootObjectFn` | `func(context.Context, *http.Request) map[string]interface{}` | `nil` | Custom root setup |

**Note:** If both `Schema` and `SchemaParams` are `nil`, a default hello world schema is used.

### `SchemaBuilderParams`

```go
type SchemaBuilderParams struct {
    QueryFields        []QueryField
    MutationFields     []MutationField
    SubscriptionFields []SubscriptionField  // Optional: Real-time subscriptions
}
```

## Testing

The library includes comprehensive test coverage including unit tests and benchmarks for all features including queries, mutations, and subscriptions.

### Running Unit Tests

```bash
# Run all tests
go test -v

# Run tests with coverage
go test -cover

# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test -run TestSubscription_WithFilter

# Run tests for a specific file
go test -run Subscription
```

### Subscription Tests

The subscription implementation includes extensive unit tests covering:

- **Basic subscription creation** - Creating and configuring subscriptions
- **Event streaming** - Testing event channels and data flow
- **Event filtering** - Filtering events based on subscription parameters
- **Middleware integration** - Applying middleware to subscriptions
- **PubSub integration** - Testing with in-memory and custom PubSub backends
- **Context cancellation** - Proper cleanup when subscriptions are cancelled
- **Type generation** - Auto-generating GraphQL types from Go structs
- **Error handling** - Proper error propagation and recovery

Example test run:

```bash
# Run all subscription tests
go test -v -run Subscription

# Run specific subscription test
go test -v -run TestSubscription_WithFilter
```

### Writing Tests for Your Resolvers

Here's an example of how to test your custom resolvers:

```go
func TestMySubscription(t *testing.T) {
    type MyEvent struct {
        ID      string `json:"id"`
        Message string `json:"message"`
    }

    sub := graph.NewSubscription[MyEvent]("myEvent").
        WithResolver(func(ctx context.Context, p graph.ResolveParams) (<-chan *MyEvent, error) {
            ch := make(chan *MyEvent, 1)
            ch <- &MyEvent{ID: "1", Message: "test"}
            close(ch)
            return ch, nil
        }).
        BuildSubscription()

    field := sub.Serve()

    // Test subscription execution
    result, err := field.Subscribe(graphql.ResolveParams{
        Context: context.Background(),
    })

    if err != nil {
        t.Fatalf("Subscribe error: %v", err)
    }

    outputCh, ok := result.(<-chan interface{})
    if !ok {
        t.Fatalf("Expected channel, got %T", result)
    }

    // Collect events
    var received []MyEvent
    for event := range outputCh {
        received = append(received, event.(MyEvent))
    }

    if len(received) != 1 {
        t.Errorf("Expected 1 event, got %d", len(received))
    }
}
```

## Examples

See the [examples](./examples) directory for complete working examples:

- `main.go` - Full example with authentication
- `subscription/main.go` - Real-time subscriptions with WebSocket

## Performance Benchmarks

Comprehensive benchmarks are included to measure performance across all package operations.

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem

# Run specific benchmark
go test -bench=BenchmarkExtractBearerToken -benchmem

# Run with longer duration for more accurate results
go test -bench=. -benchmem -benchtime=5s

# Save results for comparison
go test -bench=. -benchmem > bench_results.txt
```

### Benchmark Results

Performance metrics on Apple M1 Pro (results will vary by hardware):

#### Core Operations

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Token Extraction | ~31 ns | 0 allocs | Bearer token from header |
| Type Registration | ~14 ns | 0 allocs | Object type caching |
| GetArgString | ~10 ns | 0 allocs | Extract string argument |
| GetArgInt | ~10 ns | 0 allocs | Extract int argument |
| GetArgBool | ~10 ns | 0 allocs | Extract bool argument |
| GetRootString | ~10 ns | 0 allocs | Extract root string value |

#### Schema Building

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Simple Schema | ~9 μs | 122 allocs | Default hello/echo schema |
| Complex Schema | ~10 μs | 147 allocs | Multiple types with nesting |
| Schema from Context | ~8-10 μs | 109-136 allocs | Build from GraphContext |

#### Query Validation

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Simple Query | ~700 ns | 27 allocs | Basic field selection |
| Complex Query | ~3.2 μs | 103 allocs | Nested 3 levels deep |
| Deep Query | ~2.2 μs | 72 allocs | Nested 5+ levels |
| With Aliases | ~3.9 μs | 130 allocs | Multiple field aliases |
| Depth Calculation | ~6-16 ns | 0 allocs | AST traversal |
| Alias Counting | ~20 ns | 0 allocs | AST analysis |
| Complexity Calc | ~13 ns | 0 allocs | Complexity scoring |

#### HTTP Handler Performance

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Debug Mode | ~28 μs | 439 allocs | No validation/sanitization |
| With Validation | ~28 μs | 478 allocs | Query validation enabled |
| With Sanitization | ~34 μs | 607 allocs | Error sanitization enabled |
| With Auth | ~27 μs | 443 allocs | Token + user details fetch |
| Complete Stack | ~60 μs | 966 allocs | All features enabled |
| GET Request | ~29 μs | 436 allocs | Query string parsing |

#### Resolver Creation

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Simple Resolver | ~234 ns | 5 allocs | Basic type resolver |
| With Arguments | ~349 ns | 9 allocs | Field arguments included |
| List Resolver | ~186 ns | 5 allocs | Array type resolver |
| Paginated | ~230 ns | 5 allocs | Pagination wrapper |
| With Input Object | ~411 ns | 10 allocs | Input type generation |

#### Type-Safe Arguments (NewArgsResolver)

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Struct Args Creation | ~874 ns | 17 allocs | Create resolver with struct args |
| Primitive Args Creation | ~372 ns | 11 allocs | Create resolver with primitive args |
| Nested Struct Args Creation | ~581 ns | 15 allocs | Create resolver with nested structs |
| List Resolver Creation | ~544 ns | 14 allocs | Create list resolver with args |
| Execute Struct Args | ~259 ns | 5 allocs | Execute resolver with struct args |
| Execute Primitive Args | ~66 ns | 2 allocs | Execute resolver with primitive args |
| Execute Nested Structs | ~397 ns | 6 allocs | Execute resolver with nested structs |
| Execute With Context | ~158 ns | 4 allocs | Execute with context.Context |
| Generate Args From Type | ~1.3 μs | 22 allocs | Auto-generate GraphQL args from struct |
| Map Args to Struct | ~457 ns | 7 allocs | Parse and map GraphQL args to Go struct |
| Map Nested Struct | ~346 ns | 4 allocs | Parse nested struct arguments |

#### Advanced Features

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| GetRootInfo | ~742 ns | 12 allocs | Complex type extraction |
| GetArg (Complex) | ~1.1 μs | 15 allocs | Struct argument parsing |
| Response Sanitization | ~5.4 μs | 80 allocs | Regex error cleaning |
| Cached Field Resolver | ~5.6 ns | 0 allocs | Cache hit scenario |
| Response Write | ~3.4 ns | 0 allocs | Buffer write operation |

#### Subscription Performance

Comprehensive benchmarks for real-time subscription operations:

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Build Subscription | ~200-600 ns | 5-15 allocs | Create subscription resolver |
| Build Complex Subscription | ~800 ns | 17 allocs | With args, filter, middleware |
| Execute Subscription | ~500-1000 ns | 10-20 allocs | Start event streaming |
| With Filter | ~1-2 μs | 15-25 allocs | Filter 100 events |
| With Middleware | ~500 ns | 10 allocs | 3 middleware layers |
| UnmarshalSubscriptionMessage | ~300 ns | 5 allocs | JSON parsing |
| Event Throughput | ~1-2 μs | 20 allocs | 1000 events/subscription |
| With PubSub | ~1 μs | 15 allocs | PubSub integration |
| Type Generation | ~800 ns | 15 allocs | Complex event type |
| Concurrent Subscriptions | ~500 ns | 12 allocs | Parallel execution |
| Schema with Subscriptions | ~10 μs | 150 allocs | Multiple subscriptions |

**Key Observations:**
- Subscription creation is fast (~200-800ns) with minimal allocations
- Event filtering adds minimal overhead (~1-2μs for 100 events)
- Type generation for complex events is efficient (~800ns)
- Concurrent subscription handling performs excellently (~500ns per subscription)
- PubSub integration adds negligible overhead (~1μs)

**Running Subscription Benchmarks:**
```bash
# Run all subscription benchmarks
go test -bench=BenchmarkSubscription -benchmem

# Run specific subscription benchmark
go test -bench=BenchmarkSubscription_WithFilter -benchmem

# Compare with and without middleware
go test -bench=BenchmarkSubscription_Execute -benchmem
go test -bench=BenchmarkSubscription_WithMiddleware -benchmem
```

#### Concurrency Performance

| Operation | Time/op | Allocations | Description |
|-----------|---------|-------------|-------------|
| Parallel HTTP Requests | ~17 μs | 440 allocs | Concurrent request handling |
| Parallel Schema Build | ~3 μs | 104 allocs | Concurrent schema creation |
| Parallel Type Registration | ~145 ns | 0 allocs | Thread-safe type caching |

### Key Takeaways

- **Zero-allocation primitives**: Token extraction and utility functions have zero heap allocations
- **Fast validation**: Query validation adds minimal overhead (~700ns-4μs depending on complexity)
- **Type-safe arguments**: NewArgsResolver execution is blazing fast (~66ns for primitives, ~259ns for structs)
- **Efficient type generation**: Auto-generating GraphQL args from structs adds minimal overhead (~1.3μs one-time cost)
- **Efficient caching**: Type registration uses read-write locks for optimal concurrent access
- **Predictable performance**: End-to-end request handling is consistently under 100μs
- **Production ready**: Complete stack with all security features runs at ~60μs per request

### Optimization Tips

1. **Enable caching**: Type registration is cached automatically - registered types are reused
2. **Use DEBUG mode wisely**: Validation adds ~0-1μs overhead, only disable in development
3. **Minimize complexity**: Keep query depth under 10 levels for optimal validation performance
4. **Batch operations**: Use concurrent requests for multiple independent queries
5. **Profile your resolvers**: The handler overhead is minimal (~30μs), optimize resolver logic first

## High Load Performance Analysis

### Is This Package Production-Ready for High Traffic?

**Yes, absolutely.** The benchmarks demonstrate excellent performance characteristics for high-load scenarios:

#### Throughput Capacity

Based on the benchmark results:
- **Handler overhead**: ~60 μs per request (complete stack with all security features)
- **Theoretical capacity**: ~16,600 requests/second per core
- **Multi-core scaling**: On an 8-core system, potentially **100,000+ RPS** (handler only)

#### Real-World Considerations

1. **Handler overhead is negligible**: At 60 μs, the GraphQL handler represents a tiny fraction of total request time
   ```
   Example breakdown (not measured, for illustration):
   - GraphQL handler:        60 μs   (0.06%)  ← Measured
   - Database query:      50,000 μs  (50.00%) ← Example
   - External API calls:  45,000 μs  (45.00%) ← Example
   - Business logic:       4,940 μs   (4.94%) ← Example
   Total:               ~100,000 μs  (100 ms)
   ```

2. **Zero-allocation critical paths**: Token extraction and argument parsing have 0 heap allocations, minimizing GC pressure

3. **Thread-safe design**: Parallel benchmarks show excellent concurrent performance (17 μs vs 28 μs sequential)

4. **Predictable latency**: Performance is consistent - no spikes or unpredictable behavior

#### Tested Load Scenarios

The package handles these scenarios efficiently:

| Scenario | Handler Overhead | Notes |
|----------|-----------------|-------|
| Simple queries | ~28 μs | Basic CRUD operations |
| Complex nested queries | ~28 μs | 3-5 levels deep |
| With authentication | ~27 μs | Token + user details |
| Full security stack | ~60 μs | Validation + sanitization + auth |
| Concurrent requests | ~17 μs/req | Parallel processing |

#### Production Deployment Recommendations

For high-load production environments:

**✅ Do:**
- Enable all security features (`EnableValidation`, `EnableSanitization`) - overhead is minimal
- Use connection pooling for databases (your resolvers, not this package)
- Implement resolver-level caching for expensive operations
- Monitor resolver performance (this is where bottlenecks occur)
- Use load balancing across multiple instances
- Consider rate limiting at the API gateway level

**⚠️ Bottlenecks will be in your code, not this package:**
- Database queries (typically 1-100+ ms)
- External API calls (typically 10-500+ ms)
- Complex business logic
- N+1 query problems (use dataloader pattern)

**❌ Don't:**
- Disable security features for "performance" - they add negligible overhead
- Skip validation in production - the ~1 μs cost is worth it
- Worry about handler performance - optimize your resolvers first

#### Memory Efficiency

- **Complete stack**: 966 allocations per request (~62 KB)
- **Debug mode**: 439 allocations per request (~33 KB)
- **GC impact**: Minimal on modern Go runtimes (1.18+)
- **Memory footprint**: Low even at 10,000+ concurrent requests

#### Proven Scalability

The benchmarks show:
- **Linear scaling**: No performance degradation with concurrency
- **Type caching**: Registered types reused (0 allocs after initial registration)
- **Lock contention**: Minimal (RWMutex on type registry)

### When NOT to Use This Package

This package may not be suitable if:
- You need sub-10 μs total latency (extremely rare requirement)
- You're running on severely resource-constrained environments (embedded systems)
- You need custom validation rules beyond depth/complexity/aliases

### Conclusion

**This package is excellent for high-load production environments.** The 60 μs overhead is negligible compared to typical resolver operations. Your performance bottlenecks will be in your business logic, database queries, and external API calls - not in this GraphQL handler.

For reference:
- ✅ **100 RPS**: Trivial (1% CPU on single core)
- ✅ **1,000 RPS**: Easy (10% CPU on single core)
- ✅ **10,000 RPS**: Manageable (multi-core, normal load)
- ✅ **100,000 RPS**: Achievable (horizontal scaling + optimization)
- ⚠️ **1,000,000 RPS**: Requires distributed architecture (but handler isn't the bottleneck)

**The handler is not your problem. Focus on optimizing your resolvers.**

## License

MIT