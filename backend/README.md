# AI Transit Hub - Go Backend

这是 AI Transit Hub 的 Go 后端实现，完全复刻了原 PHP 后端的所有功能和接口。

## 项目结构

```
backend/
├── cmd/
│   └── server/
│       └── main.go                  # 服务器入口
├── internal/
│   ├── ai/                          # AI 提供商适配器（OpenAI/Claude/豆包/DeepSeek/千问/智谱/混元）
│   ├── api/
│   │   ├── handlers/                # 控制器/处理器
│   │   ├── middleware/              # 中间件（认证、CORS 等）
│   │   └── routes.go                # 路由定义
│   ├── auth/                        # JWT 认证
│   ├── cache/                       # 缓存
│   ├── config/                      # 配置
│   ├── logger/                      # 日志
│   ├── metrics/                     # 监控指标（Prometheus）
│   ├── models/                      # 数据库模型
│   ├── repository/                  # 数据库连接
│   ├── scheduler/                   # 定时调度
│   ├── service/                     # 业务服务（Agent 运行时、技能包、计费等）
│   └── utils/                       # 工具函数
├── migrations/                      # 数据库迁移 SQL（Agent 任务、技能表）
├── tools/                           # 辅助工具（JWT 密钥生成）
├── deploy.sh                        # Linux 部署脚本
├── start.ps1                        # Windows 启动脚本
├── aitb-backend.service             # systemd 服务配置
├── aitb-backend.nginx.conf          # Nginx 配置示例
├── go.mod
├── .env.example
└── README.md
```

## 已实现的接口

### 认证接口
- `POST /api/register` - 注册（已关闭）
- `POST /api/login` - 登录
- `POST /api/logout` - 登出
- `GET /api/user` - 获取当前用户信息
- `PUT /api/user/profile` - 更新用户资料
- `POST /api/user/avatar` - 上传头像
- `POST /api/user/change-password` - 修改密码

### 模型接口
- `GET /api/models` - 获取可用模型列表（支持分组）
- `GET /api/models/:id` - 获取单个模型详情

### 聊天接口
- `POST /api/chat/send` - 发送聊天消息（支持流式和非流式）

### 会话接口
- `GET /api/sessions` - 获取会话列表
- `POST /api/sessions` - 创建新会话
- `GET /api/sessions/:id` - 获取会话详情
- `PUT /api/sessions/:id` - 更新会话
- `DELETE /api/sessions/:id` - 删除会话
- `GET /api/sessions/:session_id/messages` - 获取会话消息
- `GET /api/sessions/:session_id/messages/:id` - 获取单条消息
- `DELETE /api/sessions/:session_id/messages/:id` - 删除消息

### 用户 API 密钥接口
- `GET /api/user/api-keys` - 获取 API 密钥列表
- `POST /api/user/api-keys` - 创建 API 密钥
- `GET /api/user/api-keys/:id` - 获取单个 API 密钥
- `PUT /api/user/api-keys/:id` - 更新 API 密钥
- `DELETE /api/user/api-keys/:id` - 删除 API 密钥

### 日志接口
- `GET /api/user/logs/usage` - 获取使用日志
- `GET /api/user/logs/operation` - 获取操作日志
- `GET /api/user/logs/stats` - 获取使用统计

### 工具接口
- `GET /api/tools` - 获取可用工具列表

## 安装和运行

### 1. 安装 Go

首先需要安装 Go 1.21 或更高版本：

访问 [https://golang.org/dl/](https://golang.org/dl/) 下载并安装。

### 2. 配置环境变量

复制 `.env.example` 为 `.env` 并配置：

```env
# Server Configuration
SERVER_PORT=8080
GIN_MODE=debug

# Database Configuration
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=ai_transit_hub
DB_USERNAME=root
DB_PASSWORD=
DB_CHARSET=utf8mb4

# JWT Configuration
JWT_SECRET=your-secret-key-here-change-this-in-production
JWT_EXPIRY=24h

# Application Configuration
APP_NAME=AI Transit Hub
APP_ENV=development
```

### 3. 安装依赖

```bash
cd backend
go mod download
```

### 4. 运行服务器

```bash
go run cmd/server/main.go
```

服务器将在 `http://localhost:8080` 启动。

## 响应格式

所有 API 响应都遵循统一的格式：

### 成功响应
```json
{
  "success": true,
  "message": "Success",
  "data": { ... }
}
```

### 错误响应
```json
{
  "success": false,
  "message": "Error message",
  "errors": { ... }  // 可选
}
```

## 与 PHP 后端的对比

- **路由完全一致** - 所有接口路径都与原 PHP 后端相同
- **响应格式兼容** - 返回的数据结构保持完全一致
- **数据库兼容** - 使用相同的数据库表结构
- **认证方式相同** - JWT + 用户 API 密钥认证

## 注意事项

- 后端复用原有数据库表结构（AutoMigrate 已跳过），新增表通过 `migrations/` 下的 SQL 脚本手动执行
- Agent 任务工作区脚本执行默认走沙箱自适应链，相关行为可通过 `.env` 中的 `AGENT_*` 配置调整，详见 `.env.example`
