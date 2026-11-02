<template>
  <div class="tools-container">
    <div class="page-header">
      <h1>工具管理</h1>
      <p>管理AI功能调用工具</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="12" :md="8" :lg="6"></a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="18" style="text-align: right;">
        <a-button type="primary" @click="showCreateModal = true">
          <template #icon><icon-plus /></template>
          新建工具
        </a-button>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
      <a-table
        :data="tools"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1200 }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="工具名称" data-index="name" width="150" fixed="left" />
          <a-table-column title="显示名称" data-index="display_name" width="200" />
          <a-table-column title="类型" data-index="tool_type" width="120" />
          <a-table-column title="描述" data-index="description" :ellipsis="true" width="300" />
          <a-table-column title="状态" data-index="enabled" width="100">
            <template #cell="{ record }">
              <a-switch
                :model-value="record.enabled"
                size="small"
                @change="(val) => toggleToolStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at" width="200">
            <template #cell="{ record }">
              {{ formatDate(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column title="操作" width="80" fixed="right">
            <template #cell="{ record }">
              <a-button size="small" @click="showEditModal(record)">编辑</a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建工具弹窗 -->
    <a-modal
      v-model:visible="showCreateModal"
      title="新建工具"
      @ok="createTool"
      :confirmLoading="createLoading"
      width="600px"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="工具名称" field="name">
          <a-input v-model="createForm.name" placeholder="唯一标识符" />
        </a-form-item>
        <a-form-item label="显示名称" field="display_name">
          <a-input v-model="createForm.display_name" placeholder="显示给用户的名称" />
        </a-form-item>
        <a-form-item label="工具类型" field="tool_type">
          <a-select v-model="createForm.tool_type" style="width: 100%">
            <a-option value="builtin">内置工具</a-option>
            <a-option value="custom">自定义工具</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="createForm.description" :rows="3" />
        </a-form-item>
        <a-form-item label="参数定义" field="parameters">
          <a-textarea
            v-model="parametersJson"
            placeholder="JSON格式的参数定义"
            :rows="6"
          />
        </a-form-item>
        <a-form-item label="状态" field="enabled">
          <a-switch v-model="createForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 编辑工具弹窗 -->
    <a-modal
      v-model:visible="showEditModalVisible"
      title="编辑工具"
      @ok="updateTool"
      :confirmLoading="updateLoading"
      width="600px"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="工具名称" field="name">
          <a-input v-model="editForm.name" placeholder="唯一标识符" />
        </a-form-item>
        <a-form-item label="显示名称" field="display_name">
          <a-input v-model="editForm.display_name" placeholder="显示给用户的名称" />
        </a-form-item>
        <a-form-item label="工具类型" field="tool_type">
          <a-select v-model="editForm.tool_type" style="width: 100%">
            <a-option value="builtin">内置工具</a-option>
            <a-option value="custom">自定义工具</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="editForm.description" :rows="3" />
        </a-form-item>
        <a-form-item label="参数定义" field="parameters">
          <a-textarea
            v-model="editParametersJson"
            placeholder="JSON格式的参数定义"
            :rows="6"
          />
        </a-form-item>
        <a-form-item label="状态" field="enabled">
          <a-switch v-model="editForm.enabled" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'

const tools = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)

const createForm = ref({
  name: '',
  display_name: '',
  tool_type: 'builtin',
  description: '',
  parameters: null,
  enabled: true
})

const editForm = ref({
  id: null,
  name: '',
  display_name: '',
  tool_type: 'builtin',
  description: '',
  parameters: null,
  enabled: true
})

const parametersJson = computed({
  get: () => createForm.value.parameters ? JSON.stringify(createForm.value.parameters, null, 2) : '',
  set: (val) => {
    try {
      createForm.value.parameters = val ? JSON.parse(val) : null
    } catch {
      createForm.value.parameters = null
    }
  }
})

const editParametersJson = computed({
  get: () => editForm.value.parameters ? JSON.stringify(editForm.value.parameters, null, 2) : '',
  set: (val) => {
    try {
      editForm.value.parameters = val ? JSON.parse(val) : null
    } catch {
      editForm.value.parameters = null
    }
  }
})

onMounted(() => {
  loadTools()
})

async function loadTools() {
  loading.value = true
  try {
    const response = await api.get('/admin/tools', {
      params: { page: pagination.value.current, per_page: pagination.value.pageSize }
    })
    if (response.data.success) {
      tools.value = response.data.data.data || response.data.data
      pagination.value.total = response.data.data.total || tools.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

async function createTool() {
  if (!createForm.value.name || !createForm.value.display_name) {
    Message.error('请填写完整信息')
    return
  }

  createLoading.value = true
  try {
    const response = await api.post('/admin/tools', createForm.value)
    if (response.data.success) {
      showCreateModal.value = false
      Message.success('创建成功')
      await loadTools()
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

async function updateTool() {
  if (!editForm.value.name || !editForm.value.display_name) {
    Message.error('请填写完整信息')
    return
  }

  updateLoading.value = true
  try {
    const response = await api.put(`/admin/tools/${editForm.value.id}`, editForm.value)
    if (response.data.success) {
      showEditModalVisible.value = false
      Message.success('更新成功')
      await loadTools()
    }
  } catch (error) {
    console.error(error)
  } finally {
    updateLoading.value = false
  }
}

async function deleteTool(id) {
  try {
    const response = await api.delete(`/admin/tools/${id}`)
    if (response.data.success) {
      Message.success('删除成功')
      await loadTools()
    }
  } catch (error) {
    console.error(error)
  }
}

async function toggleToolStatus(record, val) {
  try {
    await api.put(`/admin/tools/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    loadTools()
  } catch (error) {
    Message.error('状态更新失败')
  }
}

function handlePageChange(page) {
  pagination.value.current = page
  loadTools()
}

function formatDate(date) {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}
</script>

<style scoped>
.tools-container {
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
