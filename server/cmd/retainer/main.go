// Command retainer runs the Retainer server.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"retainer/server/internal/authsvc"
	"retainer/server/internal/db"
	"retainer/server/internal/httpapi"
	"retainer/server/internal/purge"
	"retainer/server/internal/wsserver"
)

const shutdownTimeout = 10 * time.Second

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "retainer.db"
	}

	if len(os.Args) > 1 && os.Args[1] == "set-password" {
		runSetPassword(dbPath)
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	sqlDB, err := db.Open(dbPath)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8070"
	}
	staticDir := os.Getenv("STATIC_DIR")

	hub := wsserver.NewHub()
	hubStop := make(chan struct{})
	go hub.Run(hubStop)

	purgeCtx, cancelPurge := context.WithCancel(context.Background())
	go purge.Run(purgeCtx, sqlDB, time.Hour)

	handler := httpapi.NewRouter(sqlDB, staticDir, hub)
	srv := &http.Server{Addr: addr, Handler: handler}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr, "db", dbPath, "static", staticDir)
		serveErr <- srv.ListenAndServe()
	}()

	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining in-flight requests")
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancelShutdown()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown", "error", err)
		}
	}

	cancelPurge()
	close(hubStop)
	slog.Info("shutdown complete")
}

func runSetPassword(dbPath string) {
	sqlDB, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer sqlDB.Close()

	stdin := bufio.NewReader(os.Stdin)
	password := readPassword(stdin, "New password: ")
	confirm := readPassword(stdin, "Confirm password: ")
	if password != confirm {
		log.Fatal("passwords do not match")
	}
	if strings.TrimSpace(password) == "" {
		log.Fatal("password must not be empty")
	}

	if err := authsvc.SetPassword(sqlDB, password); err != nil {
		log.Fatalf("set password: %v", err)
	}
	fmt.Println("password set")
}

func readPassword(stdin *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			log.Fatalf("read password: %v", err)
		}
		return string(b)
	}
	// Non-TTY (e.g. piped input in scripts/tests) — fall back to a plain line read.
	line, err := stdin.ReadString('\n')
	if err != nil {
		log.Fatalf("read password: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}
