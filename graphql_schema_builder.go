package graph

import (
	"github.com/graphql-go/graphql"
)

// SchemaBuilderParams configures the fields for building a GraphQL schema.
// Use this with NewSchemaBuilder to construct schemas using the builder pattern.
//
// Example:
//
//	params := graph.SchemaBuilderParams{
//	    QueryFields: []graph.QueryField{
//	        getUserQuery(),
//	        listUsersQuery(),
//	    },
//	    MutationFields: []graph.MutationField{
//	        createUserMutation(),
//	        updateUserMutation(),
//	    },
//	}
//	schema, err := graph.NewSchemaBuilder(params).Build()
type SchemaBuilderParams struct {
	// QueryFields: List of query fields to include in the schema
	QueryFields []QueryField `group:"query_fields"`

	// MutationFields: List of mutation fields to include in the schema
	MutationFields []MutationField `group:"mutation_fields"`

	// SubscriptionFields: List of subscription fields to include in the schema
	// Requires WebSocket support and PubSub configuration
	SubscriptionFields []SubscriptionField `group:"subscription_fields"`
}

// SchemaBuilder builds GraphQL schemas from QueryFields and MutationFields.
// Use NewSchemaBuilder to create an instance and Build() to generate the schema.
type SchemaBuilder struct {
	queryFields        []QueryField
	mutationFields     []MutationField
	subscriptionFields []SubscriptionField
}

// NewSchemaBuilder creates a new schema builder with the provided query and mutation fields.
//
// Example:
//
//	params := graph.SchemaBuilderParams{
//	    QueryFields:    []graph.QueryField{getUserQuery()},
//	    MutationFields: []graph.MutationField{createUserMutation()},
//	}
//	builder := graph.NewSchemaBuilder(params)
//	schema, err := builder.Build()
func NewSchemaBuilder(params SchemaBuilderParams) *SchemaBuilder {
	return &SchemaBuilder{
		queryFields:        params.QueryFields,
		mutationFields:     params.MutationFields,
		subscriptionFields: params.SubscriptionFields,
	}
}

// Build constructs and returns a graphql.Schema from the configured fields.
// It creates Query and Mutation root types based on the provided fields.
//
// Returns an error if:
//   - Schema construction fails due to type conflicts
//   - Field configurations are invalid
//
// The schema can have:
//   - Only queries (no mutations)
//   - Only mutations (no queries)
//   - Both queries and mutations
//   - Neither (empty schema)
func (sb *SchemaBuilder) Build() (graphql.Schema, error) {
	queryFields := graphql.Fields{}
	for _, field := range sb.queryFields {
		queryFields[field.Name()] = field.Serve()
	}

	mutationFields := graphql.Fields{}
	for _, field := range sb.mutationFields {
		mutationFields[field.Name()] = field.Serve()
	}

	subscriptionFields := graphql.Fields{}
	for _, field := range sb.subscriptionFields {
		subscriptionFields[field.Name()] = field.Serve()
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

	if len(subscriptionFields) > 0 {
		schemaConfig.Subscription = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Subscription",
			Fields: subscriptionFields,
		})
	}

	return graphql.NewSchema(schemaConfig)
}
