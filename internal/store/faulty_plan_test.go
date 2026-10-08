package store_test

import (
	"encoding/json"
	"testing"
	"time"

	"kanarche.eu/internal/store"
)

type planNode struct {
	RelationName string     `json:"Relation Name"`
	Plans        []planNode `json:"Plans"`
}

func (n planNode) relations(out []string) []string {
	if n.RelationName != "" {
		out = append(out, n.RelationName)
	}
	for _, c := range n.Plans {
		out = c.relations(out)
	}
	return out
}

// The faulty set is a sum over the hourly counts; it must never read raw
// readings, which is the scan the counts table exists to avoid.
func TestRefreshFaultyPlanNeverReadsRawReadings(t *testing.T) {
	ctx, pool, s := faultyStore(t)
	now := time.Now().UTC()
	hour := store.TruncateHour(now).Add(-time.Hour)
	seedSensorReading(t, ctx, pool, 1, 23.3, 42.7, "P2", 10, "ok", hour)
	if _, err := s.RollupHour(ctx, hour); err != nil {
		t.Fatalf("RollupHour: %v", err)
	}

	var raw []byte
	if err := pool.QueryRow(ctx, `EXPLAIN (FORMAT JSON) `+store.RefreshFaultySelectSQL,
		store.TruncateHour(now).Add(time.Hour), (24 * time.Hour).Seconds(), 0.5, 6).Scan(&raw); err != nil {
		t.Fatalf("explain: %v", err)
	}
	var plans []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("decode plan: %v (%d plans)", err, len(plans))
	}
	rels := plans[0].Plan.relations(nil)
	if len(rels) == 0 {
		t.Fatalf("plan names no relation: %s", raw)
	}
	for _, rel := range rels {
		var table string
		err := pool.QueryRow(ctx, `SELECT COALESCE(
			(SELECT hypertable_name FROM timescaledb_information.chunks WHERE chunk_name = $1), $1)`, rel).Scan(&table)
		if err != nil {
			t.Fatalf("resolve %s: %v", rel, err)
		}
		if table != "reading_quality_hourly" {
			t.Errorf("faulty plan reads %s (%s), want only reading_quality_hourly", rel, table)
		}
	}
}
