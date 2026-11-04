<template>
  <div class="api-keys-container">
    <div class="page-header">
      <div>
        <h1>API 密钥</h1>
        <p>管理您的 API 密钥以访问 API 接口</p>
      </div>
      <a-tooltip content="新建密钥">
        <a-button type="primary" @click="showCreateModal = true" class="create-btn">
          <template #icon><icon-plus /></template>
        </a-button>
      </a-tooltip>
    </div>

    <a-card class="table-card">
      <a-table
        :data="apiKeys"
        :loading="loading"
        :pagination="pagination"
        @page-change="handlePageChange"
        :scroll="{ x: 1000 }"
      >
        <template #columns>
          <a-table-column title="密钥名称" data-index="key_name" :width="160" />
          <a-table-column title="API 密钥" :width="280">
            <template #cell="{ record }">
              <a-space>
                <a-tag color="blue" class="key-tag">
                  {{ maskApiKey(record.api_key) }}
                </a-tag>
                <a-tooltip content="复制密钥">
                  <a-button type="text" size="small" @click="copyApiKey(record.api_key)">
                    <template #icon><icon-copy /></template>
                  </a-button>
                </a-tooltip>
              </a-space>
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at" :width="180">
            <template #cell="{ record }">
              {{ formatDate(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column title="最后使用" data-index="last_used_at" :width="180">
            <template #cell="{ record }">
              <a-tag v-if="record.last_used_at" color="cyan">{{ formatDate(record.last_used_at) }}</a-tag>
              <a-tag v-else color="gray">未使用</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="过期时间" data-index="expires_at" :width="180">
            <template #cell="{ record }">
              <a-tag v-if="record.expires_at" color="orange">{{ formatDate(record.expires_at) }}</a-tag>
              <a-tag v-else color="green">永不过期</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" data-index="status" :width="100" align="center">
            <template #cell="{ record }">
              <a-switch
                :model-value="!!record.status"
                :active-value="true"
                :inactive-value="false"
                size="small"
                @change="(val) => toggleKeyStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="70" align="center" fixed="right">
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

    <a-drawer
      v-model:visible="showCreateModal"
      title="创建 API 密钥"
      :width="480"
      unmount-on-close
      @ok="createKey"
      :ok-loading="createLoading"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="密钥名称" field="key_name" :rules="[{required:true}]">
          <a-input v-model="createForm.key_name" placeholder="请输入密钥名称" />
        </a-form-item>
        <a-form-item label="过期时间" field="expires_at">
          <a-date-picker
            v-model="createForm.expires_at"
            placeholder="选择过期时间（可选）"
            :show-time="true"
            style="width: 100%"
          />
        </a-form-item>
      </a-form>
    </a-drawer>

    <a-drawer
      v-model:visible="showEditModalVisible"
      title="编辑 API 密钥"
      :width="480"
      unmount-on-close
      @ok="updateKey"
      :ok-loading="updateLoading"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="密钥名称" field="key_name">
          <a-input v-model="editForm.key_name" placeholder="请输入密钥名称" />
        </a-form-item>
        <a-form-item label="过期时间" field="expires_at">
          <a-date-picker
            v-model="editForm.expires_at"
            placeholder="选择过期时间（可选）"
            :show-time="true"
            :allow-clear="true"
            style="width: 100%"
          />
        </a-form-item>
      </a-form>
    </a-drawer>

    <a-drawer
      v-model:visible="showNewKeyModal"
      title="API 密钥已创建"
      :width="480"
      :footer="false"
    >
      <a-alert type="warning" :style="{ marginBottom: '20px' }">
        <div class="alert-content">
          <icon-info-circle />
          <span>请妥善保管您的 API 密钥，只显示一次！</span>
        </div>
      </a-alert>
      <a-input-group class="key-input-group">
        <a-input v-model="newKey" readonly :style="{ background: '#F7F8FA' }" />
        <a-button type="primary" @click="copyKey" class="copy-btn">
          <template #icon><icon-copy /></template>
          复制
        </a-button>
      </a-input-group>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { IconPlus, IconCopy, IconInfoCircle, IconEdit } from '@arco-design/web-vue/es/icon'

const apiKeys = ref([])
const loading = ref(false)
const pagination = ref({
  current: 1,
  pageSize: 10,
  total: 0
})

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const showNewKeyModal = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)
const newKey = ref('')

const createForm = ref({
  key_name: '',
  expires_at: null
})

const editForm = ref({
  id: null,
  key_name: '',
  expires_at: null
})

onMounted(() => {
  loadApiKeys()
})

async function loadApiKeys() {
  loading.value = true
  try {
    const response = await api.get('/user/api-keys', {
      params: {
        page: pagination.value.current,
        per_page: pagination.value.pageSize
      }
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
  if (!createForm.value.key_name.trim()) {
    Message.error('请输入密钥名称')
    return
  }

  createLoading.value = true
  try {
    const response = await api.post('/user/api-keys', {
      key_name: createForm.value.key_name,
      expires_at: createForm.value.expires_at
    })
    if (response.data.success) {
      newKey.value = response.data.data.api_key
      showCreateModal.value = false
      showNewKeyModal.value = true
      createForm.value = { key_name: '', expires_at: null }
      await loadApiKeys()
    }
  } catch (error) {
    console.error(error)
  } finally {
    createLoading.value = false
  }
}

function showEditModal(record) {
  editForm.value = {
    id: record.id,
    key_name: record.key_name,
    expires_at: record.expires_at
  }
  showEditModalVisible.value = true
}

async function updateKey() {
  if (!editForm.value.key_name.trim()) {
    Message.error('请输入密钥名称')
    return
  }

  updateLoading.value = true
  try {
    const response = await api.put(`/user/api-keys/${editForm.value.id}`, {
      key_name: editForm.value.key_name,
      expires_at: editForm.value.expires_at
    })
    if (response.data.success) {
      showEditModalVisible.value = false
      editForm.value = { id: null, key_name: '', expires_at: null }
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
    const response = await api.delete(`/user/api-keys/${id}`)
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
    await api.put(`/user/api-keys/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    // 更新本地数据
    record.status = val
  } catch (error) {
    Message.error('状态更新失败')
    // 还原状态
    record.status = !val
  }
}

function copyKey() {
  navigator.clipboard.writeText(newKey.value).then(() => {
    Message.success('已复制到剪贴板')
  }).catch(() => {
    Message.error('复制失败，请手动复制')
  })
}

function maskApiKey(key) {
  if (!key) return ''
  if (key.length <= 8) {
    return '*'.repeat(key.length)
  }
  return key.slice(0, 6) + '••••••••' + key.slice(-6)
}

function copyApiKey(key) {
  if (!key) {
    Message.error('没有可复制的密钥')
    return
  }
  navigator.clipboard.writeText(key).then(() => {
    Message.success('已复制到剪贴板')
  }).catch(() => {
    Message.error('复制失败')
  })
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
.api-keys-container {
  padding: 0;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
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

.table-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.create-btn {
  height: 40px;
  padding: 0 20px;
  font-size: 14px;
}

.key-tag {
  font-family: monospace;
  font-size: 13px;
}

.key-input-group {
  display: flex;
}

.key-input-group :deep(.arco-input-group-wrapper) {
  flex: 1;
}

.copy-btn {
  margin-left: -1px;
}

.alert-content {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
