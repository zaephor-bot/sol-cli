package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Every test that touches TZ uses t.Setenv, which saves the previous value
// and restores it on cleanup, so tests cannot poison each other or depend
// on the host's zone.

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestScheduleInRequestedZoneWithOffsets(t *testing.T) {
	t.Setenv("TZ", "") // pin the environment: this test is about --tz, not $TZ
	// 2026-01-15: deep winter, every event on Pacific standard time.
	code, out, stderr := run(t,
		"-lat", "33.6", "-lon", "-117.8",
		"--tz", "America/Los_Angeles",
		"--date", "2026-01-15",
	)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("unexpected stderr: %s", stderr)
	}
	for _, want := range []string{
		"America/Los_Angeles", "2026-01-15",
		"sunrise", "solar noon", "sunset",
		"golden hour", "blue hour",
		"-08:00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[2:] {
		if !strings.HasSuffix(line, "-08:00") {
			t.Errorf("event line %q does not carry the -08:00 offset", line)
		}
	}
}

func TestScheduleOn2026SpringForwardDayPrintsPDT(t *testing.T) {
	t.Setenv("TZ", "")
	// The 02:00 PST -> 03:00 PDT jump precedes every solar event on
	// 2026-03-08, so the whole schedule prints with the -07:00 label.
	code, out, stderr := run(t,
		"-lat", "33.6", "-lon", "-117.8",
		"--tz", "America/Los_Angeles",
		"--date", "2026-03-08",
	)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[2:] {
		if !strings.HasSuffix(line, "-07:00") {
			t.Errorf("event line %q does not carry the -07:00 offset", line)
		}
	}
}

func TestTZEnvUsedWhenFlagAbsent(t *testing.T) {
	t.Setenv("TZ", "Europe/Berlin")
	code, out, stderr := run(t, "-lat", "52.52", "-lon", "13.405", "--date", "2026-01-15")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "Europe/Berlin") {
		t.Errorf("expected Berlin zone:\n%s", out)
	}
	if !strings.Contains(out, "+01:00") {
		t.Errorf("expected Berlin standard-time offset +01:00:\n%s", out)
	}
}

func TestEmptyTZAndNoFlagPrintUTC(t *testing.T) {
	// TZ="" counts as unset, so the schedule falls back to UTC.
	t.Setenv("TZ", "")
	code, out, stderr := run(t, "-lat", "33.6", "-lon", "-117.8", "--date", "2026-01-15")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "(UTC)") {
		t.Errorf("expected UTC zone name:\n%s", out)
	}
	if !strings.Contains(out, "+00:00") {
		t.Errorf("expected UTC labeled +00:00:\n%s", out)
	}
	if strings.Contains(out, "+01:00") || strings.Contains(out, "-08:00") {
		t.Errorf("non-UTC offsets leaked into UTC output:\n%s", out)
	}
}

func TestBadZoneFlagExitsNonzeroWithoutSchedule(t *testing.T) {
	t.Setenv("TZ", "Europe/Berlin") // a valid env value must not mask the bad flag
	code, out, stderr := run(t, "-lat", "33.6", "-lon", "-117.8", "--tz", "Not/AZone")
	if code == 0 {
		t.Fatal("invalid --tz must exit non-zero")
	}
	if !strings.Contains(stderr, "Not/AZone") {
		t.Errorf("error must name the bad zone, got: %s", stderr)
	}
	if strings.Contains(out, "sunrise") || strings.Contains(out, "sunset") {
		t.Errorf("no schedule may print on error, got:\n%s", out)
	}
}

func TestBadTZEnvFailsLoudlyNotSilentlyAsUTC(t *testing.T) {
	t.Setenv("TZ", "Not/AZone")
	code, out, stderr := run(t, "-lat", "33.6", "-lon", "-117.8")
	if code == 0 {
		t.Fatal("invalid TZ must exit non-zero")
	}
	if !strings.Contains(stderr, "Not/AZone") {
		t.Errorf("error must name the bad zone, got: %s", stderr)
	}
	if !strings.Contains(stderr, "TZ") {
		t.Errorf("error must say the value came from TZ, got: %s", stderr)
	}
	if strings.Contains(out, "sunrise") {
		t.Errorf("no schedule may print on error, got:\n%s", out)
	}
}

func TestJSONTimestampsCarryOffset(t *testing.T) {
	t.Setenv("TZ", "")
	code, out, stderr := run(t,
		"-lat", "33.6", "-lon", "-117.8",
		"--tz", "America/Los_Angeles",
		"--date", "2026-03-08",
		"--json",
	)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}

	var doc struct {
		Date     string `json:"date"`
		Timezone string `json:"timezone"`
		Events   []struct {
			Name  string  `json:"name"`
			Time  *string `json:"time"`
			Start *string `json:"start"`
			End   *string `json:"end"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc.Timezone != "America/Los_Angeles" {
		t.Errorf("timezone = %q, want America/Los_Angeles", doc.Timezone)
	}
	if doc.Date != "2026-03-08" {
		t.Errorf("date = %q", doc.Date)
	}

	wantNames := []string{
		"sunrise", "solar_noon", "sunset",
		"dawn_golden_hour", "dusk_golden_hour",
		"dawn_blue_hour", "dusk_blue_hour",
	}
	if len(doc.Events) != len(wantNames) {
		t.Fatalf("want %d events, got %d: %+v", len(wantNames), len(doc.Events), doc.Events)
	}
	rfc3339 := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`)
	// Every event on 2026-03-08 falls after the 02:00 jump, so all carry
	// the PDT offset; the straddling-day behavior is covered in the
	// render package tests with synthetic instants.
	labels := map[string]bool{}
	check := func(name, s string) {
		t.Helper()
		if !rfc3339.MatchString(s) {
			t.Errorf("event %q timestamp %q is not RFC 3339 with numeric offset", name, s)
		}
		labels[s[len(s)-6:]] = true
	}
	for i, ev := range doc.Events {
		if ev.Name != wantNames[i] {
			t.Errorf("event %d is %q, want %q", i, ev.Name, wantNames[i])
		}
		switch ev.Name {
		case "sunrise", "solar_noon", "sunset":
			if ev.Time == nil {
				t.Fatalf("event %q has null time", ev.Name)
			}
			check(ev.Name, *ev.Time)
		default:
			if ev.Start == nil || ev.End == nil {
				t.Fatalf("window %q has a null end", ev.Name)
			}
			check(ev.Name, *ev.Start)
			check(ev.Name, *ev.End)
		}
	}
	if len(labels) != 1 || !labels["-07:00"] {
		t.Errorf("every event on 2026-03-08 must carry -07:00, got %v", labels)
	}
}

// The JSON run and the text run must agree on every instant they both
// report, to the minute (the text stamp drops seconds).
func TestJSONAndTextRunsCarryTheSameTimes(t *testing.T) {
	t.Setenv("TZ", "")
	args := []string{"-lat", "33.6", "-lon", "-117.8", "--tz", "America/Los_Angeles", "--date", "2026-06-21"}
	_, textOut, _ := run(t, args...)
	code, jsonOut, stderr := run(t, append(append([]string{}, args...), "--json")...)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}

	var doc struct {
		Events []struct {
			Name  string  `json:"name"`
			Time  *string `json:"time"`
			Start *string `json:"start"`
			End   *string `json:"end"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonOut)
	}

	// Text rows: "name  stamp" or "name  stamp to stamp".
	rowStart := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2} `)
	textRows := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(textOut), "\n")[2:] {
		at := rowStart.FindStringIndex(line)
		if at == nil {
			continue // a "none" row carries no time to compare
		}
		name := strings.TrimSpace(line[:at[0]])
		textRows[name] = strings.Split(strings.TrimSpace(line[at[0]:]), " to ")
	}

	equalToMinute := func(json, text string) bool {
		jt, err := time.Parse(time.RFC3339, json)
		if err != nil {
			t.Fatalf("JSON timestamp %q: %v", json, err)
		}
		tt, err := time.Parse("2006-01-02 15:04 -07:00", text)
		if err != nil {
			t.Fatalf("text stamp %q: %v", text, err)
		}
		return jt.Truncate(time.Minute).Equal(tt.Truncate(time.Minute))
	}

	for _, ev := range doc.Events {
		name := strings.ReplaceAll(ev.Name, "_", " ")
		var jsonStamps []string
		if ev.Time != nil {
			jsonStamps = append(jsonStamps, *ev.Time)
		}
		if ev.Start != nil {
			jsonStamps = append(jsonStamps, *ev.Start)
		}
		if ev.End != nil {
			jsonStamps = append(jsonStamps, *ev.End)
		}
		textStamps, ok := textRows[name]
		if !ok {
			t.Errorf("event %q appears in JSON but not in text:\n%s", ev.Name, textOut)
			continue
		}
		if len(jsonStamps) != len(textStamps) {
			t.Errorf("event %q carries %d JSON times but %d text times", ev.Name, len(jsonStamps), len(textStamps))
			continue
		}
		for i, js := range jsonStamps {
			if !equalToMinute(js, textStamps[i]) {
				t.Errorf("event %q time %d: JSON %s disagrees with text %s", ev.Name, i, js, textStamps[i])
			}
		}
	}
}

func TestDefaultDateIsTodayInResolvedZone(t *testing.T) {
	t.Setenv("TZ", "")
	code, out, stderr := run(t, "-lat", "33.6", "-lon", "-117.8")
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "(UTC)") || !strings.Contains(out, "+00:00") {
		t.Errorf("expected a UTC schedule for today:\n%s", out)
	}
}

func TestMissingFlagsAreNamed(t *testing.T) {
	t.Setenv("TZ", "")
	t.Run("missing lon", func(t *testing.T) {
		code, _, stderr := run(t, "-lat", "33.6")
		if code == 0 {
			t.Fatal("missing -lon must exit non-zero")
		}
		if !strings.Contains(stderr, "-lon is required") {
			t.Errorf("error must name the missing flag -lon: %s", stderr)
		}
	})
	t.Run("missing lat", func(t *testing.T) {
		code, _, stderr := run(t, "-lon", "-117.8")
		if code == 0 {
			t.Fatal("missing -lat must exit non-zero")
		}
		if !strings.Contains(stderr, "-lat is required") {
			t.Errorf("error must name the missing flag -lat: %s", stderr)
		}
	})
	t.Run("missing both", func(t *testing.T) {
		code, _, stderr := run(t)
		if code == 0 {
			t.Fatal("missing both flags must exit non-zero")
		}
		for _, want := range []string{"-lat", "-lon"} {
			if !strings.Contains(stderr, want) {
				t.Errorf("error must name %q: %s", want, stderr)
			}
		}
	})
}

func TestOutOfRangeCoordinatesAreNamed(t *testing.T) {
	t.Setenv("TZ", "")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-lat", "91", "-lon", "-117.8"}, "-lat 91"},
		{[]string{"-lat", "-91", "-lon", "-117.8"}, "-lat -91"},
		{[]string{"-lat", "33.6", "-lon", "181"}, "-lon 181"},
		{[]string{"-lat", "33.6", "-lon", "-181"}, "-lon -181"},
	}
	for _, tc := range cases {
		code, _, stderr := run(t, tc.args...)
		if code == 0 {
			t.Fatalf("%v must exit non-zero", tc.args)
		}
		if !strings.Contains(stderr, tc.want) {
			t.Errorf("error must name the offending value %q: %s", tc.want, stderr)
		}
	}
}

func TestInvalidDateIsRejected(t *testing.T) {
	t.Setenv("TZ", "")
	for _, bad := range []string{"2026-13-40", "2026-02-30"} {
		code, _, stderr := run(t, "-lat", "33.6", "-lon", "-117.8", "--date", bad)
		if code == 0 {
			t.Fatalf("invalid --date %s must exit non-zero", bad)
		}
		if !strings.Contains(stderr, bad) {
			t.Errorf("error must name the bad date %s: %s", bad, stderr)
		}
		if !strings.Contains(stderr, "-date") {
			t.Errorf("error must name the -date flag: %s", stderr)
		}
	}
}

// Tromsø on the summer solstice: the sun never sets, so the rise and
// set rows explain themselves and solar noon still prints.
func TestPolarDayPrintsNoneRows(t *testing.T) {
	t.Setenv("TZ", "")
	code, out, stderr := run(t,
		"-lat", "69.6492", "-lon", "18.9553",
		"--tz", "Europe/Oslo", "--date", "2026-06-21",
	)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(out, "solar noon") {
		t.Errorf("solar noon must still print:\n%s", out)
	}
	if n := strings.Count(out, "none"); n < 2 {
		t.Errorf("missing rise/set and window events must print none rows:\n%s", out)
	}
	if !strings.Contains(out, "midnight sun") {
		t.Errorf("the polar reason must be named:\n%s", out)
	}
}
