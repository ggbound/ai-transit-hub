<template>
  <div class="skills-container">
    <div class="page-header">
      <h1>Skill 管理</h1>
      <p>管理 SKILL.md 技能包（上传 / 在线创建 / 预览 / 下载）</p>
    </div>

    <a-row :gutter="16" style="margin-bottom: 16px;">
      <a-col :xs="24" :sm="12" :md="8" :lg="10">
        <a-space>
          <a-input-search
            v-model="searchKeyword"
            placeholder="搜索名称或描述"
            style="width: 240px;"
            allow-clear
            @search="handleSearch"
            @press-enter="handleSearch"
            @clear="handleSearch"
          />
          <a-select
            v-model="statusFilter"
            placeholder="状态"
            style="width: 120px;"
            allow-clear
            @change="handleSearch"
          >
            <a-option value="true">已启用</a-option>
            <a-option value="false">已停用</a-option>
          </a-select>
        </a-space>
      </a-col>
      <a-col :xs="24" :sm="12" :md="16" :lg="14" style="text-align: right;">
        <a-tooltip content="新建 Skill">
          <a-button type="primary" @click="openCreateModal">
            <template #icon><icon-plus /></template>
          </a-button>
        </a-tooltip>
      </a-col>
    </a-row>

    <a-card class="content-card">
      <a-table
        :data="skills"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1200 }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="标识 (name)" data-index="name" width="180" fixed="left" />
          <a-table-column title="描述" data-index="description" :ellipsis="true" width="280" />
          <a-table-column title="版本" data-index="version" width="90" />
          <a-table-column title="标签" width="180">
            <template #cell="{ record }">
              <a-space wrap size="mini">
                <a-tag v-for="tag in parseTags(record.tags)" :key="tag" size="small" color="arcoblue">
                  {{ tag }}
                </a-tag>
              </a-space>
            </template>
          </a-table-column>
          <a-table-column title="附件" data-index="file_count" width="80" align="center" />
          <a-table-column title="状态" data-index="enabled" width="90">
            <template #cell="{ record }">
              <a-switch
                :model-value="record.enabled"
                size="small"
                @change="(val) => toggleSkillStatus(record, val)"
              />
            </template>
          </a-table-column>
          <a-table-column title="创建时间" data-index="created_at" width="180">
            <template #cell="{ record }">
              {{ formatDate(record.created_at) }}
            </template>
          </a-table-column>
          <a-table-column title="操作" width="170" align="center" fixed="right">
            <template #cell="{ record }">
              <a-button-group size="small">
                <a-tooltip content="查看">
                  <a-button type="text" size="small" @click="showDetail(record)">
                    <template #icon><icon-eye /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip content="编辑">
                  <a-button type="text" size="small" @click="showEditModal(record)">
                    <template #icon><icon-edit /></template>
                  </a-button>
                </a-tooltip>
                <a-tooltip content="下载">
                  <a-button type="text" size="small" @click="downloadSkill(record)">
                    <template #icon><icon-download /></template>
                  </a-button>
                </a-tooltip>
                <a-popconfirm content="确定删除该 Skill 吗？附件将一并删除。" type="warning" @ok="deleteSkill(record.id)">
                  <a-tooltip content="删除">
                    <a-button type="text" size="small" status="danger">
                      <template #icon><icon-delete /></template>
                    </a-button>
                  </a-tooltip>
                </a-popconfirm>
              </a-button-group>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 新建 Skill 抽屉 -->
    <a-drawer
      v-model:visible="showCreateModal"
      title="新建 Skill"
      :width="720"
      unmount-on-close
      :ok-loading="createLoading"
      @before-ok="createSkill"
    >
      <a-tabs v-model:active-key="createTab" type="card">
        <a-tab-pane key="online" title="在线创建">
          <a-form :model="createForm" layout="vertical">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="标识 name" field="name" required>
                  <a-input v-model="createForm.name" placeholder="小写字母、数字、连字符，如 code-review" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="版本" field="version">
                  <a-input v-model="createForm.version" placeholder="0.0.1" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-form-item label="描述" field="description" required>
              <a-textarea v-model="createForm.description" :rows="2" placeholder="技能描述（不超过 1024 字符）" />
            </a-form-item>
            <a-form-item label="标签" field="tags">
              <a-input v-model="createForm.tagsText" placeholder="多个标签用英文逗号分隔，如 编码,审查" />
            </a-form-item>
            <a-form-item label="SKILL.md 内容" field="content" required>
              <a-textarea
                v-model="createForm.content"
                :rows="12"
                class="mono-textarea"
                placeholder="粘贴完整的 SKILL.md（含 YAML frontmatter）"
              />
            </a-form-item>
          </a-form>
        </a-tab-pane>

        <a-tab-pane key="upload" title="上传技能包">
          <a-upload
            draggable
            :auto-upload="false"
            :limit="1"
            accept=".zip"
            :file-list="uploadFileList"
            @change="onUploadChange"
          >
            <template #upload-button>
              <div class="upload-area">
                <icon-upload style="font-size: 32px; color: var(--color-text-3);" />
                <div style="margin-top: 8px;">点击或拖拽 zip 技能包到此处上传</div>
                <div class="upload-tip">包内需包含 SKILL.md，单个包不超过 20MB</div>
              </div>
            </template>
          </a-upload>
        </a-tab-pane>
      </a-tabs>
    </a-drawer>

    <!-- 编辑 Skill 抽屉 -->
    <a-drawer
      v-model:visible="showEditModalVisible"
      title="编辑 Skill"
      :width="720"
      unmount-on-close
      :ok-loading="updateLoading"
      @before-ok="updateSkill"
    >
      <a-form :model="editForm" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="标识 name" field="name" required>
              <a-input v-model="editForm.name" placeholder="小写字母、数字、连字符" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="版本" field="version">
              <a-input v-model="editForm.version" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="描述" field="description" required>
          <a-textarea v-model="editForm.description" :rows="2" />
        </a-form-item>
        <a-form-item label="标签" field="tags">
          <a-input v-model="editForm.tagsText" placeholder="多个标签用英文逗号分隔" />
        </a-form-item>
        <a-form-item label="SKILL.md 内容" field="content" required>
          <a-textarea v-model="editForm.content" :rows="12" class="mono-textarea" />
        </a-form-item>
        <a-form-item label="状态" field="enabled">
          <a-switch v-model="editForm.enabled" />
        </a-form-item>
      </a-form>
    </a-drawer>

    <!-- Skill 详情抽屉 -->
    <a-drawer
      v-model:visible="showDetailDrawer"
      :title="detailSkill?.name || 'Skill 详情'"
      :width="680"
      :footer="false"
    >
      <template v-if="detailSkill">
        <a-descriptions :column="2" size="medium" bordered>
          <a-descriptions-item label="标识">{{ detailSkill.name }}</a-descriptions-item>
          <a-descriptions-item label="版本">{{ detailSkill.version || '-' }}</a-descriptions-item>
          <a-descriptions-item label="许可证" :span="detailSkill.license ? 1 : 2">
            {{ detailSkill.license || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="允许工具" :span="2">
            {{ detailSkill.allowed_tools || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="描述" :span="2">
            {{ detailSkill.description || '-' }}
          </a-descriptions-item>
          <a-descriptions-item label="标签" :span="2">
            <a-space wrap size="mini">
              <a-tag v-for="tag in parseTags(detailSkill.tags)" :key="tag" size="small" color="arcoblue">
                {{ tag }}
              </a-tag>
              <span v-if="!parseTags(detailSkill.tags).length">-</span>
            </a-space>
          </a-descriptions-item>
        </a-descriptions>

        <div class="detail-section-title">
          附件（{{ detailFiles.length }}）
          <a-button size="mini" type="text" @click="downloadSkill(detailSkill)">
            <template #icon><icon-download /></template>
            下载整包
          </a-button>
        </div>
        <a-table
          v-if="detailFiles.length"
          :data="detailFiles"
          :pagination="false"
          size="small"
          :scroll="{ y: 200 }"
        >
          <template #columns>
            <a-table-column title="文件" data-index="path" :ellipsis="true" />
            <a-table-column title="大小" width="100">
              <template #cell="{ record }">{{ formatSize(record.size) }}</template>
            </a-table-column>
            <a-table-column title="类型" data-index="mime_type" width="140" :ellipsis="true" />
          </template>
        </a-table>
        <a-empty v-else description="无附件，仅 SKILL.md" />

        <div class="detail-section-title">SKILL.md 预览</div>
        <div class="md-preview" v-html="renderedContent"></div>
      </template>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../../utils/api'
import { Message } from '@arco-design/web-vue'
import { marked } from 'marked'
import { IconPlus, IconUpload, IconDownload, IconEye, IconEdit, IconDelete } from '@arco-design/web-vue/es/icon'

const skills = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 10, total: 0 })
const searchKeyword = ref('')
const statusFilter = ref(null)

const showCreateModal = ref(false)
const showEditModalVisible = ref(false)
const showDetailDrawer = ref(false)
const createLoading = ref(false)
const updateLoading = ref(false)
const createTab = ref('online')

const uploadFileList = ref([])
const detailSkill = ref(null)
const detailFiles = ref([])

const defaultSkillContent = `---
name: my-skill
description: 技能描述（不超过 1024 字符）
version: 0.0.1
---

# 技能正文

在这里编写技能的 Markdown 指令内容。
`

const createForm = ref({
  name: '',
  description: '',
  version: '',
  tagsText: '',
  content: ''
})

const editForm = ref({
  id: null,
  name: '',
  description: '',
  version: '',
  license: '',
  allowed_tools: '',
  tagsText: '',
  content: '',
  enabled: true
})

const renderedContent = computed(() => {
  if (!detailSkill.value?.content) return ''
  try {
    return marked.parse(detailSkill.value.content)
  } catch {
    return detailSkill.value.content
  }
})

onMounted(() => {
  loadSkills()
})

async function loadSkills() {
  loading.value = true
  try {
    const params = {
      page: pagination.value.current,
      per_page: pagination.value.pageSize
    }
    if (searchKeyword.value) params.search = searchKeyword.value
    if (statusFilter.value) params.enabled = statusFilter.value

    const response = await api.get('/admin/skills', { params })
    if (response.data.success) {
      skills.value = response.data.data.data || []
      pagination.value.total = response.data.data.meta?.total || 0
    }
  } catch (error) {
    console.error(error)
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  pagination.value.current = 1
  loadSkills()
}

function openCreateModal() {
  createTab.value = 'online'
  uploadFileList.value = []
  createForm.value = { name: '', description: '', version: '', tagsText: '', content: defaultSkillContent }
  showCreateModal.value = true
}

async function createSkill() {
  if (createTab.value === 'upload') {
    const ok = await uploadSkill()
    if (ok) showCreateModal.value = false
    return false
  }

  if (!createForm.value.name || !createForm.value.content) {
    Message.error('请填写 name 和 SKILL.md 内容')
    return false
  }

  createLoading.value = true
  try {
    const payload = {
      name: createForm.value.name,
      description: createForm.value.description,
      version: createForm.value.version,
      content: createForm.value.content,
      tags: tagsTextToArray(createForm.value.tagsText)
    }
    const response = await api.post('/admin/skills', payload)
    if (response.data.success) {
      Message.success('创建成功')
      await loadSkills()
      return true
    }
    return false
  } catch (error) {
    console.error(error)
    return false
  } finally {
    createLoading.value = false
  }
}

async function onUploadChange(fileList) {
  uploadFileList.value = fileList
}

async function uploadSkill() {
  const fileItem = uploadFileList.value[0]?.file
  if (!fileItem) {
    Message.error('请选择要上传的 zip 技能包')
    return false
  }
  if (!fileItem.name.toLowerCase().endsWith('.zip')) {
    Message.error('仅支持 .zip 文件')
    return false
  }

  createLoading.value = true
  try {
    const formData = new FormData()
    formData.append('file', fileItem)
    const response = await api.post('/admin/skills/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
    if (response.data.success) {
      Message.success('上传成功')
      await loadSkills()
      return true
    }
    return false
  } catch (error) {
    console.error(error)
    return false
  } finally {
    createLoading.value = false
  }
}

function showEditModal(record) {
  editForm.value = {
    id: record.id,
    name: record.name,
    description: record.description,
    version: record.version || '',
    license: record.license || '',
    allowed_tools: record.allowed_tools || '',
    tagsText: parseTags(record.tags).join(','),
    content: record.content,
    enabled: record.enabled
  }
  showEditModalVisible.value = true
}

async function updateSkill() {
  if (!editForm.value.name || !editForm.value.content) {
    Message.error('请填写 name 和 SKILL.md 内容')
    return false
  }

  updateLoading.value = true
  try {
    const payload = {
      name: editForm.value.name,
      description: editForm.value.description,
      version: editForm.value.version,
      license: editForm.value.license,
      allowed_tools: editForm.value.allowed_tools,
      content: editForm.value.content,
      tags: tagsTextToArray(editForm.value.tagsText),
      enabled: editForm.value.enabled
    }
    const response = await api.put(`/admin/skills/${editForm.value.id}`, payload)
    if (response.data.success) {
      Message.success('更新成功')
      await loadSkills()
      return true
    }
    return false
  } catch (error) {
    console.error(error)
    return false
  } finally {
    updateLoading.value = false
  }
}

async function showDetail(record) {
  try {
    const response = await api.get(`/admin/skills/${record.id}`)
    if (response.data.success) {
      detailSkill.value = response.data.data
      detailFiles.value = response.data.data.files || []
      showDetailDrawer.value = true
    }
  } catch (error) {
    console.error(error)
  }
}

async function downloadSkill(record) {
  try {
    const response = await api.get(`/admin/skills/${record.id}/download`, { responseType: 'blob' })
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url
    link.download = `${record.name}.zip`
    link.click()
    window.URL.revokeObjectURL(url)
  } catch (error) {
    console.error(error)
    Message.error('下载失败')
  }
}

async function deleteSkill(id) {
  try {
    const response = await api.delete(`/admin/skills/${id}`)
    if (response.data.success) {
      Message.success('删除成功')
      await loadSkills()
    }
  } catch (error) {
    console.error(error)
  }
}

async function toggleSkillStatus(record, val) {
  try {
    await api.put(`/admin/skills/${record.id}/toggle-status`)
    Message.success('状态更新成功')
    loadSkills()
  } catch (error) {
    Message.error('状态更新失败')
  }
}

function tagsTextToArray(text) {
  const arr = (text || '')
    .split(/[,，]/)
    .map((t) => t.trim())
    .filter(Boolean)
  return arr.length ? arr : null
}

function parseTags(tags) {
  if (Array.isArray(tags)) return tags
  if (typeof tags === 'string') {
    try {
      const parsed = JSON.parse(tags)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

function handlePageChange(page) {
  pagination.value.current = page
  loadSkills()
}

function formatDate(date) {
  if (!date) return ''
  return new Date(date).toLocaleString('zh-CN')
}

function formatSize(bytes) {
  if (bytes == null) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
</script>

<style scoped>
.skills-container {
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

.mono-textarea :deep(textarea) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}

.upload-area {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 16px;
  border: 1px dashed var(--border-color);
  border-radius: 8px;
  background: var(--color-fill-1);
  cursor: pointer;
  transition: border-color 0.2s;
}

.upload-area:hover {
  border-color: rgb(var(--primary-6));
}

.upload-tip {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.detail-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 20px 0 10px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.md-preview {
  padding: 16px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--color-fill-1);
  font-size: 14px;
  line-height: 1.7;
  overflow-y: auto;
  max-height: 480px;
}

.md-preview :deep(h1),
.md-preview :deep(h2),
.md-preview :deep(h3) {
  margin: 16px 0 8px;
}

.md-preview :deep(p) {
  margin: 8px 0;
}

.md-preview :deep(pre) {
  padding: 12px;
  border-radius: 6px;
  background: var(--color-neutral-2);
  overflow-x: auto;
}

.md-preview :deep(code) {
  font-family: 'SF Mono', Menlo, Consolas, monospace;
  font-size: 13px;
}
</style>
