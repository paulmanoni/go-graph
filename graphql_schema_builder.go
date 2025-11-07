package graph

import (
	"github.com/graphql-go/graphql"
	"go.uber.org/fx"
)

type SchemaBuilderParams struct {
	fx.In

	QueryFields    []QueryField    `group:"query_fields"`
	MutationFields []MutationField `group:"mutation_fields"`
}

type SchemaBuilder struct {
	queryFields    []QueryField
	mutationFields []MutationField
}

func NewSchemaBuilder(params SchemaBuilderParams) *SchemaBuilder {
	return &SchemaBuilder{
		queryFields:    params.QueryFields,
		mutationFields: params.MutationFields,
	}
}

func (sb *SchemaBuilder) Build() (graphql.Schema, error) {
	queryFields := graphql.Fields{}
	for _, field := range sb.queryFields {
		queryFields[field.Name()] = field.Serve()
	}

	mutationFields := graphql.Fields{}
	for _, field := range sb.mutationFields {
		mutationFields[field.Name()] = field.Serve()
	}

	schemaConfig := graphql.SchemaConfig{}

	if len(queryFields) > 0 {
		schemaConfig.Query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: queryFields,
		})
	}

	if len(mutationFields) > 0 {
		schemaConfig.Mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: mutationFields,
		})
	}

	return graphql.NewSchema(schemaConfig)
}

func ProvideSchema(sb *SchemaBuilder) (graphql.Schema, error) {
	return sb.Build()
}

var GraphQLModule = fx.Options(
	fx.Provide(
		NewSchemaBuilder,
		ProvideSchema,
	),
)
