<template>
  <div class="login-container">
    <div class="login-wrapper">
      <!-- 左侧 - 品牌展示 -->
      <div class="login-left">
        <div class="brand-content">
          <div class="logo-section">
            <svg class="logo-icon" viewBox="0 0 80 80" fill="none">
              <rect width="80" height="80" rx="24" fill="url(#grad)" />
              <path d="M24 28L40 56L56 28" stroke="white" stroke-width="6" stroke-linecap="round" stroke-linejoin="round" />
              <circle cx="40" cy="36" r="6" fill="white" />
              <defs>
                <linearGradient id="grad" x1="0" y1="0" x2="80" y2="80">
                  <stop offset="0%" stop-color="#165DFF" />
                  <stop offset="100%" stop-color="#722ED1" />
                </linearGradient>
              </defs>
            </svg>
            <h1 class="brand-name">AI Transit Hub</h1>
            <p class="brand-slogan">统一管理多家AI模型，让AI触手可及</p>
          </div>
          <div class="feature-list">
            <div class="feature-item">
              <div class="feature-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
                  <circle cx="9" cy="7" r="4"></circle>
                  <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
                  <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
                </svg>
              </div>
              <div class="feature-text">
                <h4>多模型接入</h4>
                <p>支持OpenAI、Anthropic、豆包等主流模型</p>
              </div>
            </div>
            <div class="feature-item">
              <div class="feature-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
                </svg>
              </div>
              <div class="feature-text">
                <h4>统一接口</h4>
                <p>标准化API，实现零成本迁移</p>
              </div>
            </div>
            <div class="feature-item">
              <div class="feature-icon">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M12 2v20M2 12h20"></path>
                </svg>
              </div>
              <div class="feature-text">
                <h4>实时监控</h4>
                <p>Token使用、费用统计一目了然</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧 - 登录表单 -->
      <div class="login-right">
        <div class="login-form-wrapper">
          <div class="form-header">
            <h2 class="form-title">欢迎回来</h2>
            <p class="form-subtitle">请登录您的账号以继续使用</p>
          </div>

          <a-form :model="form" @submit="handleSubmit" class="login-form">
            <a-form-item field="email" label="邮箱">
              <a-input
                v-model="form.email"
                type="email"
                placeholder="请输入您的邮箱地址"
              >
                <template #prefix>
                  <svg class="prefix-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"></path>
                    <polyline points="22,6 12,13 2,6"></polyline>
                  </svg>
                </template>
              </a-input>
            </a-form-item>

            <a-form-item field="password" label="密码">
              <a-input
                v-model="form.password"
                type="password"
                placeholder="请输入您的密码"
                show-password
              >
                <template #prefix>
                  <svg class="prefix-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
                    <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
                  </svg>
                </template>
              </a-input>
            </a-form-item>

            <a-form-item>
              <a-button type="primary" html-type="submit" long :loading="loading" class="login-button">
                登录
              </a-button>
            </a-form-item>
          </a-form>

          <div class="login-tips">
            <div class="divider">
              <span></span>
              <span>登录提示</span>
              <span></span>
            </div>
            <ul class="tips-list">
              <li>如果您没有账号，请联系管理员创建</li>
              <li>忘记密码请通过管理员重置</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const form = ref({
  email: '',
  password: ''
})
const loading = ref(false)

async function handleSubmit() {
  loading.value = true
  try {
    const result = await authStore.login(form.value)
    Message.success('登录成功')
    router.push('/')
  } catch (error) {
    console.error(error)
    Message.error(error.message || '登录失败，请稍后重试')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  overflow: hidden;
}

.login-container::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-image: 
    radial-gradient(circle at 20% 50%, rgba(22, 93, 255, 0.3) 0%, transparent 50%),
    radial-gradient(circle at 80% 50%, rgba(114, 46, 209, 0.3) 0%, transparent 50%),
    radial-gradient(circle at 50% 80%, rgba(102, 126, 234, 0.2) 0%, transparent 50%);
  animation: float 15s ease-in-out infinite;
}

.login-container::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-image: 
    repeating-linear-gradient(
      0deg,
      transparent,
      transparent 2px,
      rgba(255, 255, 255, 0.03) 2px,
      rgba(255, 255, 255, 0.03) 4px
    ),
    repeating-linear-gradient(
      90deg,
      transparent,
      transparent 2px,
      rgba(255, 255, 255, 0.02) 2px,
      rgba(255, 255, 255, 0.02) 4px
    );
}

@keyframes float {
  0%, 100% { 
    transform: translateY(0) scale(1); 
    opacity: 1;
  }
  50% { 
    transform: translateY(-30px) scale(1.1); 
    opacity: 0.8;
  }
}

.login-wrapper {
  display: flex;
  width: 100%;
  min-height: 100vh;
  position: relative;
  z-index: 1;
}

.login-left {
  flex: 1.2;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  position: relative;
}

.brand-content {
  max-width: 500px;
  position: relative;
  z-index: 2;
}

.logo-section {
  margin-bottom: 60px;
}

.logo-icon {
  width: 100px;
  height: 100px;
  margin-bottom: 24px;
  filter: drop-shadow(0 8px 24px rgba(0, 0, 0, 0.15));
}

.brand-name {
  font-size: 42px;
  font-weight: 700;
  color: white;
  margin: 0 0 12px 0;
  letter-spacing: 1px;
}

.brand-slogan {
  font-size: 18px;
  color: rgba(255, 255, 255, 0.85);
  margin: 0;
  font-weight: 400;
}

.feature-list {
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.feature-item {
  display: flex;
  align-items: flex-start;
  gap: 16px;
}

.feature-icon {
  width: 48px;
  height: 48px;
  min-width: 48px;
  min-height: 48px;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  backdrop-filter: blur(10px);
}

.feature-icon svg {
  width: 24px;
  height: 24px;
}

.feature-text h4 {
  color: white;
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 6px 0;
}

.feature-text p {
  color: rgba(255, 255, 255, 0.75);
  font-size: 14px;
  margin: 0;
  line-height: 1.6;
}

.login-right {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(20px);
}

.login-form-wrapper {
  width: 100%;
  max-width: 420px;
}

.form-header {
  margin-bottom: 40px;
}

.form-title {
  font-size: 32px;
  font-weight: 700;
  color: #1d2129;
  margin: 0 0 8px 0;
}

.form-subtitle {
  font-size: 15px;
  color: #86909c;
  margin: 0;
}

.login-form {
  margin-bottom: 32px;
}

.prefix-icon {
  width: 18px;
  height: 18px;
  color: #86909c;
  opacity: 0.8;
}

.login-button {
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
  margin-top: 8px;
}

.login-tips {
  padding-top: 24px;
  border-top: 1px solid #f2f3f5;
}

.divider {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.divider span:first-child,
.divider span:last-child {
  flex: 1;
  height: 1px;
  background: #f2f3f5;
}

.divider span:nth-child(2) {
  color: #86909c;
  font-size: 12px;
  font-weight: 500;
}

.tips-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.tips-list li {
  color: #86909c;
  font-size: 13px;
  line-height: 1.8;
  padding-left: 16px;
  position: relative;
}

.tips-list li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 9px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #165DFF;
}

/* 响应式 */
@media (max-width: 1024px) {
  .login-wrapper {
    flex-direction: column;
  }

  .login-left {
    padding: 40px 24px;
  }

  .login-right {
    padding: 40px 24px;
  }

  .brand-content {
    max-width: 100%;
  }

  .logo-icon {
    width: 80px;
    height: 80px;
  }

  .brand-name {
    font-size: 32px;
  }

  .brand-slogan {
    font-size: 16px;
  }

  .feature-list {
    display: none;
  }
}

@media (max-width: 640px) {
  .login-left {
    display: none;
  }

  .login-form-wrapper {
    max-width: 100%;
  }

  .form-title {
    font-size: 28px;
  }
}
</style>
