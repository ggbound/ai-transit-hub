<template>
  <div class="leaderboards-container">
    <div class="page-header">
      <div>
        <h1>排行榜</h1>
        <p>查看各维度的用户排名情况</p>
      </div>
      <div class="page-actions">
        <a-button type="secondary" size="small">
          <template #icon><icon-refresh /></template>
          刷新数据
        </a-button>
      </div>
    </div>

    <a-row :gutter="24">
      <a-col :xs="24" :sm="24" :md="12" :lg="8" v-for="(leaderboard, index) in leaderboards" :key="index">
        <a-card class="leaderboard-card" :bordered="false">
          <template #title>
            <div class="card-title">
              <div class="title-icon" :class="`icon-${index + 1}`">
                <icon-trophy />
              </div>
              <div class="title-content">
                <div class="title-text">{{ leaderboard.config.display_name }}</div>
                <div class="title-desc">{{ leaderboard.config.description || '实时更新' }}</div>
              </div>
            </div>
          </template>
          
          <div class="leaderboard-list">
            <div 
              v-for="(item, idx) in leaderboard.data" 
              :key="item.user_id"
              class="rank-item"
              :class="`rank-${idx + 1}`"
            >
              <div class="rank-number" :class="getRankClass(idx)">
                {{ idx + 1 }}
              </div>
              
              <div class="user-info">
                <a-avatar 
                  :size="36" 
                  :style="{ backgroundColor: getAvatarColor(item.user_id) }"
                  class="user-avatar"
                >
                  {{ item.user_name?.charAt(0)?.toUpperCase() || 'U' }}
                </a-avatar>
                <div class="user-detail">
                  <div class="user-name">{{ item.user_name }}</div>
                </div>
              </div>
              
              <div class="score-section">
                <div class="score-value">{{ formatScore(item.total, leaderboard.config.metric) }}</div>
                <div class="score-label">{{ getMetricLabel(leaderboard.config.metric) }}</div>
              </div>
            </div>
            
            <div v-if="!leaderboard.data || leaderboard.data.length === 0" class="empty-state">
              <icon-file class="empty-icon" />
              <div class="empty-text">暂无排名数据</div>
            </div>
          </div>
        </a-card>
      </a-col>
    </a-row>
    
    <div v-if="isAllEmpty" class="global-empty">
      <div class="global-empty-wrapper">
        <icon-trophy class="global-empty-icon" />
        <div class="global-empty-title">暂无排行榜数据</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import { 
  IconTrophy, 
  IconRefresh, 
  IconFile
} from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'

const leaderboards = ref([])
const loading = ref(false)

const isAllEmpty = computed(() => {
  return leaderboards.value.every(lb => !lb.data || lb.data.length === 0)
})

onMounted(() => {
  loadLeaderboards()
})

async function loadLeaderboards() {
  loading.value = true
  try {
    const response = await api.get('/leaderboards')
    if (response.data.success) {
      leaderboards.value = response.data.data
    }
  } catch (error) {
    Message.error('加载排行榜失败')
    console.error(error)
  } finally {
    loading.value = false
  }
}

function getRankClass(index) {
  if (index === 0) return 'rank-gold'
  if (index === 1) return 'rank-silver'
  if (index === 2) return 'rank-bronze'
  return 'rank-normal'
}

function getAvatarColor(id) {
  const colors = [
    '#165DFF', '#00B42A', '#FF7D00', '#F53F3F', '#722ED1',
    '#13C2C2', '#EB2F96', '#FA541C', '#86909C', '#2F54FF'
  ]
  return colors[(id || 0) % colors.length]
}

function getMetricLabel(metric) {
  const labels = {
    'total_tokens': '总Token数',
    'total_cost': '总费用',
    'request_count': '调用次数'
  }
  return labels[metric] || metric
}

function formatScore(value, metric) {
  if (!value && value !== 0) return '-'
  
  switch (metric) {
    case 'total_cost':
      return `¥${Number(value).toFixed(4)}`
    case 'total_tokens':
      return Number(value).toLocaleString()
    case 'request_count':
      return `${Number(value).toLocaleString()} 次`
    default:
      return Number(value).toLocaleString()
  }
}
</script>

<style scoped>
.leaderboards-container {
  padding: 0;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.page-header h1 {
  font-size: 24px;
  font-weight: 600;
  color: #1D2129;
  margin: 0 0 6px 0;
}

.page-header p {
  font-size: 14px;
  color: #86909C;
  margin: 0;
}

.leaderboard-card {
  margin-bottom: 24px;
  border-radius: 8px;
  background: #FFFFFF;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.leaderboard-card :deep(.arco-card-header) {
  border-bottom: 1px solid #E5E6EB;
  padding: 0 20px;
  background: #FFFFFF;
  border-radius: 8px 8px 0 0;
  height: 60px;
  display: flex;
  align-items: center;
  box-sizing: border-box;
}

.leaderboard-card :deep(.arco-card-header-title) {
  height: 100%;
  display: flex;
  align-items: center;
  padding: 0;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.title-icon.icon-1 {
  background: linear-gradient(135deg, #FFF7E6 0%, #FFEFD3 100%);
  color: #FF7D00;
}

.title-icon.icon-2 {
  background: linear-gradient(135deg, #F7F8FA 0%, #F2F3F5 100%);
  color: #86909C;
}

.title-icon.icon-3 {
  background: linear-gradient(135deg, #FFF1E6 0%, #FFE7D3 100%);
  color: #CD7F32;
}

.title-content {
  flex: 1;
}

.title-text {
  font-size: 15px;
  font-weight: 600;
  color: #1D2129;
  margin-bottom: 4px;
}

.title-desc {
  font-size: 12px;
  color: #86909C;
}

.leaderboard-list {
  padding: 16px 0;
}

.rank-item {
  display: flex;
  align-items: center;
  padding: 14px 20px;
  border-bottom: 1px solid #F2F3F5;
  background: #FFFFFF;
  transition: background 0.2s;
}

.rank-item:last-child {
  border-bottom: none;
}

.rank-item:hover {
  background: #F7F8FA;
}

.rank-item.rank-1 {
  background: #FFF9E6;
}

.rank-item.rank-2 {
  background: #F7F8FA;
}

.rank-item.rank-3 {
  background: #FFF5EB;
}

.rank-number {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
  flex-shrink: 0;
}

.rank-gold {
  background: linear-gradient(135deg, #FFD700 0%, #FFA500 100%);
  color: #FFFFFF;
}

.rank-silver {
  background: linear-gradient(135deg, #C0C0C0 0%, #A8A8A8 100%);
  color: #FFFFFF;
}

.rank-bronze {
  background: linear-gradient(135deg, #CD7F32 0%, #B87333 100%);
  color: #FFFFFF;
}

.rank-normal {
  background: #F2F3F5;
  color: #4E5969;
  font-weight: 600;
}

.user-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-left: 12px;
  min-width: 0;
}

.user-avatar {
  flex-shrink: 0;
}

.user-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: #1D2129;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.score-section {
  text-align: right;
  min-width: 100px;
}

.score-value {
  font-size: 16px;
  font-weight: 700;
  color: #1D2129;
}

.score-gold {
  color: #FF7D00;
}

.score-silver {
  color: #86909C;
}

.score-bronze {
  color: #CD7F32;
}

.score-normal {
  color: #1D2129;
}

.score-label {
  font-size: 12px;
  color: #86909C;
  margin-top: 2px;
}

.empty-state {
  padding: 48px 20px;
  text-align: center;
}

.empty-icon {
  font-size: 40px;
  color: #C9CDD4;
  margin-bottom: 12px;
}

.empty-text {
  font-size: 14px;
  color: #86909C;
}

.global-empty {
  padding: 120px 20px;
  text-align: center;
}

.global-empty-wrapper {
  max-width: 400px;
  margin: 0 auto;
}

.global-empty-icon {
  font-size: 64px;
  color: #C9CDD4;
  margin-bottom: 16px;
}

.global-empty-title {
  font-size: 16px;
  font-weight: 500;
  color: #86909C;
}

@media (max-width: 1200px) {
  .leaderboards-container {
    padding: 0 16px;
  }
}

@media (max-width: 992px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
  
  .page-actions {
    width: 100%;
    justify-content: flex-start;
  }
}

@media (max-width: 768px) {
  .rank-item {
    padding: 12px 16px;
  }
  
  .user-info {
    margin-left: 10px;
    gap: 8px;
  }
  
  .score-section {
    min-width: 80px;
  }
  
  .score-value {
    font-size: 15px;
  }
  
  .leaderboard-card {
    margin-bottom: 20px;
  }
}
</style>
