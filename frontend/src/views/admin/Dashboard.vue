<template>
  <div class="dashboard">
    <div class="header-section">
      <h1>仪表盘</h1>
      <p>查看系统的运行统计和管理数据</p>
    </div>

    <div class="stat-cards">
      <div class="stat-card">
        <div class="stat-icon blue">
          <icon-user />
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ formatNumber(stats.totalUsers) }}</div>
          <div class="stat-label">总用户数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon green">
          <icon-robot />
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ formatNumber(stats.totalModels) }}</div>
          <div class="stat-label">模型总数</div>
        </div>
      </div>
      <div class="stat-card" style="display: none;">
        <div class="stat-icon orange">
          <icon-message />
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ formatNumber(stats.totalSessions) }}</div>
          <div class="stat-label">会话总数</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon purple">
          <icon-code />
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ displayTokens.shortValue }}</div>
          <div class="stat-label">总Token</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon cyan">
          <icon-fire />
        </div>
        <div class="stat-info">
          <div class="stat-value">{{ formatNumber(stats.todayCalls) }}</div>
          <div class="stat-label">今日调用</div>
        </div>
      </div>
    </div>

    <div class="content-row">
      <div class="content-card">
        <div class="card-title">热门模型 TOP5</div>
        <div class="list">
          <div v-for="(item, index) in topModels" :key="index" class="list-item">
            <div class="item-index" :class="index < 3 ? 'top' : ''">{{ index + 1 }}</div>
            <div class="item-content">
              <div class="item-name">{{ item.name }}</div>
              <div class="item-code">{{ item.full_code || item.code }}</div>
            </div>
            <div class="item-value">{{ formatNumber(item.call_count) }} 次</div>
          </div>
          <div v-if="topModels.length === 0" class="empty">暂无数据</div>
        </div>
      </div>

      <div class="content-card">
        <div class="card-title">活跃用户 TOP5</div>
        <div class="list">
          <div v-for="(item, index) in topUsers" :key="index" class="list-item">
            <div class="item-index" :class="index < 3 ? 'top' : ''">{{ index + 1 }}</div>
            <div class="item-content">
              <div class="item-name">{{ item.name }}</div>
              <div class="item-code">{{ item.email }}</div>
            </div>
            <div class="item-value">{{ formatNumber(item.tokens) }} T</div>
          </div>
          <div v-if="topUsers.length === 0" class="empty">暂无数据</div>
        </div>
      </div>

      <div class="content-card">
        <div class="card-title">系统状态</div>
        
        <div class="status-group">
          <div class="status-item">
            <div class="status-label">AI 提供商</div>
            <div class="status-progress">
              <div class="status-bar" :style="{ width: getPercent(stats.totalProviders, stats.totalProviders) + '%' }"></div>
            </div>
            <div class="status-text">{{ systemStatus.providers.enabled }} / {{ systemStatus.providers.total }}</div>
          </div>

          <div class="status-item">
            <div class="status-label">AI 模型</div>
            <div class="status-progress">
              <div class="status-bar" :style="{ width: getPercent(stats.totalModels, stats.totalModels) + '%' }"></div>
            </div>
            <div class="status-text">{{ systemStatus.models.enabled }} / {{ systemStatus.models.total }}</div>
          </div>
        </div>

        <div class="card-title" style="margin-top:24px">快捷操作</div>
        <div class="action-buttons">
          <a-button type="outline" @click="$router.push('/admin/ai-providers')" class="action-btn">
            <icon-apps />
            提供商管理
          </a-button>
          <a-button type="outline" @click="$router.push('/admin/models')" class="action-btn">
            <icon-robot />
            模型管理
          </a-button>
          <a-button type="outline" @click="$router.push('/admin/users')" class="action-btn">
            <icon-user />
            用户管理
          </a-button>
          <a-button type="outline" @click="$router.push('/admin/logs')" class="action-btn">
            <icon-history />
            日志审计
          </a-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import api from '../../utils/api'
import {
  IconUser,
  IconRobot,
  IconMessage,
  IconCode,
  IconFire,
  IconApps,
  IconHistory
} from '@arco-design/web-vue/es/icon'

const stats = ref({
  totalUsers: 0,
  totalModels: 0,
  totalSessions: 0,
  totalTokens: 0,
  totalCost: 0,
  todayCalls: 0,
  totalProviders: 0,
})

const topModels = ref([])
const topUsers = ref([])
const systemStatus = ref({
  providers: { total: 0, enabled: 0 },
  models: { total: 0, enabled: 0 },
})

const displayTokens = computed(() => {
  const num = Number(stats.value.totalTokens) || 0
  if (num >= 1000000) {
    return { shortValue: (num / 1000000).toFixed(1) + 'M' }
  }
  if (num >= 1000) {
    return { shortValue: (num / 1000).toFixed(1) + 'K' }
  }
  return { shortValue: num.toString() }
})

onMounted(async () => {
  await loadStats()
})

async function loadStats() {
  try {
    const res = await api.get('/admin/dashboard/stats')
    if (res.data.success) {
      const data = res.data.data
      stats.value = data.stats
      stats.value.totalProviders = data.systemStatus.providers.total
      topModels.value = data.topModels
      topUsers.value = data.topUsers
      systemStatus.value = data.systemStatus
    }
  } catch (error) {
    Message.error('加载统计数据失败')
  }
}

function formatNumber(num) {
  if (!num) return 0
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  }
  return num.toString()
}

function getPercent(enabled, total) {
  if (total === 0) return 0
  return Math.round((enabled / total) * 100)
}
</script>

<style scoped>
.dashboard {
  padding: 24px;
  background-color: #f5f7fa;
  min-height: 100vh;
}

.header-section {
  margin-bottom: 24px;
}

.header-section h1 {
  font-size: 24px;
  font-weight: 600;
  color: #1d2129;
  margin: 0 0 8px 0;
}

.header-section p {
  font-size: 14px;
  color: #86909c;
  margin: 0;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background-color: #ffffff;
  border: 1px solid #e5e6eb;
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ffffff;
  flex-shrink: 0;
}

.stat-icon.blue { background-color: #165dff; }
.stat-icon.green { background-color: #00b42a; }
.stat-icon.orange { background-color: #ff7d00; }
.stat-icon.purple { background-color: #722ed1; }
.stat-icon.cyan { background-color: #0fc6c2; }

.stat-info {
  flex: 1;
  min-width: 0;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #1d2129;
  margin-bottom: 8px;
  line-height: 1;
}

.stat-label {
  font-size: 14px;
  color: #86909c;
}

.content-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.content-card {
  background-color: #ffffff;
  border: 1px solid #e5e6eb;
  padding: 24px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #1d2129;
  margin-bottom: 20px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.list-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid #e5e6eb;
}

.item-index {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f2f3f5;
  color: #86909c;
  font-weight: 700;
  font-size: 14px;
  flex-shrink: 0;
}

.item-index.top {
  background-color: #ffd700;
  color: #1d2129;
}

.item-content {
  flex: 1;
  min-width: 0;
}

.item-name {
  font-size: 14px;
  font-weight: 500;
  color: #1d2129;
  margin-bottom: 4px;
}

.item-code {
  font-size: 12px;
  color: #86909c;
}

.item-value {
  font-size: 14px;
  font-weight: 600;
  color: #165dff;
  flex-shrink: 0;
}

.empty {
  text-align: center;
  padding: 32px;
  color: #86909c;
  font-size: 14px;
}

.status-group {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.status-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.status-label {
  font-size: 14px;
  color: #1d2129;
}

.status-progress {
  width: 100%;
  height: 8px;
  background-color: #e5e6eb;
}

.status-bar {
  height: 100%;
  background-color: #165dff;
  transition: width 0.3s ease;
}

.status-text {
  font-size: 13px;
  color: #86909c;
  text-align: right;
}

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 44px;
}

@media (max-width: 1400px) {
  .stat-cards {
    grid-template-columns: repeat(4, 1fr);
  }
  .content-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 1200px) {
  .stat-cards {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 900px) {
  .stat-cards {
    grid-template-columns: repeat(2, 1fr);
  }
  .content-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .dashboard {
    padding: 16px;
  }
  .stat-cards {
    grid-template-columns: 1fr;
  }
}
</style>
