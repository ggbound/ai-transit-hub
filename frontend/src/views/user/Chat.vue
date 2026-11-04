<template>
  <div class="chat-layout">
    <!-- 侧边栏 -->
    <aside class="sidebar" :class="{ collapsed: sidebarCollapsed }">
      <div class="sidebar-header">
        <a-button type="primary" block @click="createNewSession" class="new-chat-btn">
          <template #icon><icon-plus /></template>
          新建对话
        </a-button>
      </div>

      <div class="session-list">
        <div
          v-for="session in sessions"
          :key="session.id"
          :class="['session-item', { active: currentSessionId === session.id }]"
          @click="selectSession(session.id)"
        >
          <div class="session-content">
            <icon-message class="session-icon" />
            <span class="session-title">{{ session.title || '新对话' }}</span>
          </div>
          <a-dropdown position="br" @click.stop>
            <a-button type="text" size="mini" class="session-menu-btn">
              <icon-more />
            </a-button>
            <template #content>
              <a-doption @click="renameSession(session)">
                <template #icon><icon-edit /></template>
                重命名
              </a-doption>
              <a-doption @click="deleteSession(session)" style="color: #f53f3f;">
                <template #icon><icon-delete /></template>
                删除
              </a-doption>
            </template>
          </a-dropdown>
        </div>
        <div v-if="sessions.length === 0" class="empty-sessions">
          <icon-folder class="empty-icon" />
          <span>暂无对话</span>
        </div>
      </div>

      <div class="sidebar-footer">
        <a-button type="text" @click="toggleSidebar" class="collapse-btn">
          <template #icon><icon-left /></template>
          收起
        </a-button>
      </div>
    </aside>

    <!-- 主内容区 -->
    <main class="main-content">
      <!-- 顶部极简栏 - 仅保留侧边栏开关 -->
      <div class="top-bar">
        <a-button
          v-if="sidebarCollapsed"
          type="text"
          class="sidebar-toggle-btn"
          @click="toggleSidebar"
          title="展开侧边栏"
        >
          <template #icon><icon-right /></template>
        </a-button>
        <div class="top-bar-spacer"></div>
        <div class="top-bar-actions">
          <a-tooltip content="清空对话" v-if="currentSessionId">
            <a-button type="text" size="small" @click="clearSession">
              <template #icon><icon-eraser /></template>
            </a-button>
          </a-tooltip>
        </div>
      </div>

      <!-- 消息区域 -->
      <div class="messages-container" ref="messagesContainer">
        <!-- 欢迎页 -->
        <div v-if="messages.length === 0 && !isLoading" class="welcome-area">
          <div class="welcome-icon-wrapper">
            <icon-robot class="welcome-icon" />
          </div>
          <h2 class="welcome-title">有什么可以帮您的？</h2>
          <p class="welcome-desc">选择下方的模型开始对话</p>
        </div>

        <!-- 消息列表 -->
        <div v-else class="messages-list">
          <div
            v-for="(message, index) in messages"
            :key="message.id"
            :class="['message-item', message.role]"
          >
            <div class="message-content-wrapper">
              <div class="message-avatar">
                <img v-if="message.role === 'assistant'" src="/ai-avatar.svg" alt="AI" />
                <a-avatar v-else :size="36" :style="{ backgroundColor: '#165dff' }">
                  <icon-user />
                </a-avatar>
              </div>
              <div class="message-body">
                <div class="message-header-row">
                  <span class="author">{{ message.role === 'user' ? '您' : 'AI助手' }}</span>
                  <span class="time">{{ formatTime(message.created_at) }}</span>
                </div>
                <div class="message-text" v-html="renderMessage(message.content)"></div>
                <div class="message-actions-row" v-if="message.role === 'assistant' && !isLoading">
                  <a-button type="text" size="mini" @click="copyMessage(message.content)">
                    <template #icon><icon-copy /></template>
                    复制
                  </a-button>
                  <a-button
                    v-if="index === messages.length - 1"
                    type="text"
                    size="mini"
                    @click="regenerateMessage(index)"
                  >
                    <template #icon><icon-refresh /></template>
                    重新生成
                  </a-button>
                </div>
              </div>
            </div>
          </div>

          <!-- 加载状态 -->
          <div v-if="isLoading && messages[messages.length - 1]?.role !== 'assistant'" class="message-item assistant loading">
            <div class="message-content-wrapper">
              <div class="message-avatar">
                <img src="/ai-avatar.svg" alt="AI" />
              </div>
              <div class="message-body">
                <div class="typing-indicator">
                  <span></span>
                  <span></span>
                  <span></span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 滚动到底部按钮 -->
        <a-button
          v-if="showScrollBtn"
          type="primary"
          shape="circle"
          class="scroll-to-bottom"
          @click="scrollToBottom"
        >
          <template #icon><icon-down /></template>
        </a-button>
      </div>

      <!-- 企业级输入区域 -->
      <div class="composer-area">
        <div class="composer-wrapper">
          <!-- 工具栏 -->
          <div class="composer-toolbar">
            <div class="toolbar-left">
              <!-- 模型选择 -->
              <div class="model-select-container">
                <icon-send class="model-icon" />
                <a-select
                  v-model="selectedModelId"
                  placeholder="选择模型"
                  class="model-select-pro"
                  size="small"
                >
                  <a-select-group
                    v-for="group in modelGroups"
                    :key="group.provider_code"
                    :label="group.provider_name"
                  >
                    <a-option
                      v-for="model in group.models"
                      :key="model.id"
                      :value="model.id"
                    >
                      {{ model.name }} ({{ model.full_model_id || model.provider?.code + '/' + model.code }})
                    </a-option>
                  </a-select-group>
                </a-select>
              </div>
              
              <!-- 流式开关 -->
              <div class="stream-toggle">
                <a-tooltip :content="streamEnabled ? '已启用流式输出' : '已禁用流式输出'">
                  <a-switch
                    v-model="streamEnabled"
                    size="small"
                    checked-children="流式"
                    un-checked-children="普通"
                  />
                </a-tooltip>
              </div>
            </div>

            <div class="toolbar-right">
              <span class="composer-hint">Enter 发送 · Shift + Enter 换行</span>
            </div>
          </div>

          <!-- 输入框区域 -->
          <div class="composer-input-area">
            <a-textarea
              v-model="inputMessage"
              :placeholder="isLoading ? 'AI正在思考中...' : '输入您的消息...'"
              :auto-size="{ minRows: 2, maxRows: 8 }"
              :disabled="isLoading"
              class="composer-input"
              @keydown="handleKeydown"
            />
            <a-button
              type="primary"
              shape="round"
              :loading="isLoading"
              :disabled="!inputMessage.trim()"
              @click="sendMessage"
              class="send-button-pro"
            >
              <template #icon>
                <icon-send v-if="!isLoading" />
                <icon-loading v-else />
              </template>
              <span v-if="!isLoading">发送</span>
            </a-button>
          </div>
        </div>
      </div>
    </main>

    <!-- 重命名抽屉 -->
    <a-drawer
      v-model:visible="renameModalVisible"
      title="重命名对话"
      :width="420"
      unmount-on-close
      @ok="confirmRename"
      @cancel="renameModalVisible = false"
    >
      <a-input
        v-model="renameForm.title"
        placeholder="请输入对话名称"
        size="large"
        allow-clear
      />
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Message, Modal } from '@arco-design/web-vue'
import { marked } from 'marked'
import hljs from 'highlight.js'
import api from '../../utils/api'
import config from '@/config/api.config.js'
import {
  IconPlus, IconUser, IconRobot, IconMore, IconEdit, IconDelete,
  IconMessage, IconFile, IconLeft, IconRight, IconEraser,
  IconCopy, IconRefresh, IconDown, IconSend, IconLoading, IconFolder
} from '@arco-design/web-vue/es/icon';

const router = useRouter()
const route = useRoute()

const sessions = ref([])
const messages = ref([])
const models = ref([])
const modelGroups = ref([])
const selectedModelId = ref(null)
const currentSessionId = ref(null)
const inputMessage = ref('')
const isLoading = ref(false)
const streamEnabled = ref(true)
const messagesContainer = ref(null)
const sidebarCollapsed = ref(false)
const showScrollBtn = ref(false)

const renameModalVisible = ref(false)
const renameForm = ref({ id: null, title: '' })

marked.setOptions({
  highlight: function(code, lang) {
    const language = hljs.getLanguage(lang) ? lang : 'plaintext'
    return hljs.highlight(code, { language }).value
  },
  langPrefix: 'hljs language-',
  breaks: true,
  gfm: true
})

onMounted(() => {
  loadSessions()
  loadModels()
  if (route.params.sessionId) {
    selectSession(route.params.sessionId)
  }
  if (messagesContainer.value) {
    messagesContainer.value.addEventListener('scroll', handleScroll)
  }
})

function renderMessage(content) {
  if (!content) return ''
  return marked(content)
}

function formatTime(time) {
  if (!time) return ''
  const date = new Date(time)
  const now = new Date()
  const diff = now - date
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`
  return date.toLocaleDateString('zh-CN')
}

async function loadSessions() {
  try {
    const response = await api.get('/sessions')
    if (response.data.success) {
      sessions.value = response.data.data.data || response.data.data
    }
  } catch (error) {
    console.error(error)
  }
}

async function loadModels() {
  try {
    const response = await api.get('/models')
    if (response.data.success) {
      const data = response.data.data
      // 只筛选文本类模型
      const textModels = (data.models || []).filter(m => m.type === 'text')
      models.value = textModels
      
      // 重新构建分组
      if (data.grouped && data.grouped.length) {
        modelGroups.value = data.grouped
          .map(group => ({
            ...group,
            models: group.models.filter(m => m.type === 'text')
          }))
          .filter(group => group.models.length > 0)
      }
      
      if (models.value.length && !selectedModelId.value) {
        selectedModelId.value = models.value[0].id
      }
    }
  } catch (error) {
    console.error(error)
  }
}

async function selectSession(id) {
  currentSessionId.value = id
  try {
    const response = await api.get(`/sessions/${id}`)
    if (response.data.success) {
      const session = response.data.data
      messages.value = session.messages || []
      selectedModelId.value = session.model_id
      streamEnabled.value = session.stream_enabled
      await nextTick()
      scrollToBottom()
    }
  } catch (error) {
    console.error(error)
  }
}

function createNewSession() {
  currentSessionId.value = null
  messages.value = []
  inputMessage.value = ''
  router.push('/chat')
}

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

function renameSession(session) {
  renameForm.value = { id: session.id, title: session.title || '' }
  renameModalVisible.value = true
}

async function confirmRename() {
  if (!renameForm.value.title.trim()) {
    Message.warning('请输入对话名称')
    return
  }
  try {
    await api.put(`/sessions/${renameForm.value.id}`, {
      title: renameForm.value.title.trim()
    })
    Message.success('重命名成功')
    renameModalVisible.value = false
    loadSessions()
  } catch (error) {
    Message.error('重命名失败')
  }
}

function deleteSession(session) {
  Modal.confirm({
    title: '删除对话',
    content: `确定要删除 "${session.title || '新对话'}" 吗？对话历史将被永久删除。`,
    okText: '删除',
    okButtonProps: { status: 'danger' },
    cancelText: '取消',
    onOk: async () => {
      try {
        await api.delete(`/sessions/${session.id}`)
        Message.success('删除成功')
        if (session.id === currentSessionId.value) {
          currentSessionId.value = null
          messages.value = []
          router.push('/chat')
        }
        loadSessions()
      } catch (error) {
        Message.error('删除失败')
      }
    }
  })
}

function clearSession() {
  if (!currentSessionId.value) return
  Modal.confirm({
    title: '清空对话',
    content: '确定要清空当前对话的所有消息吗？此操作不可恢复。',
    okText: '清空',
    okButtonProps: { status: 'danger' },
    cancelText: '取消',
    onOk: async () => {
      try {
        await api.delete(`/sessions/${currentSessionId.value}/messages`)
        messages.value = []
        Message.success('已清空')
      } catch (error) {
        Message.error('操作失败')
      }
    }
  })
}

function copyMessage(content) {
  navigator.clipboard.writeText(content).then(() => {
    Message.success('已复制到剪贴板')
  }).catch(() => {
    Message.error('复制失败')
  })
}

async function regenerateMessage(index) {
  let userMessageIndex = index - 1
  while (userMessageIndex >= 0 && messages.value[userMessageIndex].role !== 'user') {
    userMessageIndex--
  }
  if (userMessageIndex < 0) return

  const userMessage = messages.value[userMessageIndex].content
  messages.value = messages.value.slice(0, userMessageIndex + 1)
  await sendMessageContent(userMessage)
}

function handleScroll() {
  if (!messagesContainer.value) return
  const { scrollTop, scrollHeight, clientHeight } = messagesContainer.value
  showScrollBtn.value = scrollHeight - scrollTop - clientHeight > 100
}

function scrollToBottom() {
  if (messagesContainer.value) {
    messagesContainer.value.scrollTo({
      top: messagesContainer.value.scrollHeight,
      behavior: 'smooth'
    })
  }
}

async function sendMessage() {
  const content = inputMessage.value.trim()
  if (!content || isLoading.value) return
  inputMessage.value = ''
  await sendMessageContent(content)
}

async function sendMessageContent(content) {
  isLoading.value = true
  messages.value.push({
    id: Date.now(),
    role: 'user',
    content: content,
    created_at: new Date().toISOString()
  })
  await nextTick()
  scrollToBottom()

  try {
    if (streamEnabled.value) {
      await sendStreamMessage(content)
    } else {
      await sendNormalMessage(content)
    }
    await loadSessions()
  } catch (error) {
    Message.error('发送失败，请重试')
    console.error(error)
  } finally {
    isLoading.value = false
  }
}

async function sendStreamMessage(message) {
  const response = await fetch(config.api.chatSend, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${localStorage.getItem('token')}`
    },
    body: JSON.stringify({
      session_id: currentSessionId.value,
      model_id: selectedModelId.value,
      messages: [{ role: 'user', content: message }],
      stream: true
    })
  })

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  const messageIndex = messages.value.length

  messages.value.push({
    id: Date.now() + 1,
    role: 'assistant',
    content: '',
    created_at: new Date().toISOString()
  })

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    const chunk = decoder.decode(value)
    const lines = chunk.split('\n').filter(line => line.trim())

    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          const data = JSON.parse(line.slice(6))
          if (data.content) {
            messages.value[messageIndex] = {
              ...messages.value[messageIndex],
              content: messages.value[messageIndex].content + data.content
            }
            await nextTick()
            scrollToBottom()
          }
        } catch (e) {
          // ignore
        }
      }
    }
  }
}

async function sendNormalMessage(message) {
  const response = await api.post('/chat/send', {
    session_id: currentSessionId.value,
    model_id: selectedModelId.value,
    messages: [{ role: 'user', content: message }],
    stream: false
  })

  if (response.data.success) {
    const session = response.data.data.session
    const assistantMessage = response.data.data.message
    currentSessionId.value = session.id
    messages.value.push({
      ...assistantMessage,
      created_at: new Date().toISOString()
    })
    await nextTick()
    scrollToBottom()
  }
}

function handleKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
}
</script>

<style scoped>
/* 基础布局 */
.chat-layout {
  display: flex;
  width: 100%;
  height: 100%;
  background: #f7f8fa;
}

/* 侧边栏 */
.sidebar {
  width: 260px;
  height: 100%;
  background: #ffffff;
  border-right: 1px solid #e5e6eb;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: width 0.3s ease;
}

.sidebar.collapsed {
  width: 0;
  overflow: hidden;
  border-right: none;
}

.sidebar-toggle-btn {
  margin-right: 12px;
  color: #86909c;
  transition: all 0.2s;
}

.sidebar-toggle-btn:hover {
  color: #165dff;
  background: #f2f3f5;
}

.sidebar-header {
  height: 44px;
  padding: 0 16px;
  border-bottom: 1px solid #e5e6eb;
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

.new-chat-btn {
  font-weight: 500;
  height: 32px;
  border-radius: 8px;
  padding: 0 12px;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.new-chat-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(22, 93, 255, 0.25);
}

.new-chat-btn .arco-btn-icon {
  font-size: 16px;
}

.session-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.session-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 4px;
  transition: all 0.2s;
}

.session-item:hover {
  background-color: #f7f8fa;
}

.session-item.active {
  background-color: #e8f3ff;
}

.session-content {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  overflow: hidden;
}

.session-icon {
  font-size: 16px;
  color: #86909c;
  flex-shrink: 0;
}

.session-title {
  font-size: 14px;
  font-weight: 500;
  color: #1d2129;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-menu-btn {
  opacity: 0;
  transition: opacity 0.2s;
  padding: 4px;
  color: #86909c;
  flex-shrink: 0;
}

.session-item:hover .session-menu-btn {
  opacity: 1;
}

.empty-sessions {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
  color: #86909c;
  gap: 8px;
}

.empty-icon {
  font-size: 32px;
}

.sidebar-footer {
  padding: 12px 16px;
  border-top: 1px solid #e5e6eb;
  display: flex;
  justify-content: flex-end;
  flex-shrink: 0;
}

.collapse-btn {
  padding: 4px 8px;
}

/* 主内容区 */
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  min-width: 0;
}

/* 顶部极简栏 */
.top-bar {
  height: 44px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #ffffff;
  border-bottom: 1px solid #e5e6eb;
  flex-shrink: 0;
}

.top-bar-spacer {
  flex: 1;
}

.top-bar-actions {
  display: flex;
  gap: 4px;
}

.top-bar-actions .arco-btn {
  color: #86909c;
}

.top-bar-actions .arco-btn:hover {
  color: #1d2129;
  background: #f7f8fa;
}

/* 消息容器 */
.messages-container {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  position: relative;
  min-height: 0;
  background: #f7f8fa;
}

/* 欢迎页 */
.welcome-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 40px 32px;
  text-align: center;
}

.welcome-icon-wrapper {
  width: 80px;
  height: 80px;
  border-radius: 20px;
  background: linear-gradient(135deg, #165dff 0%, #722ed1 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
  box-shadow: 0 10px 30px rgba(22, 93, 255, 0.25);
}

.welcome-icon {
  font-size: 40px;
  color: #ffffff;
}

.welcome-title {
  font-size: 28px;
  font-weight: 700;
  color: #1d2129;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.welcome-desc {
  font-size: 14px;
  color: #86909c;
  margin: 0;
}

/* 消息列表 */
.messages-list {
  padding: 24px 0;
}

.message-item {
  padding: 20px 24px;
}

.message-item.user {
  background: #ffffff;
}

.message-item.user .message-content-wrapper {
  flex-direction: row-reverse;
}

.message-item.user .message-body {
  align-items: flex-end;
}

.message-item.user .message-header-row {
  flex-direction: row-reverse;
}

.message-item.user .message-text {
  text-align: right;
}

.message-item.assistant {
  background: transparent;
}

.message-content-wrapper {
  max-width: 1000px;
  margin: 0 auto;
  display: flex;
  gap: 16px;
  padding: 0;
}

.message-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.message-avatar {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
}

.message-avatar img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

.message-header-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.author {
  font-weight: 600;
  font-size: 14px;
  color: #1d2129;
}

.time {
  font-size: 12px;
  color: #86909c;
}

.message-text {
  font-size: 15px;
  line-height: 1.7;
  color: #1d2129;
  word-break: break-word;
}

.message-text :deep(p) {
  margin: 0 0 14px 0;
}

.message-text :deep(p:last-child) {
  margin-bottom: 0;
}

.message-text :deep(pre) {
  background: #1e1e1e;
  border-radius: 10px;
  padding: 16px;
  overflow-x: auto;
  margin: 16px 0;
}

.message-text :deep(code) {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
}

.message-text :deep(pre code) {
  color: #d4d4d4;
}

.message-text :deep(p code) {
  background: #f2f3f5;
  padding: 3px 8px;
  border-radius: 6px;
  color: #165dff;
  font-size: 13px;
}

.message-text :deep(ul),
.message-text :deep(ol) {
  margin: 14px 0;
  padding-left: 24px;
}

.message-text :deep(li) {
  margin: 6px 0;
}

.message-text :deep(a) {
  color: #165dff;
  text-decoration: none;
  border-bottom: 1px solid rgba(22, 93, 255, 0.2);
}

.message-text :deep(a:hover) {
  border-bottom-color: #165dff;
}

.message-text :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0;
  border-radius: 8px;
  overflow: hidden;
}

.message-text :deep(th),
.message-text :deep(td) {
  border: 1px solid #e5e6eb;
  padding: 10px 14px;
  text-align: left;
}

.message-text :deep(th) {
  background: #f7f8fa;
  font-weight: 600;
  color: #1d2129;
}

.message-text :deep(blockquote) {
  border-left: 4px solid #165dff;
  margin: 16px 0;
  padding-left: 16px;
  color: #4e5969;
  background: #f7f8fa;
  padding: 12px 16px;
  border-radius: 0 8px 8px 0;
}

.message-text :deep(img) {
  max-width: 100%;
  border-radius: 10px;
  margin: 16px 0;
}

/* 消息操作按钮 */
.message-actions-row {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  opacity: 0;
  transition: opacity 0.2s;
}

.message-item:hover .message-actions-row {
  opacity: 1;
}

/* 加载动画 */
.typing-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
}

.typing-indicator span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #165dff;
  animation: typing 1.4s infinite ease-in-out both;
}

.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }

@keyframes typing {
  0%, 80%, 100% { transform: scale(0.6); opacity: 0.5; }
  40% { transform: scale(1); opacity: 1; }
}

/* 滚动按钮 */
.scroll-to-bottom {
  position: fixed;
  right: 32px;
  bottom: 200px;
  z-index: 10;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
}

/* 企业级输入区域 */
.composer-area {
  background: #ffffff;
  border-top: 1px solid #e5e6eb;
  padding: 16px 24px 20px;
  flex-shrink: 0;
}

.composer-wrapper {
  max-width: 1000px;
  margin: 0 auto;
  background: #f7f8fa;
  border-radius: 16px;
  border: 1px solid #e5e6eb;
  transition: all 0.3s;
  overflow: hidden;
}

.composer-wrapper:focus-within {
  border-color: #165dff;
  box-shadow: 0 0 0 4px rgba(22, 93, 255, 0.08);
  background: #ffffff;
}

/* 工具栏 */
.composer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: #ffffff;
  border-bottom: 1px solid #e5e6eb;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* 模型选择容器 */
.model-select-container {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: #f7f8fa;
  border-radius: 8px;
  transition: background 0.2s;
}

.model-select-container:hover {
  background: #e8f3ff;
}

.model-icon {
  color: #165dff;
  font-size: 16px;
}

.model-select-pro {
  font-weight: 500;
}

.model-select-pro :deep(.arco-select-view) {
  background: transparent;
  border: none;
  padding: 0;
  font-weight: 500;
  color: #1d2129;
}

.model-select-pro :deep(.arco-select-view:hover) {
  background: transparent;
}

/* 流式开关 */
.stream-toggle {
  display: flex;
  align-items: center;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.composer-hint {
  font-size: 12px;
  color: #86909c;
}

/* 输入框区域 */
.composer-input-area {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  padding: 16px;
  background: transparent;
}

.composer-input {
  flex: 1;
  border: none;
  background: transparent;
  font-size: 15px;
  line-height: 1.6;
  resize: none;
  min-height: 40px;
}

.composer-input :deep(.arco-textarea) {
  background: transparent;
  border: none;
  box-shadow: none;
  padding: 0;
}

.composer-input :deep(.arco-textarea:focus-within) {
  box-shadow: none;
}

/* 发送按钮 */
.send-button-pro {
  height: 40px;
  padding: 0 20px;
  font-weight: 500;
  border-radius: 10px;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
}

.send-button-pro:not(:disabled):hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 20px rgba(22, 93, 255, 0.35);
}

/* 响应式 */
@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 100;
    transform: translateX(-100%);
    transition: transform 0.3s ease;
    width: 260px;
  }

  .sidebar:not(.collapsed) {
    transform: translateX(0);
  }

  .sidebar.collapsed {
    transform: translateX(-100%);
  }

  .message-content-wrapper {
    padding: 0 16px;
    max-width: 100%;
  }

  .top-bar {
    padding: 0 16px;
  }

  .composer-area {
    padding: 12px 16px 16px;
  }

  .composer-toolbar {
    flex-direction: column;
    gap: 10px;
    align-items: stretch;
  }

  .toolbar-left {
    flex-wrap: wrap;
  }

  .toolbar-right {
    justify-content: flex-end;
  }

  .composer-input-area {
    flex-direction: column;
    align-items: stretch;
  }

  .send-button-pro {
    align-self: flex-end;
  }

  .scroll-to-bottom {
    bottom: 220px;
    right: 16px;
  }
}
</style>
