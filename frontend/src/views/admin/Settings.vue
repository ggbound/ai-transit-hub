<template>
  <div class="settings-container">
    <div class="page-header">
      <h1>系统设置</h1>
      <p>配置系统的各项参数和功能</p>
    </div>
    
    <a-card class="content-card" style="margin-top: 16px;">
      <a-tabs v-model:active-tab="activeTab">
        <a-tab-pane key="general" title="基础配置">
          <a-form :model="generalForm" layout="vertical">
            <a-form-item label="系统名称">
              <a-input v-model="generalForm.systemName" />
            </a-form-item>
            <a-form-item label="系统Logo">
              <a-upload
                action="/upload"
                list-type="picture-card"
                :file-list="logoFile"
                :limit="1"
              >
                <template #upload-button>
                  <div class="upload-icon">
                    <icon-plus />
                    <div style="margin-top: 8px;">上传Logo</div>
                  </div>
                </template>
              </a-upload>
            </a-form-item>
            <a-form-item label="系统描述">
              <a-textarea v-model="generalForm.description" :rows="3" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveGeneral">保存配置</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>

        <a-tab-pane key="security" title="安全配置">
          <a-form :model="securityForm" layout="vertical">
            <a-form-item label="登录失败次数限制">
              <a-input-number v-model="securityForm.loginAttempts" :min="3" :max="10" />
            </a-form-item>
            <a-form-item label="Token有效期（小时）">
              <a-input-number v-model="securityForm.tokenExpiry" :min="1" :max="720" />
            </a-form-item>
            <a-form-item label="强制MFA认证">
              <a-switch v-model="securityForm.requireMFA" />
            </a-form-item>
            <a-form-item label="IP白名单">
              <a-textarea v-model="securityForm.ipWhitelist" :rows="3" placeholder="每行一个IP地址" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveSecurity">保存配置</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>

        <a-tab-pane key="rate" title="限流配置">
          <a-form :model="rateForm" layout="vertical">
            <a-form-item label="默认每分钟请求数">
              <a-input-number v-model="rateForm.requestsPerMinute" :min="10" :max="10000" />
            </a-form-item>
            <a-form-item label="默认每分钟Token数">
              <a-input-number v-model="rateForm.tokensPerMinute" :min="1000" :max="1000000" />
            </a-form-item>
            <a-form-item label="并发请求数">
              <a-input-number v-model="rateForm.concurrentRequests" :min="1" :max="100" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveRate">保存配置</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>

        <a-tab-pane key="notification" title="通知配置">
          <a-form :model="notificationForm" layout="vertical">
            <a-form-item label="系统通知邮箱">
              <a-input v-model="notificationForm.email" />
            </a-form-item>
            <a-form-item label="钉钉Webhook">
              <a-input v-model="notificationForm.dingtalkWebhook" />
            </a-form-item>
            <a-form-item label="企业微信Webhook">
              <a-input v-model="notificationForm.wechatWebhook" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveNotification">保存配置</a-button>
              <a-button style="margin-left: 8px;" @click="testNotification">发送测试通知</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>

        <a-tab-pane key="log" title="日志配置">
          <a-form :model="logForm" layout="vertical">
            <a-form-item label="日志保留天数">
              <a-input-number v-model="logForm.retentionDays" :min="7" :max="365" />
            </a-form-item>
            <a-form-item label="日志级别">
              <a-select v-model="logForm.level">
                <a-option value="debug">Debug</a-option>
                <a-option value="info">Info</a-option>
                <a-option value="warning">Warning</a-option>
                <a-option value="error">Error</a-option>
              </a-select>
            </a-form-item>
            <a-form-item label="记录敏感操作">
              <a-switch v-model="logForm.logSensitive" />
            </a-form-item>
            <a-form-item>
              <a-button type="primary" @click="saveLog">保存配置</a-button>
            </a-form-item>
          </a-form>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconPlus } from '@arco-design/web-vue/es/icon'

const activeTab = ref('general')

const generalForm = ref({
  systemName: 'AI Transit Hub',
  description: '企业级AI模型中转站',
  logo: null
})

const securityForm = ref({
  loginAttempts: 5,
  tokenExpiry: 24,
  requireMFA: false,
  ipWhitelist: ''
})

const rateForm = ref({
  requestsPerMinute: 60,
  tokensPerMinute: 100000,
  concurrentRequests: 10
})

const notificationForm = ref({
  email: 'admin@example.com',
  dingtalkWebhook: '',
  wechatWebhook: ''
})

const logForm = ref({
  retentionDays: 30,
  level: 'info',
  logSensitive: true
})

const logoFile = ref([])

async function saveGeneral() {
  Message.success('基础配置保存成功')
}

async function saveSecurity() {
  Message.success('安全配置保存成功')
}

async function saveRate() {
  Message.success('限流配置保存成功')
}

async function saveNotification() {
  Message.success('通知配置保存成功')
}

async function testNotification() {
  Message.success('测试通知已发送')
}

async function saveLog() {
  Message.success('日志配置保存成功')
}
</script>

<style scoped>
.settings-container {
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

.upload-icon {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: #86909c;
}
</style>
