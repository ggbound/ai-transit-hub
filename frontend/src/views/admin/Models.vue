<template>
  <div class="models-container">
    <div class="page-header">
      <h1>模型管理</h1>
      <p>管理系统中的AI模型配置和价格策略</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-input-search
          v-model="searchQuery"
          placeholder="搜索模型名称"
          allow-clear
          @search="loadModels"
          @press-enter="loadModels"
        />
      </a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="18" style="text-align: right;">
        <a-button type="primary" @click="showCreateModal = true">
          <template #icon><icon-plus /></template>
          新建模型
        </a-button>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
    <a-table
      :data="filteredModels"
      :loading="loading"
      :pagination="pagination"
      :scroll="{ x: 1200 }"
      @page-change="handlePageChange"
    >
      <template #columns>
        <a-table-column title="提供商" data-index="provider" width="120" fixed="left">
          <template #cell="{ record }">
            {{ record.provider?.name || '-' }}
          </template>
        </a-table-column>
        <a-table-column title="模型代码" data-index="code" width="200">
          <template #cell="{ record }">
            <a-tag color="arcoblue">
              {{ record.provider ? record.provider.code + '/' + record.code : record.code }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column title="模型类型" data-index="type" width="100">
          <template #cell="{ record }">
            <a-tag :color="getTypeColor(record.type)">
              {{ getTypeName(record.type) }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column title="显示名称" data-index="name" width="150" />
        <a-table-column title="最大Tokens" data-index="max_tokens" width="100" />
        <a-table-column title="状态" data-index="status" width="80">
          <template #cell="{ record }">
            <a-switch
              :model-value="!!record.status"
              :active-value="true"
              :inactive-value="false"
              size="small"
              @change="(val) => toggleModelStatus(record, val)"
            />
          </template>
        </a-table-column>
        <a-table-column title="Responses API" data-index="supports_response" width="120">
          <template #cell="{ record }">
            <a-tag :color="record.supports_response ? 'cyan' : 'default'">
              {{ record.supports_response ? '已启用' : '未启用' }}
            </a-tag>
          </template>
        </a-table-column>
        <a-table-column title="Messages API" data-index="supports_messages" width="120">
          <template #cell="{ record }">
            <a-tag :color="record.supports_messages ? 'purple' : 'default'">
              {{ record.supports_messages ? '已启用' : '未启用' }}
            </a-tag>
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

    <!-- 创建模型弹窗 -->
    <a-modal
      v-model:visible="showCreateModal"
      title="新建模型"
      @ok="createModel"
      :confirm-loading="createLoading"
      width="600px"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="提供商" field="provider_id" required>
          <a-select v-model="createForm.provider_id" placeholder="请选择提供商">
            <a-option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.name }} ({{ provider.code }})
            </a-option>
          </a-select>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="模型代码" field="code" required>
              <a-input v-model="createForm.code" placeholder="如: gpt-4" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="模型类型" field="type" required>
              <a-select v-model="createForm.type" placeholder="请选择类型">
                <a-option value="text">文本对话</a-option>
                <a-option value="embedding">向量嵌入</a-option>
                <a-option value="image">图像生成</a-option>
                <a-option value="video">视频生成</a-option>
                <a-option value="audio">语音生成</a-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="显示名称" field="name" required>
              <a-input v-model="createForm.name" placeholder="如: GPT-4" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="createForm.description" :rows="3" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="最大Tokens" field="max_tokens">
              <a-input-number v-model="createForm.max_tokens" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="支持能力" field="capabilities">
          <a-checkbox-group v-model="createForm.capabilities">
            <a-checkbox value="chat">对话</a-checkbox>
            <a-checkbox value="vision">视觉</a-checkbox>
            <a-checkbox value="function_calling">函数调用</a-checkbox>
            <a-checkbox value="code">代码</a-checkbox>
          </a-checkbox-group>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="状态" field="status">
              <a-switch v-model="createForm.status" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="公开可见" field="is_public">
              <a-switch v-model="createForm.is_public" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="排序" field="sort_order">
              <a-input-number v-model="createForm.sort_order" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-divider orientation="left">快速配置（推荐）</a-divider>
        <a-form-item label="自定义端点">
          <a-input 
            v-model="createForm.custom_endpoint"
            placeholder="/api/v3/chat/completions"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的接口路径，完整地址 = 提供商 base_url + 该路径
          </div>
        </a-form-item>
        
        <template v-if="createForm.type === 'video'">
          <a-divider orientation="left">视频任务查询配置</a-divider>
          <a-form-item label="视频任务查询端点">
            <a-input 
              v-model="createForm.video_endpoint"
              placeholder="/contents/generations/tasks"
            />
            <div style="color: #999; font-size: 12px; margin-top: 4px;">
              用于查询视频生成任务状态的端点，完整地址 = 提供商 base_url + 该路径
            </div>
          </a-form-item>
        </template>
        
        <a-divider orientation="left">Responses API 配置</a-divider>
        <a-form-item label="支持 Responses API">
          <a-switch v-model="createForm.supports_response" />
        </a-form-item>
        <a-form-item label="Responses API 端点" v-if="createForm.supports_response">
          <a-input 
            v-model="createForm.response_endpoint"
            placeholder="https://ark.cn-beijing.volces.com/api/v3/responses"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的 Responses API 端点（完整地址）
          </div>
        </a-form-item>
        <a-divider orientation="left">Anthropic Messages API 配置</a-divider>
        <a-form-item label="支持 Messages API">
          <a-switch v-model="createForm.supports_messages" />
        </a-form-item>
        <a-form-item label="Messages API 端点" v-if="createForm.supports_messages">
          <a-input 
            v-model="createForm.messages_endpoint"
            placeholder="https://api.anthropic.com/v1/messages"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的 Messages API 端点（完整地址）
          </div>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 编辑模型弹窗 -->
    <a-modal
      v-model:visible="showEditModalVisible"
      title="编辑模型"
      @ok="updateModel"
      :confirm-loading="updateLoading"
      width="600px"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="提供商" field="provider_id" required>
          <a-select v-model="editForm.provider_id" placeholder="请选择提供商">
            <a-option v-for="provider in providers" :key="provider.id" :value="provider.id">
              {{ provider.name }} ({{ provider.code }})
            </a-option>
          </a-select>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="模型代码" field="code" required>
              <a-input v-model="editForm.code" placeholder="如: gpt-4" disabled />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="模型类型" field="type" required>
              <a-select v-model="editForm.type" placeholder="请选择类型">
                <a-option value="text">文本对话</a-option>
                <a-option value="embedding">向量嵌入</a-option>
                <a-option value="image">图像生成</a-option>
                <a-option value="video">视频生成</a-option>
                <a-option value="audio">语音生成</a-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="显示名称" field="name" required>
              <a-input v-model="editForm.name" placeholder="如: GPT-4" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="描述" field="description">
          <a-textarea v-model="editForm.description" :rows="3" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="最大Tokens" field="max_tokens">
              <a-input-number v-model="editForm.max_tokens" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="支持能力" field="capabilities">
          <a-checkbox-group v-model="editForm.capabilities">
            <a-checkbox value="chat">对话</a-checkbox>
            <a-checkbox value="vision">视觉</a-checkbox>
            <a-checkbox value="function_calling">函数调用</a-checkbox>
            <a-checkbox value="code">代码</a-checkbox>
          </a-checkbox-group>
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="状态" field="status">
              <a-switch v-model="editForm.status" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="公开可见" field="is_public">
              <a-switch v-model="editForm.is_public" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="排序" field="sort_order">
              <a-input-number v-model="editForm.sort_order" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-divider orientation="left">快速配置（推荐）</a-divider>
        <a-form-item label="自定义端点">
          <a-input 
            v-model="editForm.custom_endpoint"
            placeholder="/api/v3/chat/completions"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的接口路径，完整地址 = 提供商 base_url + 该路径，留空则使用提供商默认配置
          </div>
        </a-form-item>
        
        <template v-if="editForm.type === 'video'">
          <a-divider orientation="left">视频任务查询配置</a-divider>
          <a-form-item label="视频任务查询端点">
            <a-input 
              v-model="editForm.video_endpoint"
              placeholder="/contents/generations/tasks"
            />
            <div style="color: #999; font-size: 12px; margin-top: 4px;">
              用于查询视频生成任务状态的端点，完整地址 = 提供商 base_url + 该路径
            </div>
          </a-form-item>
        </template>
        
        <a-divider orientation="left">Responses API 配置</a-divider>
        <a-form-item label="支持 Responses API">
          <a-switch v-model="editForm.supports_response" />
        </a-form-item>
        <a-form-item label="Responses API 端点" v-if="editForm.supports_response">
          <a-input 
            v-model="editForm.response_endpoint"
            placeholder="https://ark.cn-beijing.volces.com/api/v3/responses"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的 Responses API 端点（完整地址）
          </div>
        </a-form-item>
        <a-divider orientation="left">Anthropic Messages API 配置</a-divider>
        <a-form-item label="支持 Messages API">
          <a-switch v-model="editForm.supports_messages" />
        </a-form-item>
        <a-form-item label="Messages API 端点" v-if="editForm.supports_messages">
          <a-input 
            v-model="editForm.messages_endpoint"
            placeholder="https://api.anthropic.com/v1/messages"
          />
          <div style="color: #999; font-size: 12px; margin-top: 4px;">
            填该模型对应的 Messages API 端点（完整地址）
          </div>
        </a-form-item>
        
        <a-divider orientation="left">高级配置（不推荐）</a-divider>
        <a-collapse v-model="advancedConfigOpen">
          <a-collapse-panel key="1" header="完整配置（JSON）">
            <a-textarea 
              v-model="editForm.configJson"
              :rows="8"
              placeholder='{
  "base_url": "https://custom-api.example.com",
  "request_format": {
    "chat_endpoint": "/v1/chat"
  }
}'
            />
            <div style="color: #999; font-size: 12px; margin-top: 4px;">
              完整配置可以覆盖提供商的所有设置（base_url、request_format、response_format 等），通常不需要
            </div>
          </a-collapse-panel>
        </a-collapse>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'

const models = ref([])
const providers = ref([])
const loading = ref(false)
const searchQuery = ref('')
const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)
const advancedConfigOpen = ref(false)

const createForm = ref({
  provider_id: null,
  code: '',
  type: 'text',
  name: '',
  description: '',
  max_tokens: 4096,
  capabilities: ['chat'],
  is_public: true,
  status: true,
  sort_order: 0,
  custom_endpoint: '',
  video_endpoint: '',
  supports_response: false,
  response_endpoint: '',
  supports_messages: false,
  messages_endpoint: ''
})

const editForm = ref({
  id: null,
  provider_id: null,
  code: '',
  type: 'text',
  name: '',
  description: '',
  max_tokens: 4096,
  capabilities: ['chat'],
  is_public: true,
  status: true,
  sort_order: 0,
  configJson: '',
  custom_endpoint: '',
  chat_endpoint: '',
  embeddings_endpoint: '',
  image_endpoint: '',
  video_endpoint: '',
  audio_endpoint: '',
  supports_response: false,
  response_endpoint: '',
  supports_messages: false,
  messages_endpoint: ''
})

const filteredModels = computed(() => {
  if (!searchQuery.value) return models.value
  const query = searchQuery.value.toLowerCase()
  return models.value.filter(m => 
    m.name.toLowerCase().includes(query) || 
    m.code.toLowerCase().includes(query)
  )
})

onMounted(() => {
  loadModels()
  loadProviders()
})

async function loadModels() {
  loading.value = true
  try {
    const response = await api.get('/admin/models')
    if (response.data.success) {
      models.value = response.data.data
      pagination.value.total = response.data.data.length
    }
  } catch (error) {
    console.error(error)
    Message.error('加载模型列表失败')
  } finally {
    loading.value = false
  }
}

async function loadProviders() {
  try {
    const response = await api.get('/admin/ai-providers')
    if (response.data.success) {
      providers.value = response.data.data
    }
  } catch (error) {
    console.error(error)
    Message.error('加载提供商列表失败')
  }
}

async function createModel() {
  createLoading.value = true
  try {
    const response = await api.post('/admin/models', createForm.value)
    if (response.data.success) {
      showCreateModal.value = false
      Message.success('创建成功')
      await loadModels()
    }
  } catch (error) {
    console.error(error)
    Message.error(error.response?.data?.message || '创建失败')
  } finally {
    createLoading.value = false
  }
}

function showEditModal(record) {
  editForm.value = { 
    ...record, 
    capabilities: record.capabilities || ['chat'],
    // 确保编辑时使用原始模型代码，而不是完整格式
    code: record.code,
    configJson: record.config ? JSON.stringify(record.config, null, 2) : '',
    custom_endpoint: record.custom_endpoint || '',
    chat_endpoint: record.chat_endpoint || '',
    embeddings_endpoint: record.embeddings_endpoint || '',
    image_endpoint: record.image_endpoint || '',
    video_endpoint: record.video_endpoint || '',
    audio_endpoint: record.audio_endpoint || '',
    supports_response: record.supports_response || false,
    response_endpoint: record.response_endpoint || '',
    supports_messages: record.supports_messages || false,
    messages_endpoint: record.messages_endpoint || ''
  }
  advancedConfigOpen.value = false  // 默认收起高级配置
  showEditModalVisible.value = true
}

async function updateModel() {
  updateLoading.value = true
  try {
    const data = { ...editForm.value }
    
    // 解析 configJson 为 config 对象
    if (data.configJson && data.configJson.trim()) {
      try {
        data.config = JSON.parse(data.configJson)
      } catch (e) {
        Message.error('配置JSON格式错误')
        return
      }
    } else {
      data.config = null
    }
    
    // 删除临时字段
    delete data.configJson
    
    const response = await api.put(`/admin/models/${editForm.value.id}`, data)
    if (response.data.success) {
      showEditModalVisible.value = false
      Message.success('更新成功')
      await loadModels()
    }
  } catch (error) {
    console.error(error)
    Message.error(error.response?.data?.message || '更新失败')
  } finally {
    updateLoading.value = false
  }
}

async function deleteModel(id) {
  try {
    const response = await api.delete(`/admin/models/${id}`)
    if (response.data.success) {
      Message.success('删除成功')
      await loadModels()
    }
  } catch (error) {
    console.error(error)
    Message.error(error.response?.data?.message || '删除失败')
  }
}

function handlePageChange(page) {
  pagination.value.current = page
}

async function toggleModelStatus(record, val) {
  try {
    await api.put(`/admin/models/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    // 更新本地数据
    record.status = val
  } catch (error) {
    Message.error('状态更新失败')
    // 还原状态
    record.status = !val
  }
}

function getTypeName(type) {
  const typeMap = {
    'text': '文本对话',
    'embedding': '向量嵌入',
    'image': '图像生成',
    'video': '视频生成',
    'audio': '语音生成'
  }
  return typeMap[type] || type
}

function getTypeColor(type) {
  const colorMap = {
    'text': 'arcoblue',
    'embedding': 'green',
    'image': 'orange',
    'video': 'purple',
    'audio': 'red'
  }
  return colorMap[type] || 'default'
}
</script>

<style scoped>
.models-container {
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
