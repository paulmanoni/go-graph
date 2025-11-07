package graph

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/graphql-go/graphql"
	"gorm.io/gorm"
)

// PageableRequest represents pagination parameters
type PageableRequest struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Sort       *SortRequest `json:"sort,omitempty"`
}

// SortRequest represents sorting parameters
type SortRequest struct {
	Orders []SortOrder `json:"orders"`
}

// SortOrder represents individual sort criteria
type SortOrder struct {
	Property   string `json:"property"`
	Direction  string `json:"direction"` // ASC or DESC
	IgnoreCase bool   `json:"ignoreCase"`
}

// SearchRequest represents search/filter parameters
type SearchRequest struct {
	SearchFields      []SearchField `json:"searchFields"`
	SearchCombination string        `json:"searchCombinationType"` // And or Or
}

// SearchField represents individual search criteria
type SearchField struct {
	Field    string      `json:"field_name"`
	Value    interface{} `json:"field_value"`
	Operator string      `json:"search_type"` // LIKE, EQUAL, GT, LT, etc.
}

// PageableResponse represents the paginated response structure
type PageableResponse[T any] struct {
	Size             int   `json:"size"`
	Last             bool  `json:"last"`
	HasNext          bool  `json:"hasNext"`
	HasContent       bool  `json:"hasContent"`
	NumberOfElements int   `json:"numberOfElements"`
	Number           int   `json:"number"`
	TotalElements    int64 `json:"totalElements"`
	Content          []T   `json:"content"`
}

// QueryOptions contains all query parameters
type QueryOptions struct {
	Pageable     *PageableRequest       `json:"pageable,omitempty"`
	Search       *SearchRequest         `json:"search,omitempty"`
	Preloads     []string               `json:"preloads,omitempty"`   // For eager loading relationships
	Conditions   map[string]interface{} `json:"conditions,omitempty"` // Additional where conditions
	Joins        []string               `json:"joins,omitempty"`
	JoinPreloads map[string]interface{} `json:"joinPreloads,omitempty"`
}

// GenericQueryPageable executes a pageable query for any GORM model
func GenericQueryPageable[T any](db *gorm.DB, options *QueryOptions) (*PageableResponse[T], error) {
	var models []T
	var totalCount int64

	// Build base query
	query := db.Model(new(T))

	// Apply search conditions
	if options.Search != nil && len(options.Search.SearchFields) > 0 {
		query = applySearchConditions(query, options.Search)
	}

	// Apply additional conditions
	if options.Conditions != nil {
		for field, value := range options.Conditions {
			if strings.Contains(field, ">") || strings.Contains(field, "<") || strings.Contains(field, ">=") || strings.Contains(field, "<=") || strings.Contains(field, "=") {
				query = query.Where(fmt.Sprintf("%s ?", field), value)
			} else {
				query = query.Where(fmt.Sprintf("%s = ?", field), value)
			}
		}
	}

	if options.Joins != nil {
		for _, value := range options.Joins {
			fmt.Println(value)
			query = query.Joins(value)
		}
	}

	// Count total records (before pagination)
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count records: %w", err)
	}

	// Apply preloads for relationships
	if options.Preloads != nil {
		for _, preload := range options.Preloads {
			query = query.Preload(preload)
		}
	}

	if options.JoinPreloads != nil {
		for field, value := range options.JoinPreloads {
			query = query.Joins(field, value)
		}
	}

	// Apply sorting
	if options.Pageable != nil && options.Pageable.Sort != nil {
		query = applySorting(query, options.Pageable.Sort)
	}

	// Apply pagination
	if options.Pageable != nil {
		offset := options.Pageable.PageNumber * options.Pageable.PageSize
		query = query.Offset(offset).Limit(options.Pageable.PageSize)
	}

	// Execute query

	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Build response
	response := buildPageableResponse[T](models, totalCount, options.Pageable)

	return response, nil
}

func camelToSnake(s string) string {
	// insert underscore before capital letters
	re := regexp.MustCompile(`([a-z0-9])([A-Z])`)
	snake := re.ReplaceAllString(s, `${1}_${2}`)
	// to lowercase
	return strings.ToLower(snake)
}

// applySearchConditions applies search filters to the query
func applySearchConditions(query *gorm.DB, search *SearchRequest) *gorm.DB {
	fmt.Printf("Debug applySearchConditions - SearchFields count: %d\n", len(search.SearchFields))
	for i, sf := range search.SearchFields {
		fmt.Printf("Debug SearchField[%d]: Field=%s, Value=%v, Operator=%s\n", i, sf.Field, sf.Value, sf.Operator)
	}
	if len(search.SearchFields) == 0 {
		return query
	}

	combination := strings.ToUpper(search.SearchCombination)
	if combination == "" {
		combination = "AND"
	}

	var conditions []string
	var values []interface{}

	for _, field := range search.SearchFields {
		condition, value := buildSearchCondition(field)
		if condition != "" {
			conditions = append(conditions, condition)
			if value != nil {
				values = append(values, value)
			}
		}
	}

	if len(conditions) > 0 {
		var whereClause string
		if combination == "OR" {
			whereClause = "(" + strings.Join(conditions, " OR ") + ")"
		} else {
			whereClause = "(" + strings.Join(conditions, " AND ") + ")"
		}

		fmt.Println(whereClause, values)

		if len(values) > 0 {
			query = query.Where(whereClause, values...)
		} else {
			query = query.Where(whereClause)
		}
	}

	return query
}

// buildSearchCondition builds individual search condition
func buildSearchCondition(field SearchField) (string, interface{}) {
	fmt.Printf("Debug buildSearchCondition - Field: %+v\n", field)

	// Check if the field name is empty
	if field.Field == "" {
		fmt.Printf("ERROR: Empty field name in SearchField: %+v\n", field)
		return "", nil
	}

	// Convert camelCase to snake_case
	columnName := field.Field

	// Add table prefix for ambiguous columns
	// Assuming these fields belong to the candidates table
	// You can make this configurable if needed

	qualifiedColumn := columnName

	operator := strings.ToUpper(field.Operator)
	if operator == "" {
		operator = "EQUAL"
	}

	switch operator {
	case "LIKE":
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", qualifiedColumn), fmt.Sprintf("%%%v%%", field.Value)
	case "ILIKE": // Case insensitive like
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", qualifiedColumn), fmt.Sprintf("%%%v%%", field.Value)
	case "EQUAL", "EQ":
		return fmt.Sprintf("%s = ?", qualifiedColumn), field.Value
	case "NOT_EQUAL", "NE":
		return fmt.Sprintf("%s != ?", qualifiedColumn), field.Value
	case "GREATER_THAN", "GT":
		return fmt.Sprintf("%s > ?", qualifiedColumn), field.Value
	case "GREATER_THAN_EQUAL", "GTE":
		return fmt.Sprintf("%s >= ?", qualifiedColumn), field.Value
	case "LESS_THAN", "LT":
		return fmt.Sprintf("%s < ?", qualifiedColumn), field.Value
	case "LESS_THAN_EQUAL", "LTE":
		return fmt.Sprintf("%s <= ?", qualifiedColumn), field.Value
	case "IN":
		return fmt.Sprintf("%s IN ?", qualifiedColumn), field.Value
	case "NOT_IN":
		return fmt.Sprintf("%s NOT IN ?", qualifiedColumn), field.Value
	case "IS_NULL":
		return fmt.Sprintf("%s IS NULL", qualifiedColumn), nil
	case "IS_NOT_NULL":
		return fmt.Sprintf("%s IS NOT NULL", qualifiedColumn), nil
	default:
		return fmt.Sprintf("%s = ?", qualifiedColumn), field.Value
	}
}

// applySorting applies sorting to the query
func applySorting(query *gorm.DB, sort *SortRequest) *gorm.DB {
	for _, order := range sort.Orders {
		direction := strings.ToUpper(order.Direction)
		if direction != "ASC" && direction != "DESC" {
			direction = "ASC"
		}

		orderClause := fmt.Sprintf("%s %s", order.Property, direction)
		fmt.Println(orderClause)
		query = query.Order(orderClause)
	}

	return query
}

// buildPageableResponse builds the pageable response structure
func buildPageableResponse[T any](content []T, totalCount int64, pageable *PageableRequest) *PageableResponse[T] {
	numberOfElements := len(content)

	var pageNumber, pageSize int
	if pageable != nil {
		pageNumber = pageable.PageNumber
		pageSize = pageable.PageSize
	}

	var hasNext bool
	if pageSize > 0 {
		hasNext = int64((pageNumber+1)*pageSize) < totalCount
	}

	return &PageableResponse[T]{
		Size:             pageSize,
		Last:             !hasNext,
		HasNext:          hasNext,
		HasContent:       numberOfElements > 0,
		NumberOfElements: numberOfElements,
		Number:           pageNumber,
		TotalElements:    totalCount,
		Content:          content,
	}
}

func normalize(in map[interface{}]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range in {
		key := fmt.Sprintf("%v", k)
		out[key] = v
	}
	return out
}

// MapGraphQLParamsToQueryOptions maps GraphQL ResolveParams to QueryOptions
// argName is optional - defaults to "searchDataDto" if not provided
func MapGraphQLParamsToQueryOptions(p graphql.ResolveParams, Preloads []string, Conditions map[string]interface{}, argName ...string) *QueryOptions {
	options := &QueryOptions{}
	options.Preloads = Preloads
	options.Conditions = Conditions

	// Use "searchDataDto" as default, or the provided argument name
	searchArgName := "searchDataDto"
	if len(argName) > 0 && argName[0] != "" {
		searchArgName = argName[0]
	}

	// Extract search data from params using the specified or default argument name
	if searchData, ok := p.Args[searchArgName].(map[string]interface{}); ok {
		// Handle pagination
		if pageable, ok := searchData["pageable"].(map[string]interface{}); ok {
			options.Pageable = &PageableRequest{}

			if pageNumber, ok := pageable["pageNumber"].(int); ok {
				options.Pageable.PageNumber = pageNumber
			}

			if pageSize, ok := pageable["pageSize"].(int); ok {
				options.Pageable.PageSize = pageSize
			}

			// Handle sorting
			if sort, ok := pageable["sort"].(map[string]interface{}); ok {
				if orders, ok := sort["orders"].([]interface{}); ok {
					var sortOrders []SortOrder
					for _, order := range orders {
						if orderMap, ok := order.(map[string]interface{}); ok {
							sortOrder := SortOrder{}

							if property, ok := orderMap["property"].(string); ok {
								sortOrder.Property = property
							}

							if direction, ok := orderMap["direction"].(string); ok {
								sortOrder.Direction = direction
							}

							if ignoreCase, ok := orderMap["ignoreCase"].(bool); ok {
								sortOrder.IgnoreCase = ignoreCase
							}

							sortOrders = append(sortOrders, sortOrder)
						}
					}

					if len(sortOrders) > 0 {
						options.Pageable.Sort = &SortRequest{
							Orders: sortOrders,
						}
					}
				}
			}
		}

		// Handle search/filter
		options.Search = &SearchRequest{}

		if searchCombination, ok := searchData["searchCombinationType"].(string); ok {
			options.Search.SearchCombination = searchCombination
		}

		if rawFields, ok := searchData["searchFields"]; ok {
			if searchFields, ok := rawFields.([]interface{}); ok {
				var fields []SearchField

				for _, f := range searchFields {
					sf := SearchField{}

					fmt.Printf("Debug: Processing search field: %+v (type: %T)\n", f, f)

					switch v := f.(type) {
					case map[string]interface{}:
						fmt.Printf("Debug: Field is map[string]interface{}: %+v\n", v)

						// Try snake_case first (field_name), then camelCase (fieldName)
						if name, ok := v["field_name"].(string); ok {
							sf.Field = name
							fmt.Printf("Debug: Found field_name: %s\n", name)
						} else if name, ok := v["fieldName"].(string); ok {
							sf.Field = name
							fmt.Printf("Debug: Found fieldName: %s\n", name)
						}

						// Try snake_case first (field_value), then camelCase (fieldValue)
						if val, exists := v["field_value"]; exists {
							sf.Value = val
							fmt.Printf("Debug: Found field_value: %v\n", val)
						} else if val, exists := v["fieldValue"]; exists {
							sf.Value = val
							fmt.Printf("Debug: Found fieldValue: %v\n", val)
						}

						// Try search_type first, then operator, then searchType
						if st, ok := v["search_type"].(string); ok {
							sf.Operator = st
							fmt.Printf("Debug: Found search_type: %s\n", st)
						} else if op, ok := v["operator"].(string); ok {
							sf.Operator = op
							fmt.Printf("Debug: Found operator: %s\n", op)
						} else if st, ok := v["searchType"].(string); ok {
							sf.Operator = st
							fmt.Printf("Debug: Found searchType: %s\n", st)
						}

						fmt.Printf("Debug: Final SearchField: %+v\n", sf)
					case string:
						// Check if it's a string representation of a map
						if strings.HasPrefix(v, "map[") {
							fmt.Printf("Debug: Found map string representation: %s\n", v)
							// Parse the map string representation
							// Format: map[fieldName:fullName fieldValue:MAGES searchType:Like]
							// or: map[field_name:full_name field_value:mages search_type:like]

							// Extract key-value pairs
							mapContent := strings.TrimPrefix(v, "map[")
							mapContent = strings.TrimSuffix(mapContent, "]")

							// Parse the key-value pairs using a more robust approach
							mapData := make(map[string]string)

							// Find all key positions first
							keyRegex := regexp.MustCompile(`(\w+):`)
							keyMatches := keyRegex.FindAllStringSubmatchIndex(mapContent, -1)

							// Extract key-value pairs based on positions
							for i, match := range keyMatches {
								if len(match) >= 4 {
									// Extract the key
									key := mapContent[match[2]:match[3]]

									// Extract the value (from after the colon to the next key or end)
									valueStart := match[1] // Position after the colon
									var valueEnd int

									if i+1 < len(keyMatches) {
										// Value ends where the next key starts
										valueEnd = keyMatches[i+1][0]
									} else {
										// Last key-value pair, value goes to the end
										valueEnd = len(mapContent)
									}

									value := strings.TrimSpace(mapContent[valueStart:valueEnd])
									mapData[key] = value
								}
							}

							// Try to extract field name (support both camelCase and snake_case)
							if name, ok := mapData["field_name"]; ok {
								sf.Field = name
							} else if name, ok := mapData["fieldName"]; ok {
								sf.Field = name
							}

							// Try to extract field value
							if val, ok := mapData["field_value"]; ok {
								sf.Value = val
							} else if val, ok := mapData["fieldValue"]; ok {
								sf.Value = val
							}

							// Try to extract operator/search type
							if op, ok := mapData["search_type"]; ok {
								sf.Operator = op
							} else if op, ok := mapData["searchType"]; ok {
								sf.Operator = op
							} else if op, ok := mapData["operator"]; ok {
								sf.Operator = op
							}

							fmt.Printf("Debug: Parsed SearchField from string: %+v\n", sf)
						} else {
							// if only a raw field name is provided
							sf.Field = v
							sf.Value = nil
							sf.Operator = "=" // default operator
						}

					default:
						fmt.Printf("skipping unexpected type: %T\n", v)
						continue
					}

					fields = append(fields, sf)
				}

				if len(fields) > 0 {
					options.Search.SearchFields = fields
				}
			}
		}

		// Handle preloads
		if preloads, ok := searchData["preloads"].([]interface{}); ok {
			for _, preload := range preloads {
				if preloadStr, ok := preload.(string); ok {
					options.Preloads = append(options.Preloads, preloadStr)
				}
			}
		}

		// Handle additional conditions
		if conditions, ok := searchData["conditions"].(map[string]interface{}); ok {
			options.Conditions = conditions
		}
	}

	// Set defaults if not provided
	if options.Pageable == nil {
		options.Pageable = &PageableRequest{
			PageNumber: 0,
			PageSize:   10,
		}
	}

	return options
}

// CreateQueryOptionsFromGraphQL creates query options from individual parameters
func CreateQueryOptionsFromGraphQL(pageNumber, pageSize int, sortOrders []SortOrder, searchFields []SearchField, searchCombination string) *QueryOptions {
	options := &QueryOptions{
		Pageable: &PageableRequest{
			PageNumber: pageNumber,
			PageSize:   pageSize,
		},
	}

	if len(sortOrders) > 0 {
		options.Pageable.Sort = &SortRequest{
			Orders: sortOrders,
		}
	}

	if len(searchFields) > 0 {
		options.Search = &SearchRequest{
			SearchFields:      searchFields,
			SearchCombination: searchCombination,
		}
	}

	return options
}
