package data

import "time"

var phillyLocation = mustLoadLocation("America/New_York")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("ET", -5*60*60)
	}
	return loc
}

func PhillyTime(t time.Time) time.Time {
	return t.In(phillyLocation)
}

func NowPhilly() time.Time {
	return time.Now().In(phillyLocation)
}

func DatePhilly(year int, month time.Month, day int, hour int, min int, sec int) time.Time {
	return time.Date(year, month, day, hour, min, sec, 0, phillyLocation)
}
