// Package schedule assembles the day's solar schedule for the sol
// command: sunrise, solar noon, sunset, and the golden-hour and
// blue-hour windows.
//
// Every time in this package comes from the shared library
// github.com/zaephor-bot/sol-lib; the package holds no solar math of
// its own. The library reads a date's location both to select the
// calendar day and to stamp the results, so For takes the location and
// builds the local midnight the library is handed.
package schedule

import (
	"errors"
	"time"

	"github.com/zaephor-bot/sol-lib/solar"
	"github.com/zaephor-bot/sol-lib/windows"
)

// Day is the solar schedule for one calendar date at one location.
//
// Sunrise and Sunset are nil when the event does not happen on the
// requested day; Polar then names why ("polar night" or "midnight
// sun"). SolarNoon always exists, even on a polar day or night. Each
// window field holds the library's Window record, whose Absent flag
// marks a window that does not occur that day.
type Day struct {
	Sunrise        *time.Time
	Sunset         *time.Time
	Polar          string
	SolarNoon      time.Time
	DawnGoldenHour windows.Window
	DuskGoldenHour windows.Window
	DawnBlueHour   windows.Window
	DuskBlueHour   windows.Window
}

// For computes the schedule for the calendar date (year, month, day)
// read in loc, at latitude and longitude in decimal degrees (north and
// east positive). Every event is asked of sol-lib.
func For(year int, month time.Month, day int, loc *time.Location, lat, lon float64) Day {
	local := time.Date(year, month, day, 0, 0, 0, 0, loc)

	var d Day
	if rise, err := solar.Sunrise(local, lat, lon); err == nil {
		d.Sunrise = &rise
	} else {
		d.Polar = polarNote(err)
	}
	if set, err := solar.Sunset(local, lat, lon); err == nil {
		d.Sunset = &set
	} else if d.Polar == "" {
		d.Polar = polarNote(err)
	}
	d.SolarNoon, _ = solar.SolarNoon(local, lat, lon)
	d.DawnGoldenHour, d.DuskGoldenHour = windows.GoldenHour(local, lat, lon)
	d.DawnBlueHour, d.DuskBlueHour = windows.BlueHour(local, lat, lon)
	return d
}

// polarNote maps the library's sentinel errors for a missing rise or
// set event to the spoken reason.
func polarNote(err error) string {
	switch {
	case errors.Is(err, solar.ErrSunNeverRises):
		return "polar night"
	case errors.Is(err, solar.ErrSunNeverSets):
		return "midnight sun"
	default:
		return ""
	}
}
