package events

import (
	"strings"
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	for _, in := range []string{"2024-06-21", "2023-02-28", "2024-02-29", "1999-12-31", "0001-01-01"} {
		if _, err := ParseDate(in); err != nil {
			t.Errorf("ParseDate(%q) unexpected error: %v", in, err)
		}
	}
	for _, in := range []string{
		"",             // empty
		"2024-6-21",    // not zero-padded
		"24-06-21",     // two-digit year
		"2024/06/21",   // wrong separator
		"2024-06-21T00:00:00Z", // trailing time
		"2024-13-01",   // month out of range
		"2024-00-10",   // month out of range
		"2024-02-30",   // day out of range for February
		"2023-02-29",   // not a leap year
		"2024-04-31",   // April has 30 days
		"2024-06-32",   // day out of range
		"not-a-date",
	} {
		if _, err := ParseDate(in); err == nil {
			t.Errorf("ParseDate(%q) accepted invalid date", in)
		}
	}
}

func TestParseQueryMissingParameters(t *testing.T) {
	cases := []struct {
		lat, lon, date string
		param          string
	}{
		{"", "-74.006", "2024-06-21", "lat"},
		{"40.7128", "", "2024-06-21", "lon"},
		{"40.7128", "-74.006", "", "date"},
	}
	for _, tc := range cases {
		_, err := ParseQuery(tc.lat, tc.lon, tc.date)
		if err == nil {
			t.Errorf("ParseQuery(lat=%q, lon=%q, date=%q) accepted missing %s", tc.lat, tc.lon, tc.date, tc.param)
			continue
		}
		if !strings.Contains(err.Error(), tc.param) {
			t.Errorf("ParseQuery(lat=%q, lon=%q, date=%q) error %q does not name %s", tc.lat, tc.lon, tc.date, err, tc.param)
		}
	}
}

func TestParseQueryInvalidValues(t *testing.T) {
	cases := []struct {
		lat, lon, date string
		param          string
	}{
		{"abc", "-74.006", "2024-06-21", "lat"},
		{"91", "-74.006", "2024-06-21", "lat"},
		{"-90.5", "-74.006", "2024-06-21", "lat"},
		{"NaN", "-74.006", "2024-06-21", "lat"},
		{"40.7128", "xyz", "2024-06-21", "lon"},
		{"40.7128", "180.001", "2024-06-21", "lon"},
		{"40.7128", "-180.5", "2024-06-21", "lon"},
		{"40.7128", "-74.006", "2024-02-30", "date"},
		{"40.7128", "-74.006", "2024-6-21", "date"},
		{"40.7128", "-74.006", "20240621", "date"},
	}
	for _, tc := range cases {
		_, err := ParseQuery(tc.lat, tc.lon, tc.date)
		if err == nil {
			t.Errorf("ParseQuery(lat=%q, lon=%q, date=%q) accepted invalid %s", tc.lat, tc.lon, tc.date, tc.param)
			continue
		}
		if !strings.Contains(err.Error(), tc.param) {
			t.Errorf("ParseQuery(lat=%q, lon=%q, date=%q) error %q does not name %s", tc.lat, tc.lon, tc.date, err, tc.param)
		}
	}
}

func TestParseQueryValidBoundaries(t *testing.T) {
	for _, tc := range []struct{ lat, lon, date string }{
		{"90", "180", "2024-06-21"},
		{"-90", "-180", "2024-06-21"},
		{"0", "0", "2024-01-01"},
	} {
		if _, err := ParseQuery(tc.lat, tc.lon, tc.date); err != nil {
			t.Errorf("ParseQuery(lat=%q, lon=%q, date=%q) unexpected error: %v", tc.lat, tc.lon, tc.date, err)
		}
	}
}

// referenceTimes are published almanac times for New York City on the 2024
// summer solstice, converted to UTC.
func TestCalculateMatchesAlmanac(t *testing.T) {
	d, err := ParseDate("2024-06-21")
	if err != nil {
		t.Fatal(err)
	}
	e := Calculate(Query{Lat: 40.7128, Lon: -74.006, Date: d})

	wantSunrise := time.Date(2024, 6, 21, 9, 25, 44, 0, time.UTC)   // 5:25:44 AM EDT
	wantSunset := time.Date(2024, 6, 22, 0, 31, 7, 0, time.UTC)     // 8:31:07 PM EDT
	wantNoon := time.Date(2024, 6, 21, 16, 57, 46, 0, time.UTC)     // 12:57:46 PM EDT

	assertClose(t, "sunrise", e.Sunrise, wantSunrise)
	assertClose(t, "sunset", e.Sunset, wantSunset)
	assertClose(t, "solar_noon", &e.SolarNoon, wantNoon)
}

func TestCalculateWindowInvariants(t *testing.T) {
	cases := []struct {
		name      string
		lat, lon  float64
		date      string
	}{
		{"nyc summer solstice", 40.7128, -74.006, "2024-06-21"},
		{"nyc winter solstice", 40.7128, -74.006, "2024-12-21"},
		{"london equinox", 51.5074, -0.1278, "2024-03-20"},
		{"sydney summer solstice", -33.8688, 151.2093, "2024-12-21"},
		{"equator", 0, 0, "2024-09-22"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := ParseDate(tc.date)
			if err != nil {
				t.Fatal(err)
			}
			e := Calculate(Query{Lat: tc.lat, Lon: tc.lon, Date: d})

			if e.MorningGoldenHour == nil || e.EveningGoldenHour == nil {
				t.Fatalf("expected non-nil golden-hour windows for %v", tc)
			}
			if e.Sunrise == nil || e.Sunset == nil {
				t.Fatalf("expected non-nil sunrise/sunset for %v", tc)
			}

			mg, eg := e.MorningGoldenHour, e.EveningGoldenHour
			if !mg.Start.Before(mg.End) {
				t.Errorf("morning window not ordered: %v >= %v", mg.Start, mg.End)
			}
			if !eg.Start.Before(eg.End) {
				t.Errorf("evening window not ordered: %v >= %v", eg.Start, eg.End)
			}
			// Sunrise (-0.833°) must fall inside the morning golden-hour
			// window (-4° .. +6°), and sunset inside the evening window.
			if !(*e.Sunrise).After(mg.Start) || !(*e.Sunrise).Before(mg.End) {
				t.Errorf("sunrise %v outside morning window [%v, %v]", *e.Sunrise, mg.Start, mg.End)
			}
			if !(*e.Sunset).After(eg.Start) || !(*e.Sunset).Before(eg.End) {
				t.Errorf("sunset %v outside evening window [%v, %v]", *e.Sunset, eg.Start, eg.End)
			}
			// Solar noon sits between the end of the morning window and the
			// start of the evening window.
			if !e.SolarNoon.After(mg.End) || !e.SolarNoon.Before(eg.Start) {
				t.Errorf("solar noon %v not between windows ending %v and starting %v", e.SolarNoon, mg.End, eg.Start)
			}
		})
	}
}

func TestCalculateSydneyCrossesUTCMidnight(t *testing.T) {
	d, _ := ParseDate("2024-12-21")
	e := Calculate(Query{Lat: -33.8688, Lon: 151.2093, Date: d})
	// Sydney (UTC+11) starts its December day well before UTC midnight: the
	// sunrise computed for the requested date falls on the previous UTC day.
	if got := e.Sunrise.UTC().Format(DateLayout); got != "2024-12-20" {
		t.Errorf("sydney sunrise UTC date = %q, want 2024-12-20", got)
	}
	if got := e.Sunset.UTC().Format(DateLayout); got != "2024-12-21" {
		t.Errorf("sydney sunset UTC date = %q, want 2024-12-21", got)
	}
}

func TestCalculateEchoesQuery(t *testing.T) {
	d, _ := ParseDate("2024-06-21")
	e := Calculate(Query{Lat: 40.7128, Lon: -74.006, Date: d})
	if e.Date != "2024-06-21" || e.Lat != 40.7128 || e.Lon != -74.006 {
		t.Errorf("events echo mismatch: %+v", e)
	}
}

func assertClose(t *testing.T, name string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %v", name, want)
	}
	diff := got.Sub(want)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Minute {
		t.Errorf("%s = %v, want ~%v (diff %v)", name, *got, want, diff)
	}
}
