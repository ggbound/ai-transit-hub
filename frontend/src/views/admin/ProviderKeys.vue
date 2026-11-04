<template>
  <div class="provider-keys-container">
    <div class="page-header">
      <h1>API密钥管理</h1>
      <p>管理AI服务提供商的API密钥配置</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="24" :md="16" :lg="18" style="text-align: right;">
        <a-tooltip content="新建密钥">
          <a-button type="primary" @click="showCreateModal = true">
            <template #icon><icon-plus /></template>
          </a-button>
        </a-tooltip>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
      <a-table
        :data="apiKeys"
        :loading="loading"
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="提供商" data-index="provider" width="120" />
          <a-table-column title="密钥名称" data-index="key_name" />
          <a-table-column title="权重" data-index="weight" width="80" />
          <a-table-column title="使用次数" data-index="usage_count" width="100" />
          <a-table-column title="状态" data-index="status" width="100">
            <template #cell="{ record }">
              <a-switch
                :model-value="record.status"
                size="small"
                @change="(val) => toggleKeyStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at">
            <template #cell="{ record }">
              {{ formatDate(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column title="操作" width="70" align="center">
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

    <!-- 创建密钥弹窗 -->
    <a-modal
      v-model:visible="showCreateModal"
      title="新建提供商密钥"
      @ok="createKey"
      :confirmLoading="createLoading"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="提供商" field="provider">
          <a-input v-model="createForm.provider" placeholder="如: openai" />
        </a-form-item>
        <a-form-item label="密钥名称" field="key_name">
          <a-input v-model="createForm.key_name" placeholder="给密钥起个名字" />
        </a-form-item>
        <a-form-item label="API密钥" field="api_key">
          <a-input-password v-model="createForm.api_key" placeholder="输入API密钥" />
        </a-form-item>
        <a-form-item label="权重" field="weight">
          <a-input-number v-model="createForm.weight" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item label="状态" field="status">
          <a-switch v-model="createForm.status" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 编辑密钥弹窗 -->
    <a-modal
      v-model:visible="showEditModalVisible"
      title="编辑提供商密钥"
      @ok="updateKey"
      :confirmLoading="updateLoading"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="提供商" field="provider">
          <a-input v-model="editForm.provider" placeholder="如: openai" />
        </a-form-item>
        <a-form-item label="密钥名称" field="key_name">
          <a-input v-model="editForm.key_name" placeholder="给密钥起个名字" />
        </a-form-item>
        <a-form-item label="API密钥" field="api_key">
          <a-input-password v-model="editForm.api_key" placeholder="输入API密钥（留空不修改）" />
        </a-form-item>
        <a-form-item label="权重" field="weight">
          <a-input-number v-model="editForm.weight" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item label="状态" field="status">
          <a-switch v-model="editForm.status" />
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

const apiKeys = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)

const createForm = ref({
  provider: '',
  key_name: '',
  api_key: '',
  weight: 1,
  status: true
})

const editForm = ref({
  id: null,
  provider: '',
  key_name: '',
  api_key: '',
  weight: 1,
  status: true
})

onMounted(() => {
  loadApiKeys()
})

async function loadApiKeys() {
  loading.value = true
  try {
    const response = await api.get('/admin/provider-keys', {
      params: { page: pagination.value.current, per_page: pagination.value.pageSize }
    })
    if (response.data.success) {
      apiKeys.value = response.data.data.data || response.data.data
      pagination.value.total = response.data.data.total || apiKeys.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

async function createKey() {
  if (!createForm.value.provider || !createForm.value.key_name || !createForm.value.api_key) {
    Message.error('请填写完整信息')
    return
  }

  createLoading.value = true
  try {
    const response = await api.post('/admin/provider-keys', createForm.value)
    if (response.data.success) {
      showCreateModal.value = false
      Message.success('创建成功')
      await loadApiKeys()
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

async function updateKey() {
  if (!editForm.value.provider || !editForm.value.key_name) {
    Message.error('请填写完整信息')
    return
  }

  updateLoading.value = true
  try {
    const data = { ...editForm.value }
    if (!data.api_key) {
      delete data.api_key
    }
    const response = await api.put(`/admin/provider-keys/${editForm.value.id}`, data)
    if (response.data.success) {
      showEditModalVisible.value = false
      Message.success('更新成功')
      await loadApiKeys()
    }
  } catch (error) {
    console.error(error)
  } finally {
    updateLoading.value = false
  }
}

async function deleteKey(id) {
  try {
    const response = await api.delete(`/admin/provider-keys/${id}`)
    if (response.data.success) {
      Message.success('删除成功')
      await loadApiKeys()
    }
  } catch (error) {
    console.error(error)
  }
}

async function toggleKeyStatus(record, val) {
  try {
    await api.put(`/admin/provider-keys/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    loadApiKeys()
  } catch (error) {
    Message.error('状态更新失败')
  }
}

function handlePageChange(page) {
  pagination.value.current = page
  loadApiKeys()
}

function formatDate(date) {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}
</script>

<style scoped>
.provider-keys-container {
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
