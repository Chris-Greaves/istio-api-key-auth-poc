// Command telemetrygen generates a sustained, rate-controlled stream of
// traffic against a running instance of the service, to exercise its
// telemetry-emitting surfaces (the Check Service's /authz/check endpoint and
// the management API) without standing up a real Istio mesh. See
// ./README.md.
//
// Run via `go run ./tools/telemetrygen [flags]`. It's a dev-only utility:
// not built into the service's Docker image, and not referenced by
// docker-compose.yml.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// poolKeyOwner and dedicatedExpiredKeyOwner label keys this tool creates, so
// they're identifiable (e.g. via the Web UI) as telemetrygen's own throwaway
// keys rather than real consumers.
const (
	poolKeyOwner              = "telemetrygen"
	dedicatedExpiredKeyOwner  = "telemetrygen-expired"
	dedicatedExpiredKeyMaxAge = time.Hour

	// bootstrapConcurrency bounds how many pool keys are created in parallel
	// at startup, so a large --pool-size doesn't open an unbounded number of
	// simultaneous connections to the service.
	bootstrapConcurrency = 10
)

type config struct {
	baseURL       string
	checkRate     float64
	checkInterval time.Duration
	duration      time.Duration
	poolSize      int
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("telemetrygen", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://localhost:8080", "base URL of the running service")
	checkRate := fs.Float64("check-rate", 10, "target rate, in requests/sec, for the check-endpoint stream")
	duration := fs.Duration("duration", 0, "optional total run duration (e.g. 5m); runs until interrupted if unset")
	poolSize := fs.Int("pool-size", 20, "number of valid API keys to bootstrap into the shared pool")

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}

	if *checkRate <= 0 {
		return config{}, fmt.Errorf("--check-rate must be positive, got %v", *checkRate)
	}
	if *poolSize <= 0 {
		return config{}, fmt.Errorf("--pool-size must be positive, got %v", *poolSize)
	}
	if *duration < 0 {
		return config{}, fmt.Errorf("--duration must not be negative, got %v", *duration)
	}

	checkInterval := time.Duration(float64(time.Second) / *checkRate)
	if checkInterval <= 0 {
		return config{}, fmt.Errorf("--check-rate is too high: %v requests/sec leaves no measurable interval between requests", *checkRate)
	}

	return config{
		baseURL:       *baseURL,
		checkRate:     *checkRate,
		checkInterval: checkInterval,
		duration:      *duration,
		poolSize:      *poolSize,
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		logger.Error("invalid flags", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.duration)
		defer cancel()
	}

	client := newAPIClient(cfg.baseURL)

	pool, err := bootstrap(ctx, client, cfg.poolSize)
	if err != nil {
		logger.Error("bootstrap failed", "base_url", cfg.baseURL, "error", err)
		os.Exit(1)
	}
	logger.Info("bootstrap complete", "base_url", cfg.baseURL, "pool_size", cfg.poolSize)

	logger.Info("check stream starting", "rate_per_sec", cfg.checkRate)
	runCheckStream(ctx, logger, client, pool, cfg.checkInterval)
	logger.Info("check stream stopped")
}

// bootstrap stands up the shared pool of valid API Keys plus one dedicated
// key with an expires_at already in the past, held outside the pool for a
// later ticket's "expired" check-mix scenario. Any failure here (e.g. an
// unreachable base URL) aborts the tool immediately, since it usually means
// telemetry would otherwise be silently generated at zero volume.
func bootstrap(ctx context.Context, client *apiClient, poolSize int) (*Pool, error) {
	pool := NewPool()

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(bootstrapConcurrency)
	for i := range poolSize {
		g.Go(func() error {
			key, err := client.createKey(gctx, poolKeyOwner, nil)
			if err != nil {
				return fmt.Errorf("creating pool key %d/%d: %w", i+1, poolSize, err)
			}
			pool.Add(key)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(-dedicatedExpiredKeyMaxAge)
	if _, err := client.createKey(ctx, dedicatedExpiredKeyOwner, &expiresAt); err != nil {
		return nil, fmt.Errorf("creating dedicated expired key: %w", err)
	}

	return pool, nil
}

// runCheckStream sends requests to /authz/check at the given interval until
// ctx is cancelled, each drawing a valid key from pool and presenting it via
// X-API-Key. The rate is open-loop — ticker-driven against the target
// interval rather than tied to response latency — so a slow or hanging
// response never throttles the send rate; each request runs in its own
// goroutine.
func runCheckStream(ctx context.Context, logger *slog.Logger, client *apiClient, pool *Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	rng := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(os.Getpid())))
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case <-ticker.C:
			key, ok := pool.Random(rng)
			if !ok {
				logger.Warn("pool is empty, skipping check request")
				continue
			}

			wg.Add(1)
			go func(key string) {
				defer wg.Done()
				status, err := client.check(ctx, key)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					logger.Warn("check request failed", "error", err)
					return
				}
				if status != 200 {
					logger.Warn("unexpected check response for a valid key", "status", status)
				}
			}(key)
		}
	}
}
