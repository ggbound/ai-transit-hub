# 前端多环境配置说明

## 概述

前端支持根据运行模式（development/production）自动加载对应的 `.env` 文件配置，无需修改代码即可在开发和生产环境使用不同的 API 地址。

## 配置文件优先级

Vite 加载环境变量的优先级（从高到低）：

```
.env.[mode].local > .env.[mode] > .env.local > .env
```

## 配置文件说明

### 1. `.env` - 公共配置
- **所有环境都会加载**
- 用于存放通用的、不敏感的配置
- 会被提交到 git

### 2. `.env.development` - 开发环境
- **开发模式**（`npm run dev`）自动加载
- 用于本地开发
- 会被提交到 git

### 3. `.env.production` - 生产环境
- **生产模式**（`npm run build`）自动加载
- 用于构建生产版本
- 会被提交到 git

### 4. `.env.local` - 本地覆盖（可选）
- **所有环境都会加载，优先级最高**
- 用于本地开发时覆盖配置
- **不会被提交到 git**（已添加到 .gitignore）

### 5. `.env.[mode].local` - 特定模式本地覆盖（可选）
- 如 `.env.development.local`、`.env.production.local`
- 优先级最高
- **不会被提交到 git**

## 当前配置

### 开发环境 `.env.development`
```env
VITE_API_BASE_URL=http://localhost:8000/api
VITE_APP_ENV=development
VITE_DEBUG=true
```

### 生产环境 `.env.production`
```env
VITE_API_BASE_URL=https://your-production-api.com/api
VITE_APP_ENV=production
VITE_DEBUG=false
```

## 使用方法

### 开发环境

```bash
# 会自动加载 .env.development
npm run dev
```

控制台输出：
```
🚀 当前运行模式: development
🔗 API 基础地址: http://localhost:8000/api
```

### 生产构建

```bash
# 会自动加载 .env.production
npm run build
```

### 本地临时覆盖（不提交到 git）

```bash
# 创建本地覆盖配置
cp .env.local.example .env.local

# 修改 .env.local
VITE_API_BASE_URL=http://192.168.1.100:8000/api

# 运行（会优先使用 .env.local 的配置）
npm run dev
```

## 在代码中使用

### 基础使用

```javascript
import config from '@/config/api.config.js'

// 获取当前环境
console.log(config.env)        // 'development' 或 'production'
console.log(config.isDev)        // true 或 false
console.log(config.isProd)     // false 或 true
console.log(config.debug)      // true 或 false

// 获取 API 地址
console.log(config.baseURL)      // http://localhost:8000/api
console.log(config.api.chatSend) // http://localhost:8000/api/chat/send
```

### 环境判断

```javascript
import config from '@/config/api.config.js'

if (config.isDev) {
  console.log('开发环境')
}

if (config.isProd) {
  console.log('生产环境')
}
```

### 调试模式

```javascript
import config from '@/config/api.config.js'

if (config.debug) {
  console.log('调试信息...')
}
```

## 切换环境示例

### 场景 1：开发时连接测试服务器

```bash
# 方法 1：修改 .env.development
VITE_API_BASE_URL=http://192.168.1.100:8000/api

# 方法 2：使用 .env.local 覆盖（推荐）
echo "VITE_API_BASE_URL=http://192.168.1.100:8000/api" > .env.local
```

### 场景 2：生产部署到不同环境

```bash
# 构建前设置环境变量
export VITE_API_BASE_URL=https://api-prod.yoursite.com/api
npm run build

# 或者修改 .env.production 后构建
npm run build
```

### 场景 3：本地开发使用生产 API（谨慎）

```bash
# 创建开发环境本地覆盖
echo "VITE_API_BASE_URL=https://api-prod.yoursite.com/api" > .env.development.local
npm run dev
```

## 自定义环境变量

添加新的环境变量：

1. 在 `.env` 文件中添加（所有环境可用）：
```env
VITE_MY_CUSTOM_VAR=value
```

2. 必须以 `VITE_` 开头才能在前端访问

3. 在代码中使用：
```javascript
const myVar = import.meta.env.VITE_MY_CUSTOM_VAR
```

## 注意事项

1. **安全性**
   - 不要在前端暴露敏感信息（如密钥）
   - 环境变量会在构建时被打包到代码中

2. **变量命名**
   - 必须以 `VITE_` 开头
   - 例如：`VITE_API_BASE_URL` ✅
   - 例如：`API_BASE_URL` ❌（无法访问）

3. **修改后重启**
   - 修改 `.env` 文件后需要重启开发服务器
   - 生产构建需要重新执行 `npm run build`

4. **Git 提交**
   - `.env.local` 和 `.env.*.local` 不会被提交
   - `.env.development` 和 `.env.production` 会被提交

## 快速命令参考

```bash
# 开发（使用 .env.development）
npm run dev

# 构建生产（使用 .env.production）
npm run build

# 预览生产构建
npm run preview

# 创建本地覆盖配置
cp .env.local.example .env.local

# 查看当前配置
cat .env.development
```
