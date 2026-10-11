package rzi

import (
	"context"
	"fmt"

	"kanarche.eu/internal/metrics"
	"kanarche.eu/internal/store"
)

var (
	fileStatus = metrics.GaugeVec("airbg_rzi_file_status",
		"1 for the status of the newest RZI file of a source, 0 for the other statuses.", "source", "status")
	lastOK = metrics.GaugeVec("airbg_rzi_last_ok_timestamp",
		"Unix time the newest ok RZI file of a source was stored.", "source")
)

// statuses is the closed set of values for the status label.
var statuses = []string{store.RZIStatusOK, store.RZIStatusNeedsReview, store.RZIStatusRejected}

// PublishMetrics sets both gauges from the store, so a restart or an
// operator approval is reflected on the next call. A source with no file has
// no series.
func (g *Ingester) PublishMetrics(ctx context.Context) error {
	for _, src := range Sources() {
		latest, ok, err := g.sink.RZILatest(ctx, src)
		if err != nil {
			return fmt.Errorf("rzi metrics: %w", err)
		}
		if !ok {
			continue
		}
		for _, s := range statuses {
			v := 0.0
			if s == latest.Status {
				v = 1
			}
			fileStatus.With(src, s).Set(v)
		}
		good, ok, err := g.sink.RZILastGood(ctx, src)
		if err != nil {
			return fmt.Errorf("rzi metrics: %w", err)
		}
		if ok {
			lastOK.With(src).Set(float64(good.FetchedAt.Unix()))
		}
	}
	return nil
}
