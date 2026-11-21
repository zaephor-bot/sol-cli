package schedule

import (
	"testing"
	"time"

	// Embedded tzdata so this test package resolves IANA zones even on
	// hosts without /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/zaephor-bot/sol-lib/solar"
	"github.com/zaephor-bot/sol-lib/windows"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return loc
}

// The schedule must reproduce the library's own answers verbatim: every
// field is the library's answer for the same local day, never a second
// computation.
func TestForReturnsTheLibrarysAnswersVerbatim(t *testing.T) {
	loc := zone(t, "America/Los_Angeles")
	const lat, lon = 33.6, -117.8

	day := For(2026, time.June, 21, loc, lat, lon)
	local := time.Date(2026, time.June, 21, 0, 0, 0, 0, loc)

	if day.Polar != "" {
		t.Errorf("Polar = %q, want empty for a normal day", day.Polar)
	}
	if day.Sunrise == nil || day.Sunset == nil {
		t.Fatal("sunrise and sunset must exist for Irvine in June")
	}

	wantRise, err := solar.Sunrise(local, lat, lon)
	if err != nil {
		t.Fatalf("library sunrise: %v", err)
	}
	if !day.Sunrise.Equal(wantRise) {
		t.Errorf("sunrise = %v, want the library's %v", *day.Sunrise, wantRise)
	}

	wantNoon, _ := solar.SolarNoon(local, lat, lon)
	if !day.SolarNoon.Equal(wantNoon) {
		t.Errorf("solar noon = %v, want the library's %v", day.SolarNoon, wantNoon)
	}

	wantSet, err := solar.Sunset(local, lat, lon)
	if err != nil {
		t.Fatalf("library sunset: %v", err)
	}
	if !day.Sunset.Equal(wantSet) {
		t.Errorf("sunset = %v, want the library's %v", *day.Sunset, wantSet)
	}

	wantDawnGolden, wantDuskGolden := windows.GoldenHour(local, lat, lon)
	if day.DawnGoldenHour != wantDawnGolden || day.DuskGoldenHour != wantDuskGolden {
		t.Errorf("golden hours = %+v, %+v; want the library's %+v, %+v",
			day.DawnGoldenHour, day.DuskGoldenHour, wantDawnGolden, wantDuskGolden)
	}

	wantDawnBlue, wantDuskBlue := windows.BlueHour(local, lat, lon)
	if day.DawnBlueHour != wantDawnBlue || day.DuskBlueHour != wantDuskBlue {
		t.Errorf("blue hours = %+v, %+v; want the library's %+v, %+v",
			day.DawnBlueHour, day.DuskBlueHour, wantDawnBlue, wantDuskBlue)
	}
}

// Published almanac times for Irvine, CA (33.6, -117.8) on the 2026
// June solstice: sunrise 05:41, solar noon 12:53, sunset 20:05 PDT.
// The library's NOAA equations must land within a minute of each.
func TestForIrvineJuneSolsticeMatchesPublishedAlmanac(t *testing.T) {
	loc := zone(t, "America/Los_Angeles")
	day := For(2026, time.June, 21, loc, 33.6, -117.8)

	withinMinute := func(name string, got time.Time, hour, min int) {
		t.Helper()
		want := time.Date(2026, time.June, 21, hour, min, 0, 0, loc)
		diff := got.Sub(want)
		if diff < 0 {
			diff = -diff
		}
		if diff > time.Minute {
			t.Errorf("%s = %s, want within a minute of %s", name, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	}
	withinMinute("sunrise", *day.Sunrise, 5, 41)
	withinMinute("solar noon", day.SolarNoon, 12, 53)
	withinMinute("sunset", *day.Sunset, 20, 5)

	for _, win := range []windows.Window{day.DawnGoldenHour, day.DuskGoldenHour, day.DawnBlueHour, day.DuskBlueHour} {
		if win.Absent {
			t.Errorf("%s must happen in Irvine in June, got %+v", win.Event, win)
		}
		if !win.End.After(win.Start) {
			t.Errorf("%s window %v..%v must end after it starts", win.Event, win.Start, win.End)
		}
	}
}

// Tromsø on the summer solstice: the sun never sets, so there is no
// sunrise or sunset and every golden or blue window is absent, while
// solar noon still happens.
func TestForPolarDayHasNoRiseSetOrWindows(t *testing.T) {
	loc := zone(t, "Europe/Oslo")
	day := For(2026, time.June, 21, loc, 69.6492, 18.9553)

	if day.Sunrise != nil {
		t.Errorf("sunrise = %v, want none under the midnight sun", *day.Sunrise)
	}
	if day.Sunset != nil {
		t.Errorf("sunset = %v, want none under the midnight sun", *day.Sunset)
	}
	if day.Polar != "midnight sun" {
		t.Errorf("Polar = %q, want %q", day.Polar, "midnight sun")
	}
	if day.SolarNoon.IsZero() {
		t.Error("solar noon must still exist on the polar day")
	}
	for _, win := range []windows.Window{day.DawnGoldenHour, day.DuskGoldenHour, day.DawnBlueHour, day.DuskBlueHour} {
		if !win.Absent {
			t.Errorf("%s must be absent on the polar day, got %v..%v", win.Event, win.Start, win.End)
		}
	}
}

// Tromsø on the winter solstice: the sun never rises, but it climbs
// to within a few degrees of the horizon — above the −4° edge of blue
// hour — so the golden windows are absent while both blue hours still
// happen around solar noon.
func TestForPolarNightHasNoRiseSetAndAbsentGoldenHours(t *testing.T) {
	loc := zone(t, "Europe/Oslo")
	day := For(2026, time.December, 21, loc, 69.6492, 18.9553)

	if day.Sunrise != nil || day.Sunset != nil {
		t.Errorf("rise/set = %v, %v; want none during the polar night", day.Sunrise, day.Sunset)
	}
	if day.Polar != "polar night" {
		t.Errorf("Polar = %q, want %q", day.Polar, "polar night")
	}
	if day.SolarNoon.IsZero() {
		t.Error("solar noon must still exist during the polar night")
	}
	for _, win := range []windows.Window{day.DawnGoldenHour, day.DuskGoldenHour} {
		if !win.Absent {
			t.Errorf("%s must be absent during the polar night, got %v..%v", win.Event, win.Start, win.End)
		}
	}
	for _, win := range []windows.Window{day.DawnBlueHour, day.DuskBlueHour} {
		if win.Absent {
			t.Errorf("%s must still happen: the winter sun clears the −4° edge, got %+v", win.Event, win)
			continue
		}
		if win.End.Before(win.Start) {
			t.Errorf("%s window %v..%v must end after it starts", win.Event, win.Start, win.End)
		}
	}
}
