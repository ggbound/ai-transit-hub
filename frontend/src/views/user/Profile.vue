<template>
  <div class="profile-container">
    <!-- 页面标题 -->
    <div class="page-header">
      <div>
        <h1>个人中心</h1>
        <p>管理您的个人信息和使用统计</p>
      </div>
    </div>

    <!-- 统计卡片 -->
    <a-row :gutter="16" class="stat-row">
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon blue-icon">
            <icon-message />
          </div>
          <div class="stat-content">
            <div class="stat-title">今日调用次数</div>
            <div class="stat-value">{{ formatNumber(stats.todayCalls) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon green-icon">
            <icon-swap />
          </div>
          <div class="stat-content">
            <div class="stat-title">今日Token消耗</div>
            <div class="stat-value">{{ formatNumber(stats.todayTokens) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon orange-icon">
            <icon-calendar />
          </div>
          <div class="stat-content">
            <div class="stat-title">本月Token消耗</div>
            <div class="stat-value">{{ formatNumber(stats.monthTokens) }}</div>
          </div>
        </div>
      </a-col>
    </a-row>

    <a-row :gutter="16" class="stat-row">
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon purple-icon">
            <icon-apps />
          </div>
          <div class="stat-content">
            <div class="stat-title">本月调用次数</div>
            <div class="stat-value">{{ formatNumber(stats.monthCalls) }}</div>
          </div>
        </div>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon cyan-icon">
            <icon-fire />
          </div>
          <div class="stat-content">
            <div class="stat-title">总Token消耗</div>
            <div class="stat-value">{{ formatNumber(stats.totalTokens) }}</div>
          </div>
        </div>
      </a-col>
      <!-- 总会话数量统计已隐藏 -->
      <a-col v-if="false" :xs="24" :sm="12" :md="8" :lg="8" :xl="8">
        <div class="stat-card">
          <div class="stat-icon pink-icon">
            <icon-user />
          </div>
          <div class="stat-content">
            <div class="stat-title">总会话数量</div>
            <div class="stat-value">{{ formatNumber(stats.totalSessions) }}</div>
          </div>
        </div>
      </a-col>
    </a-row>

    <!-- 内容区域 -->
    <a-row :gutter="16" class="content-row">
      <!-- 左侧：个人信息 -->
      <a-col :xs="24" :sm="24" :md="8" :lg="8" :xl="8">
        <a-card title="个人信息" class="info-card">
          <a-form :model="userForm" layout="vertical">
            <a-form-item label="头像">
              <div class="avatar-upload">
                <a-avatar :size="80" :src="userForm.avatar || defaultAvatar">
                  <template #icon><icon-user /></template>
                </a-avatar>
                <a-button type="text" size="small" @click="uploadAvatar">
                  <template #icon><icon-camera /></template>
                  更换头像
                </a-button>
              </div>
            </a-form-item>
            <a-form-item label="姓名" required>
              <a-input v-model="userForm.name" placeholder="请输入姓名" />
            </a-form-item>
            <a-form-item label="邮箱">
              <a-input v-model="userForm.email" readonly disabled />
            </a-form-item>
            <a-form-item label="部门">
              <a-input v-model="userForm.department" placeholder="请输入部门" />
            </a-form-item>
            <a-form-item label="工号">
              <a-input v-model="userForm.employee_id" placeholder="请输入工号" />
            </a-form-item>
            <a-form-item>
              <a-space>
                <a-button type="primary" @click="updateProfile" :loading="updateLoading">
                  <template #icon><icon-edit /></template>
                  更新信息
                </a-button>
                <a-button @click="showPasswordModal = true">
                  <template #icon><icon-lock /></template>
                  修改密码
                </a-button>
              </a-space>
            </a-form-item>
          </a-form>
        </a-card>
      </a-col>

      <!-- 右侧：使用记录 -->
      <a-col :xs="24" :sm="24" :md="16" :lg="16" :xl="16">
        <a-card title="最近使用记录" class="table-card">
          <a-table
            :data="usageRecords"
            :loading="tableLoading"
            :pagination="pagination"
            @page-change="handlePageChange"
            :scroll="{ x: 800 }"
          >
            <template #columns>
              <a-table-column title="时间" data-index="created_at" :width="170">
                <template #cell="{ record }">
                  {{ formatDate(record.created_at) }}
                </template>
              </a-table-column>
              <a-table-column title="模型" data-index="model" :width="200">
                <template #cell="{ record }">
                  <a-tag v-if="record.model" color="arcoblue">
                    {{ record.model.provider?.code }}/{{ record.model.code }}
                  </a-tag>
                  <span v-else>-</span>
                </template>
              </a-table-column>
              <a-table-column title="输入Token" data-index="prompt_tokens" :width="100" />
              <a-table-column title="输出Token" data-index="completion_tokens" :width="100" />
              <a-table-column title="总Token" data-index="total_tokens" :width="100" />
              <a-table-column title="费用" data-index="cost" :width="100">
                <template #cell="{ record }">
                  <span class="cost-text">${{ Number(record.cost || 0).toFixed(4) }}</span>
                </template>
              </a-table-column>
            </template>
          </a-table>
        </a-card>
      </a-col>
    </a-row>

    <!-- 修改密码弹窗 -->
    <a-modal
      v-model:visible="showPasswordModal"
      title="修改密码"
      @ok="changePassword"
      @cancel="resetPasswordForm"
      :confirm-loading="passwordLoading"
    >
      <a-form :model="passwordForm" layout="vertical">
        <a-form-item label="旧密码" required>
          <a-input-password
            v-model="passwordForm.oldPassword"
            placeholder="请输入旧密码"
            autocomplete="current-password"
          />
        </a-form-item>
        <a-form-item label="新密码" required>
          <a-input-password
            v-model="passwordForm.newPassword"
            placeholder="请输入新密码（至少6位）"
            autocomplete="new-password"
          />
        </a-form-item>
        <a-form-item label="确认新密码" required>
          <a-input-password
            v-model="passwordForm.confirmPassword"
            placeholder="请再次输入新密码"
            autocomplete="new-password"
          />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useAuthStore } from '../../stores/auth'
import { Message } from '@arco-design/web-vue'
import api from '../../utils/api'
import {
  IconUser,
  IconMessage,
  IconSwap,
  IconCalendar,
  IconApps,
  IconFire,
  IconEdit,
  IconLock,
  IconCamera,
} from '@arco-design/web-vue/es/icon'

const authStore = useAuthStore()
const defaultAvatar = 'https://api.dicebear.com/7.x/avataaars/svg?seed=Felix'

// 加载状态
const loading = ref(false)
const updateLoading = ref(false)
const passwordLoading = ref(false)
const tableLoading = ref(false)

// 用户信息表单
const userForm = ref({
  name: '',
  email: '',
  department: '',
  employee_id: '',
  avatar: '',
})

// 统计数据
const stats = ref({
  todayCalls: 0,
  todayTokens: 0,
  monthTokens: 0,
  monthCalls: 0,
  totalTokens: 0,
  totalSessions: 0,
})

// 使用记录
const usageRecords = ref([])
const pagination = ref({
  current: 1,
  pageSize: 10,
  total: 0,
})

// 密码弹窗
const showPasswordModal = ref(false)
const passwordForm = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

// 格式化数字
function formatNumber(num) {
  if (num === null || num === undefined) return '0'
  return num.toLocaleString()
}

// 格式化日期
function formatDate(dateString) {
  if (!dateString) return '-'
  const date = new Date(dateString)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// 获取用户统计数据
async function loadStats() {
  try {
    const response = await api.get('/user/logs/stats')
    if (response.data?.success) {
      stats.value = response.data.data
    }
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

// 获取使用记录
async function loadUsageRecords() {
  tableLoading.value = true
  try {
    const response = await api.get('/user/logs/usage', {
      params: {
        page: pagination.value.current,
        per_page: pagination.value.pageSize,
      },
    })
    if (response.data?.success) {
      usageRecords.value = response.data.data.data || []
      pagination.value.total = response.data.data.total || 0
    }
  } catch (error) {
    console.error('加载使用记录失败:', error)
    Message.error('加载使用记录失败')
  } finally {
    tableLoading.value = false
  }
}

// 分页变化
function handlePageChange(page) {
  pagination.value.current = page
  loadUsageRecords()
}

// 获取用户信息
async function loadUserInfo() {
  try {
    const response = await api.get('/user')
    if (response.data?.success) {
      const user = response.data.data
      userForm.value = {
        name: user.name || '',
        email: user.email || '',
        department: user.department || '',
        employee_id: user.employee_id || '',
        avatar: user.avatar || '',
      }
    }
  } catch (error) {
    console.error('加载用户信息失败:', error)
    Message.error('加载用户信息失败')
  }
}

// 更新个人信息
async function updateProfile() {
  if (!userForm.value.name.trim()) {
    Message.warning('请输入姓名')
    return
  }

  updateLoading.value = true
  try {
    const response = await api.put('/user/profile', {
      name: userForm.value.name,
      department: userForm.value.department,
      employee_id: userForm.value.employee_id,
      avatar: userForm.value.avatar,
    })

    if (response.data?.success) {
      Message.success('个人信息更新成功')
      // 更新本地存储的用户信息
      await authStore.updateUserInfo(userForm.value)
    }
  } catch (error) {
    console.error('更新失败:', error)
    Message.error(error.response?.data?.message || '更新失败')
  } finally {
    updateLoading.value = false
  }
}

// 上传头像
async function uploadAvatar() {
  // 创建文件输入元素
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = async (e) => {
    const file = e.target.files[0]
    if (!file) return

    // 验证文件类型和大小
    if (!file.type.startsWith('image/')) {
      Message.error('请选择图片文件')
      return
    }
    if (file.size > 2 * 1024 * 1024) {
      Message.error('图片大小不能超过2MB')
      return
    }

    try {
      const formData = new FormData()
      formData.append('avatar', file)

      const response = await api.post('/user/avatar', formData, {
        headers: {
          'Content-Type': 'multipart/form-data',
        },
      })

      if (response.data?.success) {
        userForm.value.avatar = response.data.data.avatar_url
        Message.success('头像上传成功')
        await authStore.updateUserInfo({ avatar: response.data.data.avatar_url })
      }
    } catch (error) {
      console.error('上传失败:', error)
      Message.error('头像上传失败')
    }
  }
  input.click()
}

// 修改密码
async function changePassword() {
  // 表单验证
  if (!passwordForm.value.oldPassword) {
    Message.warning('请输入旧密码')
    return
  }
  if (!passwordForm.value.newPassword || passwordForm.value.newPassword.length < 6) {
    Message.warning('新密码至少6位')
    return
  }
  if (passwordForm.value.newPassword !== passwordForm.value.confirmPassword) {
    Message.warning('两次输入的新密码不一致')
    return
  }

  passwordLoading.value = true
  try {
    const response = await api.post('/user/change-password', {
      old_password: passwordForm.value.oldPassword,
      new_password: passwordForm.value.newPassword,
    })

    if (response.data?.success) {
      Message.success('密码修改成功')
      showPasswordModal.value = false
      resetPasswordForm()
    }
  } catch (error) {
    console.error('修改密码失败:', error)
    Message.error(error.response?.data?.message || '修改密码失败')
  } finally {
    passwordLoading.value = false
  }
}

// 重置密码表单
function resetPasswordForm() {
  passwordForm.value = {
    oldPassword: '',
    newPassword: '',
    confirmPassword: '',
  }
}

// 页面加载
onMounted(async () => {
  loading.value = true
  await Promise.all([loadUserInfo(), loadStats(), loadUsageRecords()])
  loading.value = false
})
</script>

<style scoped>
.profile-container {
  padding: 0;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h1 {
  font-size: 24px;
  font-weight: 500;
  margin-bottom: 8px;
}

.page-header p {
  color: var(--color-text-3);
  font-size: 14px;
}

.stat-row {
  margin-bottom: 16px;
}

.stat-card {
  background: var(--color-bg-2);
  border-radius: 8px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transition: all 0.3s ease;
}

.stat-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: #fff;
}

.blue-icon {
  background: linear-gradient(135deg, #1890ff 0%, #36cfc9 100%);
}

.green-icon {
  background: linear-gradient(135deg, #52c41a 0%, #95de64 100%);
}

.orange-icon {
  background: linear-gradient(135deg, #fa8c16 0%, #ffc53d 100%);
}

.purple-icon {
  background: linear-gradient(135deg, #722ed1 0%, #b37feb 100%);
}

.cyan-icon {
  background: linear-gradient(135deg, #13c2c2 0%, #5cdbd3 100%);
}

.pink-icon {
  background: linear-gradient(135deg, #eb2f96 0%, #ff85c0 100%);
}

.stat-content {
  flex: 1;
}

.stat-title {
  font-size: 14px;
  color: var(--color-text-3);
  margin-bottom: 4px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: var(--color-text-1);
}

.content-row {
  margin-top: 16px;
}

.info-card {
  height: 100%;
}

.table-card {
  height: 100%;
}

.avatar-upload {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.cost-text {
  color: var(--color-text-2);
  font-weight: 500;
}

@media (max-width: 768px) {
  .stat-card {
    margin-bottom: 16px;
  }

  .content-row .arco-col {
    margin-bottom: 16px;
  }
}
</style>
