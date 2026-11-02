# 快速开始指南

## 步骤 1: 安装 Go

1. 访问 [https://golang.org/dl/](https://golang.org/dl/)
2. 下载 Windows 版本的 Go 安装包（如 `go1.21.x.windows-amd64.msi`）
3. 运行安装程序，按默认设置安装
4. 重启终端或命令提示符，让环境变量生效
5. 验证安装：
   ```powershell
   go version
   ```

## 步骤 2: 配置环境

1. 复制 `.env.example` 为 `.env`（如果还没有）：
   ```powershell
   cd backend
   copy .env.example .env
   ```

2. 编辑 `.env` 文件，配置数据库连接：
   ```env
   DB_HOST=127.0.0.1
   DB_PORT=3306
   DB_DATABASE=ai_transit_hub
   DB_USERNAME=root
   DB_PASSWORD=your_password
   ```

## 步骤 3: 启动服务

### 方式 1: 使用启动脚本（推荐）
```powershell
cd backend
.\start.ps1
```

### 方式 2: 手动启动
```powershell
cd backend
go mod download
go run cmd/server/main.go
```

## 步骤 4: 验证服务

服务启动后，访问：
- 健康检查：http://localhost:8080/health

## API 测试

你可以使用 Postman 或 curl 测试 API。

### 测试登录
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"test@example.com\",\"password\":\"password123\"}"
```

### 获取模型列表（需要先登录获取 token）
```bash
curl -X GET http://localhost:8080/api/models \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

## 故障排除

### Go 命令未找到
- 确认 Go 已安装
- 重启终端或命令提示符
- 检查 `PATH` 环境变量是否包含 Go 的 bin 目录

### 数据库连接失败
- 确认 MySQL 服务正在运行
- 检查 `.env` 中的数据库配置
- 确认数据库 `ai_transit_hub` 已创建

### 端口被占用
修改 `.env` 中的 `SERVER_PORT` 为其他端口，如 8081
