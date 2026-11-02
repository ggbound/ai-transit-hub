package repository

import (
	"fmt"
	stdlog "log"
	"os"
	"strconv"
	"time"

	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDatabase initializes database connection
func InitDatabase(cfg *config.Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.Charset,
	)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		// 只在 Warn 及以上（慢查询/错误）打 SQL 日志：Info 级别会把每条 SQL 连同参数全量输出，
		// 高 QPS 下日志量巨大且拖慢请求；超过 200ms 视为慢查询
		Logger: gormlogger.New(stdlog.New(os.Stdout, "\r\n", stdlog.LstdFlags), gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// 连接池参数：可通过环境变量覆盖，默认值适合中小规模网关
	if sqlDB, dbErr := DB.DB(); dbErr == nil {
		maxOpen := envIntOrDefault("DB_MAX_OPEN_CONNS", 100)
		maxIdle := envIntOrDefault("DB_MAX_IDLE_CONNS", 20)
		connMaxLifetime := time.Duration(envIntOrDefault("DB_CONN_MAX_LIFETIME_MINUTES", 60)) * time.Minute
		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetConnMaxLifetime(connMaxLifetime)
		logger.Info("Database pool configured: max_open=%d max_idle=%d conn_max_lifetime=%v", maxOpen, maxIdle, connMaxLifetime)
	}

	logger.Info("Database connected successfully")
	return nil
}

// envIntOrDefault 读取整型环境变量，空/非法时用默认值
func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// AutoMigrate runs database migrations
func AutoMigrate() error {
	// Since database tables already exist from the original PHP project,
	// we can skip auto-migration to avoid schema conflicts
	logger.Info("Skipping AutoMigrate - using existing PHP database tables")
	return nil
}
