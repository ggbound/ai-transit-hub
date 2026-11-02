import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { Message } from '@arco-design/web-vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue'),
    meta: { requiresAuth: false }
  },
  {
    path: '/',
    name: 'UserLayout',
    component: () => import('../layouts/UserLayout.vue'),
    meta: { requiresAuth: true },
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'UserDashboard',
        component: () => import('../views/user/Dashboard.vue')
      },
      {
        path: 'chat',
        name: 'Chat',
        component: () => import('../views/user/Chat.vue')
      },
      {
        path: 'chat/:sessionId',
        name: 'ChatSession',
        component: () => import('../views/user/Chat.vue')
      },
      {
        path: 'api-keys',
        name: 'UserApiKeys',
        component: () => import('../views/user/ApiKeys.vue')
      },
      {
        path: 'skills',
        name: 'UserSkills',
        component: () => import('../views/user/Skills.vue')
      },
      {
        path: 'docs',
        name: 'ApiDocs',
        component: () => import('../views/user/ApiDocs.vue')
      },
      {
        path: 'test',
        name: 'ApiTest',
        component: () => import('../views/user/ApiTest.vue')
      },
      {
        path: 'image-test',
        name: 'ImageTest',
        component: () => import('../views/user/ImageTest.vue')
      },
      {
        path: 'video-test',
        name: 'VideoTest',
        component: () => import('../views/user/VideoTest.vue')
      },
      {
        path: 'logs',
        name: 'UserLogs',
        component: () => import('../views/user/Logs.vue')
      },
      // {
      //   path: 'leaderboards',
      //   name: 'Leaderboards',
      //   component: () => import('../views/user/Leaderboards.vue')
      // },
      {
        path: 'profile',
        name: 'UserProfile',
        component: () => import('../views/user/Profile.vue')
      }
    ]
  },
  {
    path: '/admin',
    name: 'AdminLayout',
    component: () => import('../layouts/AdminLayout.vue'),
    meta: { requiresAuth: true, requiresAdmin: true },
    redirect: '/admin/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'AdminDashboard',
        component: () => import('../views/admin/Dashboard.vue')
      },
      {
        path: 'ai-providers',
        name: 'AiProviders',
        component: () => import('../views/admin/AiProviders.vue')
      },
      {
        path: 'models',
        name: 'AdminModels',
        component: () => import('../views/admin/Models.vue')
      },
      {
        path: 'users',
        name: 'AdminUsers',
        component: () => import('../views/admin/Users.vue')
      },
      {
        path: 'tools',
        name: 'AdminTools',
        component: () => import('../views/admin/Tools.vue')
      },
      {
        path: 'skills',
        name: 'AdminSkills',
        component: () => import('../views/admin/Skills.vue')
      },
      {
        path: 'logs',
        name: 'AdminLogs',
        component: () => import('../views/admin/Logs.vue')
      },
      {
        path: 'leaderboards',
        name: 'AdminLeaderboards',
        component: () => import('../views/admin/Leaderboards.vue')
      },
      {
        path: 'billing',
        name: 'AdminBilling',
        component: () => import('../views/admin/Billing.vue')
      },
      {
        path: 'settings',
        name: 'AdminSettings',
        component: () => import('../views/admin/Settings.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()
  const token = localStorage.getItem('token')

  // 如果有token但没有用户信息，获取用户信息
  if (token && !authStore.user) {
    try {
      await authStore.fetchUser()
    } catch (error) {
      console.error(error)
      // 获取用户信息失败，清除token
      localStorage.removeItem('token')
      next('/login')
      return
    }
  }

  // 需要登录但未登录
  if (to.meta.requiresAuth && !token) {
    next('/login')
    return
  }
  
  // 需要管理员权限但不是管理员
  if (to.meta.requiresAdmin && token) {
    // 等待用户信息加载完成
    if (!authStore.user) {
      try {
        await authStore.fetchUser()
      } catch (error) {
        console.error(error)
        next('/login')
        return
      }
    }
    
    // 检查是否是管理员
    if (!authStore.isAdmin) {
      Message.error('您没有权限访问管理后台')
      next('/')
      return
    }
  }
  
  // 已登录用户访问登录页，重定向到首页
  if (to.path === '/login' && token) {
    next('/')
    return
  }
  
  next()
})

export default router
