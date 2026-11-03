<template>
  <div class="providers-container">
    <div class="page-header">
      <h1>AI提供商管理</h1>
      <p>管理AI服务提供商、API密钥和模型配置</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-input-search
          v-model="searchKeyword"
          placeholder="搜索提供商名称或代码"
          allow-clear
          @search="handleSearch"
          @press-enter="handleSearch"
        />
      </a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="18" style="text-align: right;">
        <a-button type="primary" @click="showAddModal = true">
          <template #icon><icon-plus /></template>
          添加提供商
        </a-button>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
      <a-table
        :data="providers"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1200 }"
        @page-change="onPageChange"
      >
        <template #columns>
          <a-table-column title="提供商" width="260" fixed="left">
            <template #cell="{ record }">
              <div style="display: flex; align-items: center; gap: 12px;">
                <div style="width: 40px; height: 40px; border-radius: 10px; background: linear-gradient(135deg, #165dff 0%, #722ed1 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: 600; flex-shrink: 0;">{{ record.name.charAt(0).toUpperCase() }}</div>
                <div style="display: flex; flex-direction: column; gap: 4px;">
                  <div style="font-weight: 600; color: var(--text-primary);">{{ record.name }}</div>
                  <a-tag size="small" color="arcoblue">{{ record.code }}</a-tag>
                </div>
              </div>
            </template>
          </a-table-column>
          
          <a-table-column title="API地址" width="300">
            <template #cell="{ record }">
              <a-typography-text ellipsis>{{ record.base_url }}</a-typography-text>
            </template>
          </a-table-column>
          
          <a-table-column title="认证类型" width="140">
            <template #cell="{ record }">
              <a-tag :color="getAuthTypeColor(record.auth_type)">{{ getAuthTypeName(record.auth_type) }}</a-tag>
            </template>
          </a-table-column>
          
          <a-table-column title="资源统计" width="180">
            <template #cell="{ record }">
              <div style="display: flex; gap: 16px;">
                <span><strong>{{ record.models_count }}</strong> 模型</span>
                <span><strong>{{ record.api_keys_count }}</strong> 密钥</span>
              </div>
            </template>
          </a-table-column>
          
          <a-table-column title="状态" width="100">
            <template #cell="{ record }">
              <a-switch
                :model-value="!!record.status"
                :active-value="true"
                :inactive-value="false"
                size="small"
                @change="(val) => toggleProviderStatus(record, val)"
              />
            </template>
          </a-table-column>
          
          <a-table-column title="操作" width="230" fixed="right">
            <template #cell="{ record }">
              <a-space>
                <a-button size="small" @click="editProvider(record)">编辑</a-button>
                <a-button size="small" @click="manageModels(record)">模型</a-button>
                <a-button size="small" @click="manageKeys(record)">密钥</a-button>
              </a-space>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 添加/编辑提供商弹窗 -->
    <a-modal
      v-model:visible="showAddModal"
      :title="editingProvider ? '编辑提供商' : '添加提供商'"
      width="650px"
      @ok="handleSubmit"
      @cancel="resetForm"
    >
      <a-form :model="form" layout="vertical">
        <a-divider orientation="left">基础信息</a-divider>
        
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="提供商名称" required>
              <a-input v-model="form.name" placeholder="如: 字节跳动-豆包" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="提供商代码" required>
              <a-input
                v-model="form.code"
                placeholder="如: doubao"
                :disabled="!!editingProvider"
              />
            </a-form-item>
          </a-col>
        </a-row>
        
        <a-form-item 
          label="API地址" 
          required
          extra="支持两种模式：1) 基础地址 https://api.example.com/v1  2) 完整地址 https://api.example.com/v3/chat/completions&lt;br/&gt;【火山引擎提示】推荐填：https://ark.cn-beijing.volces.com/api/v3"
        >
          <a-input 
            v-model="form.base_url" 
            placeholder="https://ark.cn-beijing.volces.com/api/v3" 
          />
        </a-form-item>
        
        <a-form-item label="认证类型" required>
          <a-select v-model="form.auth_type">
            <a-option value="bearer">Bearer Token (Authorization: Bearer xxx)</a-option>
            <a-option value="api_key">API Key</a-option>
            <a-option value="api_key_header">自定义Header</a-option>
            <a-option value="hmac">HMAC签名</a-option>
          </a-select>
        </a-form-item>

        <a-form-item 
          label="适配器类型" 
          extra="选择合适的适配器来处理该提供商的API调用">
          <a-select 
            v-model="form.adapter_class" 
            placeholder="选择适配器（留空则自动选择）"
            allow-clear>
            <a-option 
              v-for="adapter in availableAdapters" 
              :key="adapter.value"
              :value="adapter.value">
              {{ adapter.label }}
            </a-option>
          </a-select>
        </a-form-item>
        
        <a-form-item label="文档地址">
          <a-input v-model="form.doc_url" placeholder="https://docs.example.com" />
        </a-form-item>
        
        <a-divider orientation="left">高级配置 (可选)</a-divider>
        
        <a-form-item label="自定义 API 端点路径" extra="如果API地址中不包含具体路径，可以在此配置，如：/api/v3/chat/completions">
          <a-input v-model="form.request_format.chat_endpoint" placeholder="/chat/completions" />
        </a-form-item>
        
        <a-form-item label="启用状态">
          <a-switch 
            :model-value="!!form.status"
            :active-value="true"
            :inactive-value="false"
            @update:model-value="(val) => (form.status = val)"
          />
        </a-form-item>
      </a-form>
    </a-modal>
    

    
    <!-- 密钥管理弹窗 -->
    <a-modal
      v-model:visible="showKeysModal"
      :title="currentProvider?.name + ' - API密钥管理'"
      width="850px"
      :footer="false"
    >
      <div style="margin-bottom: 16px; text-align: right;">
        <a-button type="primary" @click="showAddKey = true">
          <template #icon><icon-plus /></template>
          添加密钥
        </a-button>
      </div>
      
      <a-table :data="providerKeys" :loading="keysLoading" :pagination="{ pageSize: 5 }" :scroll="{ x: 1200 }">
        <template #columns>
          <a-table-column title="密钥名称" data-index="key_name" width="180" fixed="left" />
          <a-table-column title="API Key" data-index="api_key_masked" width="300" />
          <a-table-column title="权重" data-index="weight" width="100" align="center" />
          <a-table-column title="使用次数" data-index="usage_count" width="120" align="center" />
          <a-table-column title="状态" data-index="status" width="100" align="center">
            <template #cell="{ record }">
              <a-switch
                :model-value="!!record.status"
                size="small"
                @change="(val) => toggleKeyStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="操作" width="80" align="center" fixed="right">
            <template #cell="{ record }">
              <a-button type="text" size="small" @click="editKey(record)">编辑</a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-modal>
    
    <!-- 添加/编辑密钥 -->
    <a-modal
      v-model:visible="showAddKey"
      :title="editingKey ? '编辑密钥' : '添加密钥'"
      @ok="handleKeySubmit"
      @cancel="resetKeyForm"
    >
      <a-form :model="keyForm" layout="vertical">
        <a-form-item label="密钥名称" required>
          <a-input v-model="keyForm.key_name" placeholder="如: 生产环境密钥" />
        </a-form-item>
        <a-form-item label="API Key" required>
          <a-input-password v-model="keyForm.api_key" placeholder="输入API密钥" />
        </a-form-item>
        <a-form-item label="权重">
          <a-input-number v-model="keyForm.weight" :min="1" :default-value="1" />
          <div style="color: var(--color-text-3); font-size: 12px; margin-top: 4px;">权重越高，被调用的概率越大</div>
        </a-form-item>
        <a-form-item label="启用状态">
          <a-switch v-model="keyForm.status" />
        </a-form-item>
      </a-form>
    </a-modal>
    
    <!-- 模型管理弹窗 -->
    <a-modal
      v-model:visible="showModelsModal"
      :title="currentProvider?.name + ' - 模型管理'"
      width="950px"
      :footer="false"
    >
      <div style="margin-bottom: 16px; display: flex; justify-content: space-between; align-items: center; gap: 12px;">
        <a-input-search
          v-model="modelSearch"
          placeholder="搜索模型代码 / 显示名称"
          allow-clear
          style="width: 260px;"
          @search="handleModelSearch"
          @press-enter="handleModelSearch"
          @clear="handleModelSearch"
        />
        <a-button type="primary" @click="showAddModel = true">
          <template #icon><icon-plus /></template>
          添加模型
        </a-button>
      </div>
      
      <a-table
        :data="providerModels"
        :loading="modelsLoading"
        :scroll="{ x: 1200 }"
        :pagination="{
          showTotal: true,
          total: modelsPagination.total,
          current: modelsPagination.page,
          pageSize: modelsPagination.pageSize
        }"
        @page-change="handleModelsPageChange"
      >
        <template #columns>
          <a-table-column title="模型类型" data-index="type" width="110" fixed="left">
            <template #cell="{ record }">
              <a-tag :color="getModelTypeColor(record.type)">{{ getModelTypeName(record.type) }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="模型代码" data-index="code" width="180">
            <template #cell="{ record }">
              <a-tag color="arcoblue">{{ record.code }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="显示名称" data-index="name" width="200" />
          <a-table-column title="最大Tokens" data-index="max_tokens" width="110" align="center" />
          <a-table-column title="输入价格" width="150" align="center">
            <template #cell="{ record }">
              <span>¥{{ Number(record.input_price || 0).toFixed(4) }} / 百万tokens</span>
            </template>
          </a-table-column>
          <a-table-column title="输出价格" width="150" align="center">
            <template #cell="{ record }">
              <span>¥{{ Number(record.output_price || 0).toFixed(4) }} / 百万tokens</span>
            </template>
          </a-table-column>
          <a-table-column title="状态" data-index="status" width="90" align="center">
            <template #cell="{ record }">
              <a-switch
                :model-value="!!record.status"
                :active-value="true"
                :inactive-value="false"
                size="small"
                @change="(val) => toggleProviderModelStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="Responses API" data-index="supports_response" width="120" align="center">
            <template #cell="{ record }">
              <a-tag :color="record.supports_response ? 'cyan' : 'default'">{{ record.supports_response ? '已启用' : '未启用' }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="Messages API" data-index="supports_messages" width="120" align="center">
            <template #cell="{ record }">
              <a-tag :color="record.supports_messages ? 'purple' : 'default'">{{ record.supports_messages ? '已启用' : '未启用' }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="操作" width="80" align="center" fixed="right">
            <template #cell="{ record }">
              <a-button type="text" size="small" @click="editModel(record)">编辑</a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-modal>
    
    <!-- 添加/编辑模型 -->
    <a-modal
      v-model:visible="showAddModel"
      :title="editingModel ? '编辑模型' : '添加模型'"
      @ok="handleModelSubmit"
      @cancel="resetModelForm"
      width="650px"
    >
      <a-form :model="modelForm" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="模型类型" field="type" required>
              <a-select v-model="modelForm.type" placeholder="请选择类型">
                <a-option value="text">文本对话</a-option>
                <a-option value="embedding">向量嵌入</a-option>
                <a-option value="image">图像生成</a-option>
                <a-option value="video">视频生成</a-option>
                <a-option value="audio">语音生成</a-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="模型代码" required>
              <a-input
                v-model="modelForm.code"
                placeholder="如: gpt-4"
                :disabled="!!editingModel"
              />
            </a-form-item>
          </a-col>
        </a-row>
        
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="显示名称" required>
              <a-input v-model="modelForm.name" placeholder="如: GPT-4" />
            </a-form-item>
          </a-col>
        </a-row>
        
        <a-form-item label="模型描述">
          <a-textarea v-model="modelForm.description" :rows="2" />
        </a-form-item>
        
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="最大Tokens">
              <a-input-number v-model="modelForm.max_tokens" :min="1" style="width: 100%;" />
            </a-form-item>
            <a-form-item label="输入价格(¥/百万tokens)">
              <a-input-number v-model="modelForm.input_price" :min="0" :precision="4" :step="0.1" placeholder="0.0000" style="width: 100%;" />
            </a-form-item>
            <a-form-item label="输出价格(¥/百万tokens)">
              <a-input-number v-model="modelForm.output_price" :min="0" :precision="4" :step="0.1" placeholder="0.0000" style="width: 100%;" />
            </a-form-item>
          </a-col>
        </a-row>
        
        <a-form-item label="支持能力">
          <a-checkbox-group v-model="modelForm.capabilities">
            <a-checkbox value="chat">对话</a-checkbox>
            <a-checkbox value="vision">视觉</a-checkbox>
            <a-checkbox value="function_calling">函数调用</a-checkbox>
            <a-checkbox value="code">代码</a-checkbox>
          </a-checkbox-group>
        </a-form-item>
        
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="公开可见">
              <a-switch v-model="modelForm.is_public" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="启用状态">
              <a-switch v-model="modelForm.status" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="排序">
              <a-input-number v-model="modelForm.sort_order" :min="0" style="width: 100%;" />
            </a-form-item>
          </a-col>
        </a-row>
        
        <a-divider orientation="left">快速配置（推荐）</a-divider>
        <a-form-item label="自定义端点" extra="填该模型对应的接口路径，完整地址 = 提供商 base_url + 该路径，留空则使用提供商默认配置">
          <a-input 
            v-model="modelForm.custom_endpoint"
            placeholder="/api/v3/chat/completions"
          />
        </a-form-item>
        
        <template v-if="modelForm.type === 'video'">
          <a-divider orientation="left">视频任务查询配置"></a-divider>
          <a-form-item label="视频任务查询端点" extra="用于查询视频生成任务状态的端点，完整地址 = 提供商 base_url + 该路径">
            <a-input 
              v-model="modelForm.video_endpoint"
              placeholder="/contents/generations/tasks"
            />
          </a-form-item>
        </template>
        
        <a-divider orientation="left">Responses API 配置</a-divider>
        <a-form-item label="支持 Responses API">
          <a-switch v-model="modelForm.supports_response" />
        </a-form-item>
        <a-form-item label="Responses API 端点" v-if="modelForm.supports_response">
          <a-input 
            v-model="modelForm.response_endpoint"
            placeholder="https://ark.cn-beijing.volces.com/api/v3/responses"
          />
        </a-form-item>
        
        <a-divider orientation="left">Anthropic Messages API 配置</a-divider>
        <a-form-item label="支持 Messages API">
          <a-switch v-model="modelForm.supports_messages" />
        </a-form-item>
        <a-form-item label="Messages API 端点" v-if="modelForm.supports_messages">
          <a-input 
            v-model="modelForm.messages_endpoint"
            placeholder="https://api.anthropic.com/v1/messages"
          />
        </a-form-item>
        
        <a-divider orientation="left">高级配置（不推荐）</a-divider>
        <a-collapse v-model="advancedConfigOpen">
          <a-collapse-panel key="1" header="完整配置（JSON）">
            <a-textarea 
              v-model="modelForm.configJson"
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
import { ref, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import api from '../../utils/api'
import { IconPlus, IconApps, IconLock, IconLink } from '@arco-design/web-vue/es/icon'

const providers = ref([])

const currentProvider = ref(null)
const providerKeys = ref([])
const providerModels = ref([])
const loading = ref(false)
const keysLoading = ref(false)
const modelsLoading = ref(false)
const modelsPagination = ref({ page: 1, pageSize: 10, total: 0 })
const modelSearch = ref('')

const searchKeyword = ref('')

const showAddModal = ref(false)

const showKeysModal = ref(false)
const showModelsModal = ref(false)
const showAddKey = ref(false)
const showAddModel = ref(false)
const advancedConfigOpen = ref(false)

const editingProvider = ref(null)
const editingKey = ref(null)
const editingModel = ref(null)

const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const form = ref({
  code: '',
  name: '',
  base_url: '',
  auth_type: 'bearer',
  doc_url: '',
  status: true,
  adapter_class: '',
  request_format: {
    chat_endpoint: ''
  }
})

// 预定义可用的适配器列表
const availableAdapters = [
  { label: '通用适配器（推荐）', value: 'generic' },
  { label: 'OpenAI 适配器', value: 'openai' },
  { label: '豆包/火山引擎适配器', value: 'doubao' },
  { label: '通义千问适配器', value: 'qwen' },
  { label: '智谱AI适配器', value: 'zhipu' },
  { label: 'DeepSeek适配器', value: 'deepseek' },
  { label: 'Claude适配器', value: 'claude' },
  { label: '混元适配器', value: 'hunyuan' },
]

const keyForm = ref({
  key_name: '',
  api_key: '',
  weight: 1,
  status: true,
})

const modelForm = ref({
  type: 'text',
  code: '',
  name: '',
  description: '',
  max_tokens: 4096,
  input_price: 0,
  output_price: 0,
  capabilities: ['chat'],
  is_public: true,
  status: true,
  sort_order: 0,
  custom_endpoint: '',
  video_endpoint: '',
  configJson: '',
  supports_response: false,
  response_endpoint: '',
  supports_messages: false,
  messages_endpoint: '',
})

onMounted(() => {
  loadProviders()

})

async function loadProviders() {
  loading.value = true
  try {
    const params = {
      page: pagination.value.current,
      per_page: pagination.value.pageSize
    }
    if (searchKeyword.value) {
      params.search = searchKeyword.value
    }
    const res = await api.get('/admin/ai-providers', { params })
    if (res.data.success) {
      providers.value = res.data.data.data || res.data.data
      pagination.value.total = res.data.data.total || providers.value.length
    }
  } catch (error) {
    Message.error('加载提供商失败: ' + (error.response?.data?.message || error.message))
  } finally {
    loading.value = false
  }
}



function handleSearch() {
  pagination.value.current = 1
  loadProviders()
}

function onPageChange(page) {
  pagination.value.current = page
  loadProviders()
}

function getAuthTypeName(type) {
  const names = {
    bearer: 'Bearer Token',
    api_key: 'API Key',
    api_key_header: '自定义Header',
    hmac: 'HMAC签名',
  }
  return names[type] || type
}

function getAuthTypeColor(type) {
  const colors = {
    bearer: 'blue',
    api_key: 'purple',
    api_key_header: 'orange',
    hmac: 'red',
  }
  return colors[type] || 'default'
}

function openDoc(url) {
  window.open(url, '_blank')
}



function editProvider(provider) {
  editingProvider.value = provider
  Object.assign(form.value, {
    code: provider.code,
    name: provider.name,
    base_url: provider.base_url,
    auth_type: provider.auth_type,
    doc_url: provider.doc_url || '',
    status: provider.status,
    adapter_class: provider.adapter_class || '',
    request_format: provider.request_format || { chat_endpoint: '' }
  })
  showAddModal.value = true
}

async function toggleProviderStatus(provider, val) {
  try {
    await api.put(`/admin/ai-providers/${provider.id}/toggle-status`)
    Message.success('状态更新成功')
    loadProviders()
  } catch (error) {
    Message.error('状态更新失败')
  }
}

async function handleSubmit() {
  try {
    const data = {
      code: form.value.code,
      name: form.value.name,
      base_url: form.value.base_url,
      auth_type: form.value.auth_type,
      doc_url: form.value.doc_url,
      status: form.value.status,
      adapter_class: form.value.adapter_class,
      request_format: form.value.request_format
    }
    
    if (editingProvider.value) {
      await api.put(`/admin/ai-providers/${editingProvider.value.id}`, data)
      Message.success('更新成功')
    } else {
      await api.post('/admin/ai-providers', data)
      Message.success('创建成功')
    }
    
    showAddModal.value = false
    resetForm()
    loadProviders()
  } catch (error) {
    Message.error(error.response?.data?.message || error.response?.data?.errors?.[0] || '操作失败')
  }
}

function resetForm() {
  editingProvider.value = null
  form.value.code = ''
  form.value.name = ''
  form.value.base_url = ''
  form.value.auth_type = 'bearer'
  form.value.doc_url = ''
  form.value.status = true
  form.value.adapter_class = ''
  form.value.request_format = {
    chat_endpoint: ''
  }
}

async function deleteProvider(provider) {
  try {
    await api.delete(`/admin/ai-providers/${provider.id}`)
    Message.success('删除成功')
    loadProviders()
  } catch (error) {
    Message.error(error.response?.data?.message || '删除失败')
  }
}

async function manageKeys(provider) {
  currentProvider.value = provider
  showKeysModal.value = true
  await loadProviderKeys(provider.id)
}

async function loadProviderKeys(providerId) {
  keysLoading.value = true
  try {
    const res = await api.get(`/admin/provider-keys/by-provider/${providerId}`)
    if (res.data.success) {
      providerKeys.value = res.data.data
    }
  } catch (error) {
    Message.error('加载密钥失败')
  } finally {
    keysLoading.value = false
  }
}

function editKey(key) {
  editingKey.value = key
  Object.assign(keyForm.value, {
    key_name: key.key_name,
    api_key: '',
    weight: key.weight,
    status: key.status,
  })
  showAddKey.value = true
}

async function handleKeySubmit() {
  try {
    const data = { ...keyForm.value, ai_provider_id: currentProvider.value.id }
    
    // 编辑时如果 api_key 为空，则不更新密钥
    if (editingKey.value && !data.api_key) {
      delete data.api_key
    }
    
    if (editingKey.value) {
      await api.put(`/admin/provider-keys/${editingKey.value.id}`, data)
      Message.success('更新成功')
    } else {
      await api.post('/admin/provider-keys', data)
      Message.success('添加成功')
    }
    
    showAddKey.value = false
    resetKeyForm()
    loadProviderKeys(currentProvider.value.id)
    loadProviders()
  } catch (error) {
    Message.error(error.response?.data?.message || '操作失败')
  }
}

function resetKeyForm() {
  editingKey.value = null
  keyForm.value.key_name = ''
  keyForm.value.api_key = ''
  keyForm.value.weight = 1
  keyForm.value.status = true
}

async function deleteKey(key) {
  try {
    await api.delete(`/admin/provider-keys/${key.id}`)
    Message.success('删除成功')
    loadProviderKeys(currentProvider.value.id)
    loadProviders()
  } catch (error) {
    Message.error('删除失败')
  }
}

async function toggleKeyStatus(key, val) {
  try {
    await api.put(`/admin/provider-keys/${key.id}/toggle-status`)
    Message.success('状态更新成功')
    loadProviderKeys(currentProvider.value.id)
  } catch (error) {
    Message.error('状态更新失败')
  }
}

async function toggleProviderModelStatus(model, val) {
  try {
    await api.put(`/admin/models/${model.id}/toggle-status`)
    Message.success('状态更新成功')
    // 更新本地数据
    model.status = val
  } catch (error) {
    Message.error('状态更新失败')
    // 还原状态
    model.status = !val
  }
}

async function manageModels(provider) {
  currentProvider.value = provider
  showModelsModal.value = true
  modelsPagination.value.page = 1
  modelSearch.value = ''
  await loadProviderModels(provider.id)
}

async function loadProviderModels(providerId) {
  modelsLoading.value = true
  try {
    const params = {
      page: modelsPagination.value.page,
      per_page: modelsPagination.value.pageSize
    }
    if (modelSearch.value) params.keyword = modelSearch.value
    const res = await api.get(`/admin/ai-providers/${providerId}/models`, { params })
    if (res.data.success) {
      providerModels.value = res.data.data.data || []
      modelsPagination.value.total = res.data.data.meta?.total || 0
    }
  } catch (error) {
    Message.error('加载模型失败')
  } finally {
    modelsLoading.value = false
  }
}

function handleModelsPageChange(page) {
  modelsPagination.value.page = page
  loadProviderModels(currentProvider.value.id)
}

function handleModelSearch() {
  modelsPagination.value.page = 1
  loadProviderModels(currentProvider.value.id)
}

function editModel(model) {
  editingModel.value = model
  Object.assign(modelForm.value, {
    type: model.type || 'text',
    code: model.code,
    name: model.name,
    description: model.description || '',
    max_tokens: model.max_tokens || 4096,
    input_price: Number(model.input_price || 0),
    output_price: Number(model.output_price || 0),
    capabilities: model.capabilities || ['chat'],
    is_public: model.is_public,
    status: model.status,
    sort_order: model.sort_order || 0,
    custom_endpoint: model.custom_endpoint || '',
    video_endpoint: model.video_endpoint || '',
    configJson: model.config ? JSON.stringify(model.config, null, 2) : '',
    supports_response: model.supports_response || false,
    response_endpoint: model.response_endpoint || '',
    supports_messages: model.supports_messages || false,
    messages_endpoint: model.messages_endpoint || '',
  })
  advancedConfigOpen.value = false
  showAddModel.value = true
}

async function handleModelSubmit() {
  try {
    const data = { ...modelForm.value }
    
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
    
    delete data.configJson
    
    if (editingModel.value) {
      await api.put(`/admin/ai-providers/${currentProvider.value.id}/models/${editingModel.value.id}`, data)
      Message.success('更新成功')
    } else {
      await api.post(`/admin/ai-providers/${currentProvider.value.id}/models`, data)
      Message.success('添加成功')
    }
    
    showAddModel.value = false
    resetModelForm()
    loadProviderModels(currentProvider.value.id)
    loadProviders()
  } catch (error) {
    Message.error(error.response?.data?.message || '操作失败')
  }
}

function resetModelForm() {
  editingModel.value = null
  advancedConfigOpen.value = false
  modelForm.value = {
    type: 'text',
    code: '',
    name: '',
    description: '',
    max_tokens: 4096,
    input_price: 0,
    output_price: 0,
    capabilities: ['chat'],
    is_public: true,
    status: true,
    sort_order: 0,
    custom_endpoint: '',
    video_endpoint: '',
    configJson: '',
    supports_response: false,
    response_endpoint: '',
    supports_messages: false,
    messages_endpoint: '',
  }
}

function getModelTypeName(type) {
  const typeMap = {
    'text': '文本对话',
    'embedding': '向量嵌入',
    'image': '图像生成',
    'video': '视频生成',
    'audio': '语音生成'
  }
  return typeMap[type] || type
}

function getModelTypeColor(type) {
  const colorMap = {
    'text': 'arcoblue',
    'embedding': 'green',
    'image': 'orange',
    'video': 'purple',
    'audio': 'red'
  }
  return colorMap[type] || 'default'
}

async function deleteModel(model) {
  try {
    await api.delete(`/admin/ai-providers/${currentProvider.value.id}/models/${model.id}`)
    Message.success('删除成功')
    loadProviderModels(currentProvider.value.id)
    loadProviders()
  } catch (error) {
    Message.error('删除失败')
  }
}
</script>

<style scoped>
.providers-container {
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
