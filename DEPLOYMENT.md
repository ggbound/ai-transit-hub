
# Go 后端部署指南

本指南说明如何将 Go 后端部署到服务器并持续运行。

## 1. 编译可执行文件

### Windows 本地编译
```bash
cd backend
go build -o server.exe ./cmd/server/main.go
```

### Linux 交叉编译（在 Windows 上）
```bash
# 编译 Linux 64 位
set GOOS=linux
set GOARCH=amd64
go build -o server ./cmd/server/main.go

# 编译 Linux 32 位
set GOOS=linux
set GOARCH=386
go build -o server_386 ./cmd/server/main.go

# 编译 ARM 64 位（用于树莓派等）
set GOOS=linux
set GOARCH=arm64
go build -o server_arm64 ./cmd/server/main.go

# 编译后恢复 Windows 设置
set GOOS=windows
set GOARCH=amd64
```

### Linux 本机编译
```bash
cd backend
go build -o server ./cmd/server/main.go
```

## 2. 上传到服务器

### 上传文件
需要上传以下文件到服务器：
- 编译好的可执行文件：`server`（Linux）或 `server.exe`（Windows）
- `.env` 配置文件
- `logs/` 目录（可自动创建，但建议手动创建）

### 使用 SCP/SFTP 上传
```bash
# Linux/Mac
scp server user@your-server:/path/to/deploy/
scp .env user@your-server:/path/to/deploy/

# Windows 使用 WinSCP、FileZilla 或 PowerShell
```

### 服务器目录结构
建议在服务器上创建如下目录结构：
```
/opt/ai-transit-hub/
├── server              # 可执行文件
├── .env                # 配置文件
├── logs/               # 日志目录
└── start.sh            # 启动脚本（可选）
```

## 3. 服务器环境配置

### 3.1 Linux 服务器配置

#### 创建用户和目录
```bash
# 创建专用用户
sudo useradd -m -s /bin/bash ai-transit
sudo mkdir -p /opt/ai-transit-hub
sudo chown -R ai-transit:ai-transit /opt/ai-transit-hub
sudo su ai-transit
cd /opt/ai-transit-hub
mkdir -p logs
```

#### 配置文件权限
```bash
chmod +x server
chmod 600 .env
```

#### 安装必要依赖
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y mysql-client

# CentOS/RHEL
sudo yum install -y mysql
```

### 3.2 Windows 服务器配置

#### 创建目录
```powershell
New-Item -Path "C:\opt\ai-transit-hub" -ItemType Directory
New-Item -Path "C:\opt\ai-transit-hub\logs" -ItemType Directory
```

## 4. 部署方式

### 4.1 使用 systemd（推荐，Linux）

#### 创建 systemd 服务文件
```bash
sudo nano /etc/systemd/system/ai-transit-hub.service
```

#### 服务文件内容
```ini
[Unit]
Description=AI Transit Hub Backend Service
After=network.target mysql.service

[Service]
Type=simple
User=ai-transit
WorkingDirectory=/opt/ai-transit-hub
ExecStart=/opt/ai-transit-hub/server
Restart=always
RestartSec=5
StandardOutput=append:/opt/ai-transit-hub/logs/stdout.log
StandardError=append:/opt/ai-transit-hub/logs/stderr.log

# 环境变量（如果不使用 .env）
Environment="DB_HOST=localhost"
Environment="DB_PORT=3306"

# 资源限制（可选）
LimitNOFILE=65536
MemoryMax=512M

[Install]
WantedBy=multi-user.target
```

#### 启动服务
```bash
# 重新加载 systemd 配置
sudo systemctl daemon-reload

# 启用开机自启动
sudo systemctl enable ai-transit-hub

# 启动服务
sudo systemctl start ai-transit-hub

# 查看状态
sudo systemctl status ai-transit-hub

# 查看日志
sudo journalctl -u ai-transit-hub -f

# 停止服务
sudo systemctl stop ai-transit-hub

# 重启服务
sudo systemctl restart ai-transit-hub
```

### 4.2 使用 nohup（临时部署，Linux）

#### 创建启动脚本
```bash
nano /opt/ai-transit-hub/start.sh
```

#### 脚本内容
```bash
#!/bin/bash
cd /opt/ai-transit-hub
export GIN_MODE=release
nohup ./server > logs/server.log 2>&1 &
echo $! > server.pid
echo "Server started with PID: $(cat server.pid)"
```

#### 创建停止脚本
```bash
nano /opt/ai-transit-hub/stop.sh
```

#### 停止脚本内容
```bash
#!/bin/bash
cd /opt/ai-transit-hub
if [ -f server.pid ]; then
    PID=$(cat server.pid)
    if ps -p $PID > /dev/null; then
        kill $PID
        echo "Server stopped"
    else
        echo "Process not found"
    fi
    rm -f server.pid
else
    echo "PID file not found"
fi
```

#### 运行
```bash
chmod +x start.sh stop.sh
./start.sh
./stop.sh
```

### 4.3 使用 supervisor（Linux）

#### 安装 supervisor
```bash
# Ubuntu/Debian
sudo apt-get install supervisor

# CentOS/RHEL
sudo yum install supervisor
```

#### 创建配置文件
```bash
sudo nano /etc/supervisor/conf.d/ai-transit-hub.conf
```

#### 配置内容
```ini
[program:ai-transit-hub]
user=ai-transit
directory=/opt/ai-transit-hub
command=/opt/ai-transit-hub/server
autostart=true
autorestart=true
stdout_logfile=/opt/ai-transit-hub/logs/stdout.log
stderr_logfile=/opt/ai-transit-hub/logs/stderr.log
environment=GIN_MODE="release"
stopsignal=TERM
```

#### 启动服务
```bash
sudo supervisorctl reread
sudo supervisorctl update
sudo supervisorctl start ai-transit-hub
sudo supervisorctl status ai-transit-hub
```

### 4.4 使用 Windows 服务

#### 使用 NSSM（推荐）
```powershell
# 下载 NSSM
# https://nssm.cc/download

# 安装服务
nssm install AI-Transit-Hub
nssm set AI-Transit-Hub Application C:\opt\ai-transit-hub\server.exe
nssm set AI-Transit-Hub WorkingDirectory C:\opt\ai-transit-hub
nssm set AI-Transit-Hub DisplayName AI-Transit-Hub
nssm set AI-Transit-Hub Description AI Transit Hub Backend
nssm start AI-Transit-Hub

# 其他命令
nssm stop AI-Transit-Hub
nssm remove AI-Transit-Hub
```

#### 使用 PowerShell 后台运行
```powershell
$ErrorActionPreference = "Continue"
$LogFile = "C:\opt\ai-transit-hub\logs\server.log"
Start-Process -FilePath "C:\opt\ai-transit-hub\server.exe" -WorkingDirectory "C:\opt\ai-transit-hub" -WindowStyle Hidden -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile
```

#### 使用 Task Scheduler
1. 打开任务计划程序
2. 创建基本任务
3. 触发器选择"当计算机启动时"
4. 操作选择"启动程序"
5. 程序选择 `C:\opt\ai-transit-hub\server.exe`
6. 起始位置 `C:\opt\ai-transit-hub`
7. 完成后在任务属性中设置"不管用户是否登录都要运行"

## 5. 反向代理配置（推荐）

### 5.1 Nginx 配置

#### 安装 Nginx
```bash
# Ubuntu/Debian
sudo apt-get install nginx

# CentOS/RHEL
sudo yum install nginx
```

#### 创建配置文件
```bash
sudo nano /etc/nginx/conf.d/ai-transit-hub.conf
```

#### 配置内容
```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 日志配置
    access_log /var/log/nginx/ai-transit-hub-access.log;
    error_log /var/log/nginx/ai-transit-hub-error.log;

    # API 反向代理
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }
}

# HTTPS 配置（可选）
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL 证书
    ssl_certificate /path/to/your/cert.pem;
    ssl_certificate_key /path/to/your/key.pem;

    # 日志配置
    access_log /var/log/nginx/ai-transit-hub-access.log;
    error_log /var/log/nginx/ai-transit-hub-error.log;

    # API 反向代理
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### 重启 Nginx
```bash
sudo nginx -t
sudo systemctl restart nginx
```

### 5.2 Apache 配置

#### 安装 Apache
```bash
# Ubuntu/Debian
sudo apt-get install apache2

# CentOS/RHEL
sudo yum install httpd
```

#### 启用模块
```bash
sudo a2enmod proxy proxy_http proxy_wstunnel rewrite
```

#### 创建配置
```bash
sudo nano /etc/apache2/sites-available/ai-transit-hub.conf
```

#### 配置内容
```apache
&lt;VirtualHost *:80&gt;
    ServerName your-domain.com

    ProxyRequests Off
    ProxyPreserveHost On

    # API 代理
    ProxyPass / http://127.0.0.1:8080/
    ProxyPassReverse / http://127.0.0.1:8080/

    # WebSocket 支持
    ProxyPass /ws/ ws://127.0.0.1:8080/
    ProxyPassReverse /ws/ ws://127.0.0.1:8080/

    # 日志
    ErrorLog ${APACHE_LOG_DIR}/ai-transit-hub-error.log
    CustomLog ${APACHE_LOG_DIR}/ai-transit-hub-access.log combined
&lt;/VirtualHost&gt;
```

#### 启用站点
```bash
sudo a2ensite ai-transit-hub
sudo systemctl restart apache2
```

## 6. 防火墙配置

### 6.1 Linux (UFW)
```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 8080/tcp  # 可选，仅用于直接访问
sudo ufw status
```

### 6.2 Linux (iptables)
```bash
sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
sudo service iptables save
```

### 6.3 Windows 防火墙
```powershell
# 添加规则
New-NetFirewallRule -DisplayName "AI-Transit-Hub" -Direction Inbound -LocalPort 80,443,8080 -Protocol TCP -Action Allow
```

## 7. 数据库配置

### 7.1 配置 .env 文件
```env
# .env 示例
APP_ENV=production
GIN_MODE=release
DB_HOST=localhost
DB_PORT=3306
DB_NAME=ai_transit_hub
DB_USER=db_user
DB_PASSWORD=db_password
SERVER_PORT=8080
```

### 7.2 数据库初始化
确保你的数据库已经创建并迁移完成。

## 8. 健康检查和监控

### 8.1 添加健康检查端点
服务已经有 `/health` 端点。

### 8.2 使用简单脚本监控
```bash
nano /opt/ai-transit-hub/check.sh
```

#### 监控脚本内容
```bash
#!/bin/bash
URL="http://localhost:8080/health"
LOG_FILE="/opt/ai-transit-hub/logs/monitor.log"

while true; do
    STATUS_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$URL")
    
    if [ "$STATUS_CODE" != "200" ]; then
        echo "$(date): Service not responding, restarting..." &gt;&gt; "$LOG_FILE"
        sudo systemctl restart ai-transit-hub
    fi
    
    sleep 300
done
```

#### 运行监控
```bash
chmod +x check.sh
nohup ./check.sh &gt;&gt; logs/monitor.log 2&gt;&amp;1 &amp;
```

## 9. 日志管理

### 9.1 Logrotate（Linux）

#### 配置 logrotate
```bash
sudo nano /etc/logrotate.d/ai-transit-hub
```

#### 配置内容
```
/opt/ai-transit-hub/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0644 ai-transit ai-transit
    sharedscripts
    postrotate
        sudo systemctl reload ai-transit-hub &gt; /dev/null 2&gt;&amp;1 || true
    endscript
}
```

## 10. 更新部署流程

### 10.1 更新步骤
```bash
# 1. 本地编译新的可执行文件
go build -o server ./cmd/server/main.go

# 2. 上传到服务器（使用备份文件名）
scp server user@your-server:/opt/ai-transit-hub/server.new

# 3. 在服务器上操作
sudo su ai-transit
cd /opt/ai-transit-hub
mv server server.backup  # 备份当前版本
mv server.new server     # 替换新版本
chmod +x server

# 4. 重启服务
sudo systemctl restart ai-transit-hub

# 5. 检查服务状态
sudo systemctl status ai-transit-hub

# 6. 如有问题，快速回滚
if [ "$?" -ne "0" ]; then
    mv server.backup server
    sudo systemctl restart ai-transit-hub
fi
```

## 11. 常用命令速查

### Linux 常用
```bash
# 查看端口占用
sudo lsof -i :8080
sudo netstat -tuln | grep 8080
sudo ss -tuln | grep 8080

# 查看进程
ps aux | grep server
pstree -p

# 查看实时日志
tail -f logs/server.log
tail -n 100 logs/server.log
```

### Windows 常用
```powershell
# 查看端口占用
netstat -ano | findstr :8080

# 查看进程
Get-Process | Where-Object {$_.ProcessName -eq "server"}
```

## 12. 故障排查

### 常见问题
1. 端口被占用
   ```bash
   lsof -i :8080
   # 或修改 .env 中的 SERVER_PORT
   ```

2. 权限问题
   ```bash
   chmod +x server
   chown -R ai-transit:ai-transit /opt/ai-transit-hub
   ```

3. 数据库连接问题
   ```bash
   # 检查 .env 配置
   # 检查 MySQL 服务是否运行
   sudo systemctl status mysql
   ```

4. 服务立即退出
   ```bash
   # 查看日志
   sudo journalctl -u ai-transit-hub
   tail -f logs/stderr.log
   ```

## 13. 性能优化（可选）

### 使用 PM2（Node.js 生态工具）
```bash
# 安装 PM2
npm install -g pm2

# 使用 PM2 启动
pm2 start ./server --name ai-transit-hub
pm2 save
pm2 startup
```

### 增加系统文件描述符限制
```bash
# 临时
ulimit -n 65536

# 永久
sudo nano /etc/security/limits.conf
# 添加：
ai-transit soft nofile 65536
ai-transit hard nofile 65536
```

---

**联系与支持**  
如有部署问题，请查看日志文件或联系技术支持。

