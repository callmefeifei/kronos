package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pstrr/kronos/internal/api"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/executor"
	"github.com/pstrr/kronos/internal/model"
	"github.com/pstrr/kronos/internal/notifier"
	"github.com/pstrr/kronos/internal/scheduler"
	"github.com/pstrr/kronos/internal/store"
	"gorm.io/gorm"
)

// Server owns all Kronos components and manages the startup/shutdown lifecycle.
type Server struct {
	cfg       *config.Config
	db        *store.Database
	scheduler *scheduler.Scheduler
	httpSrv   *http.Server
}

// New creates a Server with the given configuration.
func New(cfg *config.Config) *Server {
	return &Server{cfg: cfg}
}

// Start performs the full startup sequence and blocks until a termination
// signal is received, then performs graceful shutdown.
func (s *Server) Start() error {
	slog.Info("starting kronos server")

	// --- 1. Init store (SQLite + optional MySQL/Redis, auto-migrate, file lock, stale cleanup) ---
	slog.Info("initializing database")
	db, err := store.NewDatabase(s.cfg.Database)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	s.db = db
	slog.Info("database initialized")

	// --- 2. Create stores ---
	userStore := store.NewUserStore(db.DB)
	taskStore := store.NewTaskStore(db.DB)
	taskRunStore := store.NewTaskRunStore(db.DB)
	notifStore := store.NewNotificationStore(db.DB)
	slog.Info("stores created")

	// --- 3. Create default admin user if not exists ---
	if err := s.ensureDefaultAdmin(userStore); err != nil {
		s.db.Close()
		return fmt.Errorf("ensure default admin: %w", err)
	}

	// --- 4. Init notifier ---
	slog.Info("initializing notifier")
	notifManager := notifier.NewManager(s.cfg.Notifier, notifStore)
	slog.Info("notifier initialized")

	// --- 5. Init executor ---
	slog.Info("initializing executor")
	runner := executor.NewRunner(taskStore, taskRunStore, notifManager)
	slog.Info("executor initialized")

	// --- 6. Init and start scheduler ---
	slog.Info("initializing scheduler")
	sched := scheduler.New(runner, taskStore, s.cfg.Scheduler.MaxConcurrent)
	if err := sched.Start(); err != nil {
		s.db.Close()
		return fmt.Errorf("start scheduler: %w", err)
	}
	s.scheduler = sched
	slog.Info("scheduler started")

	// --- 7. Create and start API server ---
	slog.Info("initializing api router")
	router := api.NewRouter(s.cfg, db.DB, userStore, taskStore, taskRunStore, notifStore, sched)

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start HTTP server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server listening", "addr", addr)
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	slog.Info("kronos server started successfully",
		"host", s.cfg.Server.Host,
		"port", s.cfg.Server.Port,
		"mode", s.cfg.Server.Mode,
	)

	// --- 8. Register signal handler and block ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, starting graceful shutdown", "signal", sig)
	case err := <-errCh:
		slog.Error("http server error, shutting down", "error", err)
		s.Shutdown()
		return fmt.Errorf("http server: %w", err)
	}

	// --- 9. Graceful shutdown ---
	s.Shutdown()
	return nil
}

// Shutdown performs an orderly shutdown: stop HTTP, stop scheduler, close DB.
func (s *Server) Shutdown() {
	slog.Info("shutting down kronos server")

	// Stop accepting new HTTP requests; wait up to 15s for in-flight.
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			slog.Error("http server shutdown error", "error", err)
		} else {
			slog.Info("http server stopped")
		}
	}

	// Stop scheduler (waits up to 30s internally for running tasks).
	if s.scheduler != nil {
		s.scheduler.Stop()
		slog.Info("scheduler stopped")
	}

	// Close database connections and release file lock.
	if s.db != nil {
		s.db.Close()
		slog.Info("database connections closed")
	}

	slog.Info("kronos server stopped")
}

// ensureDefaultAdmin creates a default admin user if no users exist.
func (s *Server) ensureDefaultAdmin(userStore *store.UserStore) error {
	_, err := userStore.GetByUsername("admin")
	if err == nil {
		// Admin user already exists.
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("check admin user: %w", err)
	}

	// Create default admin user.
	admin := &model.User{
		Username:    "admin",
		DisplayName: "Administrator",
		Role:        "admin",
		Status:      "active",
	}
	if err := admin.SetPassword("admin"); err != nil {
		return fmt.Errorf("hash default admin password: %w", err)
	}
	if err := userStore.Create(admin); err != nil {
		return fmt.Errorf("create default admin: %w", err)
	}

	slog.Warn("default admin user created (username: admin, password: admin) — please change the password immediately")
	return nil
}
