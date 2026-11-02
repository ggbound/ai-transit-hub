import axios from 'axios'
import { Message } from '@arco-design/web-vue'
import config from '@/config/api.config.js'

const api = axios.create({
  baseURL: config.baseURL,
  timeout: config.timeout
})

api.interceptors.request.use(
  config => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

api.interceptors.response.use(
  response => {
    return response
  },
  error => {
    if (error.response) {
      const { status, data, config } = error.response
      // 登录接口的 401 不做特殊处理，让调用方自己处理
      if (status === 401 && !config.url.includes('/login')) {
        Message.error('未授权，请重新登录')
        localStorage.removeItem('token')
        window.location.href = '/login'
      } else if (status === 403) {
        Message.error('没有权限访问')
      } else if (status === 404) {
        Message.error('请求的资源不存在')
      } else if (status === 500) {
        Message.error('服务器错误')
      } else if (data.message) {
        // 对于其他错误，如果已经有消息就显示，否则让调用方处理
        if (!config.url.includes('/login')) {
          Message.error(data.message)
        }
      }
    } else {
      Message.error('网络错误')
    }
    return Promise.reject(error)
  }
)

export default api
