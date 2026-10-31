package main

import (
	"context"
	"log"
	"myfit/backend/internal/api"
	"myfit/backend/internal/config"
	"myfit/backend/internal/store"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "memory-stats" {
		if e := memoryStats(); e != nil {
			log.Fatal(e)
		}
		return
	}
	// This command needs no database credentials or shell in the runtime image.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := http.Client{Timeout: 3 * time.Second}
		resp, e := client.Get("http://127.0.0.1:8080/health")
		if e != nil {
			os.Exit(1)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	c, e := config.Load(os.Getenv)
	if e != nil {
		log.Fatal(e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 {
		if os.Args[1] != "migrate" || len(os.Args) != 2 {
			log.Fatal("usage: myfit [migrate|healthcheck]")
		}
		migrationURL := c.MigrationDatabaseURL
		if c.Production {
			if migrationURL == "" {
				log.Fatal("MIGRATION_DATABASE_URL (direct Neon endpoint) is required")
			}
			if e = config.ValidateDatabaseURL(migrationURL); e != nil {
				log.Fatal(e)
			}
			u, _ := url.Parse(migrationURL)
			if strings.Contains(u.Hostname(), "-pooler") {
				log.Fatal("migrations require a direct, non-pooler endpoint")
			}
			runtimeURL, _ := url.Parse(c.DatabaseURL)
			if u.Path != runtimeURL.Path || u.Hostname() != strings.Replace(runtimeURL.Hostname(), "-pooler", "", 1) {
				log.Fatal("migration and runtime URLs must identify the same Neon endpoint and database")
			}
		} else if migrationURL == "" {
			migrationURL = c.DatabaseURL
		}
		migrationCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if e = store.Migrate(migrationCtx, migrationURL, c.MigrationsDir); e != nil {
			log.Fatal("migration failed: ", e)
		}
		log.Println("MyFit migrations complete")
		return
	}
	db, e := store.OpenConfigured(ctx, c)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Pool.Close()
	server := &http.Server{Addr: c.Addr, Handler: api.New(db, c.Origin, c.CookieSecure), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("MyFit API listening on %s", server.Addr)
	if e = server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
	<-shutdownDone // Wait for in-flight requests before closing the DB pool/exiting.
}
