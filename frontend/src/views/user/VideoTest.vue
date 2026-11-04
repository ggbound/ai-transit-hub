<template>
  <div class="video-test-container">
    <div class="page-header">
      <div>
        <h1>视频生成测试</h1>
        <p>支持多种视频生成模式，参数自动组装</p>
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
                <a-option v-for="model in videoModels" :key="model.id" :value="model.full_model_id">
                  {{ model.name }}
                </a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="生成模式">
              <a-radio-group v-model="currentMode" type="button">
                <a-radio value="text">文生视频</a-radio>
                <a-radio value="image">图生视频</a-radio>
                <a-radio value="first_last_frame">首尾帧</a-radio>
                <a-radio value="first_frame_audio">首帧有声</a-radio>
                <a-radio value="omni_reference">全模态参考</a-radio>
              </a-radio-group>
            </a-form-item>

            <a-divider style="margin: 8px 0;" />

            <!-- 文生视频 -->
            <template v-if="currentMode === 'text'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="textPrompt"
                  placeholder="描述要生成的视频内容..."
                  :rows="4"
                />
              </a-form-item>
            </template>

            <!-- 图生视频 -->
            <template v-else-if="currentMode === 'image'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="imagePrompt"
                  placeholder="描述视频内容..."
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

            <!-- 首尾帧 -->
            <template v-else-if="currentMode === 'first_last_frame'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="firstLastPrompt"
                  placeholder="描述视频内容..."
                  :rows="3"
                />
              </a-form-item>
              <a-form-item label="首帧图片">
                <a-upload
                  :file-list="firstFrameFileList"
                  :show-file-list="false"
                  list-type="picture-card"
                  accept="image/*"
                  @before-upload="beforeFirstFrameUpload"
                >
                  <div v-if="!firstFramePreview" class="upload-area">
                    <icon-plus class="upload-icon" />
                    <div style="font-size: 12px; margin-top: 4px;">上传首帧</div>
                  </div>
                  <img v-else :src="firstFramePreview" class="preview-image" />
                </a-upload>
                <a-button v-if="firstFramePreview" type="text" size="mini" status="danger" @click="clearFirstFrame" style="margin-top: 8px;">
                  移除
                </a-button>
              </a-form-item>
              <a-form-item label="尾帧图片">
                <a-upload
                  :file-list="lastFrameFileList"
                  :show-file-list="false"
                  list-type="picture-card"
                  accept="image/*"
                  @before-upload="beforeLastFrameUpload"
                >
                  <div v-if="!lastFramePreview" class="upload-area">
                    <icon-plus class="upload-icon" />
                    <div style="font-size: 12px; margin-top: 4px;">上传尾帧</div>
                  </div>
                  <img v-else :src="lastFramePreview" class="preview-image" />
                </a-upload>
                <a-button v-if="lastFramePreview" type="text" size="mini" status="danger" @click="clearLastFrame" style="margin-top: 8px;">
                  移除
                </a-button>
              </a-form-item>
              <a-form-item label="生成音频">
                <a-switch v-model="formData.generate_audio" />
              </a-form-item>
            </template>

            <!-- 首帧音频 -->
            <template v-else-if="currentMode === 'first_frame_audio'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="firstAudioPrompt"
                  placeholder="描述视频内容..."
                  :rows="3"
                />
              </a-form-item>
              <a-form-item label="参考图片">
                <a-upload
                  :file-list="firstAudioFileList"
                  :show-file-list="false"
                  list-type="picture-card"
                  accept="image/*"
                  @before-upload="beforeFirstAudioImageUpload"
                >
                  <div v-if="!firstAudioPreview" class="upload-area">
                    <icon-plus class="upload-icon" />
                    <div style="font-size: 12px; margin-top: 4px;">上传图片</div>
                  </div>
                  <img v-else :src="firstAudioPreview" class="preview-image" />
                </a-upload>
                <a-button v-if="firstAudioPreview" type="text" size="mini" status="danger" @click="clearFirstAudio" style="margin-top: 8px;">
                  移除
                </a-button>
              </a-form-item>
              <a-form-item label="生成音频">
                <a-switch v-model="formData.generate_audio" />
              </a-form-item>
            </template>

            <!-- 全模态参考 -->
            <template v-else-if="currentMode === 'omni_reference'">
              <a-form-item label="提示词">
                <a-textarea
                  v-model="omniPrompt"
                  placeholder="描述视频内容，可使用 @图像N @视频N 标记..."
                  :rows="5"
                />
              </a-form-item>
              <a-divider style="margin: 8px 0;" />
              <div style="margin-bottom: 8px;">
                <div style="font-size: 12px; color: var(--text-secondary); margin-bottom: 8px;">
                  参考素材（图片：使用 @图像N，视频：使用 @视频N）
                </div>
                <a-form-item label="参考图片" v-for="(img, idx) in omniImages" :key="`img-${idx}`">
                  <div style="display: flex; gap: 8px; align-items: flex-start;">
                    <a-upload
                      :file-list="img.fileList"
                      :show-file-list="false"
                      list-type="picture-card"
                      accept="image/*"
                      @before-upload="(file) => beforeOmniImageUpload(file, idx)"
                      style="flex-shrink: 0;"
                    >
                      <div v-if="!img.preview" class="upload-area-small">
                        <icon-plus class="upload-icon-small" />
                      </div>
                      <img v-else :src="img.preview" class="preview-image-small" />
                    </a-upload>
                    <div style="flex: 1;">
                      <div style="font-size: 12px; margin-bottom: 4px;">@图像{{ idx + 1 }}</div>
                      <a-button v-if="img.preview" type="text" size="mini" status="danger" @click="clearOmniImage(idx)">
                        移除
                      </a-button>
                    </div>
                  </div>
                </a-form-item>
                <a-button type="text" size="small" @click="addOmniImage" style="margin-bottom: 12px;">
                  + 添加参考图片
                </a-button>
                <a-form-item label="参考视频" v-for="(vid, idx) in omniVideos" :key="`vid-${idx}`">
                  <div style="display: flex; gap: 8px; align-items: flex-start;">
                    <a-upload
                      :file-list="vid.fileList"
                      :show-file-list="false"
                      :custom-request="(options) => handleOmniVideoUpload(options, idx)"
                      list-type="text"
                      accept="video/*"
                      @before-upload="beforeVideoUpload"
                    >
                      <div v-if="!vid.name" class="upload-video-area">
                        <icon-play-circle class="upload-icon-small" />
                        <div style="font-size: 12px; margin-top: 4px;">上传视频</div>
                      </div>
                      <div v-else class="video-name-display">{{ vid.name }}</div>
                    </a-upload>
                    <div style="flex: 1;">
                      <div style="font-size: 12px; margin-bottom: 4px;">@视频{{ idx + 1 }}</div>
                      <a-button v-if="vid.name" type="text" size="mini" status="danger" @click="clearOmniVideo(idx)">
                        移除
                      </a-button>
                    </div>
                  </div>
                </a-form-item>
                <a-button type="text" size="small" @click="addOmniVideo" style="margin-bottom: 8px;">
                  + 添加参考视频
                </a-button>
              </div>
              <a-form-item label="生成音频">
                <a-switch v-model="formData.generate_audio" />
              </a-form-item>
            </template>

            <a-divider style="margin: 8px 0;" />

            <a-form-item label="视频比例">
              <a-select v-model="formData.ratio" style="width: 100%;">
                <a-option value="adaptive">自适应</a-option>
                <a-option value="16:9">16:9</a-option>
                <a-option value="9:16">9:16</a-option>
                <a-option value="1:1">1:1</a-option>
                <a-option value="4:3">4:3</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="视频时长">
              <a-select v-model="formData.duration" style="width: 100%;">
                <a-option :value="5">5秒</a-option>
                <a-option :value="10">10秒</a-option>
                <a-option :value="15">15秒</a-option>
                <a-option :value="30">30秒</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="视频分辨率">
              <a-select v-model="formData.resolution" style="width: 100%;">
                <a-option value="480p">480p</a-option>
                <a-option value="720p">720p</a-option>
                <a-option value="1080p">1080p</a-option>
                <a-option value="4k">4K</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="输出格式">
              <a-select v-model="formData.output_format" style="width: 100%;">
                <a-option value="mp4">MP4</a-option>
                <a-option value="mov">MOV</a-option>
              </a-select>
            </a-form-item>

            <a-form-item label="水印">
              <a-switch v-model="formData.watermark" />
            </a-form-item>

            <a-form-item style="margin-top: 12px;">
              <a-button type="primary" @click="sendRequest" :loading="loading" long size="small">
                <template #icon><icon-send /></template>
                生成视频
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
                  <p>点击"生成视频"查看响应结果</p>
                </div>
              </div>
            </a-card>
          </a-col>
        </a-row>

        <a-card class="videos-card" :bordered="false" v-if="generatedVideos.length > 0">
          <template #title>
            <div class="card-title">
              <icon-play-circle class="title-icon" />
              <span>生成结果 ({{ generatedVideos.length }})</span>
            </div>
          </template>
          <a-row :gutter="16">
            <a-col :span="12" v-for="(video, index) in generatedVideos" :key="index">
              <a-card class="video-item" hoverable>
                <video
                  :src="video.url"
                  class="generated-video"
                  controls
                />
                <div class="video-actions">
                  <a-space>
                    <a-button type="text" size="small" @click="downloadVideo(video)">
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
              <a-table-column title="操作" :width="110" align="center" fixed="right">
                <template #cell="{ record }">
                  <a-button-group size="small">
                    <a-tooltip content="查询任务">
                      <a-button
                        type="text"
                        size="small"
                        @click="queryVideoTask(record)"
                        :loading="queryingTask && currentQueryingRecord?.id === record.id"
                      >
                        <template #icon><icon-refresh /></template>
                      </a-button>
                    </a-tooltip>
                    <a-tooltip content="查看详情">
                      <a-button type="text" size="small" @click="showDetail(record)">
                        <template #icon><icon-eye /></template>
                      </a-button>
                    </a-tooltip>
                  </a-button-group>
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
          <h4>基础信息</h4>          <a-descriptions :column="2" bordered size="small">
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
  IconPlayCircle,
  IconRefresh,
  IconPlus
} from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'
import config from '@/config/api.config.js'

const formData = ref({
  model: '',
  ratio: '16:9',
  duration: 5,
  resolution: '480p',
  output_format: 'mp4',
  generate_audio: false,
  watermark: false
})

const currentMode = ref('text')

// 监听模式切换
const handleModeChange = (newMode) => {
  currentMode.value = newMode
}

// 文本模式
const textPrompt = ref('')

// 图生视频模式
const imagePrompt = ref('')
const imageFileList = ref([])
const imagePreview = ref('')
const imageBase64 = ref('')

// 首尾帧模式
const firstLastPrompt = ref('')
const firstFrameFileList = ref([])
const firstFramePreview = ref('')
const firstFrameBase64 = ref('')
const lastFrameFileList = ref([])
const lastFramePreview = ref('')
const lastFrameBase64 = ref('')

// 首帧音频模式
const firstAudioPrompt = ref('')
const firstAudioFileList = ref([])
const firstAudioPreview = ref('')
const firstAudioBase64 = ref('')

// 全模态参考模式
const omniPrompt = ref('')
const omniImages = ref([])
const omniVideos = ref([])

const selectedApiKey = ref('')
const codeTab = ref('curl')
const loading = ref(false)
const responseBody = ref('')
const responseStatus = ref(null)
const responseTime = ref('')

const apiKeys = ref([])
const videoModels = ref([])
const generatedVideos = ref([])
const historyRecords = ref([])

const detailModalVisible = ref(false)
const loadingDetail = ref(false)
const detailData = ref(null)
const queryingTask = ref(false)
const currentQueryingRecord = ref(null)

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
    return JSON.stringify({ error: '组装参数错误' }, null, 2)
  }
})

const curlExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  try {
    const payload = assemblePayload()
    let bodyStr = JSON.stringify(payload, null, 2).replace(/'/g, "\\'")
    return `curl -X POST "${config.api.openai.base}/videos/generations/tasks" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${authValue}" \\
  -d '${bodyStr}'`
  } catch (e) {
    return '参数错误'
  }
})

const jsExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  const url = config.api.openai.base + '/videos/generations/tasks'
  try {
    const payload = assemblePayload()
    return `const response = await fetch('${url}', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  body: JSON.stringify(${JSON.stringify(payload, null, 2)})
});

const data = await response.json();
console.log(data);`
  } catch (e) {
    return '参数错误'
  }
})

const pythonExample = computed(() => {
  const authValue = selectedApiKey.value || '<your-api-key>'
  const url = config.api.openai.base + '/videos/generations/tasks'
  try {
    const payload = assemblePayload()
    const bodyStr = JSON.stringify(payload, null, 2)
      .replace(/true/g, 'True')
      .replace(/false/g, 'False')
      .replace(/null/g, 'None')
    return `import requests

response = requests.post(
  '${url}',
  headers={
    'Content-Type': 'application/json',
    'Authorization': 'Bearer ${authValue}'
  },
  json=${bodyStr}
)

print(response.json())`
  } catch (e) {
    return '参数错误'
  }
})

function assemblePayload(forPreview = true) {
  let modelName = formData.value.model || 'model-name'

  const payload = {
    model: modelName,
    content: [],
    duration: formData.value.duration,
    resolution: formData.value.resolution,
    watermark: formData.value.watermark
  }

  // 只有在纯文生视频模式下才添加ratio参数
  if (currentMode.value === 'text') {
    payload.ratio = formData.value.ratio
  }

  switch (currentMode.value) {
    case 'text':
      payload.content = [
        { type: 'text', text: textPrompt.value }
      ]
      break

    case 'image':
      payload.content = [
        { type: 'text', text: imagePrompt.value }
      ]
      if (imageBase64.value) {
        payload.content.push({
          type: 'image_url',
          image_url: { url: imageBase64.value }
        })
      }
      break

    case 'first_last_frame':
      payload.content = [
        { type: 'text', text: firstLastPrompt.value }
      ]
      if (firstFrameBase64.value) {
        payload.content.push({
          type: 'image_url',
          image_url: { url: firstFrameBase64.value },
          role: 'first_frame'
        })
      }
      if (lastFrameBase64.value) {
        payload.content.push({
          type: 'image_url',
          image_url: { url: lastFrameBase64.value },
          role: 'last_frame'
        })
      }
      payload.generate_audio = formData.value.generate_audio
      break

    case 'first_frame_audio':
      payload.content = [
        { type: 'text', text: firstAudioPrompt.value }
      ]
      if (firstAudioBase64.value) {
        payload.content.push({
          type: 'image_url',
          image_url: { url: firstAudioBase64.value }
        })
      }
      payload.generate_audio = formData.value.generate_audio
      break

    case 'omni_reference':
      payload.content = [
        { type: 'text', text: omniPrompt.value }
      ]
      omniImages.value.forEach((img) => {
        if (img.base64) {
          payload.content.push({
            type: 'image_url',
            image_url: { url: img.base64 },
            role: 'reference_image'
          })
        }
      })
      omniVideos.value.forEach((vid) => {
        if (vid.base64) {
          payload.content.push({
            type: 'video_url',
            video_url: { url: vid.base64 },
            role: 'reference_video'
          })
        }
      })
      payload.generate_audio = formData.value.generate_audio
      payload.output_format = formData.value.output_format
      break
  }

  return payload
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

function beforeVideoUpload(file) {
  const isVideo = file.type && file.type.startsWith('video')
  if (!isVideo) {
    Message.error('只能上传视频文件')
    return false
  }
  const sizeLimit = 100 * 1024 * 1024
  if (file.size > sizeLimit) {
    Message.error('视频大小不能超过 100MB')
    return false
  }
  return true
}

function clearImage() {
  imageFileList.value = []
  imagePreview.value = ''
  imageBase64.value = ''
}

function beforeFirstFrameUpload(file) {
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
  firstFramePreview.value = URL.createObjectURL(file)
  firstFrameFileList.value = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      firstFrameBase64.value = imageUrl // 这里存的是URL不是base64
      Message.success('首帧上传成功')
    })
    .catch((error) => {
      Message.error(error.message || '首帧上传失败')
      clearFirstFrame()
    })
  
  // 返回false阻止默认上传行为
  return false
}

function clearFirstFrame() {
  firstFrameFileList.value = []
  firstFramePreview.value = ''
  firstFrameBase64.value = ''
}

function beforeLastFrameUpload(file) {
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
  lastFramePreview.value = URL.createObjectURL(file)
  lastFrameFileList.value = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      lastFrameBase64.value = imageUrl // 这里存的是URL不是base64
      Message.success('尾帧上传成功')
    })
    .catch((error) => {
      Message.error(error.message || '尾帧上传失败')
      clearLastFrame()
    })
  
  // 返回false阻止默认上传行为
  return false
}

function clearLastFrame() {
  lastFrameFileList.value = []
  lastFramePreview.value = ''
  lastFrameBase64.value = ''
}

function beforeFirstAudioImageUpload(file) {
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
  firstAudioPreview.value = URL.createObjectURL(file)
  firstAudioFileList.value = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      firstAudioBase64.value = imageUrl // 这里存的是URL不是base64
      Message.success('图片上传成功')
    })
    .catch((error) => {
      Message.error(error.message || '图片上传失败')
      clearFirstAudio()
    })
  
  // 返回false阻止默认上传行为
  return false
}

function clearFirstAudio() {
  firstAudioFileList.value = []
  firstAudioPreview.value = ''
  firstAudioBase64.value = ''
}

function addOmniImage() {
  omniImages.value.push({ fileList: [], preview: '', base64: '' })
}

function clearOmniImage(index) {
  omniImages.value.splice(index, 1)
}

function beforeOmniImageUpload(file, index) {
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
  omniImages.value[index].preview = URL.createObjectURL(file)
  omniImages.value[index].fileList = [file]
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((imageUrl) => {
      omniImages.value[index].base64 = imageUrl // 这里存的是URL不是base64
      Message.success(`图片${index + 1}上传成功`)
    })
    .catch((error) => {
      Message.error(error.message || '图片上传失败')
      clearOmniImage(index)
    })
  
  // 返回false阻止默认上传行为
  return false
}

function addOmniVideo() {
  omniVideos.value.push({ fileList: [], name: '', base64: '' })
}

function clearOmniVideo(index) {
  omniVideos.value.splice(index, 1)
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

function handleOmniVideoUpload({ file }, index) {
  // 先显示文件名
  omniVideos.value[index].name = file.name
  
  // 上传到 OSS
  uploadFileToOSS(file)
    .then((fileUrl) => {
      omniVideos.value[index].base64 = fileUrl // 这里存的是URL不是base64
      Message.success(`视频${index + 1}上传成功`)
    })
    .catch((error) => {
      Message.error(error.message || '视频上传失败')
      clearOmniVideo(index)
    })
}

onMounted(() => {
  loadApiKeys()
  loadModels()
  loadHistory()
  // 初始化全模态参考模式的参考资源
  addOmniImage()
  addOmniVideo()
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
      videoModels.value = models.filter(m => m.type === 'video')
      if (videoModels.value.length > 0 && !formData.value.model) {
        const doubaoModel = videoModels.value.find(m => m.code?.toLowerCase().includes('doubao'))
        if (doubaoModel) {
          formData.value.model = doubaoModel.full_model_id
        } else {
          formData.value.model = videoModels.value[0].full_model_id
        }
      }
    }
  } catch (error) {
    console.error('加载模型失败:', error)
  }
}

function getModeName(mode) {
  const modeNames = {
    'text': '文生视频',
    'image': '图生视频',
    'first_last_frame': '首尾帧',
    'first_frame_audio': '首帧有声',
    'omni_reference': '全模态参考'
  }
  return modeNames[mode] || '视频'
}

function getModeFromRecord(record) {
  try {
    if (record.detail && record.detail.request_body) {
      const data = JSON.parse(record.detail.request_body)
      if (data.content) {
        const hasReferenceVideo = data.content.some(c => c.role === 'reference_video')
        if (hasReferenceVideo) return '全模态参考'

        const hasFirstFrame = data.content.some(c => c.role === 'first_frame')
        const hasLastFrame = data.content.some(c => c.role === 'last_frame')
        if (hasFirstFrame && hasLastFrame) return '首尾帧'
        if (hasFirstFrame && data.generate_audio) return '首帧有声'

        const hasImage = data.content.some(c => c.type === 'image_url')
        if (hasImage) return '图生视频'

        return '文生视频'
      }
    }
  } catch {
  }
  return '视频'
}

async function loadHistory() {
  try {
    const response = await api.get('/api-test/history', {
      params: {
        per_page: 20,
        page: 1,
        type: 'video'
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
    case 'first_last_frame':
      promptValue = firstLastPrompt.value
      break
    case 'first_frame_audio':
      promptValue = firstAudioPrompt.value
      break
    case 'omni_reference':
      promptValue = omniPrompt.value
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

    const payload = assemblePayload(false) // false 表示不是预览，用于实际发送请求

    const response = await fetch(`${config.api.openai.base}/videos/generations/tasks`, {
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
        const hasVideos = data.data.some(vid => vid.url || vid.video_url || vid.data_url)
        if (hasVideos) {
          generatedVideos.value = data.data.map(vid => ({
            ...vid,
            url: vid.url || vid.video_url || vid.data_url || `data:video/mp4;base64,${vid.b64_json}`
          }))
          Message.success(`成功生成 ${data.data.length} 个视频`)
        } else {
          generatedVideos.value = []
          Message.success('视频生成任务已创建，请点击查询任务获取结果')
        }
      } else if (data.id || data.task_id) {
        generatedVideos.value = []
        Message.success('视频生成任务已创建，请点击查询任务获取结果')
      } else {
        generatedVideos.value = []
        Message.error('未获取到生成结果')
      }
    } else {
      generatedVideos.value = []
      Message.error(data.error?.message || '视频生成失败')
    }

    let tempTaskId = null
    try {
      const parsedData = JSON.parse(responseBody.value)
      tempTaskId = parsedData.id || parsedData.task_id
    } catch {
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

    historyRecords.value.unshift({
      id: null,
      time: new Date().toISOString(),
      model: formData.value.model,
      mode: getModeName(currentMode.value),
      status: responseStatus.value,
      external_task_id: null,
      response_body: responseBody.value
    })

    if (historyRecords.value.length > 50) {
      historyRecords.value.pop()
    }
  } finally {
    loading.value = false
  }
}

async function downloadVideo(video) {
  try {
    let videoUrl = video.url
    if (video.b64_json && !video.url) {
      videoUrl = `data:video/mp4;base64,${video.b64_json}`
    }

    if (videoUrl.startsWith('data:')) {
      const link = document.createElement('a')
      link.href = videoUrl
      link.download = `generated-video-${Date.now()}.mp4`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
    } else {
      const response = await fetch(videoUrl)
      const blob = await response.blob()
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `generated-video-${Date.now()}.mp4`
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

async function queryVideoTask(record) {
  try {
    let taskId = record.external_task_id
    if (!taskId && record.response_body) {
      try {
        const data = JSON.parse(record.response_body)
        taskId = data.id || data.task_id
      } catch {
      }
    }

    if (!taskId) {
      Message.warning('无法获取任务ID')
      return
    }

    queryingTask.value = true
    currentQueryingRecord.value = record

    const response = await fetch(`${config.api.openai.base}/videos/generations/tasks/${taskId}?record_id=${record.id}`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${selectedApiKey.value}`
      }
    })

    const taskData = await response.json()
    responseBody.value = JSON.stringify(taskData, null, 2)
    Message.success('任务状态查询成功')

    if (taskData.status === 'succeeded' || taskData.state === 'succeeded') {
      if (taskData.output || taskData.data) {
        const output = taskData.output || taskData.data
        if (Array.isArray(output) && output.length > 0) {
          generatedVideos.value = output.map(item => ({
            url: item.url || item.video_url || item.data_url || (item.b64_json ? `data:video/mp4;base64,${item.b64_json}` : ''),
            ...item
          }))
        }
      } else if (taskData.url || taskData.video_url || taskData.data_url || taskData.b64_json) {
        generatedVideos.value = [{
          url: taskData.url || taskData.video_url || taskData.data_url || `data:video/mp4;base64,${taskData.b64_json}`,
          ...taskData
        }]
      }
    }

    if (record.id) {
      try {
        await api.put(`/api-test/${record.id}`, {
          status: response.status,
          response_body: JSON.stringify(taskData, null, 2)
        })
      } catch {
      }
    }
  } catch (error) {
    Message.error('查询任务状态失败')
  } finally {
    queryingTask.value = false
    currentQueryingRecord.value = null
  }
}
</script>

<style scoped>
.video-test-container {
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

.upload-video-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 16px;
  border: 1px dashed var(--border-color);
  border-radius: 4px;
}

.video-name-display {
  padding: 12px;
  background: var(--color-fill-2);
  border-radius: 4px;
  font-size: 12px;
  word-break: break-all;
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

.videos-card {
  margin-bottom: 16px;
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.videos-card :deep(.arco-card-header) {
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.video-item {
  border-radius: 8px;
  overflow: hidden;
  margin-bottom: 16px;
}

.video-item :deep(.arco-card-body) {
  padding: 0;
}

.generated-video {
  width: 100%;
  height: auto;
}

.video-actions {
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
