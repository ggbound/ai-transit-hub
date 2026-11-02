<template>
  <div class="logs-container">
    <div class="page-header">
      <h1>日志管理</h1>
      <p>查看系统使用日志和操作日志</p>
    </div>
    
    <a-tabs v-model:active-key="activeTab" @change="handleTabChange">
      <a-tab-pane key="usage" title="使用日志">
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
            :scroll="{ x: 1200 }"
            @page-change="handleUsagePageChange"
            @page-size-change="handleUsagePageSizeChange"
          >
            <template #columns>
              <a-table-column title="用户" data-index="user" width="200" fixed="left">
                <template #cell="{ record }">
                  {{ record.user?.name }} ({{ record.user?.email }})
                </template>
              </a-table-column>
              <a-table-column title="时间" data-index="created_at" width="200">
                <template #cell="{ record }">
                  {{ formatDate(record.created_at) }}
                </template>
              </a-table-column>
              <a-table-column title="模型" data-index="model" width="200">
                <template #cell="{ record }">
                  <a-tag v-if="record.model && record.model.provider" color="arcoblue">
                    {{ record.model.provider.code }}/{{ record.model.code }}
                  </a-tag>
                  <span v-else>
                    {{ record.model?.display_name || '-' }}
                  </span>
                </template>
              </a-table-column>
              <a-table-column title="输入Token" data-index="prompt_tokens" width="120" />
              <a-table-column title="输出Token" data-index="completion_tokens" width="120" />
              <a-table-column title="总Token" data-index="total_tokens" width="120" />
              <a-table-column title="状态" data-index="status" width="100">
                <template #cell="{ record }">
                  <a-tag v-if="record.status === 0" color="red">失败</a-tag>
                  <a-tag v-else-if="record.status === 1" color="green">成功</a-tag>
                  <a-tag v-else-if="record.status === 2" color="arcoblue">处理中</a-tag>
                  <a-tag v-else color="grey">未知</a-tag>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <a-tab-pane key="operation" title="操作日志">
        <a-card class="content-card">
          <a-table
            :data="operationLogs"
            :loading="operationLoading"
            :pagination="{
              showTotal: true,
              showJumper: true,
              total: operationPagination.total
            }"
            :page="operationPagination.page"
            :pageSize="operationPagination.pageSize"
            :scroll="{ x: 1200 }"
            @page-change="handleOperationPageChange"
            @page-size-change="handleOperationPageSizeChange"
          >
            <template #columns>
              <a-table-column title="用户" data-index="user" width="200" fixed="left">
                <template #cell="{ record }">
                  {{ record.user?.name }} ({{ record.user?.email }})
                </template>
              </a-table-column>
              <a-table-column title="时间" data-index="created_at" width="200">
                <template #cell="{ record }">
                  {{ formatDate(record.created_at) }}
                </template>
              </a-table-column>
              <a-table-column title="操作类型" data-index="operation_type" width="150">
                <template #cell="{ record }">
                  {{ record.operation_type?.display_name }}
                </template>
              </a-table-column>
              <a-table-column title="IP地址" data-index="ip_address" width="150" />
              <a-table-column title="状态" data-index="status" width="100">
                <template #cell="{ record }">
                  <a-tag :color="record.status ? 'green' : 'red'">
                    {{ record.status ? '成功' : '失败' }}
                  </a-tag>
                </template>
              </a-table-column>
              <a-table-column title="错误信息" data-index="error_message" :ellipsis="true" width="300" />
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../utils/api'

const activeTab = ref('usage')

const usageLogs = ref([])
const usageLoading = ref(false)
const usagePagination = ref({ page: 1, pageSize: 10, total: 0, showTotal: true, showJumper: true })

const operationLogs = ref([])
const operationLoading = ref(false)
const operationPagination = ref({ page: 1, pageSize: 10, total: 0, showTotal: true, showJumper: true })

onMounted(() => {
  loadUsageLogs()
})

function handleTabChange(key) {
  if (key === 'usage') {
    loadUsageLogs()
  } else if (key === 'operation') {
    loadOperationLogs()
  }
}

async function loadUsageLogs() {
  usageLoading.value = true
  try {
    const response = await api.get('/admin/logs/usage', {
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

async function loadOperationLogs() {
  operationLoading.value = true
  try {
    const response = await api.get('/admin/logs/operation', {
      params: { page: operationPagination.value.page, per_page: operationPagination.value.pageSize }
    })
    if (response.data.success) {
      operationLogs.value = response.data.data.data || []
      operationPagination.value.total = response.data.data.meta?.total || operationLogs.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    operationLoading.value = false
  }
}

function handleUsagePageChange(page) {
  usagePagination.value.page = page
  loadUsageLogs()
}

function handleUsagePageSizeChange(pageSize) {
  usagePagination.value.pageSize = pageSize
  usagePagination.value.page = 1
  loadUsageLogs()
}

function handleOperationPageChange(page) {
  operationPagination.value.page = page
  loadOperationLogs()
}

function handleOperationPageSizeChange(pageSize) {
  operationPagination.value.pageSize = pageSize
  operationPagination.value.page = 1
  loadOperationLogs()
}

function formatDate(date) {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
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
  border: 1px solid var(--border-color);
}
</style>
