package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 全局 logger 实例
var logger *log.Logger

// 当前日志级别
var currentLevel int

// 是否已设置日志级别
var levelSet bool = false

// 日志级别常量
const (
	LevelDebug = 0
	LevelInfo  = 1
	LevelWarn  = 2
	LevelError = 3
	LevelFatal = 4
)

// 日志级别名称映射
var levelNames = map[string]int{
	"DEBUG": LevelDebug,
	"INFO":  LevelInfo,
	"WARN":  LevelWarn,
	"ERROR": LevelError,
	"FATAL": LevelFatal,
}

// Config 日志配置
type Config struct {
	Level      string // 日志级别: DEBUG, INFO, WARN, ERROR
	Filename   string // 日志文件名
	MaxSize    int    // 单个日志文件最大大小(MB)
	MaxBackups int    // 保留的旧日志文件最大数量
	MaxAge     int    // 保留旧日志文件的最大天数
	Compress   bool   // 是否压缩/归档旧日志文件
}

// SetLevel 设置日志级别
func SetLevel(level string) {
	upperLevel := strings.ToUpper(level)
	if l, ok := levelNames[upperLevel]; ok {
		currentLevel = l
		levelSet = true
	} else {
		currentLevel = LevelInfo // 默认 INFO
		levelSet = true
	}
}

// getLevel 获取当前日志级别
func getLevel() int {
	if !levelSet {
		return LevelInfo // 默认 INFO
	}
	return currentLevel
}

// Init 初始化日志系统
func Init(config ...Config) error {
	// 加载环境变量
	if dotenvPath := os.Getenv("DOTENV_PATH"); dotenvPath != "" {
		_ = godotenv.Load(dotenvPath)
	} else {
		_ = godotenv.Load()
	}

	// 默认配置
	cfg := Config{
		Level:      os.Getenv("LOG_LEVEL"),
		Filename:   os.Getenv("LOG_FILE"),
		MaxSize:    100,
		MaxBackups: 30,
		MaxAge:     365,
		Compress:   true,
	}

	// 使用自定义配置
	if len(config) > 0 {
		cfg = config[0]
	}

	// 如果没有指定日志文件路径，设置默认路径
	if cfg.Filename == "" {
		wd, err := os.Getwd()
		if err != nil {
			wd = "./"
		}
		cfg.Filename = filepath.Join(wd, "logs", "app.log")
	}

	// 确保日志目录存在
	logDir := filepath.Dir(cfg.Filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	// 创建 lumberjack writer 用于日志轮转
	lumberjackLogger := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		LocalTime:  true, // 使用本地时间
	}

	// 同时输出到控制台和文件
	mw := io.MultiWriter(os.Stdout, lumberjackLogger)

	// 设置标准 log 的输出
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 创建 logger 实例
	logger = log.New(mw, "", log.LstdFlags|log.Lshortfile)

	// 设置日志级别
	SetLevel(cfg.Level)
	levelStr := "INFO"
	if getLevel() == LevelDebug {
		levelStr = "DEBUG"
	} else if getLevel() == LevelWarn {
		levelStr = "WARN"
	} else if getLevel() == LevelError {
		levelStr = "ERROR"
	}

	logger.Println("[INFO] Logger initialized successfully, log_file:", cfg.Filename, "level:", levelStr)

	return nil
}

// Debug 输出调试日志（仅在 DEBUG 级别输出）
func Debug(msg string, args ...interface{}) {
	if getLevel() > LevelDebug {
		return
	}
	if logger == nil {
		log.Printf("[DEBUG] "+msg, args...)
		return
	}
	logger.Printf("[DEBUG] "+msg, args...)
}

// Info 输出信息日志
func Info(msg string, args ...interface{}) {
	if logger == nil {
		log.Printf("[INFO] "+msg, args...)
		return
	}
	logger.Printf("[INFO] "+msg, args...)
}

// Warn 输出警告日志
func Warn(msg string, args ...interface{}) {
	if logger == nil {
		log.Printf("[WARN] "+msg, args...)
		return
	}
	logger.Printf("[WARN] "+msg, args...)
}

// Error 输出错误日志
func Error(msg string, args ...interface{}) {
	if logger == nil {
		log.Printf("[ERROR] "+msg, args...)
		return
	}
	logger.Printf("[ERROR] "+msg, args...)
}

// Fatal 输出致命错误并退出
func Fatal(msg string, args ...interface{}) {
	if logger == nil {
		log.Printf("[FATAL] "+msg, args...)
	} else {
		logger.Printf("[FATAL] "+msg, args...)
	}
	os.Exit(1)
}

// GetLogFilePath 获取当前日志文件路径
func GetLogFilePath() string {
	return ""
}
