// Package render formats a solar schedule as human-readable text or JSON.
//
// Every event is formatted as its own instant in the target zone, so a
// single day that spans a daylight-saving transition carries two different
// UTC offsets, one per event. The text and JSON forms list the same
// events in the same order and carry the same times, so scripts and
// humans read one schedule, not two.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zaephor-bot/sol-lib/windows"

	"github.com/zaephor-bot/sol-cli/internal/schedule"
)

// stampLayout prints a full timestamp with its numeric UTC offset, e.g.
// "2026-03-08 01:30 -08:00". UTC itself renders as "+00:00".
const stampLayout = "2006-01-02 15:04 -07:00"

// jsonLayout is RFC 3339 with a forced numeric offset (never the "Z"
// form), so every JSON timestamp visibly carries its zone.
const jsonLayout = "2006-01-02T15:04:05-07:00"

// Stamp renders one instant in loc with its numeric UTC offset.
func Stamp(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(stampLayout)
}

// Text writes the schedule as plain text. Every printed time carries its
// UTC offset in the target zone; an event that does not happen on the
// requested day prints as a "none" row naming it.
func Text(w io.Writer, loc *time.Location, zoneName, date string, lat, lon float64, day schedule.Day) {
	fmt.Fprintf(w, "Solar schedule for %s at %g, %g (%s)\n\n", date, lat, lon, zoneName)
	instant(w, loc, "sunrise", day.Sunrise, day.Polar)
	instant(w, loc, "solar noon", &day.SolarNoon, "")
	instant(w, loc, "sunset", day.Sunset, day.Polar)
	window(w, loc, day.DawnGoldenHour)
	window(w, loc, day.DuskGoldenHour)
	window(w, loc, day.DawnBlueHour)
	window(w, loc, day.DuskBlueHour)
}

// instant writes one point-in-time event; t nil means the event does
// not happen on the requested day.
func instant(w io.Writer, loc *time.Location, name string, t *time.Time, polar string) {
	if t == nil {
		fmt.Fprintf(w, "%-18s none (%s)\n", name, missing(polar, name))
		return
	}
	fmt.Fprintf(w, "%-18s %s\n", name, Stamp(*t, loc))
}

// window writes one two-ended event from the library's Window record.
func window(w io.Writer, loc *time.Location, win windows.Window) {
	name := win.Event.String()
	if win.Absent {
		fmt.Fprintf(w, "%-18s none (%s)\n", name, missing("", name))
		return
	}
	fmt.Fprintf(w, "%-18s %s to %s\n", name, Stamp(win.Start, loc), Stamp(win.End, loc))
}

// missing explains a "none" row: the polar reason when the day itself
// explains it, otherwise the event's own absence.
func missing(polar, name string) string {
	if polar != "" {
		return polar
	}
	return "no " + name + " this day"
}

type jsonEvent struct {
	Name  string  `json:"name"`
	Time  *string `json:"time"`  // point events; null when the event does not occur
	Start *string `json:"start"` // window events; null when the window is absent
	End   *string `json:"end"`   // window events; null when the window is absent
}

type jsonDoc struct {
	Date      string      `json:"date"`
	Latitude  float64     `json:"latitude"`
	Longitude float64     `json:"longitude"`
	Timezone  string      `json:"timezone"`
	Events    []jsonEvent `json:"events"`
}

// JSON writes the schedule as JSON in the same event order as the text
// form. Every timestamp is an RFC 3339 string carrying its resolved UTC
// offset, so scripts see the same zone the text output shows. A window's
// name is its spoken name with spaces replaced by underscores.
func JSON(w io.Writer, loc *time.Location, zoneName, date string, lat, lon float64, day schedule.Day) error {
	doc := jsonDoc{
		Date:      date,
		Latitude:  lat,
		Longitude: lon,
		Timezone:  zoneName,
		Events:    make([]jsonEvent, 0, 7),
	}
	addInstant := func(name string, t *time.Time) {
		doc.Events = append(doc.Events, jsonEvent{Name: name, Time: jsonStamp(t, loc)})
	}
	addWindow := func(win windows.Window) {
		e := jsonEvent{Name: strings.ReplaceAll(win.Event.String(), " ", "_")}
		if !win.Absent {
			e.Start = jsonStamp(&win.Start, loc)
			e.End = jsonStamp(&win.End, loc)
		}
		doc.Events = append(doc.Events, e)
	}

	addInstant("sunrise", day.Sunrise)
	addInstant("solar_noon", &day.SolarNoon)
	addInstant("sunset", day.Sunset)
	addWindow(day.DawnGoldenHour)
	addWindow(day.DuskGoldenHour)
	addWindow(day.DawnBlueHour)
	addWindow(day.DuskBlueHour)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// jsonStamp renders one instant for JSON, or nil for a missing event.
func jsonStamp(t *time.Time, loc *time.Location) *string {
	if t == nil {
		return nil
	}
	s := t.In(loc).Format(jsonLayout)
	return &s
}
