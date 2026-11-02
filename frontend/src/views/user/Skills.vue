<template>
  <div class="skills-container">
    <div class="page-header">
      <h1>SKILL</h1>
      <p>查看平台提供的可用技能（由管理后台配置，仅支持查看）</p>
    </div>

    <a-card class="table-card">
      <div class="table-toolbar">
        <a-input-search
          v-model="searchKeyword"
          placeholder="搜索名称或描述"
          style="width: 240px;"
          allow-clear
          @search="handleSearch"
          @press-enter="handleSearch"
          @clear="handleSearch"
        />
      </div>

      <a-table
        :data="skills"
        :loading="loading"
        :pagination="pagination"
        :scroll="{ x: 1000 }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="标识 (name)" data-index="name" :width="180" fixed="left" />
          <a-table-column title="描述" data-index="description" :ellipsis="true" :width="280" />
          <a-table-column title="版本" data-index="version" :width="90">
            <template #cell="{ record }">
              {{ record.version || '-' }}
            </template>
          </a-table-column>
          <a-table-column title="标签" :width="180">
            <template #cell="{ record }">
              <a-space wrap size="mini">
                <a-tag v-for="tag in parseTags(record.tags)" :key="tag" size="small" color="arcoblue">
                  {{ tag }}
                </a-tag>
                <span v-if="!parseTags(record.tags).length">-</span>
              </a-space>
            </template>
          </a-table-column>
          <a-table-column title="附件数" data-index="file_count" :width="90" align="center" />
          <a-table-column title="状态" :width="100" align="center">
            <template #cell>
              <a-tag color="green" size="small">可用</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="操作" :width="100" fixed="right">
            <template #cell="{ record }">
              <a-button size="small" type="text" @click="showDetail(record)">查看</a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- Skill 详情抽屉（只读） -->
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

        <div class="detail-section-title">附件（{{ detailFiles.length }}）</div>
        <a-table
          v-if="detailFiles.length"
          :data="detailFiles"
          :pagination="false"
          size="small"
          :scroll="{ y: 200 }"
        >
          <template #columns>
            <a-table-column title="文件" data-index="path" :ellipsis="true" />
            <a-table-column title="大小" :width="100">
              <template #cell="{ record }">{{ formatSize(record.size) }}</template>
            </a-table-column>
            <a-table-column title="类型" data-index="mime_type" :width="140" :ellipsis="true" />
          </template>
        </a-table>
        <a-empty v-else description="无附件，仅 SKILL.md" />

        <div class="detail-section-title">SKILL.md 内容</div>
        <div class="md-preview" v-html="renderedContent"></div>
      </template>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../../utils/api'
import { marked } from 'marked'

const skills = ref([])
const loading = ref(false)
const pagination = ref({ current: 1, pageSize: 10, total: 0 })
const searchKeyword = ref('')

const showDetailDrawer = ref(false)
const detailSkill = ref(null)
const detailFiles = ref([])

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

    const response = await api.get('/skills', { params })
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

async function showDetail(record) {
  try {
    const response = await api.get(`/skills/${record.id}`)
    if (response.data.success) {
      detailSkill.value = response.data.data
      detailFiles.value = response.data.data.files || []
      showDetailDrawer.value = true
    }
  } catch (error) {
    console.error(error)
  }
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

.table-card {
  border-radius: 8px;
  box-shadow: var(--shadow-sm);
  border: 1px solid var(--border-color);
}

.table-toolbar {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
}

.detail-section-title {
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
