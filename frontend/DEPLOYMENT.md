# AI Transit Hub 前端部署文档

编译产物: `frontend/dist/`  
部署路径: `/data/sites/aitb/frontend`

---

## 目录

1. [编译步骤](#编译步骤)
2. [生产环境配置](#生产环境配置)
3. [部署到服务器](#部署到服务器)
4. [Nginx配置](#nginx配置)
5. [完整部署命令](#完整部署命令)

---

## 编译步骤

### 1. 安装依赖（如果尚未安装）

```bash
cd frontend
npm install
```

### 2. 配置生产环境

编辑 `.env.production` 文件：

```bash
nano .env.production
```

确保配置正确的后端 API 地址：

```env
# 生产环境后端 API 地址
VITE_API_BASE_URL=http://localhost:8080/api

VITE_APP_ENV=production
VITE_DEBUG=false
```

### 3. 编译生产环境

```bash
npm run build
```

编译完成后，所有静态文件会生成在 `dist/` 目录下。

---

## 部署到服务器

### 1. 上传编译文件

```bash
# 方式一：使用 scp 上传（本地执行）
scp -r frontend/dist/ user@your-server:/data/sites/aitb/frontend/

# 方式二：使用 rsync 同步
rsync -avz --delete frontend/dist/ user@your-server:/data/sites/aitb/frontend/

# 方式三：如果在服务器上直接操作
cd /data/sites/aitb/frontend
# 拉取代码后执行 npm install && npm run build
```

### 2. 服务器部署步骤

```bash
# 创建部署目录
mkdir -p /data/sites/aitb/frontend

# 上传 dist 目录内容到 /data/sites/aitb/frontend/
# 确保文件权限正确
chown -R www-data:www-data /data/sites/aitb/frontend
chmod -R 755 /data/sites/aitb/frontend
```

---

## Nginx配置

### 1. 安装 Nginx（如果尚未安装）

```bash
# Ubuntu/Debian
apt update && apt install nginx -y

# CentOS/RHEL
yum install nginx -y
```

### 2. 配置 Nginx

```bash
# 复制配置文件
cp /data/sites/aitb/frontend/aitb-frontend.nginx.conf /etc/nginx/sites-available/aitb-frontend

# 创建软链接
ln -s /etc/nginx/sites-available/aitb-frontend /etc/nginx/sites-enabled/

# 编辑配置（设置正确的域名）
nano /etc/nginx/sites-available/aitb-frontend

# 测试配置
nginx -t

# 重载配置
systemctl reload nginx
```

---

## 完整部署命令

以下是可以直接复制使用的完整部署命令：

```bash
# === 本地编译 ===
cd frontend
npm run build

# === 上传到服务器 ===
# 使用 scp（替换 user 和 your-server）
scp -r dist/ user@your-server:/data/sites/aitb/frontend/

# === 服务器上操作 ===
ssh user@your-server

# 创建目录
mkdir -p /data/sites/aitb/frontend
cd /data/sites/aitb/frontend

# 设置权限
chown -R www-data:www-data /data/sites/aitb/frontend
chmod -R 755 /data/sites/aitb/frontend

# 配置 Nginx
cp /data/sites/aitb/frontend/aitb-frontend.nginx.conf /etc/nginx/sites-available/aitb-frontend
ln -s /etc/nginx/sites-available/aitb-frontend /etc/nginx/sites-enabled/

# 编辑域名（如果需要）
nano /etc/nginx/sites-available/aitb-frontend

# 测试并重载 Nginx
nginx -t
systemctl reload nginx
```

---

## 文件结构

部署后，服务器目录结构如下：

```
/data/sites/aitb/
├── frontend/
│   ├── dist/          # 前端编译产物（实际 web 根目录）
│   ├── aitb-frontend.nginx.conf
│   └── DEPLOYMENT.md
└── backend/
    ├── server
    ├── .env
    ├── aitb-backend.service
    └── deploy.sh
```

---

## 配置前后端在同一个域名下（可选）

如果你希望前后端共用一个域名，可以使用以下 Nginx 配置：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    # 前端
    location / {
        root /data/sites/aitb/frontend/dist;
        try_files $uri $uri/ /index.html;
    }

    # 后端 API
    location /api/ {
        proxy_pass http://127.0.0.1:8080/;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        root /data/sites/aitb/frontend/dist;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

这样配置后，前端通过 `/api/` 访问后端接口，需要相应修改前端 `.env.production` 的 `VITE_API_BASE_URL` 为 `/api`。
