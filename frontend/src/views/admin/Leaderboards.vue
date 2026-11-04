<template>
  <div class="leaderboards-container">
    <div class="page-header">
      <h1>排行榜管理</h1>
      <p>管理排行榜配置和统计指标</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="0" :sm="12" :md="8" :lg="6"></a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="18" style="text-align: right;">
        <a-tooltip content="新建配置">
          <a-button type="primary" @click="showCreateModal = true">
            <template #icon><icon-plus /></template>
          </a-button>
        </a-tooltip>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
      <a-table
        :data="configs"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1200 }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="名称" data-index="display_name" width="150" fixed="left" />
          <a-table-column title="指标" data-index="metric" width="120">
            <template #cell="{ record }">
              {{ getMetricLabel(record.metric) }}
            </template>
          </a-table-column>
          <a-table-column title="周期" data-index="period" width="100">
            <template #cell="{ record }">
              {{ getPeriodLabel(record.period) }}
            </template>
          </a-table-column>
          <a-table-column title="限制" data-index="limit" width="80" />
          <a-table-column title="状态" data-index="enabled" width="100">
            <template #cell="{ record }">
              <a-switch
                :model-value="record.enabled"
                size="small"
                @change="(val) => toggleConfigStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="描述" data-index="description" :ellipsis="true" width="300" />
          <a-table-column title="操作" width="70" align="center" fixed="right">
            <template #cell="{ record }">
              <a-tooltip content="编辑">
                <a-button type="text" size="small" @click="showEditModal(record)">
                  <template #icon><icon-edit /></template>
                </a-button>
              </a-tooltip>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建配置弹窗 -->
    <a-modal
      v-model:visible="showCreateModal"
      title="新建排行榜配置"
      @ok="createConfig"
      :confirmLoading="createLoading"
      width="500px"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="内部名称" field="name">
          <a-input v-model="createForm.name" placeholder="唯一标识符" />
        </a-form-item>
        <a-form-item label="显示名称" field="display_name">
          <a-input v-model="createForm.display_name" placeholder="显示给用户的名称" />
        </a-form-item>
        <a-form-item label="统计指标" field="metric">
          <a-select v-model="createForm.metric" style="width: 100%">
            <a-option value="total_tokens">总Token数</a-option>
            <a-option value="total_cost">总费用</a-option>
            <a-option value="request_count">请求次数</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="统计周期" field="period">
          <a-select v-model="createForm.period" style="width: 100%">
            <a-option value="daily">每日</a-option>
            <a-option value="weekly">每周</a-option>
            <a-option value="monthly">每月</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="显示数量" field="limit">
          <a-input-number v-model="createForm.limit" :min="1" :max="100" style="width: 100%" />
        </a-form-item>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="createForm.description" :rows="3" />
        </a-form-item>
        <a-form-item label="状态" field="enabled">
          <a-switch v-model="createForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 编辑配置弹窗 -->
    <a-modal
      v-model:visible="showEditModalVisible"
      title="编辑排行榜配置"
      @ok="updateConfig"
      :confirmLoading="updateLoading"
      width="500px"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="内部名称" field="name">
          <a-input v-model="editForm.name" placeholder="唯一标识符" />
        </a-form-item>
        <a-form-item label="显示名称" field="display_name">
          <a-input v-model="editForm.display_name" placeholder="显示给用户的名称" />
        </a-form-item>
        <a-form-item label="统计指标" field="metric">
          <a-select v-model="editForm.metric" style="width: 100%">
            <a-option value="total_tokens">总Token数</a-option>
            <a-option value="total_cost">总费用</a-option>
            <a-option value="request_count">请求次数</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="统计周期" field="period">
          <a-select v-model="editForm.period" style="width: 100%">
            <a-option value="daily">每日</a-option>
            <a-option value="weekly">每周</a-option>
            <a-option value="monthly">每月</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="显示数量" field="limit">
          <a-input-number v-model="editForm.limit" :min="1" :max="100" style="width: 100%" />
        </a-form-item>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="editForm.description" :rows="3" />
        </a-form-item>
        <a-form-item label="状态" field="enabled">
          <a-switch v-model="editForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { IconPlus, IconEdit } from '@arco-design/web-vue/es/icon'

const configs = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)

const createForm = ref({
  name: '',
  display_name: '',
  metric: 'total_tokens',
  period: 'daily',
  limit: 10,
  description: '',
  enabled: true
})

const editForm = ref({
  id: null,
  name: '',
  display_name: '',
  metric: 'total_tokens',
  period: 'daily',
  limit: 10,
  description: '',
  enabled: true
})

onMounted(() => {
  loadConfigs()
})

async function loadConfigs() {
  loading.value = true
  try {
    const response = await api.get('/admin/leaderboards/configs', {
      params: { page: pagination.value.current, per_page: pagination.value.pageSize }
    })
    if (response.data.success) {
      configs.value = response.data.data.data || response.data.data
      pagination.value.total = response.data.data.total || configs.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

function getMetricLabel(metric) {
  const labels = {
    total_tokens: '总Token数',
    total_cost: '总费用',
    request_count: '请求次数'
  }
  return labels[metric] || metric
}

function getPeriodLabel(period) {
  const labels = {
    daily: '每日',
    weekly: '每周',
    monthly: '每月'
  }
  return labels[period] || period
}

async function createConfig() {
  if (!createForm.value.name || !createForm.value.display_name) {
    Message.error('请填写完整信息')
    return
  }

  createLoading.value = true
  try {
    const response = await api.post('/admin/leaderboards/configs', createForm.value)
    if (response.data.success) {
      showCreateModal.value = false
      Message.success('创建成功')
      await loadConfigs()
    }
  } catch (error) {
    console.error(error)
  } finally {
    createLoading.value = false
  }
}

function showEditModal(record) {
  editForm.value = { ...record }
  showEditModalVisible.value = true
}

async function updateConfig() {
  if (!editForm.value.name || !editForm.value.display_name) {
    Message.error('请填写完整信息')
    return
  }

  updateLoading.value = true
  try {
    const response = await api.put(`/admin/leaderboards/configs/${editForm.value.id}`, editForm.value)
    if (response.data.success) {
      showEditModalVisible.value = false
      Message.success('更新成功')
      await loadConfigs()
    }
  } catch (error) {
    console.error(error)
  } finally {
    updateLoading.value = false
  }
}

async function deleteConfig(id) {
  try {
    const response = await api.delete(`/admin/leaderboards/configs/${id}`)
    if (response.data.success) {
      Message.success('删除成功')
      await loadConfigs()
    }
  } catch (error) {
    console.error(error)
  }
}

async function toggleConfigStatus(record, val) {
  try {
    await api.put(`/admin/leaderboards/configs/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    loadConfigs()
  } catch (error) {
    Message.error('状态更新失败')
  }
}

function handlePageChange(page) {
  pagination.value.current = page
  loadConfigs()
}
</script>

<style scoped>
.leaderboards-container {
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
