<template>
  <div class="users-container">
    <div class="page-header">
      <h1>用户管理</h1>
      <p>管理系统中的用户账号和角色权限</p>
    </div>
    
    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-input-search
          v-model="searchQuery"
          placeholder="搜索用户姓名或邮箱"
          allow-clear
          @search="loadUsers"
          @press-enter="loadUsers"
        />
      </a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="18" style="text-align: right;">
        <a-tooltip content="新建用户">
          <a-button type="primary" @click="showCreateModal = true">
            <template #icon><icon-plus /></template>
          </a-button>
        </a-tooltip>
      </a-col>
    </a-row>
    
    <a-card class="content-card">
      <a-table
        :data="users"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1200 }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="ID" data-index="id" width="80" fixed="left" />
          <a-table-column title="姓名" data-index="name" width="150" />
          <a-table-column title="邮箱" data-index="email" width="250" />
          <a-table-column title="角色" data-index="roles" width="200">
            <template #cell="{ record }">
              <a-tag v-for="role in record.roles" :key="role.id">
                {{ role.display_name }}
              </a-tag>
            </template>
          </a-table-column>
          <a-table-column title="状态" data-index="status" width="100">
            <template #cell="{ record }">
              <a-switch
                :model-value="!!record.status"
                :active-value="1"
                :inactive-value="0"
                size="small"
                @change="(val) => toggleUserStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at" width="200">
            <template #cell="{ record }">
              {{ formatDate(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column title="操作" width="70" align="center" fixed="right">
            <template #cell="{ record }">
              <a-tooltip content="编辑">
                <a-button type="text" size="small" @click="showEditModal(record)">
                  <template #icon><icon-edit /></template>
                </a-button>
              </a-tooltip>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 创建用户抽屉 -->
    <a-drawer
      v-model:visible="showCreateModal"
      title="新建用户"
      :width="480"
      unmount-on-close
      @ok="createUser"
      :ok-loading="createLoading"
    >
      <a-form :model="createForm" layout="vertical">
        <a-form-item label="姓名" field="name">
          <a-input v-model="createForm.name" placeholder="请输入姓名" />
        </a-form-item>
        <a-form-item label="邮箱" field="email">
          <a-input v-model="createForm.email" placeholder="请输入邮箱" />
        </a-form-item>
        <a-form-item label="密码" field="password">
          <a-input-password v-model="createForm.password" placeholder="请输入密码" />
        </a-form-item>
        <a-form-item label="角色" field="roles">
          <a-select v-model="createForm.roles" mode="multiple" placeholder="选择角色" style="width: 100%">
            <a-option v-for="role in allRoles" :key="role.id" :value="role.id">
              {{ role.display_name }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态" field="status">
          <a-switch
            :model-value="!!createForm.status"
            :active-value="1"
            :inactive-value="0"
            @update:model-value="(val) => (createForm.status = val)"
          />
        </a-form-item>
      </a-form>
    </a-drawer>

    <!-- 编辑用户抽屉 -->
    <a-drawer
      v-model:visible="showEditModalVisible"
      title="编辑用户"
      :width="480"
      unmount-on-close
      @ok="updateUser"
      :ok-loading="updateLoading"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="姓名" field="name">
          <a-input v-model="editForm.name" placeholder="请输入姓名" />
        </a-form-item>
        <a-form-item label="邮箱" field="email">
          <a-input v-model="editForm.email" placeholder="请输入邮箱" />
        </a-form-item>
        <a-form-item label="密码" field="password">
          <a-input-password v-model="editForm.password" placeholder="留空不修改密码" />
        </a-form-item>
        <a-form-item label="角色" field="roles">
          <a-select v-model="editForm.roles" mode="multiple" placeholder="选择角色" style="width: 100%">
            <a-option v-for="role in allRoles" :key="role.id" :value="role.id">
              {{ role.display_name }}
            </a-option>
          </a-select>
        </a-form-item>
        <a-form-item label="状态" field="status">
          <a-switch
            :model-value="!!editForm.status"
            :active-value="1"
            :inactive-value="0"
            @update:model-value="(val) => (editForm.status = val)"
          />
        </a-form-item>
      </a-form>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { IconPlus, IconEdit } from '@arco-design/web-vue/es/icon'

const users = ref([])
const allRoles = ref([])
const loading = ref(false)
const searchQuery = ref('')
const pagination = ref({ current: 1, pageSize: 10, total: 0 })

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)

const createForm = ref({
  name: '',
  email: '',
  password: '',
  roles: [],
  status: 1
})

const editForm = ref({
  id: null,
  name: '',
  email: '',
  password: '',
  roles: [],
  status: 1
})

onMounted(() => {
  loadUsers()
  loadRoles()
})

async function loadUsers() {
  loading.value = true
  try {
    const params = {
      page: pagination.value.current,
      per_page: pagination.value.pageSize
    }
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    const response = await api.get('/admin/users', { params })
    if (response.data.success) {
      users.value = response.data.data.data || response.data.data
      pagination.value.total = response.data.data.total || users.value.length
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

async function loadRoles() {
  try {
    // 临时方案，直接从数据库获取默认角色
    allRoles.value = [
      { id: 1, name: 'admin', display_name: '管理员' },
      { id: 2, name: 'user', display_name: '普通用户' }
    ]
  } catch (error) {
    console.error(error)
  }
}

async function createUser() {
  if (!createForm.value.name || !createForm.value.email || !createForm.value.password) {
    Message.error('请填写完整信息')
    return
  }

  createLoading.value = true
  try {
    const response = await api.post('/admin/users', createForm.value)
    if (response.data.success) {
      showCreateModal.value = false
      Message.success('创建成功')
      await loadUsers()
    }
  } catch (error) {
    console.error(error)
  } finally {
    createLoading.value = false
  }
}

function showEditModal(record) {
  editForm.value = {
    id: record.id,
    name: record.name,
    email: record.email,
    password: '',
    roles: record.roles?.map(r => r.id) || [],
    status: record.status
  }
  showEditModalVisible.value = true
}

async function toggleUserStatus(record, val) {
  try {
    await api.put(`/admin/users/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    // 更新本地数据
    record.status = val ? 1 : 0
  } catch (error) {
    Message.error('状态更新失败')
    // 还原状态
    record.status = record.status ? 0 : 1
  }
}

async function updateUser() {
  if (!editForm.value.name || !editForm.value.email) {
    Message.error('请填写完整信息')
    return
  }

  updateLoading.value = true
  try {
    const data = { ...editForm.value }
    if (!data.password) {
      delete data.password
    }
    const response = await api.put(`/admin/users/${editForm.value.id}`, data)
    if (response.data.success) {
      showEditModalVisible.value = false
      Message.success('更新成功')
      await loadUsers()
    }
  } catch (error) {
    console.error(error)
  } finally {
    updateLoading.value = false
  }
}

function handlePageChange(page) {
  pagination.value.current = page
  loadUsers()
}

function formatDate(date) {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}
</script>

<style scoped>
.users-container {
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

.content-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}
</style>
