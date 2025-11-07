package graph

import (
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

const SpringShortLayout = "2006-01-02T15:04"

func serializeDateTime(value interface{}) interface{} {
	if t, ok := value.(time.Time); ok {
		// always UTC to match Spring Boot style
		return t.UTC().Format(SpringShortLayout)
	}
	if t, ok := value.(*time.Time); ok && t != nil {
		return t.UTC().Format(SpringShortLayout)
	}
	return nil
}

func unserializeDateTime(value interface{}) interface{} {
	if s, ok := value.(string); ok {
		if t, err := time.Parse(SpringShortLayout, s); err == nil {
			return t.UTC()
		}
	}
	return nil
}

var DateTime *graphql.Scalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "DateTime",
	Description: "The `DateTime` scalar type formatted as yyyy-MM-dd'T'HH:mm",
	Serialize:   serializeDateTime,
	ParseValue:  unserializeDateTime,
	ParseLiteral: func(valueAST ast.Value) interface{} {
		if v, ok := valueAST.(*ast.StringValue); ok {
			return unserializeDateTime(v.Value)
		}
		return nil
	},
})
