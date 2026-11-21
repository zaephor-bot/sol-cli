// Package solar computes sunrise, solar noon, and sunset for a calendar
// date and geographic location using the standard sunrise equation.
//
// All returned times are UTC instants rounded to the nearest minute; the
// caller renders each instant in whatever zone it likes.
package solar

import (
	"math"
	"time"
)

const (
	degToRad = math.Pi / 180

	// Julian day numbers.
	j2000     = 2451545.0 // 2000-01-01T12:00:00Z
	unixEpoch = 2440587.5 // 1970-01-01T00:00:00Z
	msPerDay  = 86400000.0

	// Apparent zenith of the sun's center at rise/set: -0.833° accounts
	// for atmospheric refraction and the solar radius.
	riseSetZenith = -0.833

	// Obliquity of the ecliptic.
	obliquity = 23.4397
)

var j2000Day = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// Day is the solar event schedule for one calendar date.
//
// Sunrise and Sunset are nil on polar days: during the midnight sun the
// sun never rises, and during the polar night it never sets — in both
// cases there is no rise or set event to report. SolarNoon always exists.
type Day struct {
	Sunrise   *time.Time
	SolarNoon time.Time
	Sunset    *time.Time
}

// Schedule computes the solar events for the given calendar date
// (interpreted in UTC, i.e. a plain date, not a wall-clock zone) at the
// given latitude and longitude in decimal degrees.
func Schedule(year int, month time.Month, day int, lat, lon float64) Day {
	// Whole days from J2000 to the target date.
	n := math.Round(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Sub(j2000Day).Hours() / 24)

	// Mean solar time of the date at this longitude (east positive), then
	// the solar mean anomaly and equation of the center.
	jStar := n - lon/360
	M := mod360(357.5291 + 0.98560028*jStar)
	C := 1.9148*math.Sin(M*degToRad) +
		0.02*math.Sin(2*M*degToRad) +
		0.0003*math.Sin(3*M*degToRad)

	// Ecliptic longitude and the equation of time correction to transit.
	lambda := mod360(M + C + 180 + 102.9372)
	jTransit := j2000 + jStar + 0.0053*math.Sin(M*degToRad) - 0.0069*math.Sin(2*lambda*degToRad)
	noon := roundMinute(jdToTime(jTransit))

	// Solar declination and the hour angle at the rise/set zenith.
	decl := math.Asin(math.Sin(obliquity*degToRad) * math.Sin(lambda*degToRad))
	cosOmega := (math.Sin(riseSetZenith*degToRad) - math.Sin(lat*degToRad)*math.Sin(decl)) /
		(math.Cos(lat*degToRad) * math.Cos(decl))

	if cosOmega > 1 || cosOmega < -1 {
		// Polar day or polar night: no rise or set events this date.
		return Day{Sunrise: nil, SolarNoon: noon, Sunset: nil}
	}

	omega := math.Acos(cosOmega) / degToRad // degrees
	rise := roundMinute(jdToTime(jTransit - omega/360))
	set := roundMinute(jdToTime(jTransit + omega/360))
	return Day{Sunrise: &rise, SolarNoon: noon, Sunset: &set}
}

func mod360(x float64) float64 {
	return x - 360*math.Floor(x/360)
}

func jdToTime(jd float64) time.Time {
	return time.UnixMilli(int64(math.Round((jd - unixEpoch) * msPerDay))).UTC()
}

func roundMinute(t time.Time) time.Time {
	ms := math.Round(float64(t.UnixMilli())/60000.0) * 60000.0
	return time.UnixMilli(int64(ms)).UTC()
}
