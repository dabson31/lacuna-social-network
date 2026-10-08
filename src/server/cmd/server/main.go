package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"social/src/server/internal/app"
	"social/src/server/internal/platform/config"
	"social/src/server/internal/platform/logger"
	"social/src/server/pkg/db/sqlite"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(fmt.Errorf("Error: Could not start the server; %v", err))
	}
}

func run() error {

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("Error: Could not load config.Load(); %v", err)
	}

	log := logger.New(cfg.Env)

	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("Error: Could not start the db; %v", err)
	}
	defer db.Close()

	// here, we're migrating the db schema before listening
	// else, a request will be made to a db thats incomplete, possibly resulting in errors
	if err := sqlite.Migrate(db, cfg.MigrationsPath, log); err != nil {
		return fmt.Errorf("Error: Could not migrate the db; %v", err)
	}

	handler, err := app.New(app.Dependencies{Config: cfg, DB: db, Log: log})
	if err != nil {
		return fmt.Errorf("build app: %w", err)
	}

	serve := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
		// this is to prevent against slowloris attacks, which is a type of ddos attack
		// the attack is meant to overwhelm a server by maintaining multiple concurrent HTTP connections
		// so if they send headers one byte at a time, they will be kicked out
		//! note: this occurs at the application layer of the OSI/TCP model, and according to CloudFlare,
		//! one of the ways to handle it is by using "Rate Limit incoming requests"
		ReadHeaderTimeout: 10 * time.Second,
		// bascially log out a user if the timeout takes longer than 2 mins
		IdleTimeout: 120 * time.Second,
		// instead of sending requests using the stderr, the errors will be logged by our logger instead
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	//! reference ctx: https://gobyexample.com/context
	//? ctx handles cancellation signals, request-scoped values across api and goroutines
	// okay so, a little ctrl + c gobbledygook over here:
	// ctrl + c in the the terminal is SIGINT (signal interrupt) and docker stop SIGTERM (signal terminate)
	// both cancel ctx. in docker's case, it sends a SIGTERM, waits 10 seconds, then sends SIGKILL
	// we handle this to gracefully handle a request in operation, instead of terminating mid-write
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)

	// so the goroutine
	go func() {
		log.Info("listening", "addr", serve.Addr, "env", cfg.Env)
		serveErr <- serve.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		// ctrl + c or docker stop
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return serve.Shutdown(shutdownCtx)
}
