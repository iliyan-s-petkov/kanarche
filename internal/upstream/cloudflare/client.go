// Package cloudflare pulls daily unique-visitor counts from Cloudflare's
// GraphQL analytics API. See README.md for the job's shape and scheduling.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kanarche.eu/internal/config"
)

// TokenEnv is where the API token comes from. Env-only, like
// config.DatabaseURLEnv: it is a credential and airbg.yaml is committed.
const TokenEnv = "AIRBG_CF_ANALYTICS_TOKEN"

// dateFormat is the Date scalar Cloudflare's GraphQL schema expects and
// returns: calendar days, no time-of-day.
const dateFormat = "2006-01-02"

// dailyUniquesQuery asks for one zone's daily grouped totals. limit bounds
// how many day-rows can come back; the collector sizes it to the requested
// window plus headroom.
const dailyUniquesQuery = `query DailyUniques($zoneTag: String!, $since: Date!, $until: Date!, $limit: Int!) {
  viewer {
    zones(filter: {zoneTag: $zoneTag}) {
      httpRequests1dGroups(limit: $limit, filter: {date_geq: $since, date_leq: $until}, orderBy: [date_ASC]) {
        dimensions { date }
        uniq { uniques }
        sum { requests pageViews }
      }
    }
  }
}`

// DailyPoint is one day's totals, as the API answers or a seed file supplies.
type DailyPoint struct {
	Date      time.Time
	Uniques   int
	Requests  int64
	PageViews int64
}

type Client struct {
	cfg   config.Cloudflare
	token string
	http  *http.Client
}

// New builds a client. token may be empty; a caller with no token should not
// call FetchDaily at all (see Collector.Loop) rather than rely on the server
// to reject an unauthenticated request.
func New(cfg config.Cloudflare, token string) *Client {
	return &Client{cfg: cfg, token: token, http: &http.Client{Timeout: cfg.RequestTimeout}}
}

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type gqlResponse struct {
	Data struct {
		Viewer struct {
			Zones []struct {
				HTTPRequests1dGroups []struct {
					Dimensions struct {
						Date string `json:"date"`
					} `json:"dimensions"`
					Uniq struct {
						Uniques int `json:"uniques"`
					} `json:"uniq"`
					Sum struct {
						Requests  int64 `json:"requests"`
						PageViews int64 `json:"pageViews"`
					} `json:"sum"`
				} `json:"httpRequests1dGroups"`
			} `json:"zones"`
		} `json:"viewer"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// FetchDaily returns one row per day in [since, until], inclusive, both
// truncated to the calendar date. limit is sized to the window plus a small
// margin so a slightly wider-than-expected response is never silently cut.
func (c *Client) FetchDaily(ctx context.Context, since, until time.Time) ([]DailyPoint, error) {
	days := int(until.Sub(since).Hours()/24) + 2
	if days < 1 {
		days = 1
	}
	limit := days + 5

	body, err := json.Marshal(gqlRequest{
		Query: dailyUniquesQuery,
		Variables: map[string]any{
			"zoneTag": c.cfg.ZoneID,
			"since":   since.UTC().Format(dateFormat),
			"until":   until.UTC().Format(dateFormat),
			"limit":   limit,
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Never logged: this header is the only place the token appears, and no
	// caller here writes headers to a log.
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: fetch daily: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cloudflare: fetch daily: status %d", resp.StatusCode)
	}

	var out gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("cloudflare: fetch daily: decode response: %w", err)
	}
	if len(out.Errors) > 0 {
		msgs := make([]string, len(out.Errors))
		for i, e := range out.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("cloudflare: graphql error(s): %s", strings.Join(msgs, "; "))
	}
	if len(out.Data.Viewer.Zones) == 0 {
		return nil, fmt.Errorf("cloudflare: fetch daily: no zone matched zone_id %q", c.cfg.ZoneID)
	}

	rows := out.Data.Viewer.Zones[0].HTTPRequests1dGroups
	points := make([]DailyPoint, 0, len(rows))
	for _, r := range rows {
		d, err := time.Parse(dateFormat, r.Dimensions.Date)
		if err != nil {
			return nil, fmt.Errorf("cloudflare: fetch daily: bad date %q: %w", r.Dimensions.Date, err)
		}
		points = append(points, DailyPoint{
			Date:      d.UTC(),
			Uniques:   r.Uniq.Uniques,
			Requests:  r.Sum.Requests,
			PageViews: r.Sum.PageViews,
		})
	}
	return points, nil
}
