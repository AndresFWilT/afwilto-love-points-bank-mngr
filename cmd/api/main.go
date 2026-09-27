// Command api es el composition root: el único lugar que conoce a todos los adaptadores
// y los ensambla. La flecha de dependencia apunta siempre hacia adentro.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	adapterhttp "github.com/andresfwilt/banco-de-puntitos/internal/adapter/in/http"
	adapterout "github.com/andresfwilt/banco-de-puntitos/internal/adapter/out"
	"github.com/andresfwilt/banco-de-puntitos/internal/adapter/out/postgres"
	"github.com/andresfwilt/banco-de-puntitos/internal/app/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("arranque falló", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.logLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- adaptadores de salida ---
	pool, err := newPool(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	db := postgres.NewDB(pool)
	entries := postgres.NewEntryRepository(db)
	members := postgres.NewMemberRepository(db)
	catalog := postgres.NewCatalogRepository(db)
	redemptions := postgres.NewRedemptionRepository(db)

	// --- aplicación ---
	clock := adapterout.SystemClock{}
	ids := adapterout.UUIDGenerator{}
	uc := adapterhttp.UseCases{
		Points:      service.New(entries, members, clock, ids),
		Catalog:     service.NewCatalog(catalog, clock, ids),
		Redemptions: service.NewRedemption(redemptions, catalog, clock, ids),
	}

	// --- adaptador de entrada ---
	router := adapterhttp.NewRouter(uc, cfg.corsOrigin)

	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		slog.Info("apagando servidor...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("api escuchando", "addr", cfg.httpAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("api detenida")
	return nil
}

func newPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}
