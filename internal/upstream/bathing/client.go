// Package bathing imports EEA bathing-water sites, annual classes and lab
// samples from the Discodata SQL API. See README.md for cadence and limits.
package bathing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"airbg.org/internal/config"
)

// ErrPayloadTooLarge reports a body at or over max_payload_bytes.
var ErrPayloadTooLarge = errors.New("payload exceeds max_payload_bytes")

// ErrTruncated reports a page as long as max_rows: more rows may exist behind it.
var ErrTruncated = errors.New("result page is full; raise sea.max_rows")

// SiteRow is one spatial_ProtectedArea row as Discodata returns it.
type SiteRow struct {
	ID     string  `json:"inspireIdLocalId"`
	NameBG string  `json:"nameText"`
	NameEN string  `json:"nameTextInternational"`
	Zone   string  `json:"specialisedZoneType"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Status string  `json:"statusCode"`
	Link   string  `json:"link"`
}

// StatusRow is one assessment_BathingWaterStatus row.
type StatusRow struct {
	SiteID  string  `json:"bathingWaterIdentifier"`
	Season  int     `json:"season"`
	Quality *string `json:"quality"`
}

// SampleRow is one assessment_MonitoringResult row.
type SampleRow struct {
	SiteID       string  `json:"bathingWaterIdentifier"`
	Season       int     `json:"season"`
	Date         string  `json:"sampleDate"`
	EC           *int    `json:"escherichiaColiValue"`
	ECStatus     *string `json:"escherichiaColiStatus"`
	IE           *int    `json:"intestinalEnterococciValue"`
	IEStatus     *string `json:"intestinalEnterococciStatus"`
	SampleStatus *string `json:"sampleStatus"`
}

// Raw is the three tables, unvalidated.
type Raw struct {
	Sites   []SiteRow
	Status  []StatusRow
	Samples []SampleRow
}

// The country code is the only interpolated value; Fetch checks it first.
const (
	sitesSQL   = `SELECT inspireIdLocalId, nameText, nameTextInternational, specialisedZoneType, lat, lon, statusCode, link FROM [WISE_BWD].[latest].[spatial_ProtectedArea] WHERE countryCode='%s'`
	statusSQL  = `SELECT bathingWaterIdentifier, season, quality FROM [WISE_BWD].[latest].[assessment_BathingWaterStatus] WHERE countryCode='%s'`
	samplesSQL = `SELECT bathingWaterIdentifier, season, sampleDate, escherichiaColiValue, escherichiaColiStatus, intestinalEnterococciValue, intestinalEnterococciStatus, sampleStatus FROM [WISE_BWD].[latest].[assessment_MonitoringResult] WHERE countryCode='%s'`
)

type Client struct {
	cfg  config.Sea
	http *http.Client
}

func New(cfg config.Sea) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.RequestTimeout, CheckRedirect: sameOriginOnly}}
}

// sameOriginOnly refuses a redirect off the configured scheme+host.
func sameOriginOnly(req *http.Request, via []*http.Request) error {
	first := via[0].URL
	if req.URL.Scheme != first.Scheme || req.URL.Host != first.Host {
		return fmt.Errorf("bathing: refused redirect from %s://%s to %s://%s",
			first.Scheme, first.Host, req.URL.Scheme, req.URL.Host)
	}
	if len(via) >= 10 {
		return fmt.Errorf("bathing: stopped after %d redirects", len(via))
	}
	return nil
}

// Fetch runs the three queries. Any failure fails the whole fetch.
func (c *Client) Fetch(ctx context.Context) (Raw, error) {
	var raw Raw
	if !config.IsCountryCode(c.cfg.Country) {
		return raw, fmt.Errorf("bathing: country %q is not an ISO code", c.cfg.Country)
	}
	var err error
	if raw.Sites, err = query[SiteRow](c, ctx, fmt.Sprintf(sitesSQL, c.cfg.Country)); err != nil {
		return raw, fmt.Errorf("bathing: sites: %w", err)
	}
	if raw.Status, err = query[StatusRow](c, ctx, fmt.Sprintf(statusSQL, c.cfg.Country)); err != nil {
		return raw, fmt.Errorf("bathing: status: %w", err)
	}
	if raw.Samples, err = query[SampleRow](c, ctx, fmt.Sprintf(samplesSQL, c.cfg.Country)); err != nil {
		return raw, fmt.Errorf("bathing: samples: %w", err)
	}
	return raw, nil
}

// row is the set of table shapes the client decodes.
type row interface {
	SiteRow | StatusRow | SampleRow
}

type envelope[T row] struct {
	Results []T `json:"results"`
	Errors  []struct {
		Error string `json:"error"`
	} `json:"errors"`
}

func query[T row](c *Client, ctx context.Context, sql string) ([]T, error) {
	u, err := url.Parse(c.cfg.URL)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("query", sql)
	q.Set("p", "1")
	q.Set("nrOfHits", strconv.Itoa(c.cfg.MaxRows))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		// The query string is public SQL, but the error would otherwise carry all of it.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return nil, fmt.Errorf("%s %s: %w", uerr.Op, c.cfg.URL, uerr.Err)
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.cfg.MaxPayloadBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > c.cfg.MaxPayloadBytes {
		return nil, fmt.Errorf("%w (%d bytes)", ErrPayloadTooLarge, c.cfg.MaxPayloadBytes)
	}
	var env envelope[T]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(env.Errors) > 0 {
		msgs := make([]string, len(env.Errors))
		for i, e := range env.Errors {
			msgs[i] = e.Error
		}
		return nil, fmt.Errorf("discodata: %s", strings.Join(msgs, "; "))
	}
	if len(env.Results) >= c.cfg.MaxRows {
		return nil, fmt.Errorf("%w (%d rows)", ErrTruncated, len(env.Results))
	}
	return env.Results, nil
}
