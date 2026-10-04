// Package pollen collects the CAMS Europe pollen forecast from the Open-Meteo
// air-quality API onto a lattice of model cells.
package pollen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"airbg.org/internal/config"
	"airbg.org/internal/store"
)

// Unit is what the API reports pollen in; anything else is refused.
const Unit = "grains/m³"

type Client struct {
	cfg  config.Pollen
	http *http.Client
}

func New(cfg config.Pollen) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.RequestTimeout}}
}

// apiSeries is one location's slice of the response. Hourly holds "time" as
// strings and "<species>_pollen" as nullable numbers.
type apiSeries struct {
	HourlyUnits map[string]string          `json:"hourly_units"`
	Hourly      map[string]json.RawMessage `json:"hourly"`
}

// Fetch returns every cell's hourly values in batches of PointsPerReq. A
// failed batch fails the call: a partial run would leave some areas a run behind.
func (c *Client) Fetch(ctx context.Context, cells []store.PollenCell) ([]store.PollenForecast, error) {
	var out []store.PollenForecast
	for start := 0; start < len(cells); start += c.cfg.PointsPerReq {
		end := min(start+c.cfg.PointsPerReq, len(cells))
		batch, err := c.fetchBatch(ctx, cells[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

func (c *Client) fetchBatch(ctx context.Context, cells []store.PollenCell) ([]store.PollenForecast, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.requestURL(cells), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pollen: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pollen: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.cfg.MaxPayloadBytes))
	if err != nil {
		return nil, fmt.Errorf("pollen: read body: %w", err)
	}
	return Parse(body, cells, c.species())
}

func (c *Client) species() []string {
	out := make([]string, len(c.cfg.Species))
	for i, s := range c.cfg.Species {
		out[i] = s.Name
	}
	return out
}

func (c *Client) requestURL(cells []store.PollenCell) string {
	lats := make([]string, len(cells))
	lons := make([]string, len(cells))
	for i, p := range cells {
		lats[i] = centi(p.LatC)
		lons[i] = centi(p.LonC)
	}
	vars := make([]string, len(c.cfg.Species))
	for i, s := range c.cfg.Species {
		vars[i] = s.Name + "_pollen"
	}
	q := url.Values{
		"latitude":      {strings.Join(lats, ",")},
		"longitude":     {strings.Join(lons, ",")},
		"hourly":        {strings.Join(vars, ",")},
		"domains":       {c.cfg.Domain},
		"timezone":      {"UTC"},
		"past_days":     {strconv.Itoa(c.cfg.PastDays)},
		"forecast_days": {strconv.Itoa(c.cfg.ForecastDays)},
	}
	return c.cfg.URL + "?" + q.Encode()
}

// centi renders centidegrees as a decimal: 2330 is "23.30", -5 is "-0.05".
func centi(v int) string {
	sign := ""
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// Parse maps a multi-location response onto the cells that produced it, by
// position: the response carries the model's snapped coordinates, not ours.
// Null values are gaps in the run and are dropped, not stored as zero.
func Parse(payload []byte, cells []store.PollenCell, species []string) ([]store.PollenForecast, error) {
	var series []apiSeries
	if err := json.Unmarshal(payload, &series); err != nil {
		return nil, fmt.Errorf("pollen: decode: %w", err)
	}
	if len(series) != len(cells) {
		return nil, fmt.Errorf("pollen: asked for %d locations, got %d", len(cells), len(series))
	}
	var out []store.PollenForecast
	for i, s := range series {
		var times []string
		if err := json.Unmarshal(s.Hourly["time"], &times); err != nil {
			return nil, fmt.Errorf("pollen: location %d time: %w", i, err)
		}
		stamps := make([]time.Time, len(times))
		for j, ts := range times {
			t, err := time.ParseInLocation("2006-01-02T15:04", ts, time.UTC)
			if err != nil {
				return nil, fmt.Errorf("pollen: location %d timestamp %q: %w", i, ts, err)
			}
			stamps[j] = t
		}
		for _, sp := range species {
			key := sp + "_pollen"
			if u, ok := s.HourlyUnits[key]; ok && u != Unit {
				return nil, fmt.Errorf("pollen: location %d %s is in %q, want %q", i, key, u, Unit)
			}
			raw, ok := s.Hourly[key]
			if !ok {
				return nil, fmt.Errorf("pollen: location %d has no %s", i, key)
			}
			var vals []*float64
			if err := json.Unmarshal(raw, &vals); err != nil {
				return nil, fmt.Errorf("pollen: location %d %s: %w", i, key, err)
			}
			if len(vals) != len(stamps) {
				return nil, fmt.Errorf("pollen: location %d has %d timestamps but %d %s values", i, len(stamps), len(vals), key)
			}
			for j, v := range vals {
				if v == nil || *v < 0 {
					continue
				}
				out = append(out, store.PollenForecast{
					LonC: cells[i].LonC, LatC: cells[i].LatC,
					Species: sp, ValidAt: stamps[j], Grains: *v,
				})
			}
		}
	}
	return out, nil
}
