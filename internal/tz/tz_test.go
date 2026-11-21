package tz

import (
	"strings"
	"testing"
	"time"
)

// Resolve takes the environment value as a parameter, so these tests never
// mutate the process environment and pass regardless of the host's zone.

func TestFlagZoneWinsOverEnvZone(t *testing.T) {
	loc, err := Resolve("America/Los_Angeles", "Europe/Berlin")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if loc.String() != "America/Los_Angeles" {
		t.Errorf("flag must win over TZ: got %s", loc.String())
	}
}

func TestEnvZoneUsedWhenFlagEmpty(t *testing.T) {
	loc, err := Resolve("", "Europe/Berlin")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if loc.String() != "Europe/Berlin" {
		t.Errorf("TZ should be used when --tz is absent: got %s", loc.String())
	}
	// Berlin in January is on standard time, UTC+1.
	_, off := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if off != 3600 {
		t.Errorf("Europe/Berlin in winter = UTC+1, got offset %d seconds", off)
	}
}

func TestEmptyValuesCountAsUnsetAndDefaultToUTC(t *testing.T) {
	// An empty TZ counts as unset, not as an invalid name.
	loc, err := Resolve("", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if loc.String() != "UTC" {
		t.Errorf("want UTC when --tz and TZ are both unset/empty, got %s", loc.String())
	}
	_, off := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if off != 0 {
		t.Errorf("UTC offset = 0, got %d seconds", off)
	}
}

func TestInvalidFlagZoneFailsNamingTheZone(t *testing.T) {
	_, err := Resolve("Not/AZone", "")
	if err == nil {
		t.Fatal("invalid --tz must be an error, not a silent UTC fallback")
	}
	for _, want := range []string{"Not/AZone", "--tz"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err.Error(), want)
		}
	}
}

func TestInvalidEnvZoneFailsNamingTheZone(t *testing.T) {
	// Go itself silently maps a garbage TZ to UTC via time.Local; sol must
	// instead report it loudly.
	_, err := Resolve("", "Bogus/Zone")
	if err == nil {
		t.Fatal("invalid TZ must be an error, not a silent UTC fallback")
	}
	for _, want := range []string{"Bogus/Zone", "TZ"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err.Error(), want)
		}
	}
}

func TestPOSIXTZSpellingResolves(t *testing.T) {
	// Legacy POSIX spellings that exist as tzdata entries must work.
	loc, err := Resolve("EST5EDT", "")
	if err != nil {
		t.Fatalf("EST5EDT should resolve via the embedded tzdata: %v", err)
	}
	_, off := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC).In(loc).Zone()
	if off != -5*3600 {
		t.Errorf("EST5EDT in winter = UTC-5, got offset %d seconds", off)
	}
}
