package store

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
	"github.com/pstrr/kronos/internal/config"
	"github.com/pstrr/kronos/internal/model"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database holds all database connections.
type Database struct {
	DB    *gorm.DB
	Redis *redis.Client
	flock *flock.Flock
}

// NewDatabase initializes the SQLite database (with optional MySQL and Redis),
// applies auto-migration, acquires a file lock, and performs startup cleanup.
func NewDatabase(cfg config.DatabaseConfig) (*Database, error) {
	db := &Database{}

	// --- SQLite ---
	dbPath := expandPath(cfg.SQLite.Path)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create sqlite dir: %w", err)
	}

	// File lock to prevent multi-instance.
	lockPath := dbPath + ".lock"
	fl := flock.New(lockPath)
	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire file lock %s: %w", lockPath, err)
	}
	if !locked {
		return nil, fmt.Errorf("another kronos instance is running (lock: %s)", lockPath)
	}
	db.flock = fl

	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fl.Unlock()
		return nil, fmt.Errorf("open sqlite %s: %w", dsn, err)
	}
	db.DB = gormDB

	// Auto-migrate all models.
	if err := gormDB.AutoMigrate(
		&model.User{},
		&model.Task{},
		&model.TaskRun{},
		&model.Notification{},
	); err != nil {
		fl.Unlock()
		return nil, fmt.Errorf("auto-migrate: %w", err)
	}
	slog.Info("database migrated", "path", dbPath)

	// Create composite index (user_id, enabled) if not exists.
	// GORM's tag-based composite index may not include Enabled properly for SQLite,
	// so we ensure it explicitly.
	gormDB.Exec("CREATE INDEX IF NOT EXISTS idx_user_enabled ON tasks(user_id, enabled)")

	// --- Optional MySQL ---
	if cfg.MySQL.Enabled {
		mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.MySQL.User, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)
		mysqlDB, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			slog.Warn("mysql connection failed, continuing with sqlite only", "error", err)
		} else {
			slog.Info("mysql connected", "host", cfg.MySQL.Host)
			_ = mysqlDB // Store for future use; primary DB remains SQLite.
		}
	}

	// --- Optional Redis ---
	if cfg.Redis.Enabled {
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		db.Redis = rdb
		slog.Info("redis client created", "addr", fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port))
	}

	// --- Startup cleanup: mark stale running task_runs as failed ---
	result := gormDB.Model(&model.TaskRun{}).
		Where("status = ?", "running").
		Updates(map[string]interface{}{
			"status": "failed",
			"error":  "marked as failed on startup (unclean shutdown)",
		})
	if result.RowsAffected > 0 {
		slog.Warn("startup cleanup: marked stale running task_runs as failed", "count", result.RowsAffected)
	}

	return db, nil
}

// Close releases all database resources.
func (d *Database) Close() error {
	if d.Redis != nil {
		d.Redis.Close()
	}
	if d.flock != nil {
		d.flock.Unlock()
	}
	if d.DB != nil {
		sqlDB, err := d.DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	return nil
}

// expandPath replaces ~ with the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
