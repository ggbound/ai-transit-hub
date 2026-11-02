<template>
  <div class="register-container">
    <div class="register-card">
      <h2 class="register-title">注册</h2>
      <a-form :model="form" @submit="handleSubmit">
        <a-form-item field="name" label="用户名">
          <a-input
            v-model="form.name"
            placeholder="请输入用户名"
          />
        </a-form-item>
        <a-form-item field="email" label="邮箱">
          <a-input
            v-model="form.email"
            type="email"
            placeholder="请输入邮箱"
          />
        </a-form-item>
        <a-form-item field="password" label="密码">
          <a-input
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            show-password
          />
        </a-form-item>
        <a-form-item field="password_confirmation" label="确认密码">
          <a-input
            v-model="form.password_confirmation"
            type="password"
            placeholder="请再次输入密码"
            show-password
          />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" html-type="submit" long :loading="loading">
            注册
          </a-button>
        </a-form-item>
      </a-form>
      <div class="link">
        已有账号？<router-link to="/login">立即登录</router-link>
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
  name: '',
  email: '',
  password: '',
  password_confirmation: ''
})
const loading = ref(false)

async function handleSubmit() {
  loading.value = true
  try {
    const result = await authStore.register(form.value)
    if (result.success) {
      Message.success('注册成功')
      router.push('/')
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.register-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}
.register-card {
  width: 400px;
  padding: 40px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
.register-title {
  margin: 0 0 24px 0;
  text-align: center;
  color: #1d2129;
}
.link {
  text-align: center;
  margin-top: 16px;
}
</style>
