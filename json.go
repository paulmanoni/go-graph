package pkg

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

var JSONScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSON",
	Description: "The `JSON` scalar type represents JSON objects as JSON strings",
	// Define how JSON values are serialized
	Serialize: func(value interface{}) interface{} {
		switch value := value.(type) {
		case string:
			return value
		default:
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return nil
			}
			return string(jsonBytes)
		}
	},
	// Define how JSON values are parsed from variables
	ParseValue: func(value interface{}) interface{} {
		switch value := value.(type) {
		case string:
			var result interface{}
			err := json.Unmarshal([]byte(value), &result)
			if err != nil {
				return nil
			}
			return result
		default:
			return nil
		}
	},
	// Define how JSON values are parsed from literals in the query
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			var result interface{}
			err := json.Unmarshal([]byte(valueAST.Value), &result)
			if err != nil {
				return nil
			}
			return result
		default:
			return nil
		}
	},
})

var JSONTimeScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "JSONTime",
	Description: "The `JSONTime` scalar type represents time values that can be in string or array format",
	// Define how JSONTime values are serialized
	Serialize: func(value interface{}) interface{} {
		switch v := value.(type) {
		case JSONTime:
			return time.Time(v).Format(time.RFC3339)
		case *JSONTime:
			if v == nil {
				return nil
			}
			return time.Time(*v).Format(time.RFC3339)
		case time.Time:
			return v.Format(time.RFC3339)
		default:
			return nil
		}
	},
	// Define how JSONTime values are parsed from variables
	ParseValue: func(value interface{}) interface{} {
		switch v := value.(type) {
		case string:
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return nil
			}
			return JSONTime(t)
		default:
			return nil
		}
	},
	// Define how JSONTime values are parsed from literals in the query
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			t, err := time.Parse(time.RFC3339, valueAST.Value)
			if err != nil {
				return nil
			}
			return JSONTime(t)
		default:
			return nil
		}
	},
})

// JSONTime is a custom time type for JSON operations
type JSONTime time.Time

// MarshalJSON implements the json.Marshaler interface
func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", time.Time(t).Format(time.RFC3339))
	return []byte(stamp), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (t *JSONTime) UnmarshalJSON(data []byte) error {
	// Check for null
	if string(data) == "null" {
		return nil
	}

	// Check if data is a string
	if data[0] == '"' {
		// Try standard time formats if it's a JSON string
		tt, err := time.Parse(`"`+time.RFC3339+`"`, string(data))
		if err == nil {
			*t = JSONTime(tt)
			return nil
		}
		return err
	}

	// Handle array format [year, month, day, hour, minute, second?, nanosecond?]
	var timeArray []int
	if err := json.Unmarshal(data, &timeArray); err != nil {
		return err
	}

	// Ensure we have at least year, month, day
	if len(timeArray) < 3 {
		return fmt.Errorf("invalid time array format: %v", timeArray)
	}

	// Extract values from array with safe defaults
	year := timeArray[0]
	month := time.Month(timeArray[1])
	day := timeArray[2]

	// Default time values if not provided
	hour := 0
	if len(timeArray) > 3 {
		hour = timeArray[3]
	}

	minute := 0
	if len(timeArray) > 4 {
		minute = timeArray[4]
	}

	second := 0
	if len(timeArray) > 5 {
		second = timeArray[5]
	}

	// Handle nanoseconds if available
	var nsec int
	if len(timeArray) > 6 {
		nsec = timeArray[6]
	}

	// Create the time
	tt := time.Date(year, month, day, hour, minute, second, nsec, time.UTC)
	*t = JSONTime(tt)
	return nil
}

// Add a helper method to convert back to time.Time
func (t JSONTime) Time() time.Time {
	return time.Time(t)
}
