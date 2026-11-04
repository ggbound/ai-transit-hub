<template>
  <div class="billing-container">
    <div class="page-header">
      <h1>计费管理</h1>
      <p>管理系统费用统计、模型费率和费用明细</p>
    </div>
    
    <!-- 费用统计卡片 -->
    <a-row :gutter="16" style="margin-bottom: 16px;" class="stat-row">
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-card class="content-card">
          <a-statistic 
            title="今日费用" 
            :value="stats.todayCost" 
            :precision="2" 
            :value-style="{ color: '#165dff' }"
          >
            <template #prefix>¥</template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-card class="content-card">
          <a-statistic 
            title="本周费用" 
            :value="stats.weekCost" 
            :precision="2" 
            :value-style="{ color: '#165dff' }"
          >
            <template #prefix>¥</template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-card class="content-card">
          <a-statistic 
            title="本月费用" 
            :value="stats.monthCost" 
            :precision="2" 
            :value-style="{ color: '#00b42a' }"
          >
            <template #prefix>¥</template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :md="8" :lg="6">
        <a-card class="content-card">
          <a-statistic 
            title="总费用" 
            :value="stats.totalCost" 
            :precision="2" 
            :value-style="{ color: '#f53f3f' }"
          >
            <template #prefix>¥</template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <!-- 模型费率配置 -->
    <a-card title="模型费率配置" class="content-card" style="margin-bottom: 16px;">
      <div class="table-toolbar">
        <span class="toolbar-info">共 {{ filteredRates.length }} 个模型</span>
        <a-input-search
          v-model="rateKeyword"
          placeholder="搜索模型名称 / 代码 / 提供商"
          allow-clear
          style="width: 280px;"
        />
      </div>
      <a-table
        :data="pagedRates"
        :loading="loading"
        :scroll="{ x: 760 }"
        :pagination="ratePagination"
        @page-change="(p) => ratePage = p"
      >
        <template #columns>
          <a-table-column title="模型" fixed="left" :width="240">
            <template #cell="{ record }">
              <div class="rate-model-cell">
                <div class="rate-model-name">{{ record.model_name }}</div>
                <div class="rate-model-code">{{ record.provider }} · {{ record.code }}</div>
              </div>
            </template>
          </a-table-column>
          <a-table-column title="输入价格" :width="180" align="right">
            <template #cell="{ record }">
              <span class="price-text">¥{{ record.input_price.toFixed(4) }} <em>/ 百万tokens</em></span>
            </template>
          </a-table-column>
          <a-table-column title="输出价格" :width="180" align="right">
            <template #cell="{ record }">
              <span class="price-text">¥{{ record.output_price.toFixed(4) }} <em>/ 百万tokens</em></span>
            </template>
          </a-table-column>
          <a-table-column title="操作" fixed="right" :width="100" align="center">
            <template #cell="{ record }">
              <a-button type="text" size="small" @click="editRate(record)">
                <template #icon><icon-edit /></template>
                编辑
              </a-button>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 费用明细 -->
    <a-card title="费用明细" class="content-card">
      <a-table
        :data="billingRecordsData"
        :loading="recordsLoading"
        :scroll="{ x: 1000 }"
        :pagination="{ ...pagination, showTotal: true }"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="时间" data-index="created_at" :width="170" fixed="left" />
          <a-table-column title="用户" data-index="user" :width="140" ellipsis tooltip />
          <a-table-column title="模型" data-index="model" :width="200" ellipsis tooltip />
          <a-table-column title="输入Token" :width="120" align="right">
            <template #cell="{ record }">
              {{ formatTokens(record.input_tokens) }}
            </template>
          </a-table-column>
          <a-table-column title="输出Token" :width="120" align="right">
            <template #cell="{ record }">
              {{ formatTokens(record.output_tokens) }}
            </template>
          </a-table-column>
          <a-table-column title="总Token" :width="130" align="right">
            <template #cell="{ record }">
              <span class="token-total">{{ formatTokens(record.input_tokens + record.output_tokens) }}</span>
            </template>
          </a-table-column>
          <a-table-column title="费用" :width="140" align="right" fixed="right">
            <template #cell="{ record }">
              <span class="cost-text">¥{{ record.cost.toFixed(6) }}</span>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 编辑费率抽屉 -->
    <a-drawer
      v-model:visible="editModalVisible"
      title="编辑模型费率"
      :width="480"
      unmount-on-close
      @ok="saveRate"
      @cancel="editModalVisible = false"
    >
      <a-form :model="editForm" layout="vertical">
        <a-form-item label="模型名称">
          <span>{{ editForm.model_name }}</span>
        </a-form-item>
        <a-form-item label="输入价格 (¥/百万tokens)">
          <a-input-number
            v-model="editForm.input_price"
            :min="0"
            :precision="4"
            :step="0.1"
            style="width: 100%;"
          />
        </a-form-item>
        <a-form-item label="输出价格 (¥/百万tokens)">
          <a-input-number
            v-model="editForm.output_price"
            :min="0"
            :precision="4"
            :step="0.1"
            style="width: 100%;"
          />
        </a-form-item>
      </a-form>
    </a-drawer>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconEdit } from '@arco-design/web-vue/es/icon'
import api from '../../utils/api'

const loading = ref(false)
const recordsLoading = ref(false)

// 统计数据
const stats = ref({
  todayCost: 0,
  weekCost: 0,
  monthCost: 0,
  totalCost: 0,
})

// 模型费率
const modelRates = ref([])
const modelRatesData = computed(() => Array.isArray(modelRates.value) ? modelRates.value : [])

// 费率搜索 + 本地分页
const rateKeyword = ref('')
const ratePage = ref(1)
const RATE_PAGE_SIZE = 10
const filteredRates = computed(() => {
  const kw = rateKeyword.value.trim().toLowerCase()
  if (!kw) return modelRatesData.value
  return modelRatesData.value.filter((r) =>
    [r.model_name, r.code, r.provider].some((v) => String(v || '').toLowerCase().includes(kw))
  )
})
const pagedRates = computed(() => {
  const start = (ratePage.value - 1) * RATE_PAGE_SIZE
  return filteredRates.value.slice(start, start + RATE_PAGE_SIZE)
})
const ratePagination = computed(() => ({
  current: ratePage.value,
  pageSize: RATE_PAGE_SIZE,
  total: filteredRates.value.length,
  showTotal: true,
}))
watch(rateKeyword, () => { ratePage.value = 1 })

// 费用明细
const billingRecords = ref([])
const billingRecordsData = computed(() => Array.isArray(billingRecords.value) ? billingRecords.value : [])
const pagination = ref({
  current: 1,
  pageSize: 10,
  total: 0,
})

// 编辑弹窗
const editModalVisible = ref(false)
const editForm = ref({
  id: null,
  model_name: '',
  input_price: 0,
  output_price: 0,
})

onMounted(() => {
  loadStats()
  loadModelRates()
  loadRecords()
})

// 加载统计数据
async function loadStats() {
  try {
    const res = await api.get('/admin/billing/stats')
    if (res.data.success && res.data.data) {
      stats.value = res.data.data
    }
  } catch (error) {
    Message.error('加载统计数据失败')
  }
}

// 加载模型费率
async function loadModelRates() {
  loading.value = true
  try {
    const res = await api.get('/admin/billing/rates')
    if (res.data.success && Array.isArray(res.data.data)) {
      modelRates.value = res.data.data
    } else {
      modelRates.value = []
    }
  } catch (error) {
    Message.error('加载费率配置失败')
    modelRates.value = []
  } finally {
    loading.value = false
  }
}

// 加载费用明细
async function loadRecords() {
  recordsLoading.value = true
  try {
    const res = await api.get('/admin/billing/records', {
      params: {
        page: pagination.value.current,
        per_page: pagination.value.pageSize,
      }
    })
    if (res.data.success && res.data.data) {
      billingRecords.value = Array.isArray(res.data.data.records) ? res.data.data.records : []
      pagination.value.total = res.data.data.pagination?.total || 0
    } else {
      billingRecords.value = []
      pagination.value.total = 0
    }
  } catch (error) {
    Message.error('加载费用明细失败')
    billingRecords.value = []
    pagination.value.total = 0
  } finally {
    recordsLoading.value = false
  }
}

// 编辑费率
function editRate(record) {
  editForm.value = {
    id: record.id,
    model_name: record.model_name,
    input_price: record.input_price,
    output_price: record.output_price,
  }
  editModalVisible.value = true
}

// 保存费率
async function saveRate() {
  try {
    const res = await api.put(`/admin/billing/rates/${editForm.value.id}`, {
      input_price: editForm.value.input_price,
      output_price: editForm.value.output_price,
    })
    if (res.data.success) {
      Message.success('费率更新成功')
      editModalVisible.value = false
      loadModelRates()
    }
  } catch (error) {
    Message.error(error.response?.data?.message || '更新失败')
  }
}

// 分页
function handlePageChange(page) {
  pagination.value.current = page
  loadRecords()
}

// Token 千分位格式化
function formatTokens(n) {
  return Number(n || 0).toLocaleString('en-US')
}
</script>

<style scoped>
.billing-container {
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

/* 表格工具栏 */
.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.toolbar-info {
  font-size: 13px;
  color: var(--text-secondary);
}

/* 所有单元格不换行 */
.billing-container :deep(.arco-table-th),
.billing-container :deep(.arco-table-td) {
  white-space: nowrap;
}

/* 费率表模型单元格：名称 + 代码双行 */
.rate-model-cell {
  min-width: 0;
}

.rate-model-name {
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.rate-model-code {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 价格文本：单位弱化 */
.price-text em {
  font-style: normal;
  font-size: 12px;
  color: var(--text-secondary);
}

/* 总Token 加粗 */
.token-total {
  font-weight: 500;
}

/* 费用金额 */
.cost-text {
  color: #f53f3f;
  font-weight: 500;
}
</style>
