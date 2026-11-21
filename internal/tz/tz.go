// Package tz resolves the time zone sol prints its schedule in.
//
// Resolution order: the --tz flag, then the TZ environment variable, then
// UTC when both are absent. An empty value at either step counts as unset.
// A name that fails to resolve is an error that names the bad zone — never
// a silent fallback to UTC (Go itself silently turns a garbage TZ into
// time.Local == UTC, which is exactly what this package avoids by loading
// zones explicitly instead of consulting time.Local).
package tz

import (
	"fmt"
	"time"

	// Embed the IANA time zone database so sol stays honest on hosts that
	// lack /usr/share/zoneinfo (containers, scratch images, Windows).
	_ "time/tzdata"
)

// Resolve returns the zone to print times in. flagZone (from --tz) wins
// over envZone (from $TZ); an empty value at either step counts as unset
// and falls through; when both are unset the zone is UTC.
//
// Both IANA names (America/Los_Angeles) and legacy POSIX spellings that
// exist as tzdata entries (EST5EDT) load successfully.
func Resolve(flagZone, envZone string) (*time.Location, error) {
	if flagZone != "" {
		return load(flagZone, "--tz")
	}
	if envZone != "" {
		return load(envZone, "TZ")
	}
	return time.UTC, nil
}

func load(name, source string) (*time.Location, error) {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: %v", source, name, err)
	}
	return loc, nil
}
