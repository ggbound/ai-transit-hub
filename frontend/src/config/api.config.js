// API 配置文件
// 从环境变量读取，如果没有则使用默认值

// 获取当前环境
const APP_ENV = import.meta.env.VITE_APP_ENV || 'development'
const IS_DEV = APP_ENV === 'development'
const IS_PROD = APP_ENV === 'production'

// 获取 API 基础地址
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8000/api'

// 调试模式
const DEBUG = import.meta.env.VITE_DEBUG === 'true' || IS_DEV

// 环境配置信息
export const envConfig = {
  // 当前环境
  env: APP_ENV,
  isDev: IS_DEV,
  isProd: IS_PROD,
  
  // 是否调试模式
  debug: DEBUG,
  
  // 基础 API 地址
  baseURL: API_BASE_URL,
}

// API 路径配置
export const apiPaths = {
  // 用户相关
  user: `${API_BASE_URL}/user`,
  auth: `${API_BASE_URL}/auth`,

  // 聊天相关
  chat: `${API_BASE_URL}/chat`,
  chatSend: `${API_BASE_URL}/chat/send`,
  sessions: `${API_BASE_URL}/chat/sessions`,
  messages: `${API_BASE_URL}/chat/messages`,

  // OpenAI 兼容接口
  openai: {
    base: `${API_BASE_URL}/v1`,
    chat: `${API_BASE_URL}/v1/chat/completions`,
    models: `${API_BASE_URL}/v1/models`,
    embeddings: `${API_BASE_URL}/v1/embeddings`,
    responses: `${API_BASE_URL}/v1/responses`,
  },

  // Anthropic 兼容接口
  anthropic: {
    base: `${API_BASE_URL}/v1`,
    messages: `${API_BASE_URL}/v1/messages`,
  },

  // 提供商和模型
  providers: `${API_BASE_URL}/admin/providers`,
  models: `${API_BASE_URL}/admin/models`,

  // 工具
  tools: `${API_BASE_URL}/tools`,

  // 使用日志
  logs: `${API_BASE_URL}/logs`,

  // 仪表盘
  dashboard: `${API_BASE_URL}/admin/dashboard`,

  // 文件上传
  upload: `${API_BASE_URL}/upload`,

  // OSS 上传配置
  oss: {
    uploadUrl: 'http://localhost:8080/file/upload',
    tokenId: 'zCoaBQKeQSlngJmCNREQXPypZ36cnMYm',
    tokenSecret: 'IVpuAm6hSbfimHbAz07ZXXLo5ffMHCnj',
  },
}

// 完整配置对象
export const config = {
  ...envConfig,
  api: apiPaths,
  
  // 超时配置
  timeout: 30000,

  // SSE 配置
  sse: {
    timeout: 120000,
  },
}

// 开发环境输出配置信息
if (DEBUG) {
  console.log('📋 API 配置信息:', {
    环境: APP_ENV,
    API地址: API_BASE_URL,
    调试模式: DEBUG,
  })
}

export default config
