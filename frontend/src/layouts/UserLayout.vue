<template>
  <a-layout class="layout-container">
    <!-- 侧边栏 -->
    <a-layout-sider
      theme="dark"
      breakpoint="lg"
      :width="240"
      :collapsed-width="64"
      collapsible
      :collapsed="collapsed"
      @collapse="onCollapse"
      hide-trigger
      class="sider"
    >
      <div class="logo">
        <div v-if="!collapsed" class="logo-text">
          <div class="logo-mark">
            <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect width="40" height="40" rx="12" fill="url(#grad)"/>
              <path d="M12 14L20 28L28 14" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
              <circle cx="20" cy="18" r="3" fill="white"/>
              <defs>
                <linearGradient id="grad" x1="0" y1="0" x2="40" y2="40">
                  <stop offset="0%" stop-color="#165DFF"/>
                  <stop offset="100%" stop-color="#722ED1"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <span class="logo-title">AI Transit Hub</span>
        </div>
        <div v-else class="logo-mark">
          <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect width="40" height="40" rx="12" fill="url(#grad2)"/>
            <path d="M12 14L20 28L28 14" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
            <circle cx="20" cy="18" r="3" fill="white"/>
            <defs>
              <linearGradient id="grad2" x1="0" y1="0" x2="40" y2="40">
                <stop offset="0%" stop-color="#165DFF"/>
                <stop offset="100%" stop-color="#722ED1"/>
              </linearGradient>
            </defs>
          </svg>
        </div>
      </div>
      <a-menu
        :selected-keys="[currentRoute]"
        @menu-item-click="onClickMenuItem"
        class="menu"
        auto-open-selected
      >
        <a-menu-item key="/dashboard">
          <template #icon><icon-dashboard /></template>
          <span>仪表盘</span>
        </a-menu-item>
        <!-- 对话菜单已隐藏 -->
        <a-menu-item v-if="false" key="/chat">
          <template #icon><icon-message /></template>
          <span>对话</span>
        </a-menu-item>
        <a-menu-item key="/api-keys">
            <template #icon><icon-lock /></template>
            <span>API密钥</span>
          </a-menu-item>
        <a-menu-item key="/skills">
          <template #icon><icon-bulb /></template>
          <span>SKILL</span>
        </a-menu-item>
        <a-menu-item key="/docs">
          <template #icon><icon-book /></template>
          <span>API文档</span>
        </a-menu-item>
        <a-menu-item key="/test">
          <template #icon><icon-code /></template>
          <span>接口测试</span>
        </a-menu-item>
        <a-menu-item key="/image-test">
          <template #icon><icon-image /></template>
          <span>图片生成</span>
        </a-menu-item>
        <a-menu-item key="/video-test">
          <template #icon><icon-play-circle /></template>
          <span>视频生成</span>
        </a-menu-item>
        <a-menu-item key="/logs">
          <template #icon><icon-history /></template>
          <span>使用日志</span>
        </a-menu-item>
        <!-- <a-menu-item key="/leaderboards">
          <template #icon><icon-trophy /></template>
          <span>排行榜</span>
        </a-menu-item> -->
      </a-menu>
    </a-layout-sider>

    <!-- 主内容区 -->
    <a-layout class="main-layout" :style="{ marginLeft: collapsed ? '64px' : '240px' }">
      <!-- 顶部导航 -->
      <a-layout-header class="top-header">
        <div class="header-left">
          <a-button type="text" @click="onCollapse" class="collapse-btn">
            <icon-menu-unfold v-if="collapsed" />
            <icon-menu-fold v-else />
          </a-button>
          <a-breadcrumb class="breadcrumb">
            <a-breadcrumb-item>
            <template #icon><icon-home /></template>
            首页
            </a-breadcrumb-item>
            <a-breadcrumb-item>{{ currentPageTitle }}</a-breadcrumb-item>
          </a-breadcrumb>
        </div>
        <div class="header-right">
          <a-space>
            <a-button type="text" class="header-btn">
              <template #icon><icon-notification /></template>
            </a-button>
            <a-dropdown>
              <template #content>
                <a-doption @click="goToProfile">
                  <template #icon><icon-user /></template>
                  个人中心
                </a-doption>
                <a-doption v-if="authStore.isAdmin" @click="goToAdmin">
                  <template #icon><icon-settings /></template>
                  管理后台
                </a-doption>
                <a-doption @click="handleLogout">
                  <template #icon><icon-export /></template>
                  退出登录
                </a-doption>
              </template>
              <a-button type="text" class="user-btn">
                <a-space>
                  <a-avatar :size="32" :style="{ backgroundColor: '#165DFF' }">
                    {{ (authStore.user?.name || '用户').charAt(0).toUpperCase() }}
                  </a-avatar>
                  <span class="username">{{ authStore.user?.name || '用户' }}</span>
                  <icon-down />
                </a-space>
              </a-button>
            </a-dropdown>
          </a-space>
        </div>
      </a-layout-header>

      <!-- 内容区 -->
      <a-layout-content :class="['page-content', { 'chat-page': isChatPage }]">
        <router-view />
      </a-layout-content>

      <!-- 页脚 - 聊天页面不显示 -->
      <a-layout-footer v-if="!isChatPage" class="page-footer">
        <div class="footer-content">
          <span>© 2026 AI Transit Hub. All rights reserved.</span>
          <a-space class="footer-links">
            <span>v1.0.0</span>
          </a-space>
        </div>
      </a-layout-footer>
    </a-layout>
  </a-layout>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import {
  IconDashboard,
  IconMessage,
  IconLock,
  IconBulb,
  IconBook,
  IconCode,
  IconImage,
  IconPlayCircle,
  IconHistory,
  IconTrophy,
  IconMenuFold,
  IconMenuUnfold,
  IconNotification,
  IconUser,
  IconSettings,
  IconExport,
  IconDown,
  IconHome
} from '@arco-design/web-vue/es/icon'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const collapsed = ref(false)

const currentRoute = computed(() => {
  if (route.path.startsWith('/chat')) {
    return '/chat'
  }
  return route.path
})

const isChatPage = computed(() => {
  return route.path.startsWith('/chat')
})

const currentPageTitle = computed(() => {
  const titles = {
    '/dashboard': '仪表盘',
    '/chat': '对话',
    '/api-keys': 'API密钥',
    '/skills': 'SKILL',
    '/docs': 'API文档',
    '/test': '接口测试',
    '/image-test': '图片生成',
    '/video-test': '视频生成',
    '/logs': '使用日志',
    '/leaderboards': '排行榜',
    '/profile': '个人中心'
  }
  return titles[currentRoute.value] || '首页'
})

function onCollapse() {
  collapsed.value = !collapsed.value
}

function onClickMenuItem(key) {
  router.push(key)
}

async function handleLogout() {
  await authStore.logout()
  router.push('/login')
}

function goToAdmin() {
  router.push('/admin')
}

function goToProfile() {
  router.push('/profile')
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
  width: 100%;
}

.sider {
  height: 100vh;
  position: fixed;
  left: 0;
  top: 0;
  bottom: 0;
  z-index: 100;
}

.logo {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 20px;
}

.logo-mark {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-text {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
}

.logo-title {
  font-size: 16px;
  font-weight: 600;
  color: white;
}

.menu {
  height: calc(100vh - 64px);
  overflow-y: auto;
}

.main-layout {
  min-height: 100vh;
  transition: margin-left 0.3s;
  display: flex;
  flex-direction: column;
}

/* 顶部导航 */
.top-header {
  position: sticky;
  top: 0;
  z-index: 99;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  padding: 0 24px;
  background: #fff;
  border-bottom: 1px solid var(--border-color);
}

.header-left {
  display: flex;
  align-items: center;
}

.collapse-btn {
  margin-right: 16px;
}

.breadcrumb {
  margin-left: 16px;
  font-size: 14px;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-btn {
  padding: 6px 12px;
  border-radius: 8px;
}

.username {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

/* 内容区 */
.page-content {
  padding: 24px;
  flex: 1;
  background: var(--bg-secondary);
  overflow-y: auto;
}

.page-content.chat-page {
  padding: 0;
  display: flex;
  flex-direction: column;
}

/* 页脚 */
.page-footer {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 56px;
  background: #fff;
  border-top: 1px solid var(--border-color);
  color: var(--text-tertiary);
  font-size: 13px;
}

.footer-content {
  display: flex;
  align-items: center;
  gap: 24px;
}

.footer-links {
  color: var(--text-tertiary);
}

/* 响应式 */
@media (max-width: 768px) {
  .sider {
    transform: translateX(-100%);
    transition: transform 0.3s;
  }

  .sider:not(.arco-layout-sider-collapsed) {
    transform: translateX(0);
  }

  .main-layout {
    margin-left: 0 !important;
  }

  .page-content {
    padding: 16px;
  }

  .page-content.chat-page {
    padding: 0;
  }

  .top-header {
    padding: 0 16px;
  }
}
</style>

