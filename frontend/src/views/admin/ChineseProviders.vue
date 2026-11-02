<template>
  <div class="chinese-providers-container">
    <div class="page-header">
      <h1>国产AI厂商</h1>
      <p>配置和管理国内AI服务提供商</p>
    </div>
    
    <!-- 操作栏 -->
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="24" :md="24" :lg="24" style="text-align: right;">
        <a-space>
          <a-button type="primary" @click="showAddModal = true">
            <icon-plus />
            添加配置
          </a-button>
        </a-space>
      </a-col>
    </a-row>

    <a-row :gutter="16">
      <!-- 字节跳动豆包 -->
      <a-col :xs="24" :sm="24" :md="12" :lg="12" style="margin-bottom: 16px;">
        <a-card title="字节跳动 - 豆包" class="provider-card content-card">
          <template #extra>
            <a-tag color="green">已支持</a-tag>
          </template>
          <a-descriptions :data="doubaoDescriptions" layout="vertical" bordered />
          <div style="margin-top: 16px;">
            <a-space>
              <a-button type="primary" size="small" @click="editProvider('doubao')">
                <icon-edit />
                配置密钥
              </a-button>
              <a-button size="small" @click="viewModels('doubao')">
                <icon-eye />
                查看模型
              </a-button>
            </a-space>
          </div>
        </a-card>
      </a-col>

      <!-- 阿里通义千问 -->
      <a-col :xs="24" :sm="24" :md="12" :lg="12" style="margin-bottom: 16px;">
        <a-card title="阿里云 - 通义千问" class="provider-card content-card">
          <template #extra>
            <a-tag color="green">已支持</a-tag>
          </template>
          <a-descriptions :data="qwenDescriptions" layout="vertical" bordered />
          <div style="margin-top: 16px;">
            <a-space>
              <a-button type="primary" size="small" @click="editProvider('qwen')">
                <icon-edit />
                配置密钥
              </a-button>
              <a-button size="small" @click="viewModels('qwen')">
                <icon-eye />
                查看模型
              </a-button>
            </a-space>
          </div>
        </a-card>
      </a-col>

      <!-- 腾讯混元 -->
      <a-col :xs="24" :sm="24" :md="12" :lg="12" style="margin-bottom: 16px;">
        <a-card title="腾讯云 - 混元" class="provider-card content-card">
          <template #extra>
            <a-tag color="green">已支持</a-tag>
          </template>
          <a-descriptions :data="hunyuanDescriptions" layout="vertical" bordered />
          <div style="margin-top: 16px;">
            <a-space>
              <a-button type="primary" size="small" @click="editProvider('hunyuan')">
                <icon-edit />
                配置密钥
              </a-button>
              <a-button size="small" @click="viewModels('hunyuan')">
                <icon-eye />
                查看模型
              </a-button>
            </a-space>
          </div>
        </a-card>
      </a-col>

      <!-- DeepSeek -->
      <a-col :xs="24" :sm="24" :md="12" :lg="12" style="margin-bottom: 16px;">
        <a-card title="DeepSeek" class="provider-card content-card">
          <template #extra>
            <a-tag color="green">已支持</a-tag>
          </template>
          <a-descriptions :data="deepseekDescriptions" layout="vertical" bordered />
          <div style="margin-top: 16px;">
            <a-space>
              <a-button type="primary" size="small" @click="editProvider('deepseek')">
                <icon-edit />
                配置密钥
              </a-button>
              <a-button size="small" @click="viewModels('deepseek')">
                <icon-eye />
                查看模型
              </a-button>
            </a-space>
          </div>
        </a-card>
      </a-col>
    </a-row>

    <!-- 配置列表 -->
    <a-card title="已配置密钥" class="content-card" style="margin-top: 16px;">
      <a-table :data="providerKeys" :loading="loading">
        <template #columns>
          <a-table-column title="提供商" data-index="provider">
            <template #cell="{ record }">
              <a-space>
                <a-tag :color="getProviderColor(record.provider)">
                  {{ getProviderName(record.provider) }}
                </a-tag>
              </a-space>
            </template>
          </a-table-column>
          <a-table-column title="密钥名称" data-index="name" />
          <a-table-column title="API Key" data-index="api_key">
            <template #cell="{ record }">
              {{ maskApiKey(record.api_key) }}
            </template>
          </a-table-column>
          <a-table-column title="状态" data-index="status">
            <template #cell="{ record }">
              <a-switch
                :model-value="record.status === 1"
                @change="(val) => toggleStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at" />
          <a-table-column title="操作" width="80">
            <template #cell="{ record }">
              <a-button type="text" size="small" @click="editKey(record)">
                <icon-edit />
              </a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 添加/编辑配置弹窗 -->
    <a-modal
      v-model:visible="showAddModal"
      :title="editingKey ? '编辑配置' : '添加配置'"
      @ok="handleSubmit"
      @cancel="resetForm"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item label="提供商" required>
          <a-select v-model="form.provider" placeholder="选择提供商">
            <a-option value="doubao">字节跳动 - 豆包</a-option>
            <a-option value="qwen">阿里云 - 通义千问</a-option>
            <a-option value="hunyuan">腾讯云 - 混元</a-option>
            <a-option value="deepseek">DeepSeek</a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="配置名称" required>
          <a-input v-model="form.name" placeholder="例如：豆包生产环境" />
        </a-form-item>
        <a-form-item label="API Key" required>
          <a-input v-model="form.api_key" type="password" placeholder="输入API密钥" />
        </a-form-item>
        <a-form-item v-if="form.provider === 'hunyuan'" label="Secret ID">
          <a-input v-model="form.secret_id" placeholder="腾讯云Secret ID" />
        </a-form-item>
        <a-form-item v-if="form.provider === 'hunyuan'" label="Secret Key">
          <a-input v-model="form.secret_key" type="password" placeholder="腾讯云Secret Key" />
        </a-form-item>
        <a-form-item label="备注">
          <a-textarea v-model="form.description" :rows="3" placeholder="可选，添加配置说明" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 查看模型弹窗 -->
    <a-modal
      v-model:visible="showModelsModal"
      title="支持的模型列表"
      width="800px"
    >
      <a-table :data="currentModels" :pagination="false">
        <template #columns>
          <a-table-column title="模型ID" data-index="id" />
          <a-table-column title="显示名称" data-index="display_name" />
          <a-table-column title="操作">
            <template #cell="{ record }">
              <a-button type="primary" size="small" @click="addModelToSystem(record)">
                添加到系统
              </a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import api from '../../utils/api'
import {
  IconPlus,
  IconEdit,
  IconDelete,
  IconEye
} from '@arco-design/web-vue/es/icon'

const loading = ref(false)
const showAddModal = ref(false)
const showModelsModal = ref(false)
const editingKey = ref(null)
const providerKeys = ref([])
const currentModels = ref([])

const form = reactive({
  provider: '',
  name: '',
  api_key: '',
  secret_id: '',
  secret_key: '',
  description: ''
})

// 提供商描述信息
const doubaoDescriptions = [
  { label: '接入方式', value: 'OpenAI兼容API' },
  { label: '文档地址', value: 'https://www.volcengine.com/docs/82379' },
  { label: '支持模型', value: '豆包Pro/Lite系列、Vision系列' },
  { label: '特点', value: '字节跳动自研大模型，中文理解能力强' }
]

const qwenDescriptions = [
  { label: '接入方式', value: '灵积平台API' },
  { label: '文档地址', value: 'https://help.aliyun.com/document_detail/611411.html' },
  { label: '支持模型', value: '通义千问Turbo/Plus/Max系列' },
  { label: '特点', value: '阿里云出品，多模态能力强' }
]

const hunyuanDescriptions = [
  { label: '接入方式', value: '腾讯云API (TC3-HMAC-SHA256签名)' },
  { label: '文档地址', value: 'https://cloud.tencent.com/document/product/1729' },
  { label: '支持模型', value: '混元Lite/Standard/Pro系列' },
  { label: '特点', value: '腾讯出品，具备Function Calling能力' }
]

const deepseekDescriptions = [
  { label: '接入方式', value: 'OpenAI兼容API' },
  { label: '文档地址', value: 'https://platform.deepseek.com/docs' },
  { label: '支持模型', value: 'DeepSeek-V3/R1/Coder' },
  { label: '特点', value: '国产开源大模型，性能优秀价格低' }
]

onMounted(() => {
  loadProviderKeys()
})

async function loadProviderKeys() {
  loading.value = true
  try {
    const res = await api.get('/admin/provider-keys')
    if (res.data.success) {
      // 只显示国产厂商的配置
      const chineseProviders = ['doubao', 'volcengine', 'qwen', 'dashscope', 'hunyuan', 'tencent', 'deepseek']
      providerKeys.value = res.data.data.data.filter(key => chineseProviders.includes(key.provider))
    }
  } catch (error) {
    Message.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

function getProviderName(provider) {
  const names = {
    doubao: '字节跳动-豆包',
    volcengine: '火山引擎',
    qwen: '阿里-通义千问',
    dashscope: '阿里-灵积',
    hunyuan: '腾讯-混元',
    tencent: '腾讯云',
    deepseek: 'DeepSeek'
  }
  return names[provider] || provider
}

function getProviderColor(provider) {
  const colors = {
    doubao: 'blue',
    volcengine: 'blue',
    qwen: 'orange',
    dashscope: 'orange',
    hunyuan: 'green',
    tencent: 'green',
    deepseek: 'purple'
  }
  return colors[provider] || 'default'
}

function maskApiKey(key) {
  if (!key) return ''
  return key.substring(0, 8) + '****' + key.substring(key.length - 4)
}

function editProvider(provider) {
  form.provider = provider
  showAddModal.value = true
}

async function viewModels(provider) {
  // 模拟模型数据
  const models = {
    doubao: [
      { id: 'doubao-pro-4k', display_name: '豆包Pro 4K' },
      { id: 'doubao-pro-32k', display_name: '豆包Pro 32K' },
      { id: 'doubao-pro-128k', display_name: '豆包Pro 128K' },
      { id: 'doubao-lite-4k', display_name: '豆包Lite 4K' },
      { id: 'doubao-lite-32k', display_name: '豆包Lite 32K' }
    ],
    qwen: [
      { id: 'qwen-turbo', display_name: '通义千问-Turbo' },
      { id: 'qwen-plus', display_name: '通义千问-Plus' },
      { id: 'qwen-max', display_name: '通义千问-Max' },
      { id: 'qwen-max-longcontext', display_name: '通义千问-Max-长文本' },
      { id: 'qwen-vl-plus', display_name: '通义千问VL-Plus' }
    ],
    hunyuan: [
      { id: 'hunyuan-lite', display_name: '混元Lite' },
      { id: 'hunyuan-standard', display_name: '混元Standard' },
      { id: 'hunyuan-standard-256K', display_name: '混元Standard-256K' },
      { id: 'hunyuan-pro', display_name: '混元Pro' },
      { id: 'hunyuan-code', display_name: '混元Code' }
    ],
    deepseek: [
      { id: 'deepseek-chat', display_name: 'DeepSeek-V3' },
      { id: 'deepseek-reasoner', display_name: 'DeepSeek-R1' },
      { id: 'deepseek-coder', display_name: 'DeepSeek-Coder-V2' }
    ]
  }
  currentModels.value = models[provider] || []
  showModelsModal.value = true
}

async function handleSubmit() {
  try {
    if (editingKey.value) {
      await api.put(`/admin/provider-keys/${editingKey.value.id}`, form)
      Message.success('更新成功')
    } else {
      await api.post('/admin/provider-keys', form)
      Message.success('添加成功')
    }
    showAddModal.value = false
    resetForm()
    loadProviderKeys()
  } catch (error) {
    Message.error(error.response?.data?.message || '操作失败')
  }
}

function resetForm() {
  editingKey.value = null
  form.provider = ''
  form.name = ''
  form.api_key = ''
  form.secret_id = ''
  form.secret_key = ''
  form.description = ''
}

function editKey(record) {
  editingKey.value = record
  Object.assign(form, record)
  showAddModal.value = true
}

async function deleteKey(record) {
  try {
    await api.delete(`/admin/provider-keys/${record.id}`)
    Message.success('删除成功')
    loadProviderKeys()
  } catch (error) {
    Message.error('删除失败')
  }
}

async function toggleStatus(record, val) {
  try {
    await api.put(`/admin/provider-keys/${record.id}`, {
      ...record,
      status: val ? 1 : 0
    })
    Message.success('状态更新成功')
    loadProviderKeys()
  } catch (error) {
    Message.error('更新失败')
  }
}

async function addModelToSystem(model) {
  try {
    await api.post('/admin/models', {
      name: model.id,
      display_name: model.display_name,
      provider: currentProvider.value,
      model_name: model.id,
      status: 1,
      input_price: 0.001,
      output_price: 0.002,
      tool_support: true,
      stream_support: true
    })
    Message.success('添加成功')
  } catch (error) {
    Message.error('添加失败')
  }
}

const currentProvider = ref('')
</script>

<style scoped>
.chinese-providers-container {
  width: 100%;
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

.provider-card {
  transition: all 0.3s;
}

.provider-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}

:deep(.arco-descriptions-item-label) {
  font-weight: 500;
  color: var(--color-text-2);
}

:deep(.arco-descriptions-item-value) {
  color: var(--color-text-1);
}
</style>
