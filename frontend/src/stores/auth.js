import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../utils/api'

// 解析 JWT token 的函数
function parseJwt(token) {
  try {
    const base64Url = token.split('.')[1];
    if (!base64Url) return null;
    
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
    const jsonPayload = decodeURIComponent(atob(base64).split('').map(function(c) {
      return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
    }).join(''));
    
    return JSON.parse(jsonPayload);
  } catch (error) {
    console.log('解析 JWT token 失败:', error);
    return null;
  }
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref(null)
  const token = ref(localStorage.getItem('token') || '')

  // 解析 token 的过期时间
  const tokenExpiry = computed(() => {
    if (!token.value) return null;
    
    const decoded = parseJwt(token.value);
    if (decoded && decoded.exp) {
      return new Date(decoded.exp * 1000);
    }
    return null;
  });

  // 计算 token 是否即将过期（5分钟内）
  const isTokenExpiringSoon = computed(() => {
    if (!tokenExpiry.value) return false;
    
    const now = new Date();
    const fiveMinutesLater = new Date(now.getTime() + 5 * 60 * 1000);
    
    return tokenExpiry.value < fiveMinutesLater;
  });

  // 计算 token 剩余时间（秒）
  const tokenRemainingTime = computed(() => {
    if (!tokenExpiry.value) return 0;
    
    const now = new Date();
    const diffInMs = tokenExpiry.value - now;
    return Math.max(0, Math.floor(diffInMs / 1000));
  });

  const isAuthenticated = computed(() => {
    if (!token.value) return false;
    
    // 检查 token 是否过期
    if (tokenExpiry.value) {
      const now = new Date();
      if (tokenExpiry.value < now) {
        console.log('Token 已过期');
        return false;
      }
    }
    
    return true;
  });
  
  // 是否是管理员
  const isAdmin = computed(() => {
    return user.value?.roles?.some(r => r.name === 'admin') || false
  })
  
  // 是否是普通用户
  const isUser = computed(() => {
    return user.value?.roles?.some(r => r.name === 'user') || false
  })
  
  // 检查是否有某个角色
  function hasRole(roleName) {
    return user.value?.roles?.some(r => r.name === roleName) || false
  }
  
  // 检查是否有任意一个角色
  function hasAnyRole(roles) {
    return roles.some(role => hasRole(role))
  }

  // 格式化剩余时间为可读格式
  function formatRemainingTime(seconds) {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    
    if (hours > 0) {
      return `${hours}小时${minutes}分钟`;
    } else if (minutes > 0) {
      return `${minutes}分钟${secs}秒`;
    } else {
      return `${secs}秒`;
    }
  }

  async function login(credentials) {
    try {
      const response = await api.post('/login', credentials)
      if (response.data.success) {
        user.value = response.data.data.user
        token.value = response.data.data.token
        localStorage.setItem('token', token.value)
        
        // 调试信息
        const decoded = parseJwt(token.value);
        console.log('登录成功，Token 信息:', {
          user: response.data.data.user?.email,
          tokenExpiry: decoded?.exp ? new Date(decoded.exp * 1000) : 'unknown',
          decodedPayload: decoded
        });
        return response.data
      } else {
        // 登录失败，抛出错误
        throw new Error(response.data.message || '登录失败')
      }
    } catch (error) {
      // 如果是 axios 错误，从响应中提取错误信息
      if (error.response && error.response.data) {
        throw new Error(error.response.data.message || '登录失败')
      }
      throw error
    }
  }

  async function register(data) {
    const response = await api.post('/register', data)
    if (response.data.success) {
      user.value = response.data.data.user
      token.value = response.data.data.token
      localStorage.setItem('token', token.value)
    }
    return response.data
  }

  async function logout() {
    try {
      await api.post('/logout')
    } catch (error) {
      console.error(error)
    } finally {
      user.value = null
      token.value = ''
      localStorage.removeItem('token')
    }
  }

  async function fetchUser() {
    try {
      const response = await api.get('/user')
      if (response.data.success) {
        user.value = response.data.data
      }
    } catch (error) {
      console.error(error)
    }
  }

  // 更新用户信息
  async function updateUserInfo(userData) {
    try {
      // 更新本地状态
      if (user.value) {
        Object.assign(user.value, userData)
      }
      return true
    } catch (error) {
      console.error('更新用户信息失败:', error)
      return false
    }
  }

  return {
    user,
    token,
    isAuthenticated,
    isAdmin,
    isUser,
    hasRole,
    hasAnyRole,
    login,
    register,
    logout,
    fetchUser,
    updateUserInfo,
    tokenExpiry,
    isTokenExpiringSoon,
    tokenRemainingTime,
    formatRemainingTime
  }
})
