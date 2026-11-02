<template>
  <a-layout class="admin-layout">
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
              <rect width="40" height="40" rx="12" fill="url(#adminGrad)"/>
              <path d="M12 14L20 28L28 14" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
              <circle cx="20" cy="18" r="3" fill="white"/>
              <defs>
                <linearGradient id="adminGrad" x1="0" y1="0" x2="40" y2="40">
                  <stop offset="0%" stop-color="#FF7D00"/>
                  <stop offset="100%" stop-color="#F53F3F"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
          <span class="logo-title">管理后台</span>
        </div>
        <div v-else class="logo-mark">
          <svg viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect width="40" height="40" rx="12" fill="url(#adminGrad2)"/>
            <path d="M12 14L20 28L28 14" stroke="white" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>
            <circle cx="20" cy="18" r="3" fill="white"/>
            <defs>
              <linearGradient id="adminGrad2" x1="0" y1="0" x2="40" y2="40">
                <stop offset="0%" stop-color="#FF7D00"/>
                <stop offset="100%" stop-color="#F53F3F"/>
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
        <a-menu-item key="/admin/dashboard">
          <template #icon><icon-dashboard /></template>
          <span>仪表盘</span>
        </a-menu-item>
        
        <a-sub-menu key="ai-management">
          <template #icon><icon-robot /></template>
          <template #title>
            <span>AI管理</span>
          </template>
          <a-menu-item key="/admin/ai-providers">
            <template #icon><icon-apps /></template>
            <span>提供商配置</span>
          </a-menu-item>
          <a-menu-item key="/admin/models">
            <template #icon><icon-apps /></template>
            <span>模型管理</span>
          </a-menu-item>
          <a-menu-item key="/admin/tools">
            <template #icon><icon-tool /></template>
            <span>工具管理</span>
          </a-menu-item>
          <a-menu-item key="/admin/skills">
            <template #icon><icon-bulb /></template>
            <span>Skill 管理</span>
          </a-menu-item>
        </a-sub-menu>
        
        <a-sub-menu key="user-management">
          <template #icon><icon-user-group /></template>
          <template #title>
            <span>用户管理</span>
          </template>
          <a-menu-item key="/admin/users">
            <template #icon><icon-user /></template>
            <span>用户列表</span>
          </a-menu-item>
          <!-- <a-menu-item key="/admin/leaderboards">
            <template #icon><icon-trophy /></template>
            <span>排行榜配置</span>
          </a-menu-item> -->
        </a-sub-menu>
        
        <a-sub-menu key="system">
          <template #icon><icon-settings /></template>
          <template #title>
            <span>系统管理</span>
          </template>
          <!-- <a-menu-item key="/admin/billing">
            <template #icon><icon-file /></template>
            <span>计费管理</span>
          </a-menu-item> -->
          <a-menu-item key="/admin/logs">
            <template #icon><icon-history /></template>
            <span>日志审计</span>
          </a-menu-item>
          <a-menu-item key="/admin/settings">
            <template #icon><icon-safe /></template>
            <span>系统配置</span>
          </a-menu-item>
        </a-sub-menu>
      </a-menu>
    </a-layout-sider>
    
    <a-layout class="main-layout" :style="{ marginLeft: collapsed ? '64px' : '240px' }">
      <a-layout-header class="top-header">
        <div class="header-left">
          <a-button type="text" @click="onCollapse" class="collapse-btn">
            <icon-menu-unfold v-if="collapsed" />
            <icon-menu-fold v-else />
          </a-button>
          <a-breadcrumb class="breadcrumb">
            <a-breadcrumb-item>
            <template #icon><icon-home /></template>
            管理后台
            </a-breadcrumb-item>
            <a-breadcrumb-item>{{ currentPageTitle }}</a-breadcrumb-item>
          </a-breadcrumb>
        </div>
        <div class="header-right">
          <a-space>
            <a-button type="text" @click="goToUser" class="back-btn">
              <template #icon><icon-launch /></template>
              返回用户端
            </a-button>
            <a-dropdown>
              <template #content>
                <a-doption @click="goToProfile">
                  <template #icon><icon-user /></template>
                  个人中心
                </a-doption>
                <a-doption @click="handleLogout">
                  <template #icon><icon-export /></template>
                  退出登录
                </a-doption>
              </template>
              <a-button type="text" class="user-btn">
                <a-space>
                  <a-avatar :size="32" :style="{ backgroundColor: '#FF7D00' }">
                    {{ authStore.user?.name?.charAt(0)?.toUpperCase() || 'A' }}
                  </a-avatar>
                  <span class="username">{{ authStore.user?.name || '管理员' }}</span>
                  <a-tag v-if="authStore.isAdmin" color="red" size="small">管理员</a-tag>
                  <icon-down />
                </a-space>
              </a-button>
            </a-dropdown>
          </a-space>
        </div>
      </a-layout-header>
      
      <a-layout-content class="page-content">
        <router-view />
      </a-layout-content>
      
      <a-layout-footer class="page-footer">
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
import { ref, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { Message } from '@arco-design/web-vue'
import {
  IconDashboard,
  IconRobot,
  IconApps,
  IconTool,
  IconBulb,
  IconUserGroup,
  IconUser,
  IconTrophy,
  IconSettings,
  IconHistory,
  IconMenuFold,
  IconMenuUnfold,
  IconLaunch,
  IconExport,
  IconDown,
  IconHome,
  IconFile,
  IconSafe
} from '@arco-design/web-vue/es/icon'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const collapsed = ref(false)

onMounted(async () => {
  if (!authStore.user) {
    await authStore.fetchUser()
  }
  
  if (!authStore.isAdmin) {
    Message.error('您没有权限访问管理后台')
    router.push('/')
  }
})

const currentRoute = computed(() => route.path)

const currentPageTitle = computed(() => {
  const titles = {
    '/admin/dashboard': '仪表盘',
    '/admin/ai-providers': '提供商配置',
    '/admin/models': '模型管理',
    '/admin/tools': '工具管理',
    '/admin/skills': 'Skill 管理',
    '/admin/users': '用户列表',
    '/admin/leaderboards': '排行榜配置',
    '/admin/billing': '计费管理',
    '/admin/logs': '日志审计',
    '/admin/settings': '系统配置'
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

function goToUser() {
  router.push('/')
}

function goToProfile() {
  router.push('/profile')
}
</script>

<style scoped>
.admin-layout {
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

.back-btn {
  color: var(--text-secondary);
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
  color: var(--text-primary);
}

.page-content {
  padding: 24px;
  flex: 1;
  background: var(--bg-secondary);
  overflow-y: auto;
}

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

  .top-header {
    padding: 0 16px;
  }
}
</style>

