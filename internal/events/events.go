// Package events computes solar events for a location and a date.
//
// It is the single source of truth for every solar-event calculation served by
// the API. The /v1/events and /v1/golden-hour routes both derive their output
// from Calculate, so the two routes can never disagree about a window.
//
// Conventions:
//
//   - Dates are calendar dates formatted YYYY-MM-DD and are read as the UTC
//     calendar date for which the solar day is computed.
//   - All returned times are UTC and formatted RFC3339 by the json encoder.
//   - Latitudes are north-positive; longitudes are east-positive.
package events

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// Solar altitude thresholds, in degrees above the horizon. Golden hour is the
// window during which the sun sits between altitudeGoldenLow and
// altitudeGoldenHigh; sunrise and sunset use the standard -0.833° threshold
// (upper limb touching the horizon, corrected for refraction and solar
// parallax), and civil twilight uses -6°.
const (
	altitudeSunriseSet  = -0.833
	altitudeCivil       = -6.0
	altitudeGoldenLow   = -4.0
	altitudeGoldenHigh  = 6.0
)

// DateLayout is the only accepted date input format.
const DateLayout = "2006-01-02"

// Window is a half-open [Start, End) interval of solar events.
type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Events is the full solar-event payload for one location and date. Fields
// whose event never occurs (e.g. sunrise during polar night) are nil and
// marshal as JSON null.
type Events struct {
	Date               string   `json:"date"`
	Lat                float64  `json:"lat"`
	Lon                float64  `json:"lon"`
	Sunrise            *time.Time `json:"sunrise"`
	Sunset             *time.Time `json:"sunset"`
	SolarNoon          time.Time  `json:"solar_noon"`
	CivilDawn          *time.Time `json:"civil_dawn"`
	CivilDusk          *time.Time `json:"civil_dusk"`
	MorningGoldenHour  *Window  `json:"morning_golden_hour"`
	EveningGoldenHour  *Window  `json:"evening_golden_hour"`
}

// Query is a validated request: a coordinate pair and a calendar date.
type Query struct {
	Lat  float64
	Lon  float64
	Date time.Time
}

// ParseDate parses a strict YYYY-MM-DD string that must also be a real
// calendar date (2024-02-30 is rejected). The returned time is midnight UTC.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil || t.Format(DateLayout) != s {
		return time.Time{}, fmt.Errorf("invalid parameter 'date': must be a real calendar date formatted YYYY-MM-DD")
	}
	return t, nil
}

// ParseQuery validates raw lat/lon/date request values. Both API routes use
// this function, so any request one route accepts the other accepts too.
// Every error message names the offending parameter.
func ParseQuery(latStr, lonStr, dateStr string) (Query, error) {
	var q Query

	if latStr == "" {
		return q, fmt.Errorf("missing required parameter: lat")
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil || math.IsNaN(lat) || math.IsInf(lat, 0) {
		return q, fmt.Errorf("invalid parameter 'lat': must be a number")
	}
	if lat < -90 || lat > 90 {
		return q, fmt.Errorf("invalid parameter 'lat': must be between -90 and 90")
	}

	if lonStr == "" {
		return q, fmt.Errorf("missing required parameter: lon")
	}
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil || math.IsNaN(lon) || math.IsInf(lon, 0) {
		return q, fmt.Errorf("invalid parameter 'lon': must be a number")
	}
	if lon < -180 || lon > 180 {
		return q, fmt.Errorf("invalid parameter 'lon': must be between -180 and 180")
	}

	if dateStr == "" {
		return q, fmt.Errorf("missing required parameter: date")
	}
	date, err := ParseDate(dateStr)
	if err != nil {
		return q, err
	}

	return Query{Lat: lat, Lon: lon, Date: date}, nil
}

// Calculate returns the solar events for a validated query, using the NOAA
// solar position equations (one refinement pass at solar noon).
func Calculate(q Query) Events {
	utcMidnight := q.Date.UTC()
	jd0 := julianDay(utcMidnight)

	// First pass near midnight to locate solar noon, then refine the solar
	// coordinates at solar noon itself.
	noonMin := solarNoonMinutes(jd0, q.Lon)
	s := solarAt(jd0 + noonMin/minutesPerDay)
	noonMin = solarNoonMinutesAt(s, q.Lon)
	noon := atMinute(utcMidnight, noonMin)

	riseSet := crossings(q.Lat, s, utcMidnight, noonMin)

	return Events{
		Date:               utcMidnight.Format(DateLayout),
		Lat:                q.Lat,
		Lon:                q.Lon,
		Sunrise:            riseSet(altitudeSunriseSet).rising,
		Sunset:             riseSet(altitudeSunriseSet).setting,
		SolarNoon:          noon,
		CivilDawn:          riseSet(altitudeCivil).rising,
		CivilDusk:          riseSet(altitudeCivil).setting,
		MorningGoldenHour:  window(riseSet(altitudeGoldenLow).rising, riseSet(altitudeGoldenHigh).rising),
		EveningGoldenHour:  window(riseSet(altitudeGoldenHigh).setting, riseSet(altitudeGoldenLow).setting),
	}
}

const minutesPerDay = 1440.0

// crossing is one altitude-crossing pair for a single solar day; either side
// may be nil when the sun never reaches that altitude (polar day/night).
type crossing struct {
	rising  *time.Time
	setting *time.Time
}

// solarAt returns the solar coordinates (declination in degrees, equation of
// time in minutes) for the given Julian day.
type solar struct {
	decl float64
	eqt  float64
}

func solarAt(jd float64) solar {
	t := (jd - 2451545.0) / 36525.0

	l0 := math.Mod(280.46646+t*(36000.76983+t*0.0003032), 360.0)
	m := 357.52911 + t*(35999.05029-0.0001537*t)
	e := 0.016708634 - t*(0.000042037+0.0000001267*t)

	center := math.Sin(rad(m))*(1.914602-t*(0.004817+0.000014*t)) +
		math.Sin(rad(2*m))*(0.019993-0.000101*t) +
		math.Sin(rad(3*m))*0.000289
	trueLong := l0 + center
	appLong := trueLong - 0.00569 - 0.00478*math.Sin(rad(125.04-1934.136*t))

	obliq := 23.0 + (26.0+((21.448-t*(46.815+t*(0.00059-t*0.001813))))/60.0)/60.0
	obliqCorr := obliq + 0.00256*math.Cos(rad(125.04-1934.136*t))

	decl := deg(math.Asin(math.Sin(rad(obliqCorr)) * math.Sin(rad(appLong))))

	y := math.Pow(math.Tan(rad(obliqCorr)/2), 2)
	eqt := 4 * deg(y*math.Sin(rad(2*l0))-
		2*e*math.Sin(rad(m))+
		4*e*y*math.Sin(rad(m))*math.Cos(rad(2*l0))-
		0.5*y*y*math.Sin(rad(4*l0))-
		1.25*e*e*math.Sin(rad(2*m)))

	return solar{decl: decl, eqt: eqt}
}

func solarNoonMinutes(jd, lon float64) float64 {
	return solarNoonMinutesAt(solarAt(jd), lon)
}

func solarNoonMinutesAt(s solar, lon float64) float64 {
	return 720.0 - 4.0*lon - s.eqt
}

// crossings returns a function computing the rising/setting times for a
// given altitude threshold, closed over the solar coordinates of the day and
// the UTC midnight the day begins at.
func crossings(lat float64, s solar, day time.Time, noonMin float64) func(float64) crossing {
	return func(altitude float64) crossing {
		cosH := (math.Sin(rad(altitude)) - math.Sin(rad(lat))*math.Sin(rad(s.decl))) /
			(math.Cos(rad(lat)) * math.Cos(rad(s.decl)))
		if cosH < -1 || cosH > 1 {
			return crossing{}
		}
		ha := deg(math.Acos(cosH))
		rise := atMinute(day, noonMin-4.0*ha)
		set := atMinute(day, noonMin+4.0*ha)
		return crossing{rising: &rise, setting: &set}
	}
}

func atMinute(day time.Time, minutes float64) time.Time {
	return day.Add(time.Duration(minutes) * time.Minute).Round(time.Second).UTC()
}

func window(start, end *time.Time) *Window {
	if start == nil || end == nil {
		return nil
	}
	return &Window{Start: *start, End: *end}
}

// julianDay returns the Julian day for a UTC time.
func julianDay(t time.Time) float64 {
	y := t.Year()
	m := int(t.Month())
	d := float64(t.Day()) + (float64(t.Hour())+float64(t.Minute())/60.0+float64(t.Second())/3600.0)/24.0
	if m <= 2 {
		y--
		m += 12
	}
	a := math.Floor(float64(y) / 100.0)
	b := 2.0 - a + math.Floor(a/4.0)
	return math.Floor(365.25*(float64(y)+4716.0)) + math.Floor(30.6001*(float64(m)+1.0)) + d + b - 1524.5
}

func rad(deg float64) float64 { return deg * math.Pi / 180.0 }
func deg(rad float64) float64 { return rad * 180.0 / math.Pi }
