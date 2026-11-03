<template>
  <div class="dashboard-container">
    <div class="page-header">
      <h1>仪表盘</h1>
      <p>查看您的使用统计和快速入口</p>
    </div>

    <!-- 今日统计 -->
    <a-row :gutter="16" class="stat-row">
      <a-col :xs="24" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card">
          <div class="stat-icon blue-icon">
            <icon-phone />
          </div>
          <div class="stat-content">
            <div class="stat-title">今日调用次数</div>
            <div class="stat-value">{{ todayCalls }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card">
          <div class="stat-icon green-icon">
            <icon-fire />
          </div>
          <div class="stat-content">
            <div class="stat-title">今日Token消耗</div>
            <div class="stat-value">{{ formatNumber(todayTokens) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card">
          <div class="stat-icon orange-icon">
            <icon-apps />
          </div>
          <div class="stat-content">
            <div class="stat-title">本月调用次数</div>
            <div class="stat-value">{{ formatNumber(monthCalls) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="6" :lg="6" :xl="6">
        <div class="stat-card">
          <div class="stat-icon purple-icon">
            <icon-code />
          </div>
          <div class="stat-content">
            <div class="stat-title">本月Token消耗</div>
            <div class="stat-value">{{ formatNumber(monthTokens) }}</div>
          </div>
        </div>
      </a-col>
    </a-row>

    <!-- 总计统计 - 3列布局，居中显示 -->
    <a-row :gutter="16" class="stat-row" justify="center">
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon cyan-icon">
            <icon-check-circle />
          </div>
          <div class="stat-content">
            <div class="stat-title">总调用次数</div>
            <div class="stat-value">{{ formatNumber(totalCalls) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon yellow-icon">
            <icon-swap />
          </div>
          <div class="stat-content">
            <div class="stat-title">总Token消耗</div>
            <div class="stat-value">{{ formatNumber(totalTokens) }}</div>
          </div>
        </div>
      </a-col>
      <!-- 会话数量统计已隐藏 -->
      <a-col v-if="false" :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon red-icon">
            <icon-message />
          </div>
          <div class="stat-content">
            <div class="stat-title">会话数量</div>
            <div class="stat-value">{{ totalSessions }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon pink-icon">
            <icon-user />
          </div>
          <div class="stat-content">
            <div class="stat-title">API密钥</div>
            <div class="stat-value">{{ apiKeyCount }}</div>
          </div>
        </div>
      </a-col>
    </a-row>

    <a-row :gutter="16" class="content-row">
      <a-col :xs="24" :sm="24" :md="24" :lg="24" :xl="24">
        <a-card class="content-card">
          <template #title>快速开始</template>
          <a-grid :cols="4" :col-gap="16" :row-gap="16">
            <a-grid-item>
              <a-card class="quick-link-card" hoverable @click="goTo('/api-keys')">
                <div class="quick-link-content">
                  <icon-lock class="quick-icon" />
                  <div class="quick-title">管理API密钥</div>
                  <div class="quick-desc">创建自己的API密钥</div>
                </div>
              </a-card>
            </a-grid-item>
            <a-grid-item>
              <a-card class="quick-link-card" hoverable @click="goTo('/test')">
                <div class="quick-link-content">
                  <icon-code class="quick-icon" />
                  <div class="quick-title">接口测试</div>
                  <div class="quick-desc">在线测试API接口</div>
                </div>
              </a-card>
            </a-grid-item>
            <a-grid-item>
              <a-card class="quick-link-card" hoverable @click="goTo('/docs')">
                <div class="quick-link-content">
                  <icon-book class="quick-icon" />
                  <div class="quick-title">API文档</div>
                  <div class="quick-desc">查看API文档</div>
                </div>
              </a-card>
            </a-grid-item>
            <a-grid-item>
              <a-card class="quick-link-card" hoverable @click="goTo('/logs')">
                <div class="quick-link-content">
                  <icon-history class="quick-icon" />
                  <div class="quick-title">使用日志</div>
                  <div class="quick-desc">查看使用日志</div>
                </div>
              </a-card>
            </a-grid-item>
          </a-grid>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" class="content-row" style="margin-top: 16px">
      <a-col :xs="24" :sm="24" :md="24" :lg="24" :xl="24">
        <a-card class="content-card">
          <template #title>可用模型</template>
          <div class="model-list">
            <a-grid :cols="2" :col-gap="16" :row-gap="16">
              <a-grid-item v-for="model in models" :key="model.id">
                <a-card hoverable class="model-card" size="small">
                  <div class="model-card-content">
                    <div class="model-display-name">
                      <a-tag :color="getModelColor(model.provider?.code)" size="small">{{ model.provider?.name }}</a-tag>
                      <span class="model-name-text">{{ model.name }}</span>
                      <a-tag :color="getModelTypeColor(model.type)" size="small">{{ getModelTypeName(model.type) }}</a-tag>
                    </div>
                    <div class="model-api-id">
                      <icon-code />
                      <span>{{ model.full_model_id }}</span>
                    </div>
                    <div class="model-tags capability-tags">
                      <!-- 能力标签单独一行 -->
                      <a-tag v-if="model.capabilities?.includes('chat')" color="blue" size="small">对话</a-tag>
                      <a-tag v-if="model.capabilities?.includes('vision')" color="green" size="small">视觉</a-tag>
                      <a-tag v-if="model.capabilities?.includes('function_calling')" color="orange" size="small">函数调用</a-tag>
                      <a-tag v-if="model.capabilities?.includes('code')" color="purple" size="small">代码</a-tag>
                    </div>
                    <div class="model-tags interface-tags">
                      <!-- 接口支持标签单独一行 -->
                      <a-tag v-if="model.supports_openai" color="arcoblue" size="small">OpenAI</a-tag>
                      <a-tag v-if="model.supports_anthropic" color="purple" size="small">Anthropic</a-tag>
                      <a-tag v-if="model.supports_response" color="green" size="small">Response</a-tag>
                      <a-tag v-if="model.supports_messages" color="orange" size="small">Messages</a-tag>
                      <a-tag v-if="model.supports_embeddings" color="cyan" size="small">向量</a-tag>
                      <a-tag v-if="model.supports_images" color="teal" size="small">图片</a-tag>
                      <a-tag v-if="model.supports_videos" color="red" size="small">视频</a-tag>
                      <a-tag v-if="model.supports_audio" color="pink" size="small">音频</a-tag>
                    </div>
                    <div v-if="Number(model.input_price || 0) > 0 || Number(model.output_price || 0) > 0" class="model-tags model-price">
                      <icon-fire size="12" />
                      <span>输入 ¥{{ Number(model.input_price || 0).toFixed(4) }} · 输出 ¥{{ Number(model.output_price || 0).toFixed(4) }}（¥/百万tokens）</span>
                    </div>
                  </div>
                </a-card>
              </a-grid-item>
            </a-grid>
          </div>
        </a-card>
      </a-col>
    </a-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  IconMessage,
  IconLock,
  IconCode,
  IconBook,
  IconHistory,
  IconTrophy,
  IconPhone,
  IconFire,
  IconApps,
  IconCheckCircle,
  IconSwap,
  IconUser
} from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'

const router = useRouter()

const todayCalls = ref(0)
const todayTokens = ref(0)
const monthCalls = ref(0)
const monthTokens = ref(0)
const totalCalls = ref(0)
const totalTokens = ref(0)
const totalSessions = ref(0)
const apiKeyCount = ref(0)
const models = ref([])

const colorMap = {
  openai: 'arcoblue',
  anthropic: 'purple',
  'aliyun-chat': 'green',
  'deepseek': 'red',
  'baidu-qianfan': 'orange',
  'tencent': 'cyan',
  'huawei-pangu': 'gray'
}

const modelTypeColorMap = {
  text: 'arcoblue',
  embedding: 'cyan',
  image: 'teal',
  video: 'red',
  audio: 'pink'
}

const modelTypeNameMap = {
  text: '文本',
  embedding: '向量',
  image: '图片',
  video: '视频',
  audio: '音频'
}

function formatNumber(num) {
  if (!num) return 0
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  }
  if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  }
  return num.toString()
}

function getModelColor(providerCode) {
  return colorMap[providerCode] || 'arcoblue'
}

function getModelTypeColor(modelType) {
  return modelTypeColorMap[modelType] || 'arcoblue'
}

function getModelTypeName(modelType) {
  return modelTypeNameMap[modelType] || modelType
}

function goTo(path) {
  router.push(path)
}

onMounted(async () => {
  try {
    const [modelsRes, logsRes, keysRes] = await Promise.all([
      api.get('/models'),
      api.get('/user/logs/stats'),
      api.get('/user/api-keys')
    ])

    if (modelsRes.data.success) {
      // 显示所有类型的模型，只有文本类型的默认支持OpenAI
      models.value = (modelsRes.data.data.models || []).map(model => ({
        ...model,
        // 只有文本类型的模型默认支持OpenAI
        supports_openai: model.type === 'text',
      }))
    }

    if (logsRes.data.success) {
      const stats = logsRes.data.data
      todayCalls.value = stats.todayCalls || 0
      todayTokens.value = stats.todayTokens || 0
      monthCalls.value = stats.monthCalls || 0
      monthTokens.value = stats.monthTokens || 0
      totalCalls.value = stats.totalCalls || 0
      totalTokens.value = stats.totalTokens || 0
      totalSessions.value = stats.totalSessions || 0
    }

    if (keysRes.data.success) {
      const keysData = keysRes.data.data.data || keysRes.data.data || []
      apiKeyCount.value = keysData.length
    }
  } catch (error) {
    console.error(error)
  }
})
</script>

<style scoped>
.dashboard-container {
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

.stat-row {
  margin-bottom: 16px;
}

.stat-card {
  background: #ffffff;
  border-radius: 8px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
  transition: all 0.3s;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #ffffff;
}

.blue-icon {
  background: linear-gradient(135deg, #165dff 0%, #4080ff 100%);
}

.green-icon {
  background: linear-gradient(135deg, #00b42a 0%, #23c343 100%);
}

.orange-icon {
  background: linear-gradient(135deg, #ff7d00 0%, #ff9a2e 100%);
}

.purple-icon {
  background: linear-gradient(135deg, #722ed1 0%, #9254de 100%);
}

.cyan-icon {
  background: linear-gradient(135deg, #0fc6c2 0%, #34c9c6 100%);
}

.yellow-icon {
  background: linear-gradient(135deg, #ff7d00 0%, #ffab00 100%);
}

.red-icon {
  background: linear-gradient(135deg, #f53f3f 0%, #f76560 100%);
}

.pink-icon {
  background: linear-gradient(135deg, #f5319d 0%, #f772ac 100%);
}

.stat-content {
  flex: 1;
}

.stat-title {
  font-size: 14px;
  color: var(--text-tertiary);
  margin-bottom: 8px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-primary);
}

.content-row {
  margin-top: 16px;
}

.content-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.quick-link-card {
  height: 100%;
  text-align: center;
  border-radius: 8px;
  cursor: pointer;
}

.quick-link-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0;
}

.quick-icon {
  font-size: 36px;
  color: var(--primary-color);
  margin-bottom: 12px;
}

.quick-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.quick-desc {
  font-size: 13px;
  color: var(--text-tertiary);
}

.model-list {
  display: block;
}

.model-card {
  border-radius: 8px;
  cursor: pointer;
}

.model-card-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.model-display-name {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.model-name-text {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.model-api-id {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  font-family: 'SF Mono', 'Consolas', 'Monaco', monospace;
  background: var(--color-fill-1);
  padding: 4px 8px;
  border-radius: 4px;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.model-info {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.model-price {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  color: #f77234;
  font-size: 12px;
}
.model-price span {
  font-size: 12px;
  word-break: break-all;
}

.quick-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 0;
  cursor: pointer;
  transition: all 0.2s;
}

.quick-item:hover {
  color: var(--primary-color);
}
</style>
