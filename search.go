package pkg

import "fmt"

func MapSearchFields(searchFields []SearchField, fieldMap map[string]string) []SearchField {
	var searches []SearchField

	for _, query := range searchFields {
		fmt.Print(query.Value)
		if mappedField, exists := fieldMap[query.Field]; exists {
			searches = append(searches, SearchField{
				Field:    mappedField,
				Value:    query.Value,
				Operator: query.Operator,
			})
		}
	}

	return searches
}
