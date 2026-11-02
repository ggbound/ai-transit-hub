package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/your-org/ai-transit-hub/internal/api"
	"github.com/your-org/ai-transit-hub/internal/config"
	"github.com/your-org/ai-transit-hub/internal/logger"
	"github.com/your-org/ai-transit-hub/internal/repository"
	"github.com/your-org/ai-transit-hub/internal/scheduler"
)

// 命令行参数
var (
	port        = flag.String("port", "", "服务器端口 (如: 8080)")
	configPath  = flag.String("config", "", ".env配置文件路径 (如: .env.production)")
	mode        = flag.String("mode", "", "Gin运行模式: debug/release/test")
	dbHost      = flag.String("db-host", "", "数据库主机地址")
	dbPort      = flag.String("db-port", "", "数据库端口")
	dbUser      = flag.String("db-user", "", "数据库用户名")
	dbPass      = flag.String("db-pass", "", "数据库密码")
	dbName      = flag.String("db-name", "", "数据库名")
	showHelp    = flag.Bool("help", false, "显示帮助信息")
	showVersion = flag.Bool("version", false, "显示版本信息")
)

const version = "v1.0.0"

func main() {
	flag.Parse()

	// 显示帮助
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// 显示版本
	if *showVersion {
		fmt.Printf("AI Transit Hub Server %s\n", version)
		os.Exit(0)
	}

	// 如果指定了.env文件路径，先设置环境变量
	if *configPath != "" {
		os.Setenv("DOTENV_PATH", *configPath)
	}

	// 1. 首先初始化 logger（它会自己加载 .env 文件）
	if err := logger.Init(); err != nil {
		panic(err)
	}

	// 2. 然后再加载配置
	cfg := config.Load()

	// 命令行参数覆盖配置文件
	if *port != "" {
		cfg.Server.Port = *port
	}
	if *mode != "" {
		cfg.Server.Mode = *mode
	}

	// 输出配置信息（JWT 密钥部分隐藏）
	logger.Info("Loaded configuration:")
	logger.Info("  App: %s (%s)", cfg.App.Name, cfg.App.Env)
	logger.Info("  Server: %s, mode: %s", cfg.Server.Port, cfg.Server.Mode)
	logger.Info("  JWT expiry: %v", cfg.JWT.Expiry)
	logger.Info("  Database: %s:%s/%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)

	// 数据库配置覆盖
	if *dbHost != "" {
		cfg.Database.Host = *dbHost
	}
	if *dbPort != "" {
		cfg.Database.Port = *dbPort
	}
	if *dbUser != "" {
		cfg.Database.Username = *dbUser
	}
	if *dbPass != "" {
		cfg.Database.Password = *dbPass
	}
	if *dbName != "" {
		cfg.Database.Database = *dbName
	}

	// Initialize database
	if err := repository.InitDatabase(cfg); err != nil {
		logger.Fatal("Failed to initialize database: %v", err)
	}

	// Auto migrate database
	if err := repository.AutoMigrate(); err != nil {
		logger.Fatal("Failed to migrate database: %v", err)
	}

	logger.Info("Database initialized successfully")

	// 启动定时任务调度器
	sched := scheduler.New()
	sched.RegisterJob(scheduler.NewVideoTaskPoller())
	sched.Start()

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create Gin engine
	r := gin.Default()

	// Setup routes
	api.SetupRoutes(r, cfg)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"app":    cfg.App.Name,
		})
	})

	// Prometheus 指标端点（供 Prometheus/Grafana 抓取）
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Start server
	// 不设置 ReadTimeout/WriteTimeout：避免长连接（SSE 流式）被服务端超时截断，
	// 只限制读 header 与空闲连接，慢速攻击由反向代理层兜底
	addr := ":" + cfg.Server.Port
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号（SIGINT/SIGTERM），执行优雅停机：
	// 先停止接收新请求，等待在途请求完成（最长 30s），再停调度器退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Received shutdown signal, draining in-flight requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown: %v", err)
	}

	sched.Stop()
	logger.Info("Server exited")
}

func printUsage() {
	fmt.Println("AI Transit Hub Server - 命令行启动工具")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  go run cmd/server/main.go [选项]")
	fmt.Println("  ./server [选项]")
	fmt.Println()
	fmt.Println("选项:")
	fmt.Println("  -port string       服务器端口 (默认: 8080)")
	fmt.Println("  -config string     .env配置文件路径 (默认: 加载项目根目录的.env)")
	fmt.Println("  -mode string       Gin运行模式: debug/release/test (默认: debug)")
	fmt.Println("  -db-host string    数据库主机地址")
	fmt.Println("  -db-port string    数据库端口")
	fmt.Println("  -db-user string    数据库用户名")
	fmt.Println("  -db-pass string    数据库密码")
	fmt.Println("  -db-name string    数据库名")
	fmt.Println("  -help              显示帮助信息")
	fmt.Println("  -version           显示版本信息")
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  # 使用默认配置启动")
	fmt.Println("  go run cmd/server/main.go")
	fmt.Println()
	fmt.Println("  # 指定端口启动")
	fmt.Println("  go run cmd/server/main.go -port 9090")
	fmt.Println()
	fmt.Println("  # 指定.env文件启动")
	fmt.Println("  go run cmd/server/main.go -config .env.production")
	fmt.Println()
	fmt.Println("  # 指定数据库启动")
	fmt.Println("  go run cmd/server/main.go -db-host localhost -db-port 3306 -db-user root -db-pass 123456 -db-name ai_hub")
	fmt.Println()
	fmt.Println("  # 生产模式启动")
	fmt.Println("  go run cmd/server/main.go -mode release -port 80")
}
