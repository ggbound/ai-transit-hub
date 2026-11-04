<template>
  <div class="image-test-container">
    <div class="page-header">
      <div>
        <h1>图片生成测试</h1>
        <p>支持多种图片生成模式，参数自动组装</p>
      </div>
    </div>

    <a-row :gutter="16" class="main-row">
      <a-col :xs="24" :sm="24" :md="8" :lg="7" :xl="6">
        <a-card class="config-card" :bordered="false">
          <template #title>
            <div class="card-title">
              <icon-settings class="title-icon" />
              <span>配置选项</span>
            </div>
          </template>

          <a-form :model="formData" layout="vertical" size="small">
            <a-form-item label="API Key">
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

            <a-form-item label="模型">
              <a-select
                v-model="formData.model"
                placeholder="选择模型"
                style="width: 100%;"
                allow-search
              >
                <a-option v-for="model in imageModels" :key="model.id" :value="model.full_model_id">
                  {{ model.name }}
                </a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="生成模式">
              <a-radio-group v-model="currentMode" type="button">
                <a-radio value="text">文生图</a-radio>
                <a-radio value="image">图生图</a-radio>
                <a-radio value="multi_image">多参考图</a-radio>
              </a-radio-group>
            </a-form-item>

            <a-divider style="margin: 8px 0;" />

            <!-- 文生图模式 -->
            <template v-if="currentMode === 'text'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="textPrompt"
                  placeholder="描述要生成的图片内容..."
                  :rows="4"
                />
              </a-form-item>
            </template>

            <!-- 图生图模式 -->
            <template v-else-if="currentMode === 'image'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="imagePrompt"
                  placeholder="描述图片内容..."
                  :rows="3"
                />
              </a-form-item>
              <a-form-item label="参考图片">
                <a-upload
                  :file-list="imageFileList"
                  :show-file-list="false"
                  list-type="picture-card"
                  accept="image/*"
                  @before-upload="beforeImageUpload"
                >
                  <div v-if="!imagePreview" class="upload-area">
                    <icon-plus class="upload-icon" />
                    <div style="font-size: 12px; margin-top: 4px;">上传图片</div>
                  </div>
                  <img v-else :src="imagePreview" class="preview-image" />
                </a-upload>
                <a-button v-if="imagePreview" type="text" size="mini" status="danger" @click="clearImage" style="margin-top: 8px;">
                  移除图片
                </a-button>
              </a-form-item>
            </template>

            <!-- 多参考图模式 -->
            <template v-else-if="currentMode === 'multi_image'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="multiImagePrompt"
                  placeholder="描述图片内容..."
                  :rows="3"
                />
              </a-form-item>
              <a-form-item label="参考图片" v-for="(img, index) in multiImages" :key="`img-${index}`">
                <div style="display: flex; gap: 8px; align-items: flex-start;">
                  <a-upload
                    :file-list="img.fileList"
                    :show-file-list="false"
                    list-type="picture-card"
                    accept="image/*"
                    @before-upload="(file) => beforeMultiImageUpload(file, index)"
                    style="flex-shrink: 0;"
                  >
                    <div v-if="!img.preview" class="upload-area-small">
                      <icon-plus class="upload-icon-small" />
                    </div>
                    <img v-else :src="img.preview" class="preview-image-small" />
                  </a-upload>
                  <div style="flex: 1;">
                    <div style="font-size: 12px; margin-bottom: 4px;">参考图 {{ index + 1 }}</div>
                    <a-button v-if="img.preview" type="text" size="mini" status="danger" @click="clearMultiImage(index)">
                      移除
                    </a-button>
                  </div>
                </div>
              </a-form-item>
              <a-button type="text" size="small" @click="addMultiImage" style="margin-bottom: 12px;">
                + 添加参考图片
              </a-button>
              <a-form-item label="生成张数">
                <a-input-number v-model="formData.max_images" :min="1" :max="10" style="width: 100%;" />
              </a-form-item>
            </template>

            <a-divider style="margin: 8px 0;" />

            <a-form-item label="尺寸">
              <a-select v-model="formData.size" style="width: 100%;">
                <a-option value="1k">1K</a-option>
                <a-option value="1.5k">1.5K</a-option>
                <a-option value="2k">2K</a-option>
                <a-option value="auto">Auto</a-option>
                <a-option value="3k">3K</a-option>
                <a-option value="4k">4K</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="输出格式">
              <a-select v-model="formData.output_format" style="width: 100%;">
                <a-option value="png">PNG</a-option>
                <a-option value="jpeg">JPEG</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="水印">
              <a-switch v-model="formData.watermark" />
            </a-form-item>

            <a-form-item label="流式输出">
              <a-switch v-model="formData.stream" />
            </a-form-item>

            <a-form-item style="margin-top: 12px;">
              <a-button type="primary" @click="sendRequest" :loading="loading" long size="small">
                <template #icon><icon-send /></template>
                生成图片
              </a-button>
            </a-form-item>

            <a-form-item label="JSON预览">
              <a-textarea
                v-model="jsonPreview"
                :rows="8"
                readonly
                style="font-family: 'Fira Code', Consolas, monospace; font-size: 11px; background: var(--color-fill-2);"
              />
            </a-form-item>
          </a-form>
        </a-card>
      </a-col>

      <a-col :xs="24" :sm="24" :md="16" :lg="17" :xl="18">
        <a-card class="code-card" :bordered="false" :body-style="{ padding: 0 }">
          <template #title>
            <div class="card-title">
              <icon-code class="title-icon" />
              <span>代码示例</span>
            </div>
          </template>
          <a-tabs v-model:active-key="codeTab" class="code-tabs" size="small">
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
                  <p>点击"生成图片"查看响应结果</p>
                </div>
              </div>
            </a-card>
          </a-col>
        </a-row>

        <a-card class="images-card" :bordered="false" v-if="generatedImages.length > 0">
          <template #title>
            <div class="card-title">
              <icon-image class="title-icon" />
              <span>生成结果 ({{ generatedImages.length }})</span>
            </div>
          </template>
          <a-row :gutter="16">
            <a-col :span="8" v-for="(image, index) in generatedImages" :key="index">
              <a-card class="image-item" hoverable>
                <img
                  :src="image.url"
                  class="generated-image"
                  alt="Generated image"
                />
                <div class="image-actions">
                  <a-space>
                    <a-button type="text" size="small" @click="downloadImage(image)">
                      <template #icon><icon-download /></template>
                      下载
                    </a-button>
                  </a-space>
                </div>
              </a-card>
            </a-col>
          </a-row>
        </a-card>

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
              <a-table-column title="时间" dataIndex="created_at" :width="180">
                <template #cell="{ record }">
                  <div class="time-cell">
                    <div class="time-main">{{ formatTime(record.created_at) }}</div>
                    <div class="time-sub">{{ formatDateTime(record.created_at) }}</div>
                  </div>
                </template>
              </a-table-column>
              <a-table-column title="模式" dataIndex="mode" :width="100">
                <template #cell="{ record }">
                  <a-tag color="purple" size="small">{{ record.mode }}</a-tag>
                </template>
              </a-table-column>
              <a-table-column title="模型" dataIndex="model" :width="200">
                <template #cell="{ record }">
                  <span class="model-name">{{ record.model || '-' }}</span>
                </template>
              </a-table-column>
              <a-table-column title="状态" dataIndex="status" :width="100">
                <template #cell="{ record }">
                  <a-tag :color="record.status < 400 ? 'green' : 'red'" size="small">
                    {{ record.status }}
                  </a-tag>
                </template>
              </a-table-column>
              <a-table-column title="外部ID" dataIndex="external_task_id" :width="150">
                <template #cell="{ record }">
                  <span class="external-id">{{ record.external_task_id || '-' }}</span>
                </template>
              </a-table-column>
              <a-table-column title="操作" :width="70" align="center" fixed="right">
                <template #cell="{ record }">
                  <a-tooltip content="查看详情">
                    <a-button type="text" size="small" @click="showDetail(record)">
                      <template #icon><icon-eye /></template>
                    </a-button>
                  </a-tooltip>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>

    <a-drawer
      v-model:visible="detailModalVisible"
      title="请求详情"
      :width="860"
      unmount-on-close
      :footer="false"
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
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Message } from '@arco-design/web-vue'
import {
  IconSend,
  IconCopy,
  IconDownload,
  IconSettings,
  IconCode,
  IconEye,
  IconReply,
  IconHistory,
  IconImage,
  IconRefresh,
  IconPlus
} from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'
import config from '@/config/api.config.js'

const formData = ref({
  model: '',
  size: '1k',
  output_format: 'png',
  watermark: false,
  max_images: 3,
  stream: false
})

const currentMode = ref('text')

// 文本模式
const textPrompt = ref('充满活力的特写编辑肖像，模特眼神犀利，头戴雕塑感帽子，色彩拼接丰富，眼部焦点锐利，景深较浅，具有Vogue杂志封面的美学风格，采用中画幅拍摄，工作室灯光效果强烈')

// 图生图模式
const imagePrompt = ref('保持模特姿势和液态服装的流动形状不变。将服装材质从银色金属改为完全透明的清水（或玻璃）。透过液态水流，可以看到模特的皮肤细节。光影从反射变为折射')
const imageFileList = ref([])
const imagePreview = ref('')
const imageBase64 = ref('')

// 多参考图模式
const multiImagePrompt = ref('生成3张女孩和奶牛玩偶在游乐园开心地坐过山车的图片，涵盖早晨、中午、晚上')
const multiImages = ref([])

const selectedApiKey = ref('')
const codeTab = ref('curl')
const loading = ref(false)
const responseBody = ref('')
const responseStatus = ref(null)
const responseTime = ref('')

const apiKeys = ref([])
const imageModels = ref([])
const generatedImages = ref([])
const historyRecords = ref([])

const detailModalVisible = ref(false)
const loadingDetail = ref(false)
const detailData = ref(null)

const statusColor = computed(() => {
  if (!responseStatus.value) return 'default'
  if (responseStatus.value < 400) return 'green'
  return 'red'
})

const jsonPreview = computed(() => {
  try {
    const payload = assemblePayload()
    return JSON.stringify(payload, null, 2)
  } catch (e) {
    return JSON.stringify({ error: '组装参数错误', message: e.message }, null, 2)
  }
})

const curlExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  try {
    const payload = assemblePayload()
    let bodyStr = JSON.stringify(payload, null, 2).replace(/'/g, "\\'")
    
    if (formData.value.stream) {
      return `curl -X POST "${config.api.openai.base}/images/generations" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${authValue}" \\
  -d '${bodyStr}'`
    } else {
      return `curl -X POST "${config.api.openai.base}/images/generations" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${authValue}" \\
  -d '${bodyStr}'`
    }
  } catch (e) {
    return '参数错误'
  }
})

const jsExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  const url = config.api.openai.base + '/images/generations'
  try {
    const payload = assemblePayload()
    
    if (formData.value.stream) {
      return `// 流式输出示例
const response = await fetch('${url}', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  body: JSON.stringify(${JSON.stringify(payload, null, 2)})
});

const reader = response.body.getReader();
const decoder = new TextDecoder();

while (true) {
  const { done, value } = await reader.read();
  if (done) break;
  
  const chunk = decoder.decode(value, { stream: true });
  const lines = chunk.split('\\n').filter(line => line.trim() && line.startsWith('data: '));
  
  for (const line of lines) {
    const data = JSON.parse(line.substring(6));
    console.log(data);
    
    if (data.type === 'image_generation.completed') {
      console.log('生成完成:', data);
    }
  }
}`
    } else {
      return `// 非流式输出示例
const response = await fetch('${url}', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  body: JSON.stringify(${JSON.stringify(payload, null, 2)})
});

const data = await response.json();
console.log(data);`
    }
  } catch (e) {
    return '参数错误'
  }
})

const pythonExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  const url = config.api.openai.base + '/images/generations'
  try {
    const payload = assemblePayload()
    const bodyStr = JSON.stringify(payload, null, 2)
      .replace(/true/g, 'True')
      .replace(/false/g, 'False')
      .replace(/null/g, 'None')
    
    if (formData.value.stream) {
      return `# 流式输出示例
import requests
import json

response = requests.post(
  '${url}',
  headers={
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  json=${bodyStr},
  stream=True
)

for line in response.iter_lines():
  if line and line.startswith(b'data: '):
    data_str = line[6:].decode('utf-8')
    data = json.loads(data_str)
    print(data)
    
    if data.get('type') == 'image_generation.completed':
      print('生成完成:', data)`
    } else {
      return `# 非流式输出示例
import requests

response = requests.post(
  '${url}',
  headers={
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  json=${bodyStr}
)

print(response.json())`
    }
  } catch (e) {
    return '参数错误'
  }
})

function assemblePayload() {
  // V1 接口要求 model 为 'provider/model' 完整格式，直接使用 full_model_id，不再剥离提供商前缀
  let modelName = formData.value.model || 'doubao/doubao-seedream-5-0-pro-260628'

  const payload = {
    model: modelName,
    size: formData.value.size,
    output_format: formData.value.output_format,
    watermark: formData.value.watermark
  }

  // 添加流式输出参数
  if (formData.value.stream) {
    payload.stream = true
  }

  switch (currentMode.value) {
    case 'text':
      payload.prompt = textPrompt.value
      break

    case 'image':
      payload.prompt = imagePrompt.value
      if (imageBase64.value) {
        payload.image = imageBase64.value
      }
      break

    case 'multi_image':
      payload.prompt = multiImagePrompt.value
      const images = multiImages.value.filter(img => img.base64).map(img => img.base64)
      if (images.length > 0) {
        payload.image = images
        payload.sequential_image_generation = 'auto'
        payload.sequential_image_generation_options = {
          max_images: formData.value.max_images
        }
      }
      break
  }

  return payload
}

// OSS 上传文件的公共函数
async function uploadFileToOSS(file) {
  try {
    const formData = new FormData()
    formData.append('file', file)
    
    const response = await fetch(config.api.oss.uploadUrl, {
      method: 'POST',
      headers: {
        'Token-Id': config.api.oss.tokenId,
        'Token-Secret': config.api.oss.tokenSecret,
      },
      body: formData,
    })
    
    const result = await response.json()
    if (result.code === 0 && result.content?.origin) {
      return result.content.origin
    } else {
      throw new Error(result.message || '文件上传失败')
    }
  } catch (error) {
    throw error
  }
}

function beforeImageUpload(file) {
  const isImage = file.type && file.type.startsWith('image')
  if (!isImage) {
    Message.error('只能上传图片文件')
    return false
  }
  const sizeLimit = 10 * 1024 * 1024
  if (file.size > sizeLimit) {
    Message.error('图片大小不能超过 10MB')
    return false
  }
  
  // 先显示预览
  imagePreview.value = URL.createObjectURL(file)
  imageFileList.value = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      imageBase64.value = imageUrl // 这里存的是URL不是base64
      Message.success('图片上传成功')
    })
    .catch((error) => {
      Message.error(error.message || '图片上传失败')
      clearImage()
    })
  
  // 返回false阻止默认上传行为
  return false
}

function beforeMultiImageUpload(file, index) {
  const isImage = file.type && file.type.startsWith('image')
  if (!isImage) {
    Message.error('只能上传图片文件')
    return false
  }
  const sizeLimit = 10 * 1024 * 1024
  if (file.size > sizeLimit) {
    Message.error('图片大小不能超过 10MB')
    return false
  }
  
  // 先显示预览
  multiImages.value[index].preview = URL.createObjectURL(file)
  multiImages.value[index].fileList = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      multiImages.value[index].base64 = imageUrl // 这里存的是URL不是base64
      Message.success(`参考图${index + 1}上传成功`)
    })
    .catch((error) => {
      Message.error(error.message || '图片上传失败')
      clearMultiImage(index)
    })
  
  // 返回false阻止默认上传行为
  return false
}

function clearImage() {
  imageFileList.value = []
  imagePreview.value = ''
  imageBase64.value = ''
}

function addMultiImage() {
  multiImages.value.push({ fileList: [], preview: '', base64: '' })
}

function clearMultiImage(index) {
  multiImages.value.splice(index, 1)
}

onMounted(() => {
  loadApiKeys()
  loadModels()
  loadHistory()
  // 初始化多参考图模式的参考图片
  addMultiImage()
  addMultiImage()
})

async function loadApiKeys() {
  try {
    const res = await api.get('/user/api-keys')
    if (res.data.success) {
      apiKeys.value = res.data.data.data || res.data.data
      if (apiKeys.value.length > 0) {
        selectedApiKey.value = apiKeys.value[0].api_key
      }
    }
  } catch (error) {
    console.error('加载 API Keys 失败:', error)
  }
}

async function loadModels() {
  try {
    const res = await api.get('/models')
    if (res.data.success) {
      const models = res.data.data.models || []
      imageModels.value = models.filter(m => m.type === 'image')
      if (imageModels.value.length > 0 && !formData.value.model) {
        const doubaoModel = imageModels.value.find(m => m.code?.toLowerCase().includes('doubao'))
        if (doubaoModel) {
          formData.value.model = doubaoModel.full_model_id
        } else {
          formData.value.model = imageModels.value[0].full_model_id
        }
      }
    }
  } catch (error) {
    console.error('加载模型失败:', error)
  }
}

function getModeName(mode) {
  const modeNames = {
    'text': '文生图',
    'image': '图生图',
    'multi_image': '多参考图'
  }
  return modeNames[mode] || '图片'
}

function getModeFromRecord(record) {
  try {
    if (record.detail && record.detail.request_body) {
      const data = JSON.parse(record.detail.request_body)
      if (Array.isArray(data.image)) return '多参考图'
      if (data.image) return '图生图'
      return '文生图'
    }
  } catch {
  }
  return '图片'
}

async function loadHistory() {
  try {
    const response = await api.get('/api-test/history', {
      params: {
        per_page: 20,
        page: 1,
        type: 'image'
      }
    })

    if (response.data.success) {
      historyRecords.value = response.data.data.data.map(item => ({
        id: item.id,
        created_at: item.created_at,
        api_type: item.api_type,
        endpoint: item.endpoint,
        model: item.model,
        mode: getModeFromRecord(item),
        status: item.status,
        duration: item.duration,
        external_task_id: item.external_task_id,
        user_api_key_id: item.user_api_key_id
      }))
    }
  } catch (error) {
    console.error('加载历史记录失败:', error)
  }
}

async function showDetail(record) {
  if (!record.id) {
    Message.warning('这是本地记录')
    return
  }
  detailModalVisible.value = true
  loadingDetail.value = true
  detailData.value = null

  try {
    const response = await api.get(`/api-test/${record.id}`)
    if (response.data.success) {
      detailData.value = response.data.data
    }
  } catch (error) {
    Message.error('加载详情失败')
  } finally {
    loadingDetail.value = false
  }
}

async function sendRequest() {
  if (!selectedApiKey.value) {
    Message.error('请选择 API Key')
    return
  }
  if (!formData.value.model) {
    Message.error('请选择模型')
    return
  }

  // 验证必填字段
  let promptValue = ''
  switch (currentMode.value) {
    case 'text':
      promptValue = textPrompt.value
      break
    case 'image':
      promptValue = imagePrompt.value
      break
    case 'multi_image':
      promptValue = multiImagePrompt.value
      break
  }

  if (!promptValue || !promptValue.trim()) {
    Message.error('请输入提示词')
    return
  }

  loading.value = true
  const startTime = Date.now()

  try {
    const headers = {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${selectedApiKey.value}`
    }

    const payload = assemblePayload() // 组装实际发送的请求参数

    if (formData.value.stream) {
      // 流式请求
      const response = await fetch(`${config.api.openai.base}/images/generations`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload)
      })

      responseStatus.value = response.status
      let finalData = null
      let streamResponse = ''
      generatedImages.value = []

      if (response.ok) {
        const reader = response.body.getReader()
        const decoder = new TextDecoder()

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value, { stream: true })
          streamResponse += chunk

          const lines = chunk.split('\n').filter(line => line.trim() && line.startsWith('data: '))
          for (const line of lines) {
            const dataStr = line.substring(6)
            try {
              const data = JSON.parse(dataStr)
              
              // 记录流式响应数据
              streamResponse += JSON.stringify(data, null, 2) + '\n'
              
              // 处理 image_generation.completed 事件
              if (data.type === 'image_generation.completed') {
                finalData = data
                
                // 提取图片
                if (data.data) {
                  const hasImages = data.data.some(img => img.url || img.image_url || img.data_url)
                  if (hasImages) {
                    generatedImages.value = data.data.map(img => ({
                      ...img,
                      url: img.url || img.image_url || img.data_url || `data:image/png;base64,${img.b64_json}`
                    }))
                    Message.success(`成功生成 ${data.data.length} 张图片`)
                  }
                }
              }
            } catch (e) {
              // 忽略解析错误
            }
          }
        }

        responseBody.value = streamResponse
        responseTime.value = `${Date.now() - startTime}ms`
      } else {
        const data = await response.json()
        responseBody.value = JSON.stringify(data, null, 2)
        responseTime.value = `${Date.now() - startTime}ms`
        Message.error(data.error?.message || '图片生成失败')
      }
    } else {
      // 非流式请求
      const response = await fetch(`${config.api.openai.base}/images/generations`, {
        method: 'POST',
        headers,
        body: JSON.stringify(payload)
      })

      const data = await response.json()
      responseStatus.value = response.status
      responseBody.value = JSON.stringify(data, null, 2)
      responseTime.value = `${Date.now() - startTime}ms`

      if (response.ok) {
        if (data.data) {
          const hasImages = data.data.some(img => img.url || img.image_url || img.data_url)
          if (hasImages) {
            generatedImages.value = data.data.map(img => ({
              ...img,
              url: img.url || img.image_url || img.data_url || `data:image/png;base64,${img.b64_json}`
            }))
            Message.success(`成功生成 ${data.data.length} 张图片`)
          } else {
            generatedImages.value = []
            Message.error('未获取到生成结果')
          }
        } else if (data.url || data.image_url || data.data_url || data.b64_json) {
          generatedImages.value = [{
            url: data.url || data.image_url || data.data_url || `data:image/png;base64,${data.b64_json}`,
            ...data
          }]
          Message.success('成功生成 1 张图片')
        } else {
          generatedImages.value = []
          Message.error('未获取到生成结果')
        }
      } else {
        generatedImages.value = []
        Message.error(data.error?.message || '图片生成失败')
      }
    }

    // 请求结束后，直接刷新列表获取最新数据
    await loadHistory()
  } catch (error) {
    responseStatus.value = error.response?.status || 500
    responseBody.value = JSON.stringify({
      error: error.message,
      details: error.response?.data || '请求失败'
    }, null, 2)
    Message.error('请求失败: ' + error.message)

    // 请求结束后，直接刷新列表获取最新数据
    await loadHistory()
  } finally {
    loading.value = false
  }
}

async function downloadImage(image) {
  try {
    let imageUrl = image.url
    if (image.b64_json && !image.url) {
      imageUrl = `data:image/png;base64,${image.b64_json}`
    }

    if (imageUrl.startsWith('data:')) {
      const link = document.createElement('a')
      link.href = imageUrl
      link.download = `generated-image-${Date.now()}.${formData.value.output_format === 'jpeg' ? 'jpg' : formData.value.output_format}`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
    } else {
      const response = await fetch(imageUrl)
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `generated-image-${Date.now()}.${formData.value.output_format === 'jpeg' ? 'jpg' : formData.value.output_format}`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    }
    Message.success('开始下载')
  } catch (error) {
    Message.error('下载失败')
  }
}

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

async function copyResponse() {
  try {
    await navigator.clipboard.writeText(responseBody.value)
    Message.success('已复制')
  } catch {
    Message.error('复制失败')
  }
}
</script>

<style scoped>
.image-test-container {
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

.main-row {
  align-items: flex-start;
}

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

.upload-area, .upload-area-small {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
}

.upload-area-small {
  width: 80px;
  height: 80px;
}

.upload-icon {
  font-size: 24px;
  color: var(--text-secondary);
}

.upload-icon-small {
  font-size: 20px;
  color: var(--text-secondary);
}

.preview-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.preview-image-small {
  width: 80px;
  height: 80px;
  object-fit: cover;
  border-radius: 4px;
}

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

.preview-response-row {
  margin-bottom: 16px;
}

.preview-card, .response-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.preview-card :deep(.arco-card-header), .response-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.preview-card :deep(.arco-card-body), .response-card :deep(.arco-card-body) {
  padding: 0;
}

.response-actions {
  margin-left: auto;
}

.json-preview, .response-body-container {
  background: #f7f8fa;
  padding: 16px;
  min-height: 200px;
  max-height: 300px;
  overflow: auto;
  border-radius: 0 0 8px 8px;
}

.json-preview pre, .response-content {
  margin: 0;
  font-family: 'Fira Code', Consolas, monospace;
  font-size: 11px;
  color: var(--text-primary);
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.empty-response {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--text-tertiary);
  text-align: center;
  padding: 20px;
}

.empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
  opacity: 0.5;
}

.images-card {
  margin-bottom: 16px;
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.images-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.image-item {
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.image-item :deep(.arco-card-body) {
  padding: 0;
}

.generated-image {
  width: 100%;
  height: auto;
}

.image-actions {
  padding: 12px;
  border-top: 1px solid var(--border-color);
  display: flex;
  justify-content: center;
}

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
