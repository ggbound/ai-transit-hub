# AI Transit Hub - AI 模型中转站

<p align="center">
  <img src="frontend/public/logo.svg" alt="AI Transit Hub Logo" width="200"/>
</p>

<p align="center">
  <strong>统一的 AI 模型调度管理系统</strong>
</p>

<p align="center">
  <a href="#-项目简介">项目简介</a> •
  <a href="#-功能特性">功能特性</a> •
  <a href="#️-技术架构">技术架构</a> •
  <a href="#-快速开始">快速开始</a> •
  <a href="#️-配置说明">配置说明</a> •
  <a href="#-api-文档">API 文档</a>
</p>

---

## 📖 项目简介

AI Transit Hub 是一个企业级 AI 模型中转站，提供统一的 AI 模型接入和管理平台。通过标准化的 API 接口，企业可以轻松接入和管理多家 AI 模型提供商（OpenAI、Anthropic、豆包、DeepSeek、通义千问等），实现模型的统一调度、权限管理和计费统计。

系统由 **Go 后端**（Gin + GORM）和 **Vue 3 前端**组成，包含员工使用平台与管理后台，并在传统对话补全之外提供**多模态生成（图像/视频）、向量嵌入、SKILL.md 技能包体系与虚拟 Agent 任务运行时**等能力。

### 核心优势

- 🎯 **标准化接口**：OpenAI / Anthropic / Responses 兼容 API，零成本迁移
- 🌈 **多模态能力**：文本对话、向量嵌入、图像生成、视频生成（异步任务）一站式接入
- 🤖 **虚拟 Agent**：SKILL.md 技能包 + 语义路由 + 工具循环执行，平台自身充当 Agent 宿主机
- 🔒 **安全沙箱**：脚本执行沙箱自适应链（seatbelt / bubblewrap / docker），任务级 Python/Node 环境隔离
- 📊 **多维度统计**：Token、费用、调用次数等多维度统计与用量/操作审计
- 🔐 **企业级安全**：JWT 认证、RBAC 角色控制、API 密钥管理、操作审计

---

## ✨ 功能特性

### API 兼容层（v1 兼容接口）

同时注册 **`/v1` 与 `/api/v1` 双前缀**（功能完全一致），使用用户 API Key 认证（`Authorization: Bearer` 或 `x-api-key`）：

| 接口 | 路径 | 说明 |
|------|------|------|
| 模型列表 | `GET /v1/models` | OpenAI 兼容，返回用户可用模型 |
| 对话补全 | `POST /v1/chat/completions` | 流式/非流式、工具调用（Function Calling） |
| 向量嵌入 | `POST /v1/embeddings` | OpenAI 兼容 embeddings |
| 图像生成 | `POST /v1/images/generations` | OpenAI 兼容图像生成 |
| 视频生成 | `POST /v1/videos/generations/tasks` | 创建异步视频生成任务 |
| 视频查询 | `GET /v1/videos/generations/tasks/:taskId` | 查询视频任务状态与结果 |
| Anthropic 消息 | `POST /v1/anthropic/messages` | Anthropic 兼容（`/v1/messages` 同义） |
| Anthropic 模型 | `GET /v1/anthropic/models` | Anthropic 兼容模型列表 |
| Responses | `POST /v1/responses` | OpenAI Responses API 兼容 |
| Agent 任务创建 | `POST /v1/agent/tasks` | 创建 Agent 任务（异步执行，立即返回 task_id） |
| Agent 语义路由 | `POST /v1/agent/dispatch` | 仅描述需求，由路由模型在技能目录上自动选择 SKILL 与模型 |
| Agent 任务查询 | `GET /v1/agent/tasks/:taskId` | 查询任务状态、步骤与结果 |
| Agent 资产下载 | `GET /v1/agent/tasks/:taskId/assets/*assetPath` | 下载任务产出的工作区资产 |

> v1 前缀下未匹配的路径会返回 OpenAI 风格错误信封（`invalid_request_error` / `unknown_url`）。

### 用户端功能

| 功能模块 | 特性 |
|---------|------|
| 🏠 工作台 | 个人用量概览、快捷入口 |
| 📝 对话功能 | 多轮会话管理、流式输出、Markdown 渲染、工具调用 |
| 🔑 API 密钥 | 个人密钥增删改查、启停用、配额控制 |
| 🧠 技能中心 | 查看管理端发布的 SKILL.md 技能包（只读） |
| 📚 API 文档 | 在线接口文档与调用示例 |
| 🧪 接口测试 | 在线 API 测试工具、历史记录 |
| 🖼️ 图像测试 | 图像生成在线体验 |
| 🎬 视频测试 | 视频生成任务创建、进度轮询、结果预览 |
| 📊 用量日志 | 个人调用用量、操作日志、统计图表 |
| 👤 个人资料 | 资料维护、头像上传、修改密码 |

### 后台管理功能

| 功能模块 | 特性 |
|---------|------|
| 📈 仪表盘 | 实时调用统计、费用监控、模型使用占比 |
| 🏢 提供商管理 | AI 提供商 CRUD、启停用、子模型维护、密钥分组 |
| 🤖 模型管理 | 模型配置、启停用、能力（流式/工具调用等）与价格设置 |
| 👥 用户管理 | 用户增改、启停用、角色与配额 |
| 🛠️ 工具管理 | 内置工具 CRUD、启停用、执行监控 |
| 🧠 技能包管理 | SKILL.md 技能包上传（zip ≤ 20MB）、下载、启停用 |
| 💰 计费管理 | 费率配置、用量记录、计费统计 |
| 📋 日志审计 | 全量调用用量与操作日志 |
| 🏆 排行榜配置 | 部门/公司级排行榜配置管理 |

### 虚拟 Agent 运行时

平台自身充当 Agent 宿主机（类比 Claude Code），以 HTTP API 形式对外提供任务执行能力：

- **SKILL.md 技能包**：遵循 Agent Skills 规范，zip 包上传，含 frontmatter 声明与附件资源
- **语义路由（dispatch）**：用户只描述需求，由路由模型在已启用技能目录上输出 `{skills, model, reason}` 决策，再异步执行
- **工具循环执行**：装载技能进 system prompt → 循环调用模型 → 执行工具（读取技能附件、写工作区资产），默认 100 轮上限防失控扣费
- **脚本沙箱**：自适应后端链 `seatbelt`(macOS) → `bwrap`(Linux) → `docker` → `plain` 兜底；统一边界为禁网、仅任务工作区可写、屏蔽宿主敏感路径，支持 fail-closed
- **执行环境隔离**：venv 模式下按任务构建隔离 Python 虚拟环境（uv 引导 + 托管 CPython，仅允许 wheel 安装）；Node 依赖装到任务级 node_modules，运行时三级自动确定
- **任务状态机**：`pending → running → completed/partial/failed`，任务 ID 前缀 `agt_`，产出资产按 task_id 归档可下载

### 异步任务与计费闭环

- 视频生成等异步接口创建任务后立即返回，内置**视频任务轮询器**（每 5 分钟）自动向上游查询终态并回填 Token 与费用，实现异步计费闭环

---

## 🏗️ 技术架构

### 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         外部 AI 模型                          │
│  OpenAI │ Anthropic │ 豆包 │ DeepSeek │ 通义千问 │ 智谱 │ 混元 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      AI 适配器层（多 Key 负载分摊）             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐        │
│  │  openai  │ │  claude  │ │  doubao  │ │   qwen   │ ...    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                          核心服务层                           │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐ │
│  │ v1 统一兼容管线  │  │ 虚拟 Agent 运行时│  │ 技能包/语义路由  │ │
│  └────────────────┘  └────────────────┘  └────────────────┘ │
│  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐ │
│  │ 脚本沙箱/执行环境│  │ 按需模型选型器  │  │ 计费记账服务    │ │
│  └────────────────┘  └────────────────┘  └────────────────┘ │
│  ┌────────────────┐  ┌────────────────┐                     │
│  │ 定时调度器       │  │ 操作审计服务    │                     │
│  │ (视频任务轮询)   │  │                │                     │
│  └────────────────┘  └────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────────┐   ┌─────────────────────────────┐
│        员工使用平台           │   │        后台管理系统          │
│  - 对话 / 会话管理           │   │  - 提供商 / 密钥管理          │
│  - API 密钥管理             │   │  - 模型 / 用户管理           │
│  - 接口测试(文本/图像/视频)   │   │  - 技能包 / 工具管理          │
│  - 用量统计 / 个人资料        │   │  - 计费 / 日志审计            │
└─────────────────────────────┘   └─────────────────────────────┘
```

### 技术栈

#### 后端 (Backend)

- **框架**: Go 1.25+（Gin）
- **ORM**: GORM + MySQL（复用既有表结构）
- **缓存**: Redis
- **认证**: JWT（golang-jwt/v5）
- **监控**: Prometheus（`/metrics`）
- **日志**: Lumberjack 滚动日志
- **定时任务**: 内置调度器（视频任务轮询）

#### 前端 (Frontend)

- **框架**: Vue 3.5 + Composition API
- **构建工具**: Vite 8
- **UI 组件库**: Arco Design Vue 2.58
- **状态管理**: Pinia
- **路由**: Vue Router 4
- **HTTP 客户端**: Axios
- **图表**: ECharts 6 + vue-echarts
- **Markdown 渲染**: Marked + Highlight.js

---

## 🚀 快速开始

### 环境要求

- **Go**: >= 1.25（以 `backend/go.mod` 为准）
- **Node.js**: >= 20.19（Vite 8 要求）
- **MySQL**: >= 5.7（推荐 8.0，字符集 utf8mb4）
- **Redis**: >= 5.0

### 数据库准备（重要）

系统**复用既有 PHP 项目的数据表结构，不自动建表**（启动时跳过 AutoMigrate）。请确保数据库中已存在历史表结构，并手动执行新增表的迁移 SQL：

```bash
mysql -u root -p ai_transit_hub < backend/migrations/2026_10_23_create_agent_tasks.sql
mysql -u root -p ai_transit_hub < backend/migrations/2026_10_23_create_skills_tables.sql
```

### 安装步骤

#### 1. 克隆代码

```bash
git clone https://github.com/ggbound/ai-transit-hub.git
cd ai-transit-hub
```

#### 2. 启动后端

```bash
cd backend

# 安装依赖
go mod download

# 复制并修改环境配置（数据库、Redis、JWT 等，详见 .env.example）
cp .env.example .env

# 启动开发服务器（默认端口 8080）
go run cmd/server/main.go
```

#### 3. 启动前端

```bash
cd ../frontend

# 安装依赖
npm install

# 配置环境变量：将 API 地址指向后端实际端口
cp .env.example .env
# .env 中设置：
# VITE_API_BASE_URL=http://localhost:8080/api

# 启动开发服务器（Vite 默认端口 5173）
npm run dev
```

> 仓库自带的 `frontend/.env.example` 默认指向 `http://localhost:8000/api`（历史 PHP 后端端口），本地联调请改为 Go 后端的 `8080` 端口。

#### 4. 访问系统

- **前端入口**: http://localhost:5173
- **管理后台**: http://localhost:5173/admin
- **后端 API**: http://localhost:8080/api
- **v1 兼容层**: http://localhost:8080/v1 或 http://localhost:8080/api/v1
- **健康检查**: http://localhost:8080/health
- **监控指标**: http://localhost:8080/metrics（Prometheus 格式）

---

## ⚙️ 配置说明

### 后端配置（backend/.env）

```env
# 服务配置
SERVER_PORT=8080
GIN_MODE=debug
LOG_PATH=./logs
LOG_LEVEL=INFO

# 数据库配置
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=ai_transit_hub
DB_USERNAME=root
DB_PASSWORD=
DB_CHARSET=utf8mb4

# Redis 配置
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT 配置
JWT_SECRET=your-secret-key-here-change-this-in-production
JWT_EXPIRY=24h

# 技能包存储
SKILL_STORAGE_PATH=./storage/skills

# Agent 任务工作区（产出资产按 task_id 归档）
AGENT_TASK_STORAGE_PATH=./storage/agent_tasks

# Agent 运行时
AGENT_ALLOW_WORKSPACE_SCRIPT=false      # 是否允许执行任务工作区内自建脚本
AGENT_ROUTER_MODEL=                     # dispatch 语义路由模型（provider/model）

# Agent Python 执行环境（venv 模式）
AGENT_PY_ENV_MODE=system                # system=宿主 python3；venv=任务级隔离虚拟环境
AGENT_PY_VERSION=3.12
AGENT_PY_WHEELS_ONLY=true               # 仅允许 wheel 安装（禁 sdist 防任意代码执行）

# Agent 脚本沙箱
AGENT_SANDBOX_BACKEND=auto              # auto | seatbelt | bwrap | docker | plain
AGENT_SANDBOX_IMAGE=                    # docker 后端镜像（如 python:3.12-slim）
AGENT_SANDBOX_ALLOW_PLAIN=true          # 无可用沙箱时是否允许直跑兜底
```

> Agent 相关配置的完整说明（uv/Node 镜像源、TTL 清理、超时等）见 [backend/.env.example](backend/.env.example) 注释。

### 前端配置

复制 `frontend/.env.example` 为 `.env`，按环境修改 API 地址：

```env
# 开发环境：指向本地 Go 后端
VITE_API_BASE_URL=http://localhost:8080/api
```

环境变量优先级：`.env.[mode].local` > `.env.[mode]` > `.env.local` > `.env`，详见 [frontend/ENV_CONFIG_README.md](frontend/ENV_CONFIG_README.md)。

---

## 📚 API 文档

以下示例均以 `http://localhost:8080` 为例，`/api/v1` 可替换为 `/v1`。

### 获取模型列表

```bash
curl http://localhost:8080/api/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### 对话补全（流式）

```bash
curl -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "你好！"}
    ],
    "stream": true
  }'
```

### 工具调用（Function Calling）

```bash
curl -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "北京今天天气怎么样？"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "获取指定城市的天气信息",
          "parameters": {
            "type": "object",
            "properties": {
              "city": {"type": "string", "description": "城市名称"}
            },
            "required": ["city"]
          }
        }
      }
    ],
    "tool_choice": "auto"
  }'
```

### Anthropic 兼容接口

```bash
curl -X POST http://localhost:8080/api/v1/anthropic/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "model": "claude-3-opus",
    "messages": [
      {"role": "user", "content": "你好！"}
    ],
    "max_tokens": 1024
  }'
```

### Responses 接口

```bash
curl -X POST http://localhost:8080/api/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "input": "你好！",
    "max_output_tokens": 1024,
    "stream": false
  }'
```

### 向量嵌入

```bash
curl -X POST http://localhost:8080/api/v1/embeddings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "你好，世界"
  }'
```

### 图像生成

```bash
curl -X POST http://localhost:8080/api/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "dall-e-3",
    "prompt": "一只在星空中遨游的宇航员猫",
    "n": 1,
    "size": "1024x1024"
  }'
```

### 视频生成（异步任务）

```bash
# 1. 创建视频生成任务（立即返回 task_id）
curl -X POST http://localhost:8080/api/v1/videos/generations/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "doubao-seedance",
    "prompt": "日落时分海浪拍打礁石的慢镜头"
  }'

# 2. 轮询任务状态（也可由后台轮询器自动回填计费）
curl http://localhost:8080/api/v1/videos/generations/tasks/{task_id} \
  -H "Authorization: Bearer YOUR_API_KEY"
```

### Agent 任务

```bash
# 1. 显式指定技能创建 Agent 任务（异步执行，立即返回 task_id）
curl -X POST http://localhost:8080/api/v1/agent/tasks \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "openai/gpt-4o",
    "skills": ["my-skill"],
    "input": "根据需求描述生成一个落地页",
    "max_iterations": 100,
    "options": {"temperature": 0.7}
  }'

# 2. 语义路由：只描述需求，由路由模型自动选择技能与模型
curl -X POST http://localhost:8080/api/v1/agent/dispatch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "input": "帮我做一个产品介绍单页"
  }'

# 3. 查询任务状态与结果
curl http://localhost:8080/api/v1/agent/tasks/{task_id} \
  -H "Authorization: Bearer YOUR_API_KEY"

# 4. 下载任务产出资产
curl -O http://localhost:8080/api/v1/agent/tasks/{task_id}/assets/{asset_path} \
  -H "Authorization: Bearer YOUR_API_KEY"
```

---

## 🧩 支持的模型提供商

适配器通过提供商的 `adapter_class` 字段选择，未识别类型按 provider code 匹配，最终回退 `generic` 通用适配器：

| 提供商 | 适配器 | 适配类（adapter_class） |
|--------|--------|------------------------|
| OpenAI | [openai_adapter.go](backend/internal/ai/openai_adapter.go) | `openai` |
| Anthropic | [claude_adapter.go](backend/internal/ai/claude_adapter.go) | `claude` |
| 豆包 / 火山引擎 | [doubao_adapter.go](backend/internal/ai/doubao_adapter.go) | `doubao` / `volcengine` |
| DeepSeek | [deepseek_adapter.go](backend/internal/ai/deepseek_adapter.go) | `deepseek` |
| 阿里云通义千问 | [qwen_adapter.go](backend/internal/ai/qwen_adapter.go) | `qwen` / `qianwen` |
| 智谱 AI | [zhipu_adapter.go](backend/internal/ai/zhipu_adapter.go) | `zhipu` |
| 腾讯混元 | [hunyuan_adapter.go](backend/internal/ai/hunyuan_adapter.go) | `hunyuan` |
| 其他 OpenAI 兼容服务 | [adapter.go](backend/internal/ai/adapter.go) | `generic` |

---

## 🗂️ 项目结构

```
ai-transit-hub/
├── backend/                        # 后端 Go 项目
│   ├── cmd/server/                 # 服务器入口（main.go）
│   ├── internal/
│   │   ├── ai/                     # AI 提供商适配器与工厂
│   │   ├── api/
│   │   │   ├── handlers/           # 处理器（v1 兼容管线、Agent、计费等）
│   │   │   ├── middleware/         # JWT/角色/API-Key 认证、CORS、日志
│   │   │   └── routes.go           # 路由注册
│   │   ├── service/                # 业务服务（Agent 运行时、技能包、沙箱、
│   │   │                           #   执行环境隔离、模型选型、记账、审计）
│   │   ├── scheduler/              # 定时调度器（视频任务轮询）
│   │   ├── models/                 # 数据库模型（22 个功能域）
│   │   ├── repository/             # 数据库连接（复用既有表，跳过迁移）
│   │   ├── auth/                   # JWT 认证
│   │   ├── cache/                  # Redis 缓存
│   │   ├── config/                 # 配置加载（.env + CLI 覆盖）
│   │   ├── logger/                 # 日志（Lumberjack 滚动）
│   │   └── metrics/                # Prometheus 指标
│   ├── migrations/                 # 新增表迁移 SQL（agent_tasks、skills）
│   ├── tools/                      # 辅助工具（JWT Secret 生成器）
│   ├── .env.example                # 环境配置示例（含 Agent 全量配置注释）
│   └── go.mod
│
├── frontend/                       # 前端 Vue 项目
│   ├── src/
│   │   ├── views/
│   │   │   ├── admin/              # 管理后台页面（12 个模块）
│   │   │   ├── user/               # 用户端页面（11 个模块）
│   │   │   ├── Login.vue / Register.vue
│   │   ├── layouts/                # 用户端/后台布局
│   │   ├── components/             # 公共组件
│   │   ├── stores/                 # Pinia 状态
│   │   ├── router/                 # 路由配置（含登录守卫、角色守卫）
│   │   ├── config/                 # 前端配置
│   │   ├── utils/                  # 工具函数
│   │   └── styles/                 # 全局样式
│   ├── public/                     # 静态资源（logo 等）
│   ├── .env.example                # 前端环境变量示例
│   ├── ENV_CONFIG_README.md        # 环境变量优先级说明
│   └── vite.config.js
│
├── scripts/                        # v1 兼容层回归测试脚本（Python）
│   ├── regression_full_suite.py    # 全面回归套件
│   └── regression_r02_r12_r23.py   # 关键用例回归
│
├── docs/                           # 测试报告与项目分析文档
├── DEPLOYMENT.md                   # 后端通用部署指南（systemd/Nginx 等）
└── README.md
```

---

## 🩺 健康检查与运维

- **健康检查**: `GET /health`
- **监控指标**: `GET /metrics`（Prometheus 格式，可接入 Grafana）
- **内置调度器**: 启动时自动注册视频任务轮询器（每 5 分钟扫描近 7 天未计费的视频任务并回填终态与费用），支持超时保护、panic 恢复与优雅停机
- **优雅停机**: 收到 SIGINT/SIGTERM 后最长等待 30s 完成在途请求（HTTP 服务针对 SSE 流式长连接未设读写超时）

---

## 🧪 回归测试

`scripts/` 下提供面向 v1 兼容层的 Python 回归脚本：

```bash
# 全面回归套件（共享管线、错误信封矩阵、embeddings、可选真实计费的 video/image 用例）
python3 scripts/regression_full_suite.py --key YOUR_API_KEY

# 关键用例回归（R02 embeddings / R12 视频任务 / R23 未知 URL 错误信封）
python3 scripts/regression_r02_r12_r23.py --key YOUR_API_KEY
```

历史兼容性测试报告见 [docs/](docs/) 目录。

---

## 📦 部署

生产环境部署支持 systemd / nohup / supervisor / Nginx 反代等方式，不依赖 Docker（Docker 仅作为 Agent 脚本沙箱的可选后端）：

- [DEPLOYMENT.md](DEPLOYMENT.md)：后端跨平台编译、systemd/Nginx/Apache 配置、日志轮转、更新与回滚
- [backend/DEPLOYMENT.md](backend/DEPLOYMENT.md)：后端部署细则（路径、建库、deploy.sh）
- [frontend/DEPLOYMENT.md](frontend/DEPLOYMENT.md)：前端构建产物部署与 Nginx 反代（含前后端同域方案）
- Nginx 配置参考：[backend/aitb-backend.nginx.conf](backend/aitb-backend.nginx.conf)、[frontend/aitb-frontend.nginx.conf](frontend/aitb-frontend.nginx.conf)

---

## 📝 当前实现说明

- **注册入口已关闭**：`POST /api/register` 预留但未开放，用户由管理员在后台创建
- **不自动建表**：复用既有 PHP 项目表结构，新增功能表需手动执行 `backend/migrations/` 下的 SQL
- **用户端排行榜入口暂未开放**：页面已实现（`frontend/src/views/user/Leaderboards.vue`），路由暂被注释停用，后台排行榜配置管理可用
- **统一响应格式**：业务接口返回 `{success, message, data, errors}`；v1 兼容层返回 OpenAI/Anthropic 风格响应与错误信封

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发规范

1. **代码风格**: 遵循 Go 和 Vue.js 官方代码规范
2. **提交信息**: 使用语义化的提交信息格式
3. **测试**: 提交前请确保通过所有测试（后端含 `backend/internal/service/*_test.go`）
4. **文档**: 更新相关文档

### 提交信息规范

```
feat: 新功能
fix: 修复问题
docs: 文档更新
style: 代码格式调整
refactor: 重构代码
test: 测试相关
chore: 构建/工具相关
```

---

<p align="center">
  Made with ❤️ by AI Transit Hub Team
</p>

<p align="center">
  <a href="https://github.com/ggbound/ai-transit-hub">GitHub</a>
</p>
