package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/your-org/ai-transit-hub/internal/logger"
)

// Config holds all configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	App      AppConfig
	Log      LogConfig
	Storage  StorageConfig
	Agent    AgentConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	Charset  string
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret string
	Expiry time.Duration
}

type AppConfig struct {
	Name string
	Env  string
}

type LogConfig struct {
	Path string
}

type StorageConfig struct {
	// SkillPath SKILL.md 技能包附件的磁盘存储目录
	SkillPath string
	// AgentTaskPath 虚拟 Agent 任务工作区根目录（产出资产按 task_id 归档）
	AgentTaskPath string
}

// PyEnvConfig 任务级执行环境隔离配置（AGENT_PY_ENV_MODE=venv 时启用）：
// 每个任务在专用目录构建独立 venv（uv 引导 + 托管 CPython）与 Node 环境
// （宿主 npm 安装 npm: 依赖到任务目录），沙箱内只读挂载
type PyEnvConfig struct {
	// Mode system=宿主系统 python3（默认，历史行为）；venv=按任务构建隔离虚拟环境
	Mode string
	// PythonVersion 托管 CPython 版本（uv python install，默认 3.12）
	PythonVersion string
	// UVVersion uv 工具链版本（钉住，避免上游变更影响构建可复现性）
	UVVersion string
	// UVDownloadURL 覆盖 uv 下载地址（自建镜像；默认官方 releases.astral.sh）
	UVDownloadURL string
	// UVSHA256 指定 uv 压缩包 sha256（自建镜像必填；默认自动拉取官方 .sha256 旁挂文件）
	UVSHA256 string
	// NodeVersion 自托管 Node.js 版本（宿主 PATH 无 node/npm 时自动下载引导，默认 22.20.0）
	NodeVersion string
	// NodeDownloadURL 覆盖 Node.js 发行包下载地址（自建镜像；默认官方 nodejs.org/dist）
	NodeDownloadURL string
	// NodeSHA256 指定 Node.js 发行包 sha256（自建镜像必填；默认拉取官方 SHASUMS256.txt 对应行）
	NodeSHA256 string
	// IndexURL PyPI 兼容索引（UV_DEFAULT_INDEX；默认官方 pypi.org）
	IndexURL string
	// NpmRegistry npm registry（npm_config_registry；默认官方 registry.npmjs.org）
	NpmRegistry string
	// PythonInstallMirror CPython 独立构建镜像（UV_PYTHON_INSTALL_MIRROR）
	PythonInstallMirror string
	// EnvRoot 任务执行环境根目录（venv 与任务级 node_modules 的公共父目录）；空=派生为 <AgentTaskPath>/_pyenvs
	EnvRoot string
	// InstallTimeout 单任务环境构建总超时（uv 引导 + 解释器 + 依赖安装，以及 npm install）
	InstallTimeout time.Duration
	// WheelsOnly 仅允许 wheel 安装（禁 sdist，杜绝安装期任意代码执行；默认 true）
	WheelsOnly bool
	// CleanupTTL TTL 清理循环的兜底阈值（默认 7 天；0=不清理）。
	// 正常路径任务结束即删除该任务 venv（RunTask defer RemoveTaskVenv），仅兜底崩溃孤儿
	CleanupTTL time.Duration
}

// SandboxConfig 脚本执行沙箱配置：后端抽象的自适应链选择策略。
// 链顺序（auto 模式）：darwin=seatbelt / linux=bwrap → docker（配置了镜像才参与）→ plain（默认参与作兜底）
type SandboxConfig struct {
	// Backend 后端选择策略：auto（默认，按宿主环境构造候选链逐个探测）或显式指定
	// seatbelt/bwrap/docker/plain（链仅含该后端，不满足时 fail-closed 不降级）
	Backend string
	// DockerImage 容器沙箱执行镜像（如 python:3.12-slim）；空=docker 后端不参与链。
	// 镜像内需自带脚本所需解释器/依赖（跨环境一致性由镜像保证）
	DockerImage string
	// AllowPlain 是否允许无沙箱直跑作为链末位兜底（默认 true：全部沙箱后端不可用时直跑，
	// 仅保留超时与输出截断，保证任务不中断；安全优先部署可显式设为 false——
	// 此时无可用后端将拒绝执行而非降级）
	AllowPlain bool
}

// AgentConfig 虚拟 Agent 运行时配置
type AgentConfig struct {
	// AllowWorkspaceScript 是否允许 execute_script 执行任务工作区内的自建脚本。
	// 默认关闭；开启后运行时仍要求沙箱可用（自适应链存在可用后端），否则一律拒绝执行
	AllowWorkspaceScript bool
	// RouterModel 需求→SKILL 语义路由使用的默认模型（provider/model，/v1/agent/dispatch）。
	// 兼作未显式指定 model 时的兜底执行模型；为空时 dispatch 的语义路由不可用，
	// 调用方必须显式指定 skills（及 model）
	RouterModel string
	// PyEnv 任务执行环境隔离配置（任务级 venv / 托管解释器 / uv 工具链 / 任务级 node_modules）
	PyEnv PyEnvConfig
	// Sandbox 脚本执行沙箱配置（后端选择策略 / 容器镜像 / 直跑放行）
	Sandbox SandboxConfig
}

// Load loads configuration from environment variables
func Load() *Config {
	// Load .env file if exists
	// 优先从 DOTENV_PATH 加载，如果不存在则尝试加载默认的 .env
	if dotenvPath := os.Getenv("DOTENV_PATH"); dotenvPath != "" {
		_ = godotenv.Load(dotenvPath)
	} else {
		_ = godotenv.Load()
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnv("DB_PORT", "3306"),
			Database: getEnv("DB_DATABASE", "ai_transit_hub"),
			Username: getEnv("DB_USERNAME", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			Charset:  getEnv("DB_CHARSET", "utf8mb4"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "127.0.0.1"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "your-secret-key-here"),
			Expiry: getEnvAsDuration("JWT_EXPIRY", 24*time.Hour),
		},
		App: AppConfig{
			Name: getEnv("APP_NAME", "AI Transit Hub"),
			Env:  getEnv("APP_ENV", "development"),
		},
		Log: LogConfig{
			Path: getEnv("LOG_PATH", "./logs"),
		},
		Storage: StorageConfig{
			SkillPath:     getEnv("SKILL_STORAGE_PATH", "./storage/skills"),
			AgentTaskPath: getEnv("AGENT_TASK_STORAGE_PATH", "./storage/agent_tasks"),
		},
		Agent: AgentConfig{
			AllowWorkspaceScript: getEnvAsBool("AGENT_ALLOW_WORKSPACE_SCRIPT", false),
			RouterModel:          getEnv("AGENT_ROUTER_MODEL", ""),
			PyEnv: PyEnvConfig{
				Mode:                getEnv("AGENT_PY_ENV_MODE", "system"),
				PythonVersion:       getEnv("AGENT_PY_VERSION", "3.12"),
				UVVersion:           getEnv("AGENT_UV_VERSION", "0.13.5"),
				UVDownloadURL:       getEnv("AGENT_UV_DOWNLOAD_URL", ""),
				UVSHA256:            getEnv("AGENT_UV_SHA256", ""),
				NodeVersion:         getEnv("AGENT_NODE_VERSION", "22.20.0"),
				NodeDownloadURL:     getEnv("AGENT_NODE_DOWNLOAD_URL", ""),
				NodeSHA256:          getEnv("AGENT_NODE_SHA256", ""),
				IndexURL:            getEnv("AGENT_PYPI_INDEX_URL", ""),
				NpmRegistry:         getEnv("AGENT_NPM_REGISTRY_URL", ""),
				PythonInstallMirror: getEnv("AGENT_PYTHON_INSTALL_MIRROR", ""),
				EnvRoot:             getEnv("AGENT_PY_ENV_ROOT", ""),
				InstallTimeout:      getEnvAsDuration("AGENT_PY_INSTALL_TIMEOUT", 5*time.Minute),
				WheelsOnly:          getEnvAsBool("AGENT_PY_WHEELS_ONLY", true),
				CleanupTTL:          getEnvAsDuration("AGENT_PY_ENV_TTL", 7*24*time.Hour),
			},
			Sandbox: SandboxConfig{
				Backend:     getEnv("AGENT_SANDBOX_BACKEND", "auto"),
				DockerImage: getEnv("AGENT_SANDBOX_IMAGE", ""),
				AllowPlain:  getEnvAsBool("AGENT_SANDBOX_ALLOW_PLAIN", true),
			},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
		logger.Debug("Warning: %s is not a valid bool, using default", key)
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		logger.Debug("Warning: %s is not a valid integer, using default", key)
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		// 尝试直接解析 Go 格式
		if duration, err := time.ParseDuration(value); err == nil {
			logger.Debug("Config: %s = %v", key, duration)
			return duration
		}

		// 兼容其他格式
		hours, err := strconv.ParseFloat(value, 64)
		if err == nil {
			duration := time.Duration(hours * float64(time.Hour))
			logger.Debug("Config: %s = %v (hours format)", key, duration)
			return duration
		}

		// 数字解析为小时
		hoursInt, err := strconv.Atoi(value)
		if err == nil {
			duration := time.Duration(hoursInt) * time.Hour
			logger.Debug("Config: %s = %v (hours format)", key, duration)
			return duration
		}

		logger.Debug("Warning: %s is not a valid duration, using default", key)
	}
	logger.Debug("Config: %s = %v (default)", key, defaultValue)
	return defaultValue
}
