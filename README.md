# AI Transit Hub - AI 模型中转站

<p align="center">
  <img src="frontend/public/logo.svg" alt="AI Transit Hub Logo" width="200"/>
</p>

<p align="center">
  <strong>统一的 AI 模型调度管理系统</strong>
</p>

<p align="center">
  <a href="#功能特性">功能特性</a> •
  <a href="#技术架构">技术架构</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#配置说明">配置说明</a> •
  <a href="#api文档">API 文档</a> •
  <a href="#贡献指南">贡献指南</a>
</p>

---

## 📖 项目简介

AI Transit Hub 是一个企业级 AI 模型中转站，提供统一的 AI 模型接入和管理平台。通过标准化的 API 接口，让企业能够轻松接入和管理多家 AI 模型提供商（OpenAI、Anthropic、豆包、DeepSeek、通义千问等），实现模型的统一调度、权限管理和计费统计。

### 核心优势

- 🎯 **标准化接口**：提供 OpenAI 和 Anthropic 兼容的 API，零成本迁移
- 🚀 **流式输出**：支持 SSE 流式输出，提升用户体验
- 🛠️ **工具调用**：支持 Function Calling，扩展 AI 能力边界
- 📊 **多维度统计**：Token、费用、调用次数等多维度统计
- 🔐 **企业级安全**：RBAC 权限控制、操作审计、API 密钥管理
- 🎨 **友好界面**：现代化的管理后台和用户界面

---

## ✨ 功能特性

### 模型管理能力

| 功能 | 描述 |
|------|------|
| 多提供商接入 | 支持 OpenAI、Anthropic、豆包、DeepSeek、通义千问、智谱 AI、腾讯混元等 |
| 自定义适配器 | 支持添加自定义模型提供商适配器 |
| 模型配置 | 支持配置模型参数、价格、能力（流式、工具调用等） |
| 智能调度 | 支持多密钥负载均衡、故障自动切换 |

### API 兼容能力

- **OpenAI 兼容接口**：`/api/v1/chat/completions`、`/api/v1/models`
- **Anthropic 兼容接口**：`/api/v1/anthropic/messages`
- **标准 SSE 流式输出**：逐字返回，实时响应
- **工具调用支持**：Function Calling 标准化实现

### 用户功能

| 功能模块 | 特性 |
|---------|------|
| 📝 对话功能 | 多轮对话、历史记录、流式输出、工具调用 |
| 🔑 API 密钥 | 个人密钥管理、配额控制、使用统计 |
| 📊 数据统计 | 个人 Token 使用、费用统计、趋势图表 |
| 🏆 排行榜 | 部门/公司级使用排行 |
| 🧪 接口测试 | 在线 API 测试工具、代码示例生成 |
| 📚 API 文档 | 自动生成接口文档 |

### 后台管理功能

| 功能模块 | 特性 |
|---------|------|
| 📈 仪表盘 | 实时调用统计、费用监控、模型使用占比 |
| 🤖 模型管理 | 模型配置、启停用、能力设置 |
| 🔐 密钥管理 | 提供商密钥配置、分组、熔断策略 |
| 👥 用户管理 | 用户管理、权限分配、配额设置 |
| 💰 计费管理 | 费率配置、预算预警、账单生成 |
| 📋 日志审计 | 全量操作日志、审计追踪、异常告警 |
| 🛠️ 工具管理 | 内置工具、自定义工具、执行监控 |

---

## 🏗️ 技术架构

### 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         外部AI模型                           │
│  OpenAI │ Anthropic │ 豆包 │ DeepSeek │ 通义千问 │ ...      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      API 网关/适配器层                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ OpenAI适配器 │  │ Claude适配器 │  │ Qwen适配器  │  ...    │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      核心服务层                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ 统一API服务  │  │ 调度服务    │  │ 工具执行服务 │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ 计费服务    │  │ 用户服务    │  │ 日志服务    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────────┐   ┌─────────────────────────────┐
│        员工使用平台          │   │       后台管理系统          │
│  - 对话功能                 │   │  - 模型配置管理             │
│  - API密钥管理              │   │  - 用户权限管理             │
│  - 数据统计                 │   │  - 计费统计                 │
│  - 排行榜                   │   │  - 日志审计                 │
└─────────────────────────────┘   └─────────────────────────────┘
```

### 技术栈

#### 后端 (Backend)
- **框架**: Go 1.21+（Gin）
- **数据库**: MySQL（GORM）
- **缓存**: Redis
- **认证**: JWT

#### 前端 (Frontend)
- **框架**: Vue 3 + Composition API
- **构建工具**: Vite 8.x
- **UI 组件库**: Arco Design Vue
- **状态管理**: Pinia
- **路由**: Vue Router 4
- **HTTP 客户端**: Axios
- **图表**: ECharts
- **Markdown 渲染**: Marked + Highlight.js

---

## 🚀 快速开始

### 环境要求

- **Go**: >= 1.21
- **Node.js**: >= 16.0
- **MySQL**: >= 5.7
- **Redis**: >= 5.0

### 安装步骤

#### 1. 克隆代码

```bash
git clone https://github.com/ggbound/ai-transit-hub.git
cd ai-transit-hub
```

#### 2. 后端配置

```bash
cd backend

# 安装依赖
go mod download

# 复制环境配置（数据库、Redis、JWT 等，详见 .env.example）
cp .env.example .env

# 启动开发服务器
go run cmd/server/main.go
```

> 后端复用现有数据库表结构，启动前请确保数据库已按历史表结构就绪。更多说明见 [backend/README.md](backend/README.md)。

#### 3. 前端配置

```bash
cd ../frontend

# 安装依赖
npm install

# 配置环境变量（开发环境使用默认配置即可）
cp .env.example .env

# 启动开发服务器
npm run dev
```

#### 4. 访问系统

- **用户端**: http://localhost:5173 (Vite 默认端口)
- **管理后台**: http://localhost:5173/admin
- **后端 API**: http://localhost:8080/api

---

## ⚙️ 配置说明

### 后端配置 (.env)

```env
# 服务配置
SERVER_PORT=8080
GIN_MODE=debug

# 数据库配置
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=ai_transit_hub
DB_USERNAME=root
DB_PASSWORD=your_password

# Redis 配置
REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=

# JWT 配置
JWT_SECRET=your-secret-key-here-change-this-in-production
JWT_EXPIRY=24h
```

### 前端配置

#### 开发环境 (.env.development)

```env
VITE_API_BASE_URL=http://localhost:8080/api
VITE_APP_ENV=development
VITE_DEBUG=true
```

#### 生产环境 (.env.production)

```env
VITE_API_BASE_URL=https://your-production-api.com/api
VITE_APP_ENV=production
VITE_DEBUG=false
```

#### 本地覆盖 (.env.local)

```bash
# 创建本地覆盖配置（不提交到 git）
cp .env.local.example .env.local

# 修改配置
echo "VITE_API_BASE_URL=http://192.168.1.100:8080/api" >> .env.local
```

---

## 📚 API 文档

### OpenAI 兼容接口

#### 获取模型列表

```bash
curl -X GET http://localhost:8080/api/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### 对话接口

```bash
curl -X POST http://localhost:8080/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "你好！"}
    ],
    "stream": false
  }'
```

#### 流式输出

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
    "max_output_tokens": 1024
  }'
```

#### Responses 流式输出

```bash
curl -X POST http://localhost:8080/api/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{
    "model": "gpt-4",
    "input": "你好！",
    "max_output_tokens": 1024,
    "stream": true
  }'
```

### 工具调用示例

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

---

## 🧩 支持的模型提供商

| 提供商 | 模型系列 | 流式输出 | 工具调用 | 状态 |
|--------|----------|---------|---------|------|
| OpenAI | GPT-3.5/GPT-4 | ✅ | ✅ | 已支持 |
| Anthropic | Claude 3 | ✅ | ✅ | 已支持 |
| 豆包 | 豆包系列 | ✅ | ✅ | 已支持 |
| DeepSeek | DeepSeek-V3/R1 | ✅ | ✅ | 已支持 |
| 阿里云 | 通义千问 | ✅ | ✅ | 已支持 |
| 智谱 AI | GLM 系列 | ✅ | ✅ | 已支持 |
| 腾讯 | 混元 | ✅ | ✅ | 已支持 |

---

## 🗂️ 项目结构

```
ai-transit-hub/
├── backend/                    # 后端 Go 项目
│   ├── cmd/
│   │   └── server/             # 服务器入口
│   ├── internal/
│   │   ├── ai/                 # AI 提供商适配器
│   │   ├── api/                # 路由、处理器、中间件
│   │   ├── service/            # 业务服务（Agent 运行时、技能包等）
│   │   ├── models/             # 数据库模型
│   │   ├── repository/         # 数据库连接
│   │   ├── auth/               # JWT 认证
│   │   └── config/             # 配置
│   ├── migrations/             # 数据库迁移 SQL
│   ├── .env.example            # 环境配置示例
│   └── go.mod                  # 依赖配置
│
├── frontend/                   # 前端 Vue 项目
│   ├── src/
│   │   ├── views/              # 页面视图
│   │   │   ├── admin/          # 管理后台页面
│   │   │   └── user/           # 用户端页面
│   │   ├── components/         # 公共组件
│   │   ├── utils/              # 工具函数
│   │   ├── config/             # 配置文件
│   │   └── router/             # 路由配置
│   ├── public/                 # 静态资源
│   └── package.json            # 依赖配置
│
└── docs/                       # 文档目录
    └── project_analysis.md     # 项目分析文档
```

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发规范

1. **代码风格**: 遵循 Go 和 Vue.js 官方代码规范
2. **提交信息**: 使用语义化的提交信息格式
3. **测试**: 提交前请确保通过所有测试
4. **文档**: 更新相关文档

### 提交信息规范

```
feat: 新功能ix: 修复问题
docs: 文档更新
style: 代码格式调整
refactor: 重构代码
test: 测试相关
chore: 构建/工具相关
```

---

## 📜 开源协议

本项目采用 [MIT 协议](LICENSE) 开源。

---

## 📧 联系我们

如有问题或建议，欢迎通过以下方式联系我们：

- **邮箱**: contact@ai-transit-hub.com
- **GitHub Issues**: [提交问题](https://github.com/ggbound/ai-transit-hub/issues)

---

<p align="center">
  Made with ❤️ by AI Transit Hub Team
</p>

<p align="center">
  <a href="https://github.com/ggbound/ai-transit-hub">GitHub</a> •
  <a href="https://ai-transit-hub.com">官网</a> •
  <a href="https://docs.ai-transit-hub.com">文档</a>
</p>
