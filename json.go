package graph

import (
	"encoding/json"
	"fmt"
	"time"
)

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
