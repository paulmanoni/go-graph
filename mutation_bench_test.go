package graph

import (
	"context"
	"testing"

	"github.com/graphql-go/graphql"
)

// --- shared bench fixtures ---

type benchUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type benchCreateUserInput struct {
	Name  string `json:"name" graphql:"name,required"`
	Email string `json:"email" graphql:"email,required"`
	Role  string `json:"role"`
}

type benchUpdateUserInput struct {
	ID    int    `json:"id" graphql:"id,required"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type benchDeleteUserInput struct {
	ID int `json:"id" graphql:"id,required"`
}

type benchEchoInput struct {
	Message string `json:"message" graphql:"message,required"`
}

// --- Build-time benchmarks (one-time cost per mutation definition) ---

func BenchmarkNewMutation_Build_Create(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewMutation[benchUser, benchCreateUserInput]("createBenchUser").
			Create().
			WithResolver(func(ctx context.Context, in benchCreateUserInput) (*benchUser, error) {
				return &benchUser{Name: in.Name, Email: in.Email, Role: in.Role}, nil
			}).
			Build()
	}
}

func BenchmarkNewMutation_Build_Update(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewMutation[benchUser, benchUpdateUserInput]("updateBenchUser").
			Update().
			WithResolver(func(ctx context.Context, p Patch[benchUpdateUserInput]) (*benchUser, error) {
				in := p.Get()
				return &benchUser{ID: in.ID, Name: in.Name}, nil
			}).
			Build()
	}
}

func BenchmarkNewMutation_Build_Action(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewMutation[string, benchEchoInput]("echoBench").
			Action().
			WithResolver(func(ctx context.Context, in benchEchoInput) (*string, error) {
				return &in.Message, nil
			}).
			Build()
	}
}

// --- Execution benchmarks (per-request cost) ---

func BenchmarkNewMutation_Execute_Create(b *testing.B) {
	field := NewMutation[benchUser, benchCreateUserInput]("createBenchUser_exec").
		Create().
		WithResolver(func(ctx context.Context, in benchCreateUserInput) (*benchUser, error) {
			return &benchUser{Name: in.Name, Email: in.Email, Role: in.Role}, nil
		}).
		Build().Serve()

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"name":  "Alice",
				"email": "a@example.com",
				"role":  "member",
			},
		},
		Context: context.Background(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Resolve(params)
	}
}

func BenchmarkNewMutation_Execute_Update(b *testing.B) {
	field := NewMutation[benchUser, benchUpdateUserInput]("updateBenchUser_exec").
		Update().
		WithResolver(func(ctx context.Context, p Patch[benchUpdateUserInput]) (*benchUser, error) {
			in := p.Get()
			merged := benchUpdateUserInput{ID: in.ID}
			p.Apply(&merged)
			return &benchUser{ID: merged.ID, Name: merged.Name, Email: merged.Email, Role: merged.Role}, nil
		}).
		Build().Serve()

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"id":   1,
				"name": "Alice",
			},
		},
		Context: context.Background(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Resolve(params)
	}
}

func BenchmarkNewMutation_Execute_Action(b *testing.B) {
	field := NewMutation[string, benchEchoInput]("echoBench_exec").
		Action().
		WithResolver(func(ctx context.Context, in benchEchoInput) (*string, error) {
			return &in.Message, nil
		}).
		Build().Serve()

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"message": "hello world",
			},
		},
		Context: context.Background(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Resolve(params)
	}
}

func BenchmarkNewMutation_Execute_Delete(b *testing.B) {
	field := NewMutation[benchUser, benchDeleteUserInput]("deleteBenchUser_exec").
		Delete().
		WithResolver(func(ctx context.Context, in benchDeleteUserInput) (*benchUser, error) {
			return &benchUser{ID: in.ID}, nil
		}).
		Build().Serve()

	params := graphql.ResolveParams{
		Args: map[string]interface{}{
			"input": map[string]interface{}{
				"id": 42,
			},
		},
		Context: context.Background(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = field.Resolve(params)
	}
}

// --- Isolated component benchmarks ---

func BenchmarkNewMutation_PatchApply(b *testing.B) {
	p := Patch[benchUpdateUserInput]{
		data: benchUpdateUserInput{ID: 1, Name: "Alice", Email: "a@b.c", Role: "admin"},
		present: presenceSet{
			"id":   struct{}{},
			"name": struct{}{},
		},
	}
	var dst benchUpdateUserInput

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Apply(&dst)
	}
}

func BenchmarkNewMutation_DecodeInput(b *testing.B) {
	raw := map[string]interface{}{
		"input": map[string]interface{}{
			"id":   1,
			"name": "Alice",
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = decodeInput[benchUpdateUserInput](raw, "input")
	}
}
