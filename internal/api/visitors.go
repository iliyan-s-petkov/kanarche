package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"kanarche.eu/internal/store"
)

const (
	visitorsDefaultDays = 30
	visitorsMaxDays     = 365

	// The table gains one row a day, so an hour-old answer is never wrong in a
	// way a reader can see. Same value for the in-process cache and max-age.
	visitorsTTL = time.Hour
)

// visitorCache holds the newest visitorsMaxDays rows for visitorsTTL. Every
// ?days= is a tail of that one slice, so the database sees one query per TTL
// however many distinct values callers send.
type visitorCache struct {
	mu      sync.Mutex
	rows    []store.VisitorDaily
	fetched time.Time
	ok      bool
}

// get returns the cached rows, refreshing when stale. The lock is held across
// the query so a burst after expiry makes one query, not one per request.
// A failed refresh is returned and not cached.
func (c *visitorCache) get(ctx context.Context, src DataSource, now time.Time) ([]store.VisitorDaily, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ok && now.Sub(c.fetched) < visitorsTTL {
		return c.rows, c.fetched, nil
	}
	rows, err := src.VisitorDailyLast(ctx, visitorsMaxDays)
	if err != nil {
		return nil, time.Time{}, err
	}
	c.rows, c.fetched, c.ok = rows, now, true
	return rows, now, nil
}

type visitorDay struct {
	Day       string `json:"day"`
	Uniques   int    `json:"uniques"`
	Requests  int64  `json:"requests"`
	PageViews int64  `json:"page_views"`
}

// No total of uniques: Cloudflare counts unique IPs per day, so summing days
// counts a returning visitor once per day.
type visitorsBody struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Days        []visitorDay `json:"days"`
}

// handleVisitors serves daily traffic counts, oldest first. Missing days are
// absent, not zero-filled.
func (d Deps) handleVisitors(w http.ResponseWriter, r *http.Request) {
	days := visitorsDefaultDays
	if v, present := r.URL.Query()["days"]; present {
		n, err := strconv.Atoi(v[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request",
				`The "days" parameter must be an integer.`)
			return
		}
		days = min(max(n, 1), visitorsMaxDays)
	}

	rows, generatedAt, err := d.visitors.get(r.Context(), d.Store, time.Now())
	if err != nil {
		slog.Error("visitor daily query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "Internal server error.")
		return
	}
	// Calendar window including today (UTC). Rows are ascending, so drop the
	// old prefix; a gap shortens the result instead of reaching further back.
	now := time.Now().UTC()
	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(days - 1))
	for len(rows) > 0 && rows[0].Day.Before(cutoff) {
		rows = rows[1:]
	}

	body := visitorsBody{GeneratedAt: generatedAt.UTC(), Days: make([]visitorDay, 0, len(rows))}
	for _, v := range rows {
		body.Days = append(body.Days, visitorDay{
			Day:       v.Day.Format("2006-01-02"),
			Uniques:   v.Uniques,
			Requests:  v.Requests,
			PageViews: v.PageViews,
		})
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "Internal server error.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	setCacheControl(w.Header(), cachePublic, int(visitorsTTL.Seconds()))
	_, _ = w.Write(encoded)
}
