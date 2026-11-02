<template>
  <div class="logs-container">
    <div class="page-header">
      <div>
        <h1>使用日志</h1>
        <p>查看您的 API 调用记录和费用明细</p>
      </div>
    </div>

    <a-card class="content-card">
      <a-table
        :data="usageLogs"
        :loading="usageLoading"
        :pagination="{
          showTotal: true,
          showJumper: true,
          total: usagePagination.total
        }"
        :page="usagePagination.page"
        :pageSize="usagePagination.pageSize"
        @page-change="handleUsagePageChange"
        @page-size-change="handlePageSizeChange"
        :scroll="{ x: 1000 }"
      >
        <template #columns>
          <a-table-column title="时间" data-index="created_at" width="180" fixed="left">
            <template #cell="{ record }">
              <div class="time-cell">
                <div class="time-main">{{ formatDate(record.created_at) }}</div>
                <div class="time-sub">{{ formatDateTime(record.created_at) }}</div>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="模型" data-index="model" width="280">
            <template #cell="{ record }">
              <div class="model-cell">
                <div class="model-main">
                  <a-tag v-if="record.model && record.model.provider" :color="getProviderColor(record.model.provider?.code)" size="small">
                    {{ record.model.provider.name }}
                  </a-tag>
                  <span class="model-name">
                    {{ record.model?.display_name || record.model?.code || '-' }}
                  </span>
                </div>
                <div class="model-type" v-if="record.model?.type">
                  <a-tag :color="getModelTypeColor(record.model.type)" size="small">
                    {{ getModelTypeName(record.model.type) }}
                  </a-tag>
                </div>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="输入Token" data-index="prompt_tokens" width="120">
            <template #cell="{ record }">
              <span class="token-count">{{ formatNumber(record.prompt_tokens) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="输出Token" data-index="completion_tokens" width="120">
            <template #cell="{ record }">
              <span class="token-count">{{ formatNumber(record.completion_tokens) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="总Token" data-index="total_tokens" width="100">
            <template #cell="{ record }">
              <span class="token-count total">{{ formatNumber(record.total_tokens) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="状态" data-index="status" width="100" fixed="right">
            <template #cell="{ record }">
              <a-tag v-if="record.status === 0" color="red" :bordered="false">失败</a-tag>
              <a-tag v-else-if="record.status === 1" color="green" :bordered="false">成功</a-tag>
              <a-tag v-else-if="record.status === 2" color="arcoblue" :bordered="false">处理中</a-tag>
              <a-tag v-else color="grey" :bordered="false">未知</a-tag>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../utils/api'

const usageLogs = ref([])
const usageLoading = ref(false)
const usagePagination = ref({ page: 1, pageSize: 10, total: 0, showTotal: true, showJumper: true })

onMounted(() => {
  loadUsageLogs()
})

async function loadUsageLogs() {
  usageLoading.value = true
  try {
    const response = await api.get('/user/logs/usage', {
      params: { page: usagePagination.value.page, per_page: usagePagination.value.pageSize }
    })
    if (response.data.success) {
      usageLogs.value = response.data.data.data || []
      usagePagination.value.total = response.data.data.meta?.total || usageLogs.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    usageLoading.value = false
  }
}

function handleUsagePageChange(page) {
  usagePagination.value.page = page
  loadUsageLogs()
}

function handlePageSizeChange(pageSize) {
  usagePagination.value.pageSize = pageSize
  usagePagination.value.page = 1
  loadUsageLogs()
}

const providerColorMap = {
  openai: 'arcoblue',
  anthropic: 'purple',
  'aliyun-chat': 'green',
  'deepseek': 'red',
  'baidu-qianfan': 'orange',
  'tencent': 'cyan',
  'huawei-pangu': 'gray'
}

const modelTypeColorMap = {
  text: 'arcoblue',
  embedding: 'cyan',
  image: 'teal',
  video: 'red',
  audio: 'pink'
}

const modelTypeNameMap = {
  text: '文本',
  embedding: '向量',
  image: '图片',
  video: '视频',
  audio: '音频'
}

function formatDate(date) {
  if (!date) return ''
  const d = new Date(date)
  const now = new Date()
  const diff = now - d

  // 小于1小时显示"xx分钟前"
  if (diff < 3600000) {
    const minutes = Math.floor(diff / 60000)
    return minutes < 1 ? '刚刚' : `${minutes}分钟前`
  }
  // 小于24小时显示"xx小时前"
  if (diff < 86400000) {
    return `${Math.floor(diff / 3600000)}小时前`
  }
  // 小于7天显示"xx天前"
  if (diff < 604800000) {
    return `${Math.floor(diff / 86400000)}天前`
  }

  // 否则显示日期
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit'
  }).replace(/\//g, '-')
}

function formatDateTime(date) {
  if (!date) return ''
  const d = new Date(date)
  return d.toLocaleString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function formatCost(cost) {
  if (!cost && cost !== 0) return '-'
  const num = Number(cost)
  if (isNaN(num)) return '-'
  return `$${num.toFixed(4)}`
}

function getProviderColor(providerCode) {
  return providerColorMap[providerCode] || 'arcoblue'
}

function getModelTypeColor(modelType) {
  return modelTypeColorMap[modelType] || 'arcoblue'
}

function getModelTypeName(modelType) {
  return modelTypeNameMap[modelType] || modelType
}

function formatNumber(num) {
  if (!num && num !== 0) return '0'
  const n = Number(num)
  if (isNaN(n)) return '0'
  return n.toLocaleString('en-US')
}
</script>

<style scoped>
.logs-container {
  padding: 0;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h1 {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.page-header p {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
}

.content-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
}

.time-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.time-main {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.time-sub {
  font-size: 12px;
  color: var(--text-tertiary);
}

.model-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.model-main {
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.model-type {
  margin-top: 0;
}

.token-count {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.token-count.total {
  color: var(--primary-6);
  font-weight: 600;
}

.cost-text {
  font-size: 14px;
  font-weight: 600;
  color: var(--success-6);
}
</style>
