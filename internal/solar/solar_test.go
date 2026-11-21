package solar

import (
	"testing"
	"time"

	// Embedded tzdata so wall-clock checks resolve zones on any host.
	_ "time/tzdata"
)

// Reference values (timeanddate.com) for Los Angeles. The 2026-03-08
// spring-forward (02:00 PST -> 03:00 PDT) precedes every solar event that
// day, so all three events carry the PDT (-07:00) offset. Windows are
// generous to tolerate the sunrise equation's minute-level accuracy.
func TestLosAngelesOn2026SpringForwardDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load America/Los_Angeles: %v", err)
	}
	day := Schedule(2026, time.March, 8, 34.0522, -118.2437)

	if day.Sunrise == nil || day.Sunset == nil {
		t.Fatal("sunrise and sunset must exist in Los Angeles")
	}
	rise := day.Sunrise.In(loc)
	noon := day.SolarNoon.In(loc)
	set := day.Sunset.In(loc)

	for name, ts := range map[string]time.Time{"sunrise": rise, "solar noon": noon, "sunset": set} {
		if got := ts.Format("2006-01-02"); got != "2026-03-08" {
			t.Errorf("%s lands on %s, want 2026-03-08", name, got)
		}
		if _, off := ts.Zone(); off != -7*3600 {
			t.Errorf("%s offset = %ds, want -7h (PDT after the 02:00 jump)", name, off)
		}
	}

	assertWindow := func(name string, ts time.Time, lo, hi string) {
		t.Helper()
		clock := ts.Format("15:04")
		if clock < lo || clock > hi {
			t.Errorf("%s at %s, want a wall clock in [%s, %s]", name, clock, lo, hi)
		}
	}
	assertWindow("sunrise", rise, "06:45", "07:30")
	assertWindow("solar noon", noon, "12:45", "13:20")
	assertWindow("sunset", set, "18:40", "19:10")

	// Ordering: rise before noon before sunset as UTC instants.
	if !day.Sunrise.Before(day.SolarNoon) || !day.SolarNoon.Before(*day.Sunset) {
		t.Error("events out of order: want sunrise < solar noon < sunset")
	}
}

func TestLosAngelesMidwinterIsPST(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load America/Los_Angeles: %v", err)
	}
	// timeanddate.com: 2026-01-15 sunrise 06:59, solar noon 12:03, sunset 17:07.
	day := Schedule(2026, time.January, 15, 34.0522, -118.2437)
	rise, noon, set := day.Sunrise.In(loc), day.SolarNoon.In(loc), day.Sunset.In(loc)
	for name, ts := range map[string]time.Time{"sunrise": rise, "solar noon": noon, "sunset": set} {
		if _, off := ts.Zone(); off != -8*3600 {
			t.Errorf("%s offset = %ds, want -8h (PST)", name, off)
		}
	}
	assertWindow := func(name, got, lo, hi string) {
		t.Helper()
		if got < lo || got > hi {
			t.Errorf("%s at %s, want a wall clock in [%s, %s]", name, got, lo, hi)
		}
	}
	assertWindow("sunrise", rise.Format("15:04"), "06:45", "07:10")
	assertWindow("solar noon", noon.Format("15:04"), "11:50", "12:15")
	assertWindow("sunset", set.Format("15:04"), "16:55", "17:20")
}

func TestEquatorRoughlyTwelveHourDay(t *testing.T) {
	// Quito near the equinox: about 12 hours between sunrise and sunset.
	day := Schedule(2026, time.March, 20, -0.1807, -78.4678)
	if day.Sunrise == nil || day.Sunset == nil {
		t.Fatal("sunrise and sunset must exist near the equator")
	}
	d := day.Sunset.Sub(*day.Sunrise).Hours()
	if d < 11.9 || d > 12.2 {
		t.Errorf("day length = %.2fh, want ~12h", d)
	}
}

func TestPolarNightAndMidnightSun(t *testing.T) {
	// Tromsø in deepest winter: the sun never rises.
	night := Schedule(2026, time.December, 21, 69.6492, 18.9553)
	if night.Sunrise != nil || night.Sunset != nil {
		t.Errorf("polar night should have no sunrise/sunset, got %v / %v", night.Sunrise, night.Sunset)
	}
	// Tromsø at midsummer: the sun never sets.
	midnight := Schedule(2026, time.June, 21, 69.6492, 18.9553)
	if midnight.Sunrise != nil || midnight.Sunset != nil {
		t.Errorf("midnight sun should have no sunrise/sunset, got %v / %v", midnight.Sunrise, midnight.Sunset)
	}
	// Solar noon exists on both polar days.
	if night.SolarNoon.IsZero() || midnight.SolarNoon.IsZero() {
		t.Error("solar noon must always exist")
	}
}

func TestTimesAreRoundedToTheMinute(t *testing.T) {
	day := Schedule(2026, time.July, 4, 40.7128, -74.0060)
	if day.Sunrise.Second() != 0 || day.SolarNoon.Second() != 0 || day.Sunset.Second() != 0 {
		t.Error("event times should be rounded to whole minutes")
	}
}
