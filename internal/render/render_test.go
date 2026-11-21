package render

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/zaephor-bot/sol-cli/internal/solar"

	// Embedded tzdata so this test package resolves IANA zones even on
	// hosts without /usr/share/zoneinfo.
	_ "time/tzdata"
)

func losAngeles(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load America/Los_Angeles: %v", err)
	}
	return loc
}

// The required daylight-saving test: 2026-03-08 is the US spring-forward
// (02:00 PST -> 03:00 PDT, i.e. 09:00 -> 10:00 UTC). Instants on either
// side of the transition must carry different offset labels.
func TestOffsetLabelMovesAcross2026SpringForward(t *testing.T) {
	loc := losAngeles(t)

	before := time.Date(2026, 3, 8, 9, 30, 0, 0, time.UTC) // 01:30 PST
	after := time.Date(2026, 3, 8, 10, 30, 0, 0, time.UTC) // 03:30 PDT

	if got := Stamp(before, loc); !strings.HasSuffix(got, "-08:00") {
		t.Errorf("before the transition: want a -08:00 label, got %q", got)
	}
	if got := Stamp(after, loc); !strings.HasSuffix(got, "-07:00") {
		t.Errorf("after the transition: want a -07:00 label, got %q", got)
	}
}

func TestUTCRendersAsPlusZeroZeroZeroZero(t *testing.T) {
	got := Stamp(time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC), time.UTC)
	want := "2026-01-15 08:00 +00:00"
	if got != want {
		t.Errorf("UTC stamp = %q, want %q", got, want)
	}
}

// A schedule day that straddles the 02:00 -> 03:00 jump must show both
// offsets, because each event is stamped as its own instant in the zone.
// (Real mid-latitude solar events never straddle a 2 AM transition, so
// this uses synthetic instants around the boundary.)
func TestEachEventStampedAsItsOwnInstantAcrossTransition(t *testing.T) {
	loc := losAngeles(t)
	rise := time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC) // 00:30 PST
	noon := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC) // 07:00 PST
	set := time.Date(2026, 3, 8, 19, 0, 0, 0, time.UTC)  // 12:00 PDT
	day := solar.Day{Sunrise: &rise, SolarNoon: noon, Sunset: &set}

	var buf bytes.Buffer
	Text(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day)
	out := buf.String()

	if !strings.Contains(out, "America/Los_Angeles") {
		t.Errorf("output does not name the zone:\n%s", out)
	}
	if !strings.Contains(out, "-08:00") || !strings.Contains(out, "-07:00") {
		t.Errorf("a day spanning the transition must carry both -08:00 and -07:00 labels:\n%s", out)
	}
	for _, want := range []string{"sunrise", "solar noon", "sunset"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not list %q:\n%s", want, out)
		}
	}
}

func TestTextOfRealScheduleCarriesOffsetOnEveryEvent(t *testing.T) {
	loc := losAngeles(t)
	day := solar.Schedule(2026, time.March, 8, 34.0522, -118.2437)

	var buf bytes.Buffer
	Text(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	// Every event line (not the header) must end with a numeric offset.
	for _, line := range lines[2:] {
		if !strings.HasSuffix(line, "-07:00") {
			t.Errorf("event line %q lacks its UTC offset", line)
		}
	}
}

func TestJSONTimestampsCarryResolvedOffset(t *testing.T) {
	loc := losAngeles(t)
	// A synthetic day straddling the 2026-03-08 spring-forward, so the
	// JSON must show both offset labels just like the text output.
	rise := time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC) // 00:30 PST
	set := time.Date(2026, 3, 8, 19, 0, 0, 0, time.UTC)  // 12:00 PDT
	day := solar.Day{Sunrise: &rise, SolarNoon: time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC), Sunset: &set}

	var buf bytes.Buffer
	if err := JSON(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc struct {
		Timezone string `json:"timezone"`
		Events   []struct {
			Name string  `json:"name"`
			Time *string `json:"time"`
		} `json:"events"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, buf.String())
	}
	if doc.Timezone != "America/Los_Angeles" {
		t.Errorf("timezone = %q", doc.Timezone)
	}
	if len(doc.Events) != 3 {
		t.Fatalf("want 3 events, got %d", len(doc.Events))
	}

	// RFC 3339 with a numeric offset — never the "Z" form.
	rfc3339Offset := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:[+-]\d{2}:\d{2})$`)
	offsets := map[string]bool{}
	for _, ev := range doc.Events {
		if ev.Time == nil {
			t.Fatalf("event %q has null time", ev.Name)
		}
		if !rfc3339Offset.MatchString(*ev.Time) {
			t.Errorf("event %q time %q lacks a numeric UTC offset", ev.Name, *ev.Time)
		}
		if !strings.HasPrefix(*ev.Time, "2026-03-08T") {
			t.Errorf("event %q time %q is not on 2026-03-08", ev.Name, *ev.Time)
		}
		if suffix := (*ev.Time)[len(*ev.Time)-6:]; suffix != "+00:00" {
			offsets[suffix] = true
		}
	}
	// The schedule spans the spring-forward, so both labels appear.
	if !offsets["-08:00"] || !offsets["-07:00"] {
		t.Errorf("want both -08:00 and -07:00 labels in JSON events, got %v", offsets)
	}
}

func TestPolarDayHasNoRiseOrSet(t *testing.T) {
	// Tromsø on the summer solstice: the sun never sets, so no rise/set.
	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		t.Fatalf("load Europe/Oslo: %v", err)
	}
	day := solar.Schedule(2026, time.June, 21, 69.6492, 18.9553)

	var buf bytes.Buffer
	Text(&buf, loc, "Europe/Oslo", "2026-06-21", 69.6492, 18.9553, day)
	out := buf.String()
	if !strings.Contains(out, "none") {
		t.Errorf("polar day should report missing events:\n%s", out)
	}
	if !strings.Contains(out, "solar noon") {
		t.Errorf("solar noon must still print:\n%s", out)
	}
}
