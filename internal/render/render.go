// Package render formats a solar schedule as human-readable text or JSON.
//
// Every event is formatted as its own instant in the target zone, so a
// single day that spans a daylight-saving transition carries two different
// UTC offsets, one per event.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/zaephor-bot/sol-cli/internal/solar"
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
// UTC offset in the target zone.
func Text(w io.Writer, loc *time.Location, zoneName, date string, lat, lon float64, day solar.Day) {
	fmt.Fprintf(w, "Solar schedule for %s at %g, %g (%s)\n\n", date, lat, lon, zoneName)
	row(w, "sunrise", day.Sunrise, loc)
	row(w, "solar noon", &day.SolarNoon, loc)
	row(w, "sunset", day.Sunset, loc)
}

func row(w io.Writer, name string, t *time.Time, loc *time.Location) {
	if t == nil {
		fmt.Fprintf(w, "%-12s none (no %s this day)\n", name, name)
		return
	}
	fmt.Fprintf(w, "%-12s %s\n", name, Stamp(*t, loc))
}

type jsonEvent struct {
	Name string  `json:"name"`
	Time *string `json:"time"` // RFC 3339 with numeric offset; null when the event does not occur
}

type jsonDoc struct {
	Date      string      `json:"date"`
	Latitude  float64     `json:"latitude"`
	Longitude float64     `json:"longitude"`
	Timezone  string      `json:"timezone"`
	Events    []jsonEvent `json:"events"`
}

// JSON writes the schedule as JSON. Every timestamp is an RFC 3339 string
// carrying its resolved UTC offset, so scripts see the same zone the text
// output shows.
func JSON(w io.Writer, loc *time.Location, zoneName, date string, lat, lon float64, day solar.Day) error {
	doc := jsonDoc{
		Date:      date,
		Latitude:  lat,
		Longitude: lon,
		Timezone:  zoneName,
		Events:    make([]jsonEvent, 0, 3),
	}
	add := func(name string, t *time.Time) {
		var s *string
		if t != nil {
			v := t.In(loc).Format(jsonLayout)
			s = &v
		}
		doc.Events = append(doc.Events, jsonEvent{Name: name, Time: s})
	}
	add("sunrise", day.Sunrise)
	add("solar_noon", &day.SolarNoon)
	add("sunset", day.Sunset)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
