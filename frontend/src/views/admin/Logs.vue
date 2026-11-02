<template>
  <div class="logs-container">
    <div class="page-header">
      <h1>日志管理</h1>
      <p>查看系统使用日志和操作日志</p>
    </div>

    <a-tabs v-model:active-key="activeTab" @change="handleTabChange">
      <!-- 使用日志 -->
      <a-tab-pane key="usage" title="使用日志">
        <a-row :gutter="12" style="margin-bottom: 16px;" class="search-bar">
          <a-col :xs="24" :sm="12" :md="7" :lg="6">
            <a-input-search
              v-model="usageSearch.keyword"
              placeholder="搜索用户名 / 邮箱 / 模型名"
              allow-clear
              @search="handleUsageSearch"
              @press-enter="handleUsageSearch"
              @clear="handleUsageSearch"
            />
          </a-col>
          <a-col :xs="12" :sm="6" :md="4" :lg="3">
            <a-select v-model="usageSearch.status" placeholder="状态" allow-clear style="width: 100%" @change="handleUsageSearch">
              <a-option :value="1">成功</a-option>
              <a-option :value="0">失败</a-option>
              <a-option :value="2">处理中</a-option>
            </a-select>
          </a-col>
          <a-col :xs="24" :sm="12" :md="7" :lg="7">
            <a-range-picker
              v-model="usageSearch.dateRange"
              style="width: 100%"
              value-format="YYYY-MM-DD"
              @change="handleUsageSearch"
            />
          </a-col>
          <a-col :xs="24" :sm="6" :md="6" :lg="8" class="search-actions">
            <a-button @click="resetUsageSearch">重置</a-button>
          </a-col>
        </a-row>

        <a-card class="content-card">
          <a-table
            :data="usageLogs"
            :loading="usageLoading"
            :pagination="{
              showTotal: true,
              showJumper: true,
              total: usagePagination.total,
              current: usagePagination.page,
              pageSize: usagePagination.pageSize
            }"
            :scroll="{ x: 1400 }"
            @page-change="handleUsagePageChange"
            @page-size-change="handleUsagePageSizeChange"
          >
            <template #columns>
              <a-table-column title="用户" width="200" fixed="left">
                <template #cell="{ record }">
                  {{ record.user?.name }} ({{ record.user?.email }})
                </template>
              </a-table-column>
              <a-table-column title="时间" data-index="created_at" width="180">
                <template #cell="{ record }">
                  {{ formatDate(record.created_at) }}
                </template>
              </a-table-column>
              <a-table-column title="模型" width="220">
                <template #cell="{ record }">
                  <a-tag v-if="record.model && record.model.provider" color="arcoblue">
                    {{ record.model.provider.code }}/{{ record.model.code }}
                  </a-tag>
                  <span v-else>{{ record.model?.name || record.model?.code || '-' }}</span>
                </template>
              </a-table-column>
              <a-table-column title="输入Token" data-index="prompt_tokens" width="110" />
              <a-table-column title="输出Token" data-index="completion_tokens" width="110" />
              <a-table-column title="总Token" data-index="total_tokens" width="110" />
              <a-table-column title="费用" width="110">
                <template #cell="{ record }">
                  {{ formatCost(record.cost) }}
                </template>
              </a-table-column>
              <a-table-column title="状态" data-index="status" width="90">
                <template #cell="{ record }">
                  <a-tag v-if="record.status === 0" color="red">失败</a-tag>
                  <a-tag v-else-if="record.status === 1" color="green">成功</a-tag>
                  <a-tag v-else-if="record.status === 2" color="arcoblue">处理中</a-tag>
                  <a-tag v-else color="gray">未知</a-tag>
                </template>
              </a-table-column>
              <a-table-column title="错误信息" width="240">
                <template #cell="{ record }">
                  <a-tooltip v-if="record.error_message" :content="record.error_message">
                    <span class="error-text">{{ record.error_message }}</span>
                  </a-tooltip>
                  <span v-else>-</span>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>

      <!-- 操作日志 -->
      <a-tab-pane key="operation" title="操作日志">
        <a-row :gutter="12" style="margin-bottom: 16px;" class="search-bar">
          <a-col :xs="24" :sm="12" :md="7" :lg="6">
            <a-input-search
              v-model="opSearch.keyword"
              placeholder="搜索用户名 / 邮箱"
              allow-clear
              @search="handleOpSearch"
              @press-enter="handleOpSearch"
              @clear="handleOpSearch"
            />
          </a-col>
          <a-col :xs="12" :sm="6" :md="4" :lg="3">
            <a-select v-model="opSearch.operation_type" placeholder="操作类型" allow-clear style="width: 100%" @change="handleOpSearch">
              <a-option value="login">登录</a-option>
              <a-option value="logout">登出</a-option>
              <a-option value="create">新增</a-option>
              <a-option value="update">更新</a-option>
              <a-option value="toggle">状态切换</a-option>
              <a-option value="delete">删除</a-option>
            </a-select>
          </a-col>
          <a-col :xs="12" :sm="6" :md="4" :lg="3">
            <a-select v-model="opSearch.operation_module" placeholder="模块" allow-clear style="width: 100%" @change="handleOpSearch">
              <a-option v-for="m in moduleOptions" :key="m.value" :value="m.value">{{ m.label }}</a-option>
            </a-select>
          </a-col>
          <a-col :xs="12" :sm="6" :md="4" :lg="3">
            <a-select v-model="opSearch.status" placeholder="状态" allow-clear style="width: 100%" @change="handleOpSearch">
              <a-option value="success">成功</a-option>
              <a-option value="failed">失败</a-option>
            </a-select>
          </a-col>
          <a-col :xs="24" :sm="12" :md="7" :lg="5">
            <a-range-picker
              v-model="opSearch.dateRange"
              style="width: 100%"
              value-format="YYYY-MM-DD"
              @change="handleOpSearch"
            />
          </a-col>
          <a-col :xs="24" :sm="6" :md="5" :lg="4" class="search-actions">
            <a-button @click="resetOpSearch">重置</a-button>
          </a-col>
        </a-row>

        <a-card class="content-card">
          <a-table
            :data="operationLogs"
            :loading="operationLoading"
            :pagination="{
              showTotal: true,
              showJumper: true,
              total: operationPagination.total,
              current: operationPagination.page,
              pageSize: operationPagination.pageSize
            }"
            :scroll="{ x: 1400 }"
            @page-change="handleOperationPageChange"
            @page-size-change="handleOperationPageSizeChange"
          >
            <template #columns>
              <a-table-column title="操作人" width="180" fixed="left">
                <template #cell="{ record }">
                  {{ record.username || record.user?.name || '-' }}
                </template>
              </a-table-column>
              <a-table-column title="时间" data-index="created_at" width="180">
                <template #cell="{ record }">
                  {{ formatDate(record.created_at) }}
                </template>
              </a-table-column>
              <a-table-column title="操作类型" width="100">
                <template #cell="{ record }">
                  <a-tag :color="typeColor(record.operation_type)">{{ typeLabel(record.operation_type) }}</a-tag>
                </template>
              </a-table-column>
              <a-table-column title="模块" width="120">
                <template #cell="{ record }">
                  {{ moduleLabel(record.operation_module) }}
                </template>
              </a-table-column>
              <a-table-column title="操作内容" width="260">
                <template #cell="{ record }">
                  <a-tooltip :content="record.operation_action">
                    <span class="action-text">{{ record.operation_action }}</span>
                  </a-tooltip>
                </template>
              </a-table-column>
              <a-table-column title="IP地址" data-index="ip_address" width="130" />
              <a-table-column title="状态" width="90">
                <template #cell="{ record }">
                  <a-tag :color="record.status === 'success' ? 'green' : 'red'">
                    {{ record.status === 'success' ? '成功' : '失败' }}
                  </a-tag>
                </template>
              </a-table-column>
              <a-table-column title="错误信息" width="200">
                <template #cell="{ record }">
                  <a-tooltip v-if="record.error_message" :content="record.error_message">
                    <span class="error-text">{{ record.error_message }}</span>
                  </a-tooltip>
                  <span v-else>-</span>
                </template>
              </a-table-column>
              <a-table-column title="操作" width="90" fixed="right">
                <template #cell="{ record }">
                  <a-button size="small" type="text" @click="showDetail(record)">详情</a-button>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>
      </a-tab-pane>
    </a-tabs>

    <!-- 操作日志详情弹窗 -->
    <a-modal
      v-model:visible="detailVisible"
      title="操作日志详情"
      :footer="false"
      width="640px"
    >
      <a-descriptions :column="2" size="medium" bordered v-if="detailRecord">
        <a-descriptions-item label="操作人" :span="1">{{ detailRecord.username || detailRecord.user?.name || '-' }}</a-descriptions-item>
        <a-descriptions-item label="时间" :span="1">{{ formatDate(detailRecord.created_at) }}</a-descriptions-item>
        <a-descriptions-item label="操作类型" :span="1">
          <a-tag :color="typeColor(detailRecord.operation_type)">{{ typeLabel(detailRecord.operation_type) }}</a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="模块" :span="1">{{ moduleLabel(detailRecord.operation_module) }}</a-descriptions-item>
        <a-descriptions-item label="操作内容" :span="2">{{ detailRecord.operation_action }}</a-descriptions-item>
        <a-descriptions-item label="IP地址" :span="1">{{ detailRecord.ip_address || '-' }}</a-descriptions-item>
        <a-descriptions-item label="状态" :span="1">
          <a-tag :color="detailRecord.status === 'success' ? 'green' : 'red'">
            {{ detailRecord.status === 'success' ? '成功' : '失败' }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="User-Agent" :span="2">
          <span class="ua-text">{{ detailRecord.user_agent || '-' }}</span>
        </a-descriptions-item>
        <a-descriptions-item v-if="detailRecord.error_message" label="错误信息" :span="2">
          <span class="error-text">{{ detailRecord.error_message }}</span>
        </a-descriptions-item>
      </a-descriptions>

      <div v-if="detailRecord?.request_params" class="detail-json-section">
        <div class="detail-json-title">请求参数</div>
        <pre class="detail-json">{{ prettyJSON(detailRecord.request_params) }}</pre>
      </div>
      <div v-if="detailRecord?.response_result" class="detail-json-section">
        <div class="detail-json-title">响应结果</div>
        <pre class="detail-json">{{ prettyJSON(detailRecord.response_result) }}</pre>
      </div>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import api from '../../utils/api'

const activeTab = ref('usage')

// 使用日志
const usageLogs = ref([])
const usageLoading = ref(false)
const usagePagination = ref({ page: 1, pageSize: 10, total: 0 })
const usageSearch = reactive({ keyword: '', status: null, dateRange: [] })

// 操作日志
const operationLogs = ref([])
const operationLoading = ref(false)
const operationPagination = ref({ page: 1, pageSize: 10, total: 0 })
const opSearch = reactive({ keyword: '', operation_type: '', operation_module: '', status: '', dateRange: [] })

// 详情弹窗
const detailVisible = ref(false)
const detailRecord = ref(null)

const moduleOptions = [
  { value: 'auth', label: '认证' },
  { value: 'users', label: '用户管理' },
  { value: 'models', label: '模型管理' },
  { value: 'ai-providers', label: 'AI供应商' },
  { value: 'provider-keys', label: '供应商密钥' },
  { value: 'tools', label: '工具管理' },
  { value: 'skills', label: '技能管理' },
  { value: 'billing', label: '计费管理' },
  { value: 'leaderboards', label: '排行榜' },
  { value: 'user', label: '用户端' },
  { value: 'chat', label: '会话' },
]

const typeMap = {
  login: { label: '登录', color: 'arcoblue' },
  logout: { label: '登出', color: 'gray' },
  create: { label: '新增', color: 'green' },
  update: { label: '更新', color: 'orangered' },
  toggle: { label: '状态切换', color: 'purple' },
  delete: { label: '删除', color: 'red' },
}

const moduleMap = {}
moduleOptions.forEach((m) => { moduleMap[m.value] = m.label })

function typeLabel(type) {
  return typeMap[type]?.label || type || '-'
}

function typeColor(type) {
  return typeMap[type]?.color || 'gray'
}

function moduleLabel(module) {
  return moduleMap[module] || module || '-'
}

function prettyJSON(value) {
  try {
    const obj = typeof value === 'string' ? JSON.parse(value) : value
    return JSON.stringify(obj, null, 2)
  } catch (e) {
    return String(value)
  }
}

function formatCost(cost) {
  if (cost === null || cost === undefined) return '-'
  return Number(cost).toFixed(6).replace(/0+$/, '').replace(/\.$/, '') || '0'
}

onMounted(() => {
  loadUsageLogs()
})

function handleTabChange(key) {
  if (key === 'usage' && usageLogs.value.length === 0) {
    loadUsageLogs()
  } else if (key === 'operation' && operationLogs.value.length === 0) {
    loadOperationLogs()
  }
}

function buildUsageParams() {
  const params = {
    page: usagePagination.value.page,
    per_page: usagePagination.value.pageSize
  }
  if (usageSearch.keyword) params.keyword = usageSearch.keyword
  if (usageSearch.status !== null && usageSearch.status !== undefined && usageSearch.status !== '') {
    params.status = usageSearch.status
  }
  if (usageSearch.dateRange && usageSearch.dateRange.length === 2) {
    params.start_date = usageSearch.dateRange[0]
    params.end_date = usageSearch.dateRange[1]
  }
  return params
}

async function loadUsageLogs() {
  usageLoading.value = true
  try {
    const response = await api.get('/admin/logs/usage', { params: buildUsageParams() })
    if (response.data.success) {
      usageLogs.value = response.data.data.data || []
      usagePagination.value.total = response.data.data.meta?.total || 0
    }
  } catch (error) {
    Message.error('加载使用日志失败')
    console.error(error)
  } finally {
    usageLoading.value = false
  }
}

function buildOpParams() {
  const params = {
    page: operationPagination.value.page,
    per_page: operationPagination.value.pageSize
  }
  if (opSearch.keyword) params.keyword = opSearch.keyword
  if (opSearch.operation_type) params.operation_type = opSearch.operation_type
  if (opSearch.operation_module) params.operation_module = opSearch.operation_module
  if (opSearch.status) params.status = opSearch.status
  if (opSearch.dateRange && opSearch.dateRange.length === 2) {
    params.start_date = opSearch.dateRange[0]
    params.end_date = opSearch.dateRange[1]
  }
  return params
}

async function loadOperationLogs() {
  operationLoading.value = true
  try {
    const response = await api.get('/admin/logs/operation', { params: buildOpParams() })
    if (response.data.success) {
      operationLogs.value = response.data.data.data || []
      operationPagination.value.total = response.data.data.meta?.total || 0
    }
  } catch (error) {
    Message.error('加载操作日志失败')
    console.error(error)
  } finally {
    operationLoading.value = false
  }
}

// 使用日志搜索
function handleUsageSearch() {
  usagePagination.value.page = 1
  loadUsageLogs()
}

function resetUsageSearch() {
  usageSearch.keyword = ''
  usageSearch.status = null
  usageSearch.dateRange = []
  handleUsageSearch()
}

// 操作日志搜索
function handleOpSearch() {
  operationPagination.value.page = 1
  loadOperationLogs()
}

function resetOpSearch() {
  opSearch.keyword = ''
  opSearch.operation_type = ''
  opSearch.operation_module = ''
  opSearch.status = ''
  opSearch.dateRange = []
  handleOpSearch()
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

function showDetail(record) {
  detailRecord.value = record
  detailVisible.value = true
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

.search-bar {
  align-items: center;
}

.search-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.error-text {
  color: #f53f3f;
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}

.action-text {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: bottom;
}

.ua-text {
  word-break: break-all;
  font-size: 12px;
  color: var(--text-secondary);
}

.detail-json-section {
  margin-top: 16px;
}

.detail-json-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 6px;
}

.detail-json {
  margin: 0;
  padding: 12px;
  max-height: 260px;
  overflow: auto;
  background: var(--color-fill-2, #f7f8fa);
  border: 1px solid var(--border-color, #e5e6eb);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
