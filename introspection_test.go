package graph

import (
	"errors"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

type testUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestFieldInfo_Query(t *testing.T) {
	qf := NewResolver[testUser]("getUser").
		WithDescription("Get one user by id").
		WithArgRequired("id", graphql.Int).
		WithArg("includeArchived", graphql.Boolean).
		WithResolver(func(p ResolveParams) (*testUser, error) {
			return &testUser{ID: 1, Name: "Amara"}, nil
		}).
		BuildQuery()

	info, ok := Inspect(qf)
	if !ok {
		t.Fatal("QueryField must implement Introspectable")
	}
	if info.Name != "getUser" {
		t.Fatalf("name: want getUser, got %s", info.Name)
	}
	if info.Kind != FieldKindQuery {
		t.Fatalf("kind: want query, got %s", info.Kind)
	}
	if info.Description != "Get one user by id" {
		t.Fatalf("description: %s", info.Description)
	}
	if len(info.Args) != 2 {
		t.Fatalf("want 2 args, got %d", len(info.Args))
	}

	byName := map[string]ArgInfo{}
	for _, a := range info.Args {
		byName[a.Name] = a
	}
	if !byName["id"].Required {
		t.Fatal("id should be required (NonNull)")
	}
	if byName["includeArchived"].Required {
		t.Fatal("includeArchived should not be required")
	}
	if byName["id"].Type != "Int!" {
		t.Fatalf("id type: want Int!, got %q", byName["id"].Type)
	}
}

func TestFieldInfo_MutationWithInputObject(t *testing.T) {
	type CreateUserInput struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	mf := NewResolver[testUser]("createUser").
		WithInputObject(CreateUserInput{}).
		WithResolver(func(p ResolveParams) (*testUser, error) {
			return &testUser{ID: 2, Name: "Juma"}, nil
		}).
		BuildMutation()

	info, _ := Inspect(mf)
	if info.Kind != FieldKindMutation {
		t.Fatalf("want mutation, got %s", info.Kind)
	}
	if info.InputObject == nil || info.InputObject.TypeName != "CreateUserInput" {
		t.Fatalf("InputObject unexpected: %+v", info.InputObject)
	}
}

func TestFieldInfo_NamedMiddlewares(t *testing.T) {
	pass := func(next FieldResolveFn) FieldResolveFn {
		return func(p ResolveParams) (interface{}, error) { return next(p) }
	}
	qf := NewResolver[testUser]("me").
		WithNamedMiddleware("auth", "Bearer token validation", pass).
		WithNamedMiddleware("permission:admin", "Admin role required", pass).
		WithMiddleware(pass). // anonymous
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil }).
		BuildQuery()

	info, _ := Inspect(qf)
	if len(info.Middlewares) != 3 {
		t.Fatalf("want 3 mws, got %d", len(info.Middlewares))
	}
	if info.Middlewares[0].Name != "auth" {
		t.Fatalf("mw[0]: %+v", info.Middlewares[0])
	}
	if info.Middlewares[0].Description != "Bearer token validation" {
		t.Fatalf("mw[0].Description: %s", info.Middlewares[0].Description)
	}
	if info.Middlewares[1].Name != "permission:admin" {
		t.Fatalf("mw[1]: %+v", info.Middlewares[1])
	}
	if info.Middlewares[2].Name != "anonymous" {
		t.Fatalf("mw[2] should be anonymous, got %s", info.Middlewares[2].Name)
	}
}

func TestFieldInfo_ValidatorsMetadata(t *testing.T) {
	mf := NewResolver[testUser]("createUser").
		WithArgRequired("name", graphql.String).
		WithArgRequired("email", graphql.String).
		WithArgValidator("name", Required(), StringLength(1, 100)).
		WithArgValidator("email", StringMatch(`^\S+@\S+$`, "must be an email")).
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil }).
		BuildMutation()

	info, _ := Inspect(mf)
	nameArg := findArg(info.Args, "name")
	if nameArg == nil {
		t.Fatal("name arg missing")
	}
	if len(nameArg.Validators) != 2 {
		t.Fatalf("want 2 validators on name, got %d", len(nameArg.Validators))
	}
	if nameArg.Validators[0].Kind != "required" {
		t.Fatalf("first validator kind: %s", nameArg.Validators[0].Kind)
	}
	if nameArg.Validators[1].Kind != "length" {
		t.Fatalf("second validator kind: %s", nameArg.Validators[1].Kind)
	}

	emailArg := findArg(info.Args, "email")
	if emailArg.Validators[0].Kind != "regex" {
		t.Fatalf("email validator kind: %s", emailArg.Validators[0].Kind)
	}
	if emailArg.Validators[0].Message != "must be an email" {
		t.Fatalf("email validator message: %s", emailArg.Validators[0].Message)
	}
}

func TestValidators_RunAtResolveTime(t *testing.T) {
	qf := NewResolver[testUser]("getUser").
		WithArgRequired("name", graphql.String).
		WithArgValidator("name", StringLength(3, 10)).
		WithResolver(func(p ResolveParams) (*testUser, error) {
			return &testUser{Name: p.Args["name"].(string)}, nil
		}).
		BuildQuery()

	field := qf.Serve()

	// Too short — validator should reject.
	_, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"name": "ab"}})
	if err == nil {
		t.Fatal("expected validator error for short name")
	}
	if !strings.Contains(err.Error(), "name:") {
		t.Fatalf("error should be argument-scoped: %v", err)
	}

	// Valid — should pass through.
	out, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"name": "Amara"}})
	if err != nil {
		t.Fatalf("valid input errored: %v", err)
	}
	if u, _ := out.(*testUser); u == nil || u.Name != "Amara" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestValidators_OneOf_And_IntRange(t *testing.T) {
	qf := NewResolver[testUser]("filter").
		WithArg("role", graphql.String).
		WithArg("age", graphql.Int).
		WithArgValidator("role", OneOf("admin", "user")).
		WithArgValidator("age", IntRange(0, 120)).
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil }).
		BuildQuery()
	field := qf.Serve()

	if _, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"role": "hacker", "age": 30}}); err == nil {
		t.Fatal("OneOf should reject 'hacker'")
	}
	if _, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"role": "admin", "age": 200}}); err == nil {
		t.Fatal("IntRange should reject 200")
	}
	if _, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"role": "admin", "age": 30}}); err != nil {
		t.Fatalf("valid combo errored: %v", err)
	}
}

func TestValidators_Custom(t *testing.T) {
	slug := Custom("slug", "must be lowercase alphanumeric-hyphens", func(v any) error {
		s, _ := v.(string)
		for _, r := range s {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return errors.New("invalid character")
			}
		}
		return nil
	})
	qf := NewResolver[testUser]("search").
		WithArg("slug", graphql.String).
		WithArgValidator("slug", slug).
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil }).
		BuildQuery()

	info, _ := Inspect(qf)
	a := findArg(info.Args, "slug")
	if a.Validators[0].Kind != "custom" {
		t.Fatalf("want custom kind, got %s", a.Validators[0].Kind)
	}

	field := qf.Serve()
	if _, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"slug": "Bad Slug!"}}); err == nil {
		t.Fatal("custom validator should reject")
	}
	if _, err := field.Resolve(graphql.ResolveParams{Args: map[string]any{"slug": "good-slug-42"}}); err != nil {
		t.Fatalf("valid slug errored: %v", err)
	}
}

func TestFieldInfo_Deprecation(t *testing.T) {
	qf := NewResolver[testUser]("oldMe").
		WithDeprecated("use me() instead").
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil }).
		BuildQuery()

	info, _ := Inspect(qf)
	if !info.Deprecated {
		t.Fatal("should be deprecated")
	}
	if info.DeprecationReason != "use me() instead" {
		t.Fatalf("reason: %s", info.DeprecationReason)
	}
	field := qf.Serve()
	if field.DeprecationReason != "use me() instead" {
		t.Fatalf("graphql.Field DeprecationReason: %s", field.DeprecationReason)
	}
}

func TestGetters(t *testing.T) {
	r := NewResolver[testUser]("me").
		WithMiddleware(func(next FieldResolveFn) FieldResolveFn { return next }).
		WithNamedMiddleware("auth", "", func(next FieldResolveFn) FieldResolveFn { return next }).
		AsMutation().
		WithResolver(func(p ResolveParams) (*testUser, error) { return nil, nil })

	if !r.IsMutation() {
		t.Fatal("IsMutation should be true")
	}
	if r.GetMiddlewareCount() != 2 {
		t.Fatalf("mw count: %d", r.GetMiddlewareCount())
	}
	infos := r.GetMiddlewareInfos()
	if len(infos) != 2 || infos[0].Name != "anonymous" || infos[1].Name != "auth" {
		t.Fatalf("mw infos: %+v", infos)
	}
}

func findArg(args []ArgInfo, name string) *ArgInfo {
	for i := range args {
		if args[i].Name == name {
			return &args[i]
		}
	}
	return nil
}