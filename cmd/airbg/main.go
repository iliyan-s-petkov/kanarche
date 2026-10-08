package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	// Embeds the IANA zoneinfo database, so time.LoadLocation("Europe/Sofia")
	// resolves even on a base image with no /usr/share/zoneinfo.
	_ "time/tzdata"

	"kanarche.eu/internal/area"
	"kanarche.eu/internal/backfill"
	"kanarche.eu/internal/config"
	"kanarche.eu/internal/db"
	"kanarche.eu/internal/i18n"
	"kanarche.eu/internal/ingest"
	"kanarche.eu/internal/pollen"
	"kanarche.eu/internal/quality"
	"kanarche.eu/internal/server"
	"kanarche.eu/internal/snapshot"
	"kanarche.eu/internal/store"
	"kanarche.eu/internal/upstream"
	"kanarche.eu/internal/upstream/bathing"
	"kanarche.eu/internal/upstream/cloudflare"
	"kanarche.eu/internal/upstream/eea"
	"kanarche.eu/internal/web"
	"kanarche.eu/internal/wind"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: airbg <migrate|collect|serve|backfill|rollup|import-areas|purge-outside-boundary|seed-visitor-daily|import-sea|extract-bathing-datahub|validate-config|contract|healthz>")
		os.Exit(2)
	}

	// validate-config is checked before any database or listener setup: it
	// exists so an operator can catch a bad airbg.yaml before deploying
	// rather than at server start.
	if os.Args[1] == "validate-config" {
		os.Exit(runValidateConfig(os.Stdout, os.Stderr))
	}

	// extract-bathing-datahub reads a pinned file and writes a JSON snapshot. It
	// needs no database, so it runs before the pool is opened.
	if os.Args[1] == "extract-bathing-datahub" {
		os.Exit(runExtractDatahub(os.Args[2:], os.Stdout, os.Stderr))
	}

	// contract emits the constants the frontend generates from. Checked before
	// config or a database is touched: it is a build step, not an operation.
	if os.Args[1] == "contract" {
		os.Exit(runContract(os.Args[2:], os.Stdout, os.Stderr))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		// Fail closed and print the whole list: a config error is an operator
		// error, and one problem per restart is a bad trade for them.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// serve is the only subcommand that runs two workloads in one process, so it
	// is the only one that needs the bulkhead — and it must not also hold a
	// third, unused pool. Handled before the shared pool is opened.
	if os.Args[1] == "serve" {
		if err := serveCommand(ctx, cfg); err != nil {
			slog.Error("serve", "error", err)
			os.Exit(1)
		}
		return
	}

	// healthz is the container healthcheck's probe: it must not open the
	// database pool, or a probe run every 30s would hold a connection the
	// request handlers need.
	if os.Args[1] == "healthz" {
		if err := runHealthz(cfg.Listen.MetricsAddr, 3*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	pool, err := db.Open(ctx, cfg.Database)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch os.Args[1] {
	case "migrate":
		if err := db.Migrate(ctx, pool); err != nil {
			slog.Error("migrate", "error", err)
			os.Exit(1)
		}
		slog.Info("migrations applied")

	case "collect":
		if cfg.Wind.Enabled {
			// The wind loop is started alongside the reading loop, on its own
			// interval, and stops with the same context.
			go wind.NewCollector(cfg.Wind, store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series)).Loop(ctx)
		}
		if cfg.Pollen.Enabled {
			go pollen.NewCollector(cfg.Pollen, store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series)).Loop(ctx)
		}
		if cfg.EEA.Enabled {
			go eea.NewCollector(cfg.EEA, store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series), quality.NewScorer(cfg.Quality)).Loop(ctx)
		}
		if cfg.Cloudflare.Enabled {
			cfStore := store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series)
			go cloudflare.NewCollector(cfg.Cloudflare, config.Getenv(cloudflare.TokenEnv), cfStore).Loop(ctx)
		}
		if cfg.Sea.Enabled {
			seaCollector := bathing.NewCollector(cfg.Sea, store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series))
			seaCollector.SetSupplement(loadSupplement(cfg))
			seaCollector.SetEditionWatch(editionWatch(cfg))
			go seaCollector.Loop(ctx)
		}
		client := upstream.New(cfg.Upstream)
		collectStore := store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Series)
		ing := ingest.New(client, collectStore, seededHistory(ctx, collectStore, cfg.Quality), quality.NewScorer(cfg.Quality), cfg.Database.StatementTimeouts.Assign, cfg.Upstream.Countries)
		ing.Loop(ctx, cfg.Upstream.PollInterval)

	case "backfill":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: airbg backfill <sensor_id> <archive-csv-path>")
			os.Exit(2)
		}
		sensorID, err := strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}
		// Refuse an official-range sensor_id before touching the database at
		// all: official readings reach reading_hourly only through the EEA
		// collector, and a hand-backfilled row under one of those ids would
		// be indistinguishable from real official data.
		if err := backfill.CheckNotOfficial(sensorID); err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}
		// Refuse before reading the file: a backfill for an unknown or
		// out-of-boundary sensor_id creates reading_hourly rows that the
		// documented cleanup command cannot reach by sensor.
		if err := backfill.CheckSensorInBoundary(ctx, pool, sensorID); err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}
		f, err := os.Open(os.Args[3])
		if err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}
		buckets, report, err := backfill.ParseCSV(f, sensorID, cfg.Quality)
		f.Close()
		if err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}

		// Report what filtering dropped before anything is written. An archive
		// file that is mostly rejected must be visible at the moment of
		// import — once the surviving buckets are in reading_hourly there is
		// no column recording how much of the day they were derived from, and
		// nothing ever rewrites a historical bucket.
		slog.Log(ctx, report.Level(cfg.Backfill), "backfill parsed archive file",
			append([]any{"sensor_id", sensorID, "path", os.Args[3]}, report.LogAttrs()...)...)

		n, err := backfill.WriteBuckets(ctx, pool, buckets)
		if err != nil {
			slog.Error("backfill", "error", err)
			os.Exit(1)
		}
		slog.Info("backfill complete", "sensor_id", sensorID, "buckets", n)

	case "import-areas":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: airbg import-areas <path.geojson> <city|oblast|neighbourhood|country>")
			os.Exit(2)
		}
		n, err := area.Import(ctx, pool, os.Args[2], os.Args[3])
		if err != nil {
			slog.Error("import areas", "error", err)
			os.Exit(1)
		}
		assigned, revoked, err := area.AssignSensors(ctx, pool, cfg.Database.StatementTimeouts.Assign)
		if err != nil {
			slog.Error("assign sensors", "error", err)
			os.Exit(1)
		}
		// revoked is reported because re-importing a *smaller* boundary
		// legitimately withdraws memberships, and an operator who did not
		// intend to shrink anything should see a non-zero count here rather
		// than discover it from a Phase 2 map with sensors missing.
		slog.Info("areas imported", "areas", n, "assignments", assigned, "revoked", revoked)

	// The ingest loop drains at most 24 buckets a tick and only ever walks
	// forward from the watermark, so raw readings older than the watermark —
	// a seeded database, or a host whose watermark starts at deploy time —
	// are never bucketed and are lost to raw retention. Idempotent.
	case "rollup":
		n, err := store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Operator).RollupAll(ctx)
		if err != nil {
			slog.Error("rollup", "error", err)
			os.Exit(1)
		}
		slog.Info("rollup complete", "buckets", n)

	// One-time (or re-runnable) load from a JSON snapshot, for a host that
	// cannot reach Cloudflare's API directly, or to seed history predating
	// this job. See internal/upstream/cloudflare/README.md, "Seeding".
	case "seed-visitor-daily":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: airbg seed-visitor-daily <path.json>")
			os.Exit(2)
		}
		f, err := os.Open(os.Args[2])
		if err != nil {
			slog.Error("seed visitor daily", "error", err)
			os.Exit(1)
		}
		points, err := cloudflare.ParseSeedFile(f)
		f.Close()
		if err != nil {
			slog.Error("seed visitor daily", "error", err)
			os.Exit(1)
		}
		rows := make([]store.VisitorDaily, len(points))
		fetchedAt := time.Now().UTC()
		for i, p := range points {
			rows[i] = store.VisitorDaily{Day: p.Date, Uniques: p.Uniques, Requests: p.Requests, PageViews: p.PageViews, FetchedAt: fetchedAt}
		}
		n, err := store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Operator).SeedVisitorDaily(ctx, rows)
		if err != nil {
			slog.Error("seed visitor daily", "error", err)
			os.Exit(1)
		}
		slog.Info("seed visitor daily complete", "path", os.Args[2], "rows", n)

	// Forced refresh of the bathing-water layer, ignoring refresh_interval.
	case "import-sea":
		seaCollector := bathing.NewCollector(cfg.Sea, store.New(pool, cfg.Store, cfg.Database.StatementTimeouts.Operator))
		seaCollector.SetSupplement(loadSupplement(cfg))
		st, err := seaCollector.RunOnce(ctx)
		if err != nil {
			slog.Error("import sea", "error", err)
			os.Exit(1)
		}
		slog.Info("import sea complete", "sites", st.Sites, "classes", st.Classes, "samples", st.Samples,
			"retired", st.Skipped.Retired, "invalid", st.Skipped.Invalid, "orphan", st.Skipped.Orphan,
			"supplement_applied", st.Supplement.Applied)

	case "purge-outside-boundary":
		// Deliberately a separate, operator-invoked step (task-17 review
		// finding 4) — never run automatically from import-areas or collect.
		// Deleting stored sensors must always be a decision a human makes on
		// purpose.
		result, err := area.PurgeOutsideBoundary(ctx, pool, cfg.Database.StatementTimeouts.Operator)
		if err != nil {
			slog.Error("purge outside boundary", "error", err)
			os.Exit(1)
		}
		slog.Info("purge outside boundary complete",
			"sensors_removed", result.SensorsRemoved,
			"readings_removed", result.ReadingsRemoved,
			"hourly_rows_removed", result.HourlyRowsRemoved,
			// Orphans have a different cause from foreign sensors — readings
			// written for a sensor_id that has no sensor row — so they are
			// reported separately rather than folded into the totals above.
			"orphan_readings_removed", result.OrphanRawRows,
			"orphan_hourly_rows_removed", result.OrphanHourlyRows)

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
}

// serveCommand owns the two pools' lifetimes. Separate from runServe so the
// deferred Close calls actually run: main's error paths call os.Exit, which
// skips defers.
// seededHistory builds the stuck-check history and seeds it from the reading
// table. A failed seed is logged and the history starts empty, as it used to.
func seededHistory(ctx context.Context, st *store.Store, q config.Quality) *quality.History {
	h := quality.NewHistory(q.HistoryDepth)
	n, err := st.SeedHistory(ctx, h, q.HistorySeedWindow, q.HistoryDepth)
	if err != nil {
		slog.Warn("could not seed stuck history; starting empty", "err", err)
		return h
	}
	slog.Info("stuck history seeded", "readings", n)
	return h
}

func serveCommand(ctx context.Context, cfg config.Config) error {
	apiPool, collectorPool, err := db.OpenPair(ctx, cfg.Database)
	if err != nil {
		return err
	}
	defer apiPool.Close()
	defer collectorPool.Close()

	return runServe(ctx, cfg, apiPool, collectorPool)
}

func runServe(ctx context.Context, cfg config.Config, apiPool, collectorPool *pgxpool.Pool) error {
	// Fail closed on the exact regression this function exists to prevent. A
	// future refactor that collapses the two pools back into one would otherwise
	// reintroduce the starvation silently — nothing about it is visible in a
	// response, a metric, or a log line until the poll cycle happens to overlap
	// with traffic.
	if apiPool == collectorPool {
		return errors.New("serve: the request and collector pools are the same pool; " +
			"the collector holds connections for up to " + cfg.Database.StatementTimeouts.Assign.String() +
			" per cycle and would starve request handlers (see db.OpenPair)")
	}

	log := slog.Default()

	// Request handlers get the API pool. The collector and the snapshot
	// publisher get the collector pool: building a snapshot is background work
	// that queries every area, so it belongs on the side of the bulkhead that is
	// allowed to be slow.
	apiStore := store.New(apiPool, cfg.Store, cfg.Database.StatementTimeouts.Series)
	collectorStore := store.New(collectorPool, cfg.Store, cfg.Database.StatementTimeouts.Series)

	windCfg := config.Wind{}
	if cfg.Wind.Enabled {
		windCfg = cfg.Wind
	}
	var holderOpts []snapshot.HolderOption
	if cfg.Pollen.Enabled {
		holderOpts = append(holderOpts, snapshot.WithPollen(cfg.Pollen))
	}
	holder := snapshot.NewHolder(cfg.Series, windCfg, holderOpts...)
	pub := server.NewPublisher(collectorStore, holder, log)

	cat, err := i18n.LoadWithOverrides(cfg.I18n.Dir)
	if err != nil {
		return err
	}

	// Build once at startup so the first visitor is not met with a 503 for a
	// whole poll interval. A failure here is logged, not fatal: the process
	// still serves "data is not ready yet" and the next cycle fixes it.
	if err := pub.Publish(ctx, time.Now().UTC()); err != nil {
		log.Error("initial snapshot build failed; starting with no data", "error", err)
	}

	// One line, so a developer who ran `go run ./cmd/airbg` without building the
	// frontend discovers it in one second rather than wondering why the map is
	// missing. The no-manifest path is a supported mode, not an error — hence
	// Info, not Warn.
	if assets, found := web.LoadAssets(); found {
		log.Info("assets", "state", "loaded", "script", assets.Script("main"))
	} else {
		log.Info("assets", "state", "no manifest — serving without islands (run 'npm run build' in web/)")
	}

	// Built here, in main, rather than inside server.New: this is the one place
	// the configured basemap host reaches the CSP, and it keeps the server
	// package from needing to know how a policy is assembled.
	sup := loadSupplement(cfg)
	srv, err := server.New(server.Options{
		Config:        cfg,
		Catalogue:     cat,
		Snapshots:     holder,
		Store:         apiStore,
		Publisher:     pub,
		SeaSupplement: seaSupplementMeta(sup),
		Logger:        log,
	})
	if err != nil {
		return err
	}

	// Same construction as the existing "collect" case — one poller, not two.
	ing := ingest.New(
		upstream.New(cfg.Upstream),
		collectorStore,
		seededHistory(ctx, collectorStore, cfg.Quality),
		quality.NewScorer(cfg.Quality),
		cfg.Database.StatementTimeouts.Assign,
		cfg.Upstream.Countries,
	)
	ing.SetSnapshotPublisher(pub)

	// The poller and the server share one process because the snapshot lives in
	// this process's memory: a separately deployed collector could fill the
	// database but could never swap the pointer this server reads.
	pollCtx, stopPolling := context.WithCancel(ctx)
	defer stopPolling()

	polled := make(chan struct{})
	go func() {
		defer close(polled)
		ing.Loop(pollCtx, cfg.Upstream.PollInterval) // returns when pollCtx is cancelled
	}()

	// A second, slower loop: the met model publishes hourly, so tying wind to
	// the five-minute ingest cycle would be twelve requests for one new answer.
	// It shares the collector pool, not the API pool. See docs/wind-overlay.md.
	windDone := make(chan struct{})
	if cfg.Wind.Enabled {
		wc := wind.NewCollector(cfg.Wind, collectorStore)
		go func() {
			defer close(windDone)
			wc.Loop(pollCtx)
		}()
	} else {
		close(windDone)
	}

	// Runs at cfg.Pollen.RunAtUTC, sharing the collector pool.
	pollenDone := make(chan struct{})
	if cfg.Pollen.Enabled {
		pc := pollen.NewCollector(cfg.Pollen, collectorStore)
		go func() {
			defer close(pollenDone)
			pc.Loop(pollCtx)
		}()
	} else {
		close(pollenDone)
	}

	// Runs on cfg.EEA.PollInterval, sharing the collector pool.
	eeaDone := make(chan struct{})
	if cfg.EEA.Enabled {
		ec := eea.NewCollector(cfg.EEA, collectorStore, quality.NewScorer(cfg.Quality))
		go func() {
			defer close(eeaDone)
			ec.Loop(pollCtx)
		}()
	} else {
		close(eeaDone)
	}

	// Runs on cfg.Cloudflare.PollInterval, sharing the collector pool. With no
	// AIRBG_CF_ANALYTICS_TOKEN set, Loop logs once and returns immediately —
	// see internal/upstream/cloudflare/README.md.
	cfDone := make(chan struct{})
	if cfg.Cloudflare.Enabled {
		cc := cloudflare.NewCollector(cfg.Cloudflare, config.Getenv(cloudflare.TokenEnv), collectorStore)
		go func() {
			defer close(cfDone)
			cc.Loop(pollCtx)
		}()
	} else {
		close(cfDone)
	}

	// Weekly EEA bathing-water import; see internal/upstream/bathing/README.md.
	seaDone := make(chan struct{})
	if cfg.Sea.Enabled {
		sc := bathing.NewCollector(cfg.Sea, collectorStore)
		sc.SetSupplement(sup)
		sc.SetEditionWatch(editionWatch(cfg))
		go func() {
			defer close(seaDone)
			sc.Loop(pollCtx)
		}()
	} else {
		close(seaDone)
	}

	err = srv.Run(ctx)

	// Stop the poller and wait for it, so the process does not exit with a
	// half-written cycle in flight.
	stopPolling()
	<-polled
	<-windDone
	<-pollenDone
	<-eeaDone
	<-cfDone
	<-seaDone
	return err
}
