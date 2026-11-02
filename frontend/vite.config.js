import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // 加载当前模式的环境变量
  const env = loadEnv(mode, process.cwd(), '')
  
  // 获取 API 基础地址
  const apiBaseUrl = env.VITE_API_BASE_URL || 'http://localhost:8000/api'
  
  // 在控制台输出当前配置
  console.log(`\n🚀 当前运行模式: ${mode}`)
  console.log(`🔗 API 基础地址: ${apiBaseUrl}\n`)
  
  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    // 确保环境变量在前端可用
    define: {
      __API_BASE_URL__: JSON.stringify(apiBaseUrl),
      __APP_ENV__: JSON.stringify(mode),
    },
  }
})
