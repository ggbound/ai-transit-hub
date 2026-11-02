# AI Transit Hub 后端部署文档

部署路径: `/data/sites/aitb/backend`

## 目录

1. [环境要求](#环境要求)
2. [首次部署](#首次部署)
3. [配置文件说明](#配置文件说明)
4. [日常运维](#日常运维)
5. [Nginx配置](#nginx配置)
6. [故障排查](#故障排查)

---

## 环境要求

- 操作系统: Linux (Ubuntu 20.04+ / CentOS 8+ / Debian 11+)
- Go: 1.21 或更高版本
- MySQL: 8.0+
- Systemd: 用于服务管理
- Nginx: 用于反向代理（可选但推荐）

---

## 首次部署

### 1. 准备服务器环境

```bash
# 1. 创建部署目录
mkdir -p /data/sites/aitb/backend

# 2. 上传代码到该目录
# 使用 git clone 或 scp 等方式上传

# 3. 进入目录
cd /data/sites/aitb/backend
```

### 2. 安装 Go 环境（如果尚未安装）

```bash
# 下载 Go
wget https://go.dev/dl/go1.21.13.linux-amd64.tar.gz

# 解压
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.21.13.linux-amd64.tar.gz

# 配置环境变量
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 验证安装
go version
```

### 3. 配置数据库

```bash
# 登录 MySQL
mysql -u root -p

# 创建数据库
CREATE DATABASE ai_transit_hub CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

# 创建用户（可选，推荐）
CREATE USER 'aitb_user'@'localhost' IDENTIFIED BY 'your_secure_password';
GRANT ALL PRIVILEGES ON ai_transit_hub.* TO 'aitb_user'@'localhost';
FLUSH PRIVILEGES;

# 退出
EXIT;
```

### 4. 配置环境变量

```bash
# 复制配置模板
cd /data/sites/aitb/backend
cp .env.example .env

# 编辑配置
nano .env
```

修改以下配置（根据实际情况）：

```env
# 服务配置
SERVER_PORT=8080
GIN_MODE=release

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_DATABASE=ai_transit_hub
DB_USERNAME=root
DB_PASSWORD=your_password

# JWT 密钥（生产环境必须修改！）
JWT_SECRET=your-secret-key-at-least-32-chars
JWT_EXPIRY=24h
```

### 5. 运行部署脚本

```bash
# 进入项目目录
cd /data/sites/aitb/backend

# 添加执行权限
chmod +x deploy.sh

# 运行部署脚本
./deploy.sh
```

### 6. 配置 systemd 服务（首次部署需执行）

```bash
# 复制服务文件
cp /data/sites/aitb/backend/aitb-backend.service /etc/systemd/system/

# 重新加载 systemd
systemctl daemon-reload

# 启用开机自启
systemctl enable aitb-backend
```

### 7. 验证部署

```bash
# 检查服务状态
systemctl status aitb-backend

# 查看服务日志
journalctl -u aitb-backend -f

# 测试健康检查接口
curl http://localhost:8080/health
```

---

## 配置文件说明

### .env 主要配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| SERVER_PORT | 服务端口 | 8080 |
| GIN_MODE | Gin运行模式 | debug |
| DB_HOST | 数据库主机 | localhost |
| DB_PORT | 数据库端口 | 3306 |
| DB_DATABASE | 数据库名 | ai_transit_hub |
| DB_USERNAME | 数据库用户名 | root |
| DB_PASSWORD | 数据库密码 | - |
| JWT_SECRET | JWT密钥（生产环境必须修改！） | - |
| JWT_EXPIRY | JWT过期时间 | 24h |

---

## 日常运维

### 快速重新部署

修改代码后，使用快速部署模式：

```bash
cd /data/sites/aitb/backend
./deploy.sh quick
```

### 服务管理

```bash
# 查看服务状态
systemctl status aitb-backend

# 启动服务
systemctl start aitb-backend

# 停止服务
systemctl stop aitb-backend

# 重启服务
systemctl restart aitb-backend

# 查看实时日志
journalctl -u aitb-backend -f

# 查看最近100条日志
journalctl -u aitb-backend -n 100 --no-pager
```

### 备份

```bash
# 备份数据库
mysqldump -u root -p ai_transit_hub > /backup/aitb-db-$(date +%Y%m%d).sql

# 备份代码
cd /data/sites/aitb
tar -czf /backup/aitb-backend-$(date +%Y%m%d).tar.gz backend/
```

---

## Nginx配置

### 1. 安装 Nginx

```bash
# Ubuntu/Debian
apt update && apt install nginx -y

# CentOS/RHEL
yum install nginx -y
```

### 2. 配置 Nginx

```bash
# 复制配置文件
cp /data/sites/aitb/backend/aitb-backend.nginx.conf /etc/nginx/sites-available/aitb-backend

# 创建软链接
ln -s /etc/nginx/sites-available/aitb-backend /etc/nginx/sites-enabled/

# 编辑配置（设置正确的域名）
nano /etc/nginx/sites-available/aitb-backend

# 测试配置
nginx -t

# 重载配置
systemctl reload nginx
```

### 3. 防火墙配置

```bash
# Ubuntu/Debian (ufw)
ufw allow 80/tcp
ufw allow 443/tcp

# CentOS/RHEL (firewalld)
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload
```

---

## 故障排查

### 服务无法启动

```bash
# 1. 检查服务状态
systemctl status aitb-backend

# 2. 查看详细日志
journalctl -u aitb-backend -n 100 --no-pager

# 3. 检查端口是否被占用
netstat -tlnp | grep 8080
# 或
ss -tlnp | grep 8080

# 4. 检查环境变量配置
cat /data/sites/aitb/backend/.env

# 5. 手动测试运行
cd /data/sites/aitb/backend
./server
```

### 数据库连接失败

```bash
# 1. 检查数据库服务
systemctl status mysql
# 或
systemctl status mariadb

# 2. 测试数据库连接
mysql -h localhost -u root -p -e "SELECT 1"

# 3. 检查 .env 中的数据库配置是否正确
```

### 内存不足

如果服务器内存较小（<1GB），可以考虑启用 swap：

```bash
# 创建 swap 文件
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
```

---

## 完整部署命令速查

以下是可以直接复制使用的完整部署命令序列：

```bash
# === 1. 准备目录 ===
mkdir -p /data/sites/aitb/backend
cd /data/sites/aitb/backend

# === 2. 安装 Go (如果没有) ===
wget https://go.dev/dl/go1.21.13.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.21.13.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version

# === 3. 配置 .env ===
cp .env.example .env
nano .env  # 根据实际情况修改

# === 4. 部署 ===
chmod +x deploy.sh
./deploy.sh

# === 5. 配置 systemd ===
cp /data/sites/aitb/backend/aitb-backend.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable aitb-backend

# === 6. 验证 ===
systemctl status aitb-backend
journalctl -u aitb-backend -n 50 --no-pager
curl http://localhost:8080/health
```

---

## 联系与支持

遇到问题时，请检查：
1. 服务日志: `journalctl -u aitb-backend -f`
2. 部署文档: `/data/sites/aitb/backend/DEPLOYMENT.md`
