package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
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
	for _, want := range []string{"America/Los_Angeles", "sunrise", "solar noon", "sunset", "-08:00"} {
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
			Name string  `json:"name"`
			Time *string `json:"time"`
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
	rfc3339 := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`)
	// Every event on 2026-03-08 falls after the 02:00 jump, so all carry
	// the PDT offset; the straddling-day behavior is covered in the render
	// package tests with synthetic instants.
	labels := map[string]bool{}
	for _, ev := range doc.Events {
		if ev.Time == nil {
			t.Fatalf("event %q has null time", ev.Name)
		}
		if !rfc3339.MatchString(*ev.Time) {
			t.Errorf("event %q time %q is not RFC 3339 with numeric offset", ev.Name, *ev.Time)
		}
		labels[(*ev.Time)[len(*ev.Time)-6:]] = true
	}
	if len(labels) != 1 || !labels["-07:00"] {
		t.Errorf("every event on 2026-03-08 must carry -07:00, got %v", labels)
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

func TestMissingCoordinatesIsAUsageError(t *testing.T) {
	t.Setenv("TZ", "")
	code, _, stderr := run(t, "-lat", "33.6")
	if code == 0 {
		t.Fatal("missing -lon must exit non-zero")
	}
	if !strings.Contains(stderr, "-lat") {
		t.Errorf("error should mention the required flags: %s", stderr)
	}
}

func TestInvalidDateIsRejected(t *testing.T) {
	t.Setenv("TZ", "")
	code, _, stderr := run(t, "-lat", "33.6", "-lon", "-117.8", "--date", "2026-13-40")
	if code == 0 {
		t.Fatal("invalid --date must exit non-zero")
	}
	if !strings.Contains(stderr, "2026-13-40") {
		t.Errorf("error must name the bad date: %s", stderr)
	}
}
