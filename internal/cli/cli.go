// Package cli implements the sol command line: flag parsing, zone
// resolution, and output formatting for the day's solar schedule. Every
// event time is computed by github.com/zaephor-bot/sol-lib via
// internal/schedule; this package holds no solar math.
package cli

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/zaephor-bot/sol-cli/internal/render"
	"github.com/zaephor-bot/sol-cli/internal/schedule"
	"github.com/zaephor-bot/sol-cli/internal/tz"
)

const usage = `sol prints the day's solar schedule for a location: sunrise, solar
noon, sunset, golden hour, and blue hour, with golden hour and blue hour
each appearing as a dawn and a dusk window.

Usage:

  sol -lat 33.6 -lon -117.8 [-date 2026-03-08] [--tz America/Los_Angeles] [--json]

Flags:

  -lat     latitude in decimal degrees, north positive (required)
  -lon     longitude in decimal degrees, east positive (required)
  -date    calendar date, YYYY-MM-DD, a real date (default: today in the
           resolved zone)
  -tz      IANA time zone name, e.g. America/Los_Angeles
           (default: $TZ, then UTC; an empty value counts as unset)
  -json    emit JSON; every timestamp carries its UTC offset

Every event time comes from the shared library github.com/zaephor-bot/
sol-lib. Times are printed in the resolved zone and every time carries
its numeric UTC offset, so a day spanning a daylight-saving transition
shows both offsets. An event that never happens on the requested day (a
polar day or polar night) prints as a "none" row naming it. An
unresolvable zone name in -tz or $TZ is an error, not a silent fallback
to UTC.
`

// Exit codes: 2 for usage errors, 1 for a value that fails to resolve.
const (
	exitError = 1
	exitUsage = 2
)

// Run parses args, resolves the zone, and writes the schedule. It returns
// the process exit code and writes errors to stderr.
func Run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sol", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	lat := fs.Float64("lat", math.NaN(), "latitude in decimal degrees, north positive (required)")
	lon := fs.Float64("lon", math.NaN(), "longitude in decimal degrees, east positive (required)")
	dateFlag := fs.String("date", "", "calendar date, YYYY-MM-DD (default: today in the resolved zone)")
	tzFlag := fs.String("tz", "", "IANA time zone name (default: $TZ, then UTC)")
	jsonFlag := fs.Bool("json", false, "emit JSON; timestamps carry their UTC offset")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return exitUsage
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "sol: unexpected argument %q\n\n%s", fs.Arg(0), usage)
		return exitUsage
	}
	if code := validateCoords(*lat, *lon, stderr); code != 0 {
		return code
	}

	// Zone resolution: --tz, then $TZ, then UTC. A bad name at either
	// step is a loud error naming the zone, never a silent UTC fallback.
	loc, err := tz.Resolve(*tzFlag, os.Getenv("TZ"))
	if err != nil {
		fmt.Fprintf(stderr, "sol: %v\n", err)
		return exitError
	}
	zoneName := loc.String()

	date := *dateFlag
	if date == "" {
		date = time.Now().In(loc).Format("2006-01-02")
	}
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		fmt.Fprintf(stderr, "sol: invalid -date value %q: must be a real calendar date formatted YYYY-MM-DD\n", date)
		return exitUsage
	}

	day := schedule.For(parsed.Year(), parsed.Month(), parsed.Day(), loc, *lat, *lon)

	if *jsonFlag {
		if err := render.JSON(stdout, loc, zoneName, date, *lat, *lon, day); err != nil {
			fmt.Fprintf(stderr, "sol: %v\n", err)
			return exitError
		}
		return 0
	}
	render.Text(stdout, loc, zoneName, date, *lat, *lon, day)
	return 0
}

func validateCoords(lat, lon float64, stderr io.Writer) int {
	missingLat, missingLon := math.IsNaN(lat), math.IsNaN(lon)
	switch {
	case missingLat && missingLon:
		fmt.Fprintf(stderr, "sol: both -lat and -lon are required\n\n%s", usage)
		return exitUsage
	case missingLat:
		fmt.Fprintf(stderr, "sol: -lat is required\n\n%s", usage)
		return exitUsage
	case missingLon:
		fmt.Fprintf(stderr, "sol: -lon is required\n\n%s", usage)
		return exitUsage
	}
	if lat < -90 || lat > 90 {
		fmt.Fprintf(stderr, "sol: -lat %v out of range [-90, 90]\n", lat)
		return exitUsage
	}
	if lon < -180 || lon > 180 {
		fmt.Fprintf(stderr, "sol: -lon %v out of range [-180, 180]\n", lon)
		return exitUsage
	}
	return 0
}
