package pkg

import "time"

func CurrentTime() time.Time {
	loc, _ := time.LoadLocation("Africa/Dar_es_Salaam")

	// Get current time in Dar es Salaam
	darTime := time.Now().In(loc)

	// Rebuild the same clock values, but in UTC zone
	fakeUTC := time.Date(
		darTime.Year(),
		darTime.Month(),
		darTime.Day(),
		darTime.Hour(),
		darTime.Minute(),
		darTime.Second(),
		darTime.Nanosecond(),
		time.UTC,
	)
	return fakeUTC
}
