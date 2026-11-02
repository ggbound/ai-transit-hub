<template>
  <div class="api-test-container">
    <div class="page-header">
      <div>
        <h1>接口测试</h1>
        <p>在线测试 OpenAI 和 Anthropic 兼容 API 接口</p>
      </div>
    </div>

    <!-- 第一行：请求配置 + 代码示例 -->
    <a-row :gutter="16" class="main-row">
      <a-col :xs="24" :sm="24" :md="7" :lg="7" :xl="6">
        <!-- 请求配置 -->
        <a-card class="config-card" :bordered="false">
          <template #title>
            <div class="card-title">
              <icon-settings class="title-icon" />
              <span>请求配置</span>
            </div>
          </template>
          <a-form :model="formData" layout="vertical" size="small">
            <!-- 选择 API 类型 -->
            <a-form-item label="API 类型">
              <a-radio-group v-model="apiType" type="button">
                <a-radio value="openai">OpenAI</a-radio>
                <a-radio value="anthropic">Anthropic</a-radio>
              </a-radio-group>
            </a-form-item>

            <!-- 选择 API Key -->
            <a-form-item v-if="apiType !== 'internal'" label="API Key">
              <a-select 
                v-model="selectedApiKey" 
                placeholder="选择 API Key" 
                style="width: 100%;"
              >
                <a-option v-for="key in apiKeys" :key="key.id" :value="key.api_key">
                  {{ key.key_name }}
                </a-option>
              </a-select>
            </a-form-item>

            <!-- 选择接口 -->
            <a-form-item label="接口">
              <a-select v-model="selectedEndpoint" placeholder="选择接口" style="width: 100%;">
                <template v-for="group in endpointGroups" :key="'group-' + group.label">
                  <a-option disabled style="font-weight: 600;">
                    {{ group.label }}
                  </a-option>
                  <a-option v-for="(ep, epIndex) in group.options" :key="'ep-' + group.label + '-' + epIndex" :value="ep.value">
                    &nbsp;&nbsp;{{ ep.label }}
                  </a-option>
                </template>
              </a-select>
            </a-form-item>

            <!-- 选择模型 -->
            <a-form-item v-if="!isGetEndpoint" label="模型">
              <a-select 
                v-model="formData.model" 
                placeholder="选择模型" 
                style="width: 100%;"
                allow-search
              >
                <a-option v-for="model in models" :key="model.id" :value="model.full_model_id">
                  {{ model.name }} ({{ model.provider?.name }} - {{ model.code }})
                </a-option>
              </a-select>
            </a-form-item>

            <!-- 消息列表/输入内容 -->
            <template v-if="!isGetEndpoint">
                <a-divider orientation="left" style="margin: 12px 0;">
                    <span style="font-size: 12px;">
                        {{ selectedEndpoint.includes('embeddings') ? '输入内容' : '消息列表' }}
                    </span>
                </a-divider>
                <template v-if="selectedEndpoint.includes('embeddings')">
                    <!-- 向量嵌入模式选择 -->
                    <a-form-item label="输入模式">
                        <a-radio-group v-model="formData.embeddingMode" size="small">
                            <a-radio value="simple">简单模式</a-radio>
                            <a-radio value="advanced">高级模式 (JSON)</a-radio>
                        </a-radio-group>
                    </a-form-item>
                    
                    <!-- 简单模式 - 文本输入 -->
                    <template v-if="formData.embeddingMode === 'simple'">
                        <div v-for="(msg, index) in formData.messages" :key="index" class="message-item">
                            <a-space style="width: 100%;">
                                <span style="flex: 0 0 80px; font-size: 12px;">输入 {{ index + 1 }}</span>
                                <a-textarea
                                    v-model="msg.content"
                                    placeholder="输入要向量化的内容..."
                                    :rows="2"
                                    size="small"
                                    style="flex: 1;"
                                />
                                <a-button 
                                    type="text" 
                                    status="danger" 
                                    size="small"
                                    @click="removeMessage(index)"
                                >
                                    <icon-delete />
                                </a-button>
                            </a-space>
                        </div>
                        <a-button type="dashed" long @click="addMessage" size="small" style="margin-top: 8px;">
                            <template #icon><icon-plus /></template>
                            添加输入
                        </a-button>
                    </template>
                    
                    <!-- 高级模式 - JSON 输入 -->
                    <template v-else>
                        <a-form-item label="Input JSON">
                            <a-textarea
                                v-model="formData.advancedInputJson"
                                placeholder="输入要向量化的内容JSON..."
                                :rows="12"
                                size="small"
                            />
                        </a-form-item>
                        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 16px;">
                            支持复杂格式，如视频、图片、文本混合输入
                        </div>
                    </template>
                </template>
                <template v-else-if="selectedEndpoint.includes('responses')">
                    <!-- Responses API 专用输入 -->
                    <a-form-item label="Previous Response ID">
                        <a-input
                            v-model="formData.previous_response_id"
                            placeholder="上一轮对话的 response ID，可选"
                            size="small"
                        />
                    </a-form-item>
                    
                    <a-form-item label="Input">
                        <a-radio-group v-model="formData.responseInputMode" size="small">
                            <a-radio value="simple">简单模式 (文本)</a-radio>
                            <a-radio value="advanced">高级模式 (JSON)</a-radio>
                        </a-radio-group>
                    </a-form-item>
                    
                    <template v-if="formData.responseInputMode === 'simple'">
                        <a-form-item>
                            <a-textarea
                                v-model="formData.responseInput"
                                placeholder="输入要发送的内容，如：Hi，帮我讲个笑话。"
                                :rows="4"
                                size="small"
                            />
                        </a-form-item>
                    </template>
                    <template v-else>
                        <a-form-item label="Input JSON">
                            <a-textarea
                                v-model="formData.advancedResponseInputJson"
                                placeholder='[{"role": "user", "content": "Hi，帮我讲个笑话。"}]'
                                :rows="8"
                                size="small"
                            />
                        </a-form-item>
                    </template>
                </template>
                <template v-else>
                    <!-- 对话消息输入 -->
                    <div v-for="(msg, index) in formData.messages" :key="index" class="message-item">
                        <a-space style="width: 100%;">
                            <a-select v-model="msg.role" size="small" style="flex: 0 0 100px;">
                                <a-option value="system">系统</a-option>
                                <a-option value="user">用户</a-option>
                                <a-option value="assistant">助手</a-option>
                            </a-select>
                            <a-textarea
                                v-model="msg.content"
                                placeholder="输入消息内容..."
                                :rows="1"
                                size="small"
                                style="flex: 1;"
                            />
                            <a-button 
                                type="text" 
                                status="danger" 
                                size="small"
                                @click="removeMessage(index)"
                            >
                                <icon-delete />
                            </a-button>
                        </a-space>
                    </div>
                    <a-button type="dashed" long @click="addMessage" size="small" style="margin-top: 8px;">
                        <template #icon><icon-plus /></template>
                        添加消息
                    </a-button>
                </template>
            </template>

            <!-- 高级参数 -->
            <template v-if="!isGetEndpoint && showAdvancedParams">
                <a-divider orientation="left" style="margin: 12px 0;">
                    <span style="font-size: 12px;">高级参数</span>
                </a-divider>

                <template v-if="selectedEndpoint.includes('embeddings')">
                    <!-- 向量嵌入专属参数 -->
                    <a-form-item label="Encoding Format">
                        <a-select v-model="formData.encoding_format" placeholder="选择编码格式" allow-clear style="width: 100%;" size="small">
                            <a-option value="float">float (浮点数数组)</a-option>
                            <a-option value="base64">base64 (Base64编码)</a-option>
                        </a-select>
                    </a-form-item>
                    <a-form-item label="Dimensions">
                        <a-input-number
                            v-model="formData.dimensions"
                            placeholder="向量维度"
                            :min="64"
                            :max="4096"
                            style="width: 100%;"
                            size="small"
                        />
                    </a-form-item>
                    <a-form-item label="Modality">
                        <a-select v-model="formData.modality" placeholder="选择模态" allow-clear style="width: 100%;" size="small">
                            <a-option value="text">text (文本)</a-option>
                            <a-option value="image">image (图像)</a-option>
                        </a-select>
                    </a-form-item>
                </template>
                <template v-else>
                    <!-- 对话专属参数 -->
                    <!-- Temperature -->
                    <a-form-item label="Temperature: {{ formData.temperature }}">
                        <a-slider
                            v-model="formData.temperature"
                            :min="0"
                            :max="2"
                            :step="0.1"
                            show-tooltip
                            size="small"
                        />
                    </a-form-item>

                    <!-- Max Tokens -->
                    <a-form-item label="Max Tokens">
                        <a-input-number
                            v-model="formData.max_tokens"
                            :min="1"
                            :max="8192"
                            style="width: 100%;"
                            size="small"
                        />
                    </a-form-item>

                    <!-- Stream -->
                    <a-form-item>
                        <a-space>
                            <a-switch v-model="formData.stream" :disabled="!supportsStream" size="small" />
                            <span style="font-size: 12px;">流式输出</span>
                        </a-space>
                    </a-form-item>
                </template>
            </template>

            <!-- 显示高级参数开关 -->
            <a-form-item v-if="!isGetEndpoint" style="margin-bottom: 8px;">
              <a-button type="text" @click="showAdvancedParams = !showAdvancedParams" size="small">
                <template #icon>
                  <icon-down v-if="showAdvancedParams" />
                  <icon-right v-else />
                </template>
                {{ showAdvancedParams ? '收起' : '高级参数' }}
              </a-button>
            </a-form-item>

            <a-form-item style="margin-top: 12px;">
              <a-button type="primary" @click="sendRequest" :loading="loading" long size="small">
                <template #icon><icon-send /></template>
                发送请求
              </a-button>
            </a-form-item>
          </a-form>
        </a-card>
      </a-col>

      <a-col :xs="24" :sm="24" :md="17" :lg="17" :xl="18">
        <!-- 代码示例 - 横向布局 -->
        <a-card class="code-card" :bordered="false" :body-style="{ padding: 0 }">
          <template #title>
            <div class="card-title">
              <icon-code class="title-icon" />
              <span>代码示例</span>
            </div>
          </template>
          <a-tabs v-model:activeKey="codeTab" class="code-tabs" size="small">
            <a-tab-pane key="curl" title="cURL">
              <div class="code-block">{{ curlExample }}</div>
            </a-tab-pane>
            <a-tab-pane key="js" title="JavaScript">
              <div class="code-block">{{ jsExample }}</div>
            </a-tab-pane>
            <a-tab-pane key="python" title="Python">
              <div class="code-block">{{ pythonExample }}</div>
            </a-tab-pane>
          </a-tabs>
        </a-card>

        <!-- 请求预览与响应 - 左右布局 -->
        <a-row :gutter="16" class="preview-response-row">
          <a-col :span="12">
            <a-card class="preview-card" :bordered="false">
              <template #title>
                <div class="card-title">
                  <icon-eye class="title-icon" />
                  <span>请求预览</span>
                </div>
              </template>
              <div class="json-preview">
                <pre><code>{{ jsonPreview }}</code></pre>
              </div>
            </a-card>
          </a-col>
          <a-col :span="12">
            <a-card class="response-card" :bordered="false">
              <template #title>
                <div class="card-title">
                  <icon-reply class="title-icon" />
                  <span>响应结果</span>
                  <a-space class="response-actions">
                    <a-tag :color="statusColor" size="small">
                      {{ responseStatus || '等待请求' }}
                    </a-tag>
                    <a-tag size="small">{{ responseTime || '0ms' }}</a-tag>
                    <a-button v-if="responseBody" type="text" size="mini" @click="copyResponse">
                      <template #icon><icon-copy /></template>
                    </a-button>
                  </a-space>
                </div>
              </template>
              <div class="response-body-container">
                <pre v-if="responseBody" class="response-content"><code>{{ responseBody }}</code></pre>
                <div v-else class="empty-response">
                  <icon-code class="empty-icon" />
                  <p>点击"发送请求"查看响应结果</p>
                </div>
              </div>
            </a-card>
          </a-col>
        </a-row>

        <!-- 请求历史 -->
        <a-card class="history-card" :bordered="false">
          <template #title>
            <div class="card-title">
              <icon-history class="title-icon" />
              <span>请求历史</span>
              <a-button type="text" size="small" @click="loadHistory">
                <template #icon><icon-refresh /></template>
                刷新
              </a-button>
            </div>
          </template>
          <a-table 
            :data="historyRecords" 
            :pagination="{ pageSize: 5, showTotal: true }" 
            size="small"
            class="history-table"
          >
            <template #columns>
              <a-table-column title="时间" data-index="created_at" :width="180">
                <template #cell="{ record }">
                  <div class="time-cell">
                    <div class="time-main">{{ formatTime(record.created_at) }}</div>
                    <div class="time-sub">{{ formatDateTime(record.created_at) }}</div>
                  </div>
                </template>
              </a-table-column>
              <a-table-column title="接口" data-index="endpoint" :width="200">
                <template #cell="{ record }">
                  <a-tag :color="record.api_type === 'openai' ? 'arcoblue' : 'purple'" size="small">
                    {{ record.api_type }}
                  </a-tag>
                  <span class="endpoint-name">{{ record.endpoint }}</span>
                </template>
              </a-table-column>
              <a-table-column title="模型" data-index="model" :width="200">
                <template #cell="{ record }">
                  <span class="model-name">{{ record.model || '-' }}</span>
                </template>
              </a-table-column>
              <a-table-column title="状态" data-index="status" :width="100">
                <template #cell="{ record }">
                  <a-tag :color="record.status < 400 ? 'green' : 'red'" size="small">
                    {{ record.status }}
                  </a-tag>
                </template>
              </a-table-column>
              <a-table-column title="耗时" data-index="duration" :width="100">
                <template #cell="{ record }">
                  <span class="duration-text">{{ record.duration }}ms</span>
                </template>
              </a-table-column>
              <a-table-column title="外部ID" data-index="external_task_id" :width="150">
                <template #cell="{ record }">
                  <span class="external-id">{{ record.external_task_id || '-' }}</span>
                </template>
              </a-table-column>
              <a-table-column title="操作" :width="100" fixed="right">
                <template #cell="{ record }">
                  <a-button type="text" size="small" @click="showDetail(record)">
                    详情
                  </a-button>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>

        <!-- 详情弹窗 -->
        <a-modal
          v-model:visible="detailModalVisible"
          title="请求详情"
          width="80%"
          :footer="null"
        >
          <a-spin v-if="loadingDetail">
            <div style="height: 200px; text-align: center; line-height: 200px;">
              加载中...
            </div>
          </a-spin>
          <template v-else>
            <div class="detail-section">
              <h4>基础信息</h4>
              <a-descriptions :column="2" bordered size="small">
                <a-descriptions-item label="ID">{{ detailData?.id }}</a-descriptions-item>
                <a-descriptions-item label="API类型">{{ detailData?.api_type }}</a-descriptions-item>
                <a-descriptions-item label="接口">{{ detailData?.endpoint }}</a-descriptions-item>
                <a-descriptions-item label="方法">{{ detailData?.method }}</a-descriptions-item>
                <a-descriptions-item label="模型">{{ detailData?.model || '-' }}</a-descriptions-item>
                <a-descriptions-item label="状态码">{{ detailData?.status }}</a-descriptions-item>
                <a-descriptions-item label="耗时">{{ detailData?.duration }}ms</a-descriptions-item>
                <a-descriptions-item label="创建时间">{{ formatTime(detailData?.created_at) }}</a-descriptions-item>
              </a-descriptions>
            </div>

            <a-divider />

            <div class="detail-section">
              <h4>请求体</h4>
              <div class="json-preview">
                <pre><code>{{ detailData?.detail?.request_body || '-' }}</code></pre>
              </div>
            </div>

            <a-divider />

            <div class="detail-section">
              <h4>响应体</h4>
              <div class="json-preview">
                <pre><code>{{ detailData?.detail?.response_body || '-' }}</code></pre>
              </div>
            </div>
          </template>
        </a-modal>
      </a-col>
    </a-row>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import { 
  IconSend, 
  IconCopy, 
  IconPlus, 
  IconDelete, 
  IconDown, 
  IconRight,
  IconSettings,
  IconCode,
  IconEye,
  IconReply,
  IconHistory,
  IconRefresh
} from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'
import config from '@/config/api.config.js'

// API 类型
const apiType = ref('openai')
const selectedEndpoint = ref('openai_chat')
const selectedApiKey = ref('')
const selectedApiKeyId = ref(null)
const showAdvancedParams = ref(false)

// 表单数据
const formData = ref({
    model: '',
    messages: [
        { role: 'user', content: '' }
    ],
    temperature: 1,
    max_tokens: 2048,
    top_p: 1,
    stream: false,
    encoding_format: '',
    dimensions: null,
    modality: '',
    embeddingMode: 'simple', // simple / advanced
    advancedInputJson: '[\n  {\n    "type": "text",\n    "text": "你好"\n  }\n]',
    // Responses API 专用字段
    responseInputMode: 'simple', // simple / advanced
    responseInput: '',
    advancedResponseInputJson: '[{"role": "user", "content": "Hi，帮我讲个笑话。"}]',
    previous_response_id: ''
})

const loading = ref(false)
const responseBody = ref('')
const responseStatus = ref(null)
const responseTime = ref('')
const codeTab = ref('curl')
const historyRecords = ref([])

const detailModalVisible = ref(false)
const loadingDetail = ref(false)
const detailData = ref(null)

// API Keys 列表
const apiKeys = ref([])
const models = ref([])

// 接口配置
const endpoints = {
  openai: {
    baseUrl: config.api.openai.base,
    authHeader: 'Authorization',
    authPrefix: 'Bearer',
  },
  anthropic: {
    baseUrl: config.api.anthropic.base,
    authHeader: 'x-api-key',
    authPrefix: '',
  },
  internal: {
    baseUrl: config.baseURL,
    authHeader: 'Authorization',
    authPrefix: 'Bearer',
  },
}

const endpointGroups = computed(() => {
  if (apiType.value === 'openai') {
    return [
      {
        label: 'OpenAI 兼容接口',
        options: [
          { value: 'openai_chat', label: 'POST /chat/completions - 对话' },
          { value: 'openai_responses', label: 'POST /responses - 响应' },
          { value: 'openai_embeddings', label: 'POST /embeddings - 向量嵌入' },
          { value: 'openai_models', label: 'GET /models - 获取模型列表' },
        ],
      },
    ]
  } else if (apiType.value === 'anthropic') {
    return [
      {
        label: 'Anthropic 兼容接口',
        options: [
          { value: 'anthropic_messages', label: 'POST /messages - 发送消息' },
          { value: 'anthropic_models', label: 'GET /models - 获取模型列表' },
        ],
      },
    ]
  }
  return []
})

// 是否支持流式
const supportsStream = computed(() => {
  return selectedEndpoint.value.includes('chat') || selectedEndpoint.value.includes('messages') || selectedEndpoint.value.includes('responses')
})

// 是否是 GET 请求
const isGetEndpoint = computed(() => {
  return selectedEndpoint.value.includes('models') || selectedEndpoint.value.includes('sessions')
})

// 状态颜色
const statusColor = computed(() => {
  if (!responseStatus.value) return 'default'
  if (responseStatus.value < 400) return 'green'
  return 'red'
})

// JSON 预览
const jsonPreview = computed(() => {
    if (isGetEndpoint.value) return '// GET 请求无请求体'
    
    const body = {}
    
    if (selectedEndpoint.value.includes('embeddings')) {
        // 向量嵌入接口格式
        body.model = formData.value.model
        
        // 根据模式处理 input
        if (formData.value.embeddingMode === 'advanced') {
            // 高级模式 - 直接解析 JSON
            try {
                body.input = JSON.parse(formData.value.advancedInputJson)
            } catch (e) {
                body.input = formData.value.advancedInputJson
            }
        } else {
            // 简单模式 - 文本数组
            body.input = formData.value.messages.filter(m => m.content.trim()).map(m => m.content)
        }
        
        if (formData.value.encoding_format) {
            body.encoding_format = formData.value.encoding_format
        }
        if (formData.value.dimensions) {
            body.dimensions = formData.value.dimensions
        }
        if (formData.value.modality) {
            body.modality = formData.value.modality
        }
    } else if (selectedEndpoint.value.includes('responses')) {
        // Responses API 格式
        body.model = formData.value.model
        
        // 根据模式处理 input
        if (formData.value.responseInputMode === 'advanced') {
            // 高级模式 - 直接解析 JSON
            try {
                body.input = JSON.parse(formData.value.advancedResponseInputJson)
            } catch (e) {
                body.input = formData.value.advancedResponseInputJson
            }
        } else {
            // 简单模式 - 直接使用文本
            if (formData.value.responseInput.trim()) {
                body.input = formData.value.responseInput
            }
        }
        
        // Previous Response ID（可选）
        if (formData.value.previous_response_id) {
            body.previous_response_id = formData.value.previous_response_id
        }
        
        // Stream
        if (formData.value.stream && supportsStream.value) {
            body.stream = true
        }
    } else if (apiType.value === 'internal') {
        body.model_id = 1
    } else {
        body.model = formData.value.model
    }
    
    if (!selectedEndpoint.value.includes('embeddings') && !selectedEndpoint.value.includes('responses')) {
        body.messages = formData.value.messages.filter(m => m.content.trim())
        
        if (formData.value.temperature !== undefined) {
            body.temperature = formData.value.temperature
        }
        if (formData.value.max_tokens) {
            body.max_tokens = formData.value.max_tokens
        }
        if (formData.value.top_p !== undefined && formData.value.top_p !== 1) {
            body.top_p = formData.value.top_p
        }
        if (formData.value.stream && supportsStream.value) {
            body.stream = true
        }
    }
    
    return JSON.stringify(body, null, 2)
})

// 初始化
onMounted(() => {
  loadApiKeys()
  loadModels()
  loadHistory()
})

// 加载 API Keys
async function loadApiKeys() {
  try {
    const res = await api.get('/user/api-keys')
    if (res.data.success) {
      const keys = res.data.data.data || res.data.data
      apiKeys.value = keys
      if (apiKeys.value.length > 0 && !selectedApiKey.value) {
        selectedApiKey.value = apiKeys.value[0].api_key
        selectedApiKeyId.value = apiKeys.value[0].id
      }
    }
  } catch (error) {
    console.error('加载 API Keys 失败:', error)
  }
}

// 加载模型列表
async function loadModels() {
  try {
    const res = await api.get('/models')
    if (res.data.success) {
      const allModels = res.data.data.models || []
      // 筛选文本对话、向量嵌入模型，以及支持 Responses API 的模型
      models.value = allModels.filter(m => m.type === 'text' || m.type === 'embedding' || m.supports_response)
      if (models.value.length > 0 && !formData.value.model) {
        formData.value.model = models.value[0].full_model_id
      }
      console.log('模型列表:', models.value)
    }
  } catch (error) {
    console.error('加载模型失败:', error)
  }
}

// 添加消息
function addMessage() {
  formData.value.messages.push({ role: 'user', content: '' })
}

// 删除消息
function removeMessage(index) {
  if (formData.value.messages.length > 1) {
    formData.value.messages.splice(index, 1)
  } else {
    formData.value.messages[0].content = ''
  }
}

// 监听 API Key 变化，更新对应的 ID
watch(selectedApiKey, (newVal) => {
  const selectedKey = apiKeys.value.find(k => k.api_key === newVal)
  if (selectedKey) {
    selectedApiKeyId.value = selectedKey.id
  }
})

// 监听 API 类型变化，更新默认模型和接口
watch(apiType, (newVal) => {
  // 切换 API 类型时选中该类型的第一个接口
  const currentEndpoints = endpointGroups.value
  if (currentEndpoints.length > 0 && currentEndpoints[0].options.length > 0) {
    selectedEndpoint.value = currentEndpoints[0].options[0].value
  }
  
  // 重置表单
  if (models.value.length > 0) {
    formData.value.model = models.value[0].full_model_id
  }
  formData.value.messages = [{ role: 'user', content: '' }]
  formData.value.temperature = 1
  formData.value.max_tokens = newVal === 'anthropic' ? 1024 : 2048
  formData.value.top_p = 1
  formData.value.stream = false
})

// 生成 Curl 示例
const curlExample = computed(() => {
    const config = endpoints[apiType.value]
    let url = config.baseUrl
    let method = 'POST'
    
    if (selectedEndpoint.value.includes('models')) {
        url += '/models'
        method = 'GET'
    } else if (selectedEndpoint.value.includes('chat')) {
        url += apiType.value === 'internal' ? '/chat/send' : '/chat/completions'
    } else if (selectedEndpoint.value.includes('embeddings')) {
        url += '/embeddings'
    } else if (selectedEndpoint.value.includes('messages')) {
        url += '/messages'
    } else if (selectedEndpoint.value.includes('responses')) {
        url += '/responses'
    }

    const authValue = apiType.value === 'internal' 
        ? '<your-jwt-token>' 
        : (selectedApiKey.value || '<your-api-key>')
    
    const authHeader = apiType.value === 'internal'
        ? `${config.authHeader}: ${config.authPrefix} ${authValue}`
        : `${config.authHeader}: ${config.authPrefix ? config.authPrefix + ' ' : ''}${authValue}`

    let bodyStr = ''
    if (method === 'POST') {
        bodyStr = ` -d '${jsonPreview.value.replace(/'/g, "\\'")}'`
    }

    return `curl -X ${method} "${url}" \\
  -H "Content-Type: application/json" \\
  -H "${authHeader}"${bodyStr}`
})

// 生成 JS 示例
const jsExample = computed(() => {
    const config = endpoints[apiType.value]
    let path = ''
    if (selectedEndpoint.value.includes('models')) {
        path = '/models'
    } else if (selectedEndpoint.value.includes('chat')) {
        path = apiType.value === 'internal' ? '/chat/send' : '/chat/completions'
    } else if (selectedEndpoint.value.includes('embeddings')) {
        path = '/embeddings'
    } else if (selectedEndpoint.value.includes('messages')) {
        path = '/messages'
    } else if (selectedEndpoint.value.includes('responses')) {
        path = '/responses'
    }

    const url = config.baseUrl + path

    const authValue = apiType.value === 'internal' 
        ? '<your-jwt-token>' 
        : (selectedApiKey.value || '<your-api-key>')

    return `const response = await fetch('${url}', {
  method: '${selectedEndpoint.value.includes('models') ? 'GET' : 'POST'}',
  headers: {
    'Content-Type': 'application/json',
    '${config.authHeader}': '${config.authPrefix ? config.authPrefix + ' ' : ''}${authValue}'
  }${selectedEndpoint.value.includes('models') ? '' : `,\n  body: JSON.stringify(${jsonPreview.value})`}
});

const data = await response.json();
console.log(data);`
})

// 生成 Python 示例
const pythonExample = computed(() => {
    const config = endpoints[apiType.value]
    let path = ''
    if (selectedEndpoint.value.includes('models')) {
        path = '/models'
    } else if (selectedEndpoint.value.includes('chat')) {
        path = apiType.value === 'internal' ? '/chat/send' : '/chat/completions'
    } else if (selectedEndpoint.value.includes('embeddings')) {
        path = '/embeddings'
    } else if (selectedEndpoint.value.includes('messages')) {
        path = '/messages'
    } else if (selectedEndpoint.value.includes('responses')) {
        path = '/responses'
    }

    const url = config.baseUrl + path
    const baseUrl = config.baseUrl

    const authValue = apiType.value === 'internal' 
        ? '<your-jwt-token>' 
        : (selectedApiKey.value || '<your-api-key>')
    
    // 为 Responses API 提供专门的 OpenAI SDK 风格示例
    if (selectedEndpoint.value.includes('responses')) {
        return `from openai import OpenAI

# 初始化客户端
client = OpenAI(
    base_url='${baseUrl}',
    api_key='${authValue}',
)

# 创建响应请求
response = client.responses.create(
    model='${formData.value.model || 'your-model'}',
${formData.value.previous_response_id ? `    previous_response_id='${formData.value.previous_response_id}',` : ''}
    input=${formData.value.responseInputMode === 'simple' ? `'${formData.value.responseInput || 'Hi'}'` : formData.value.advancedResponseInputJson},
${formData.value.stream ? `    stream=True,` : ''}
)

# 打印响应
print(response)
`
    }
    
    const bodyStr = jsonPreview.value
        .replace(/true/g, 'True')
        .replace(/false/g, 'False')
        .replace(/null/g, 'None')

    if (selectedEndpoint.value.includes('models')) {
        return `import requests

response = requests.get(
    '${url}',
    headers={
        'Content-Type': 'application/json',
        '${config.authHeader}': '${config.authPrefix ? config.authPrefix + ' ' : ''}${authValue}'
    }
)

print(response.json())`
    }

    return `import requests

response = requests.post(
    '${url}',
    headers={
        'Content-Type': 'application/json',
        '${config.authHeader}': '${config.authPrefix ? config.authPrefix + ' ' : ''}${authValue}'
    },
    json=${bodyStr}
)

print(response.json())`
})

// 发送请求
async function sendRequest() {
    if (apiType.value !== 'internal' && !selectedApiKey.value) {
        Message.error('请选择 API Key')
        return
    }

    if (!isGetEndpoint.value && !formData.value.model) {
        Message.error('请选择模型')
        return
    }

    // 验证请求体，根据不同接口类型检查
    if (!isGetEndpoint.value) {
        let hasContent = false
        
        if (selectedEndpoint.value.includes('embeddings')) {
            // Embeddings API：检查 input 字段
            if (formData.value.embeddingMode === 'simple') {
                hasContent = !formData.value.messages.every(m => !m.content.trim())
            } else {
                // 高级模式：只要有 JSON 内容即可（哪怕只是 JSON 结构）
                hasContent = formData.value.advancedInputJson && formData.value.advancedInputJson.trim() !== ''
            }
        } else if (selectedEndpoint.value.includes('responses')) {
            // Responses API：检查 input 字段
            if (formData.value.responseInputMode === 'simple') {
                hasContent = formData.value.responseInput && formData.value.responseInput.trim() !== ''
            } else {
                // 高级模式：只要有内容即可（哪怕只是 JSON 结构）
                hasContent = formData.value.advancedResponseInputJson && formData.value.advancedResponseInputJson.trim() !== ''
            }
        } else {
            // 其他接口：检查 messages 数组
            hasContent = !formData.value.messages.every(m => !m.content.trim())
        }
        
        if (!hasContent) {
            Message.error('请至少输入一条消息/内容')
            return
        }
    }

    loading.value = true
    const startTime = Date.now()

    let savedRequest = null
    try {
        const config = endpoints[apiType.value]
        let url = config.baseUrl
        let method = 'POST'
        let body = null

        // 构建 URL
        if (selectedEndpoint.value.includes('models')) {
            url += '/models'
            method = 'GET'
        } else if (selectedEndpoint.value.includes('chat')) {
            url += apiType.value === 'internal' ? '/chat/send' : '/chat/completions'
        } else if (selectedEndpoint.value.includes('embeddings')) {
            url += '/embeddings'
        } else if (selectedEndpoint.value.includes('messages')) {
            url += '/messages'
        } else if (selectedEndpoint.value.includes('responses')) {
            url += '/responses'
        }

        // 构建 Headers
        const headers = {
            'Content-Type': 'application/json',
        }

        if (apiType.value === 'internal') {
            const token = localStorage.getItem('token')
            headers[config.authHeader] = `${config.authPrefix} ${token}`
        } else {
            headers[config.authHeader] = config.authPrefix
                ? `${config.authPrefix} ${selectedApiKey.value}`
                : selectedApiKey.value
        }

        // 构建请求体
        if (method === 'POST') {
            body = JSON.parse(jsonPreview.value)
        }

        // 发送请求
        let response
        if (!selectedEndpoint.value.includes('embeddings') && formData.value.stream && supportsStream.value) {
            response = await fetch(url, {
                method,
                headers,
                body: body ? JSON.stringify(body) : null,
            })

            const reader = response.body.getReader()
            const decoder = new TextDecoder()
            let result = ''

            while (true) {
                const { done, value } = await reader.read()
                if (done) break
                result += decoder.decode(value)
            }

            responseStatus.value = response.status
            responseBody.value = result
        } else {
            response = await fetch(url, {
                method,
                headers,
                body: body ? JSON.stringify(body) : null,
            })

            const data = await response.json()
            responseStatus.value = response.status
            responseBody.value = JSON.stringify(data, null, 2)
        }

        const duration = Date.now() - startTime
        responseTime.value = `${duration}ms`

        // 请求结束后，直接刷新列表获取最新数据
        await loadHistory()

    } catch (error) {
        console.error(error)
        responseStatus.value = error.response?.status || 500
        responseBody.value = JSON.stringify({
            error: error.message,
            details: error.response?.data || '请求失败'
        }, null, 2)

        const duration = Date.now() - startTime

        // 请求结束后，直接刷新列表获取最新数据
        await loadHistory()
    } finally {
        loading.value = false
    }
}

// 保存到后端历史
async function saveToHistory(data) {
    try {
        const saveData = {
            api_type: data.apiType,
            endpoint: data.endpoint,
            endpoint_url: data.endpointUrl,
            method: data.method,
            model: data.model,
            status: data.status,
            duration: data.duration,
            error_message: data.errorMessage,
            request_headers: data.requestHeaders,
            request_body: data.requestBody,
            response_headers: data.responseHeaders,
            response_body: data.responseBody,
            type: 'api',
        }
        
        // 添加 user_api_key_id，只在非内部 API 时添加
        if (data.apiType !== 'internal' && selectedApiKeyId.value) {
            saveData.user_api_key_id = selectedApiKeyId.value
        }
        
        const response = await api.post('/api-test', saveData)

        if (response.data.success) {
            return response.data.data
        }
        return null
    } catch (error) {
        console.error('保存记录失败:', error)
        throw error
    }
}

// 从后端加载历史记录
async function loadHistory() {
    try {
        const response = await api.get('/api-test/history', {
            params: {
                per_page: 20,
                page: 1,
                type: 'api'
            }
        })

        if (response.data.success) {
            historyRecords.value = response.data.data.data.map(item => ({
                id: item.id,
                created_at: item.created_at,
                api_type: item.api_type,
                endpoint: item.endpoint,
                method: item.method,
                model: item.model,
                status: item.status,
                duration: item.duration,
                external_task_id: item.external_task_id,
                user_api_key_id: item.user_api_key_id,
            }))
        }
    } catch (error) {
        console.error('加载历史记录失败:', error)
    }
}

// 显示详情
async function showDetail(record) {
    if (!record.id) {
        Message.warning('这是本地记录，无法查看详情')
        return
    }
    
    detailModalVisible.value = true
    loadingDetail.value = true
    detailData.value = null
    
    try {
        const response = await api.get(`/api-test/${record.id}`)
        
        if (response.data.success) {
            detailData.value = response.data.data
            console.log('详情数据:', detailData.value)
        }
    } catch (error) {
        console.error('加载详情失败:', error)
        Message.error('加载详情失败')
    } finally {
        loadingDetail.value = false
    }
}

// 格式化时间
function formatTime(time) {
  const date = new Date(time)
  const now = new Date()
  const diff = now - date

  if (diff < 3600000) {
    const minutes = Math.floor(diff / 60000)
    return minutes < 1 ? '刚刚' : `${minutes}分钟前`
  }
  if (diff < 86400000) {
    return `${Math.floor(diff / 3600000)}小时前`
  }
  if (diff < 604800000) {
    return `${Math.floor(diff / 86400000)}天前`
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

function formatDateTime(time) {
  if (!time) return ''
  const date = new Date(time)
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// 复制响应
async function copyResponse() {
  try {
    await navigator.clipboard.writeText(responseBody.value)
    Message.success('已复制到剪贴板')
  } catch {
    Message.error('复制失败')
  }
}
</script>

<style scoped>
.api-test-container {
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

/* 卡片标题样式 */
.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.title-icon {
  font-size: 16px;
  color: var(--primary-color);
}

/* 主布局 */
.main-row {
  align-items: flex-start;
}

/* 配置卡片 */
.config-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.config-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 14px 16px;
}

.config-card :deep(.arco-card-body) {
  padding: 16px;
}

.config-card :deep(.arco-form-item) {
  margin-bottom: 16px;
}

.config-card :deep(.arco-radio-group) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.config-card :deep(.arco-radio-group .arco-radio-button) {
  flex: 1;
  min-width: 80px;
  text-align: center;
}

/* 消息列表项 */
.message-item {
  margin-bottom: 12px;
  padding: 8px;
  background: var(--bg-secondary);
  border-radius: 6px;
  border: 1px solid var(--border-color);
}

.message-item:last-child {
  margin-bottom: 0;
}

/* 代码示例卡片 */
.code-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  margin-bottom: 16px;
}

.code-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.code-tabs :deep(.arco-tabs-content) {
  padding: 0;
}

.code-tabs :deep(.arco-tabs-nav) {
  padding: 0 16px;
  margin-bottom: 0;
}

.code-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  font-family: 'Fira Code', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 200px;
  overflow: auto;
  border-radius: 0 0 8px 8px;
}

/* 请求预览与响应区域 */
.preview-response-row {
  margin-bottom: 16px;
}

.preview-card,
.response-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.preview-card :deep(.arco-card-header),
.response-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.preview-card :deep(.arco-card-body),
.response-card :deep(.arco-card-body) {
  padding: 0;
}

.response-actions {
  margin-left: auto;
}

/* JSON 预览 */
.json-preview {
  background: #f7f8fa;
  padding: 16px;
  min-height: 250px;
  max-height: 350px;
  overflow: auto;
  border-radius: 0 0 8px 8px;
}

.json-preview pre {
  margin: 0;
  font-family: 'Fira Code', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}

/* 响应内容 */
.response-body-container {
  background: #f7f8fa;
  min-height: 250px;
  max-height: 350px;
  overflow: auto;
  border-radius: 0 0 8px 8px;
}

.response-content {
  margin: 0;
  padding: 16px;
  font-family: 'Fira Code', Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--text-primary);
}

.empty-response {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 250px;
  color: var(--text-tertiary);
  text-align: center;
  padding: 20px;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  opacity: 0.5;
}

.empty-response p {
  font-size: 14px;
}

/* 请求历史卡片 */
.history-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.history-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.history-card :deep(.arco-card-body) {
  padding: 12px 16px;
}

.history-table :deep(.arco-table-cell) {
  padding: 8px 12px;
}

/* 消息列表项 */
.message-item {
  margin-bottom: 8px;
}

.message-item:last-child {
  margin-bottom: 0;
}

.detail-section {
  margin-bottom: 20px;
}

.detail-section h4 {
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 600;
}

/* 时间单元格 */
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

.endpoint-name {
  font-size: 13px;
  color: var(--text-secondary);
}

.model-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.duration-text {
  font-size: 13px;
  color: var(--text-secondary);
}

.external-id {
  font-size: 12px;
  color: var(--text-tertiary);
  font-family: 'SF Mono', 'Consolas', monospace;
}
</style>
