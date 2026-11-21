package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	validLat  = "40.7128"
	validLon  = "-74.006"
	validDate = "2024-06-21"
)

func TestGoldenHourHappyPath(t *testing.T) {
	rr := get(t, "/v1/golden-hour?lat="+validLat+"&lon="+validLon+"&date="+validDate)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rr.Code, http.StatusOK, rr.Body)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// The response carries the two windows and nothing else.
	if len(body) != 2 {
		t.Errorf("response has %d fields, want exactly 2: %v", len(body), rr.Body.String())
	}
	for _, key := range []string{"morning_golden_hour", "evening_golden_hour"} {
		raw, ok := body[key]
		if !ok {
			t.Errorf("response missing %q: %v", key, rr.Body.String())
			continue
		}
		var win struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		}
		if err := json.Unmarshal(raw, &win); err != nil {
			t.Errorf("%q not a time window: %v", key, err)
			continue
		}
		if !win.Start.Before(win.End) {
			t.Errorf("%q window not ordered: %v >= %v", key, win.Start, win.End)
		}
	}
}

// The golden-hour windows must be exactly what the events endpoint reports
// for the same inputs — same values, same time format.
func TestGoldenHourAgreesWithEvents(t *testing.T) {
	for _, query := range []string{
		"lat=" + validLat + "&lon=" + validLon + "&date=" + validDate,
		"lat=-33.8688&lon=151.2093&date=2024-12-21",
		"lat=51.5074&lon=-0.1278&date=2024-03-20",
	} {
		eventsRR := get(t, "/v1/events?"+query)
		goldenRR := get(t, "/v1/golden-hour?"+query)
		if eventsRR.Code != http.StatusOK || goldenRR.Code != http.StatusOK {
			t.Fatalf("events status %d, golden-hour status %d for %q", eventsRR.Code, goldenRR.Code, query)
		}

		var ev struct {
			MorningGoldenHour json.RawMessage `json:"morning_golden_hour"`
			EveningGoldenHour json.RawMessage `json:"evening_golden_hour"`
		}
		if err := json.Unmarshal(eventsRR.Body.Bytes(), &ev); err != nil {
			t.Fatalf("events JSON invalid for %q: %v", query, err)
		}
		var gh struct {
			MorningGoldenHour json.RawMessage `json:"morning_golden_hour"`
			EveningGoldenHour json.RawMessage `json:"evening_golden_hour"`
		}
		if err := json.Unmarshal(goldenRR.Body.Bytes(), &gh); err != nil {
			t.Fatalf("golden-hour JSON invalid for %q: %v", query, err)
		}

		if !reflect.DeepEqual(ev.MorningGoldenHour, gh.MorningGoldenHour) {
			t.Errorf("morning window mismatch for %q: events %s, golden-hour %s", query, ev.MorningGoldenHour, gh.MorningGoldenHour)
		}
		if !reflect.DeepEqual(ev.EveningGoldenHour, gh.EveningGoldenHour) {
			t.Errorf("evening window mismatch for %q: events %s, golden-hour %s", query, ev.EveningGoldenHour, gh.EveningGoldenHour)
		}
	}
}

func TestGoldenHourMissingParameters(t *testing.T) {
	cases := []struct {
		query string
		param string
	}{
		{"lat=40.7128&lon=-74.006", "date"}, // no date
		{"lon=-74.006&date=2024-06-21", "lat"},
		{"lat=&lon=-74.006&date=2024-06-21", "lat"},
		{"lat=40.7128&date=2024-06-21", "lon"}, // no lon
		{"lat=40.7128&lon=&date=2024-06-21", "lon"},
		{"lat=40.7128&lon=-74.006", "date"},
		{"lat=40.7128&lon=-74.006&date=", "date"},
	}
	for _, tc := range cases {
		rr := get(t, "/v1/golden-hour?"+tc.query)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("GET /v1/golden-hour?%s status = %d, want 400 (body %s)", tc.query, rr.Code, rr.Body)
		}
		if msg := errorBody(t, rr); !strings.Contains(msg, tc.param) {
			t.Errorf("GET /v1/golden-hour?%s error %q does not name %s", tc.query, msg, tc.param)
		}
	}
}

func TestGoldenHourInvalidValues(t *testing.T) {
	cases := []struct {
		query string
		param string
	}{
		// latitude outside ±90
		{"lat=90.0001&lon=-74.006&date=2024-06-21", "lat"},
		{"lat=-90.5&lon=-74.006&date=2024-06-21", "lat"},
		{"lat=abc&lon=-74.006&date=2024-06-21", "lat"},
		// longitude outside ±180
		{"lat=40.7128&lon=180.0001&date=2024-06-21", "lon"},
		{"lat=40.7128&lon=-181&date=2024-06-21", "lon"},
		{"lat=40.7128&lon=xyz&date=2024-06-21", "lon"},
		// dates that are not real calendar dates
		{"lat=40.7128&lon=-74.006&date=2024-02-30", "date"},
		{"lat=40.7128&lon=-74.006&date=2023-02-29", "date"},
		{"lat=40.7128&lon=-74.006&date=2024-13-01", "date"},
		{"lat=40.7128&lon=-74.006&date=2024-6-21", "date"},
		{"lat=40.7128&lon=-74.006&date=06/21/2024", "date"},
		{"lat=40.7128&lon=-74.006&date=not-a-date", "date"},
	}
	for _, tc := range cases {
		rr := get(t, "/v1/golden-hour?"+tc.query)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("GET /v1/golden-hour?%s status = %d, want 400 (body %s)", tc.query, rr.Code, rr.Body)
		}
		if msg := errorBody(t, rr); !strings.Contains(msg, tc.param) {
			t.Errorf("GET /v1/golden-hour?%s error %q does not name %s", tc.query, msg, tc.param)
		}
	}
}

// Whatever the events endpoint accepts, golden-hour must accept too — and
// whatever events rejects, golden-hour must reject. The two routes share one
// input check, so this battery pins that contract.
func TestRoutesAcceptAndRejectIdentically(t *testing.T) {
	queries := []string{
		"lat=40.7128&lon=-74.006&date=2024-06-21",   // valid
		"lat=-33.8688&lon=151.2093&date=2024-12-21", // valid, southern hemisphere
		"lat=90&lon=180&date=2024-06-21",            // boundary values
		"lat=90.1&lon=-74.006&date=2024-06-21",      // rejected
		"lat=40.7128&lon=180.1&date=2024-06-21",     // rejected
		"lat=40.7128&lon=-74.006&date=2024-04-31",   // rejected
		"lon=-74.006&date=2024-06-21",               // rejected (missing lat)
		"lat=40.7128&date=2024-06-21",               // rejected (missing lon)
		"lat=40.7128&lon=-74.006",                   // rejected (missing date)
		"lat=nonsense&lon=-74.006&date=2024-06-21",  // rejected
	}
	for _, q := range queries {
		eventsRR := get(t, "/v1/events?"+q)
		goldenRR := get(t, "/v1/golden-hour?"+q)
		if eventsRR.Code != goldenRR.Code {
			t.Errorf("routes disagree for %q: events=%d golden-hour=%d", q, eventsRR.Code, goldenRR.Code)
		}
	}
}

// The events endpoint behaves exactly as before the change: same shape, same
// golden-hour windows, same validation.
func TestEventsEndpoint(t *testing.T) {
	rr := get(t, "/v1/events?lat="+validLat+"&lon="+validLon+"&date="+validDate)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body %s)", rr.Code, http.StatusOK, rr.Body)
	}

	var ev struct {
		Date               string    `json:"date"`
		Lat                float64   `json:"lat"`
		Lon                float64   `json:"lon"`
		Sunrise            time.Time `json:"sunrise"`
		Sunset             time.Time `json:"sunset"`
		SolarNoon          time.Time `json:"solar_noon"`
		CivilDawn          time.Time `json:"civil_dawn"`
		CivilDusk          time.Time `json:"civil_dusk"`
		MorningGoldenHour  struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		} `json:"morning_golden_hour"`
		EveningGoldenHour struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		} `json:"evening_golden_hour"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ev); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if ev.Date != validDate || ev.Lat != 40.7128 || ev.Lon != -74.006 {
		t.Errorf("query echo mismatch: %v", rr.Body.String())
	}
	if !ev.MorningGoldenHour.Start.Before(ev.Sunrise) || !ev.Sunrise.Before(ev.MorningGoldenHour.End) {
		t.Errorf("sunrise %v not inside morning window [%v, %v]", ev.Sunrise, ev.MorningGoldenHour.Start, ev.MorningGoldenHour.End)
	}
	if !ev.EveningGoldenHour.Start.Before(ev.Sunset) || !ev.Sunset.Before(ev.EveningGoldenHour.End) {
		t.Errorf("sunset %v not inside evening window [%v, %v]", ev.Sunset, ev.EveningGoldenHour.Start, ev.EveningGoldenHour.End)
	}
}

func TestEventsRejectsInvalidInput(t *testing.T) {
	for _, q := range []string{
		"lat=91&lon=-74.006&date=2024-06-21",
		"lat=40.7128&lon=-181&date=2024-06-21",
		"lat=40.7128&lon=-74.006&date=2024-02-30",
		"lat=&lon=-74.006&date=2024-06-21",
	} {
		rr := get(t, "/v1/events?"+q)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("GET /v1/events?%s status = %d, want 400", q, rr.Code)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	for _, path := range []string{"/v1/events", "/v1/golden-hour"} {
		req := httptest.NewRequest(http.MethodPost, path+"?lat="+validLat+"&lon="+validLon+"&date="+validDate, nil)
		rr := httptest.NewRecorder()
		New().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", path, rr.Code)
		}
	}
}

func TestUnknownRoute(t *testing.T) {
	rr := get(t, "/v1/nope")
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func get(t *testing.T, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	New().ServeHTTP(rr, req)
	return rr
}

func errorBody(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error response not JSON with an 'error' field: %v (body %s)", err, rr.Body)
	}
	return body.Error
}
