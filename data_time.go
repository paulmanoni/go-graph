package pkg

import (
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// Initialize the Dar es Salaam location to use globally
var darEsSalaamLocation *time.Location

//
//func init() {
//	var err error
//	// Load the Africa/Dar_es_Salaam timezone
//	darEsSalaamLocation, err = time.LoadLocation("UTC")
//	if err != nil {
//		panic("Failed to load Africa/Dar_es_Salaam timezone: " + err.Error())
//	}
//}

// Define a custom time scalar using Africa/Dar_es_Salaam timezone
var DarEsSalaamTimeScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "Time",
	Description: "Time scalar type in Africa/Dar_es_Salaam timezone",
	Serialize: func(value interface{}) interface{} {
		switch t := value.(type) {
		case time.Time:
			// Convert to Dar es Salaam timezone before serializing
			return t.In(darEsSalaamLocation).Format(time.RFC3339)
		case *time.Time:
			if t == nil {
				return nil
			}
			return t.In(darEsSalaamLocation).Format(time.RFC3339)
		default:
			return nil
		}
	},
	ParseValue: func(value interface{}) interface{} {
		switch value := value.(type) {
		case string:
			t, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return nil
			}
			// Return the time in Dar es Salaam timezone
			return t.In(darEsSalaamLocation)
		default:
			return nil
		}
	},
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			t, err := time.Parse(time.RFC3339, valueAST.Value)
			if err != nil {
				return nil
			}
			// Return the time in Dar es Salaam timezone
			return t.In(darEsSalaamLocation)
		default:
			return nil
		}
	},
})
