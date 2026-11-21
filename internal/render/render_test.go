package render

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	// Embedded tzdata so this test package resolves IANA zones even on
	// hosts without /usr/share/zoneinfo.
	_ "time/tzdata"

	"github.com/zaephor-bot/sol-lib/windows"

	"github.com/zaephor-bot/sol-cli/internal/schedule"
)

func losAngeles(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load America/Los_Angeles: %v", err)
	}
	return loc
}

// A synthetic schedule straddling the 2026-03-08 US spring-forward
// (02:00 PST -> 03:00 PDT, i.e. 09:00 -> 10:00 UTC): rise in PST, the
// rest in PDT, and a dawn blue hour whose two ends straddle the jump.
func straddlingDay() schedule.Day {
	rise := time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC)           // 00:30 PST
	set := time.Date(2026, 3, 9, 2, 30, 0, 0, time.UTC)            // 19:30 PDT
	noon := time.Date(2026, 3, 8, 15, 0, 0, 0, time.UTC)           // 08:00 PDT
	blueStart := time.Date(2026, 3, 8, 9, 30, 0, 0, time.UTC)      // 01:30 PST
	blueEnd := time.Date(2026, 3, 8, 13, 30, 0, 0, time.UTC)       // 06:30 PDT
	goldenEnd := time.Date(2026, 3, 8, 14, 0, 0, 0, time.UTC)      // 07:00 PDT
	duskGoldenStart := time.Date(2026, 3, 9, 1, 0, 0, 0, time.UTC) // 18:00 PDT
	duskGoldenEnd := time.Date(2026, 3, 9, 2, 0, 0, 0, time.UTC)   // 19:00 PDT
	duskBlueStart := time.Date(2026, 3, 9, 2, 30, 0, 0, time.UTC)  // 19:30 PDT
	duskBlueEnd := time.Date(2026, 3, 9, 3, 30, 0, 0, time.UTC)    // 20:30 PDT
	return schedule.Day{
		Sunrise:        &rise,
		Sunset:         &set,
		SolarNoon:      noon,
		DawnBlueHour:   windows.Window{Event: windows.DawnBlueHour, Start: blueStart, End: blueEnd},
		DawnGoldenHour: windows.Window{Event: windows.DawnGoldenHour, Start: rise, End: goldenEnd},
		DuskGoldenHour: windows.Window{Event: windows.DuskGoldenHour, Start: duskGoldenStart, End: duskGoldenEnd},
		DuskBlueHour:   windows.Window{Event: windows.DuskBlueHour, Start: duskBlueStart, End: duskBlueEnd},
	}
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
// offsets, because each event — and each end of a window — is stamped as
// its own instant in the zone.
func TestEachEventStampedAsItsOwnInstantAcrossTransition(t *testing.T) {
	loc := losAngeles(t)
	day := straddlingDay()

	var buf bytes.Buffer
	Text(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day)
	out := buf.String()

	if !strings.Contains(out, "America/Los_Angeles") {
		t.Errorf("output does not name the zone:\n%s", out)
	}
	if !strings.Contains(out, "-08:00") || !strings.Contains(out, "-07:00") {
		t.Errorf("a day spanning the transition must carry both -08:00 and -07:00 labels:\n%s", out)
	}
	for _, want := range []string{"sunrise", "solar noon", "sunset",
		"dawn golden hour", "dawn blue hour"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not list %q:\n%s", want, out)
		}
	}
}

// A window row shows both ends, each with its own offset label; an
// absent window shows a "none" row that names it.
func TestWindowRowsShowBothEndsAndAbsentWindowsAreNamed(t *testing.T) {
	loc := losAngeles(t)
	day := straddlingDay()
	day.DuskGoldenHour = windows.Window{Event: windows.DuskGoldenHour, Absent: true}

	var buf bytes.Buffer
	Text(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day)
	out := buf.String()

	if !strings.Contains(out, "none (no dusk golden hour this day)") {
		t.Errorf("absent window must print a none row naming it:\n%s", out)
	}
	// The dawn blue hour deliberately straddles the jump: 01:30 PST to
	// 06:30 PDT, one label per end.
	if !strings.Contains(out, "2026-03-08 01:30 -08:00 to 2026-03-08 06:30 -07:00") {
		t.Errorf("window row must stamp each end with its own offset:\n%s", out)
	}
}

// A polar day has no rise or set and no windows; each missing event is a
// "none" row naming the reason.
func TestPolarDayRows(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		t.Fatalf("load Europe/Oslo: %v", err)
	}
	day := schedule.Day{
		Polar:          "midnight sun",
		SolarNoon:      time.Date(2026, 6, 21, 12, 0, 0, 0, loc),
		DawnGoldenHour: windows.Window{Event: windows.DawnGoldenHour, Absent: true},
	}

	var buf bytes.Buffer
	Text(&buf, loc, "Europe/Oslo", "2026-06-21", 69.6492, 18.9553, day)
	out := buf.String()

	for _, want := range []string{
		"none (midnight sun)",
		"none (no dawn golden hour this day)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("polar day output missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "none (midnight sun)") != 2 {
		t.Errorf("both sunrise and sunset must show the polar reason:\n%s", out)
	}
	if !strings.Contains(out, "solar noon") {
		t.Errorf("solar noon must still print:\n%s", out)
	}
}

func TestJSONTimestampsCarryResolvedOffset(t *testing.T) {
	loc := losAngeles(t)
	day := straddlingDay()

	var buf bytes.Buffer
	if err := JSON(&buf, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc jsonDoc
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, buf.String())
	}
	if doc.Timezone != "America/Los_Angeles" {
		t.Errorf("timezone = %q", doc.Timezone)
	}
	if doc.Date != "2026-03-08" {
		t.Errorf("date = %q", doc.Date)
	}
	if len(doc.Events) != 7 {
		t.Fatalf("want 7 events, got %d: %+v", len(doc.Events), doc.Events)
	}

	// RFC 3339 with a numeric offset — never the "Z" form.
	rfc3339Offset := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:[+-]\d{2}:\d{2})$`)
	offsets := map[string]bool{}
	stamps := func(ss ...*string) {
		t.Helper()
		for _, s := range ss {
			if s == nil {
				t.Fatal("window ends must not be null on a normal day")
			}
			if !rfc3339Offset.MatchString(*s) {
				t.Errorf("timestamp %q lacks a numeric UTC offset", *s)
			}
			offsets[(*s)[len(*s)-6:]] = true
		}
	}
	for _, ev := range doc.Events {
		switch ev.Name {
		case "sunrise", "solar_noon", "sunset":
			if ev.Time == nil {
				t.Fatalf("event %q has null time", ev.Name)
			}
			stamps(ev.Time)
		case "dawn_golden_hour", "dusk_golden_hour", "dawn_blue_hour", "dusk_blue_hour":
			stamps(ev.Start, ev.End)
		default:
			t.Fatalf("unexpected event %q", ev.Name)
		}
	}
	// The schedule spans the spring-forward, so both labels appear.
	if !offsets["-08:00"] || !offsets["-07:00"] {
		t.Errorf("want both -08:00 and -07:00 labels in JSON events, got %v", offsets)
	}
}

// The JSON form carries the same events in the same order as the text
// form, with the same instants (the text stamp drops seconds, so
// instants are compared to the minute).
func TestJSONAndTextCarryTheSameSchedule(t *testing.T) {
	loc := losAngeles(t)
	day := straddlingDay()

	var text, js bytes.Buffer
	Text(&text, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day)
	if err := JSON(&js, loc, "America/Los_Angeles", "2026-03-08", 34.0522, -118.2437, day); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc jsonDoc
	if err := json.Unmarshal(js.Bytes(), &doc); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, js.String())
	}

	// Parse the text form: each event row is "name  stamp" for a point
	// event or "name  stamp to stamp" for a window, and every stamp is
	// laid out "2006-01-02 15:04 -07:00".
	rowStart := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2} `)
	stampLayout := "2006-01-02 15:04 -07:00"
	textRows := map[string][]time.Time{}
	var textOrder []string
	for _, line := range strings.Split(strings.TrimSpace(text.String()), "\n")[2:] {
		at := rowStart.FindStringIndex(line)
		if at == nil {
			continue // a "none" row carries no time to compare
		}
		name := strings.TrimSpace(line[:at[0]])
		var instants []time.Time
		for _, s := range strings.Split(strings.TrimSpace(line[at[0]:]), " to ") {
			parsed, err := time.Parse(stampLayout, s)
			if err != nil {
				t.Fatalf("row %q: %v", line, err)
			}
			instants = append(instants, parsed)
		}
		textRows[name] = instants
		textOrder = append(textOrder, name)
	}

	toEqualMinute := func(a, b time.Time) bool {
		return a.Truncate(time.Minute).Equal(b.Truncate(time.Minute))
	}
	if len(doc.Events) != len(textOrder) {
		t.Fatalf("JSON has %d events, text has %d", len(doc.Events), len(textOrder))
	}
	for i, ev := range doc.Events {
		name := strings.ReplaceAll(ev.Name, "_", " ")
		if name != textOrder[i] {
			t.Errorf("event %d is %q in JSON but %q in text", i, ev.Name, textOrder[i])
			continue
		}
		var jsonInstants []time.Time
		for _, s := range []*string{ev.Time, ev.Start, ev.End} {
			if s == nil {
				continue
			}
			parsed, err := time.Parse(time.RFC3339, *s)
			if err != nil {
				t.Fatalf("JSON timestamp %q: %v", *s, err)
			}
			jsonInstants = append(jsonInstants, parsed)
		}
		if len(jsonInstants) != len(textRows[name]) {
			t.Errorf("event %q carries %d times in JSON but %d in text", name, len(jsonInstants), len(textRows[name]))
			continue
		}
		for j, jt := range jsonInstants {
			if !toEqualMinute(jt, textRows[name][j]) {
				t.Errorf("event %q time %d: JSON %s disagrees with text %s", name, j, jt, textRows[name][j])
			}
		}
	}
}
