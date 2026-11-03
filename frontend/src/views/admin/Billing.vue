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
      <a-table :data="modelRatesData" :loading="loading">
        <template #columns>
          <a-table-column title="模型名称" data-index="model_name" />
          <a-table-column title="提供商" data-index="provider" />
          <a-table-column title="模型代码" data-index="code" />
          <a-table-column title="输入价格" width="180">
            <template #cell="{ record }">
              ¥{{ record.input_price.toFixed(4) }} / 百万tokens
            </template>
          </a-table-column>
          <a-table-column title="输出价格" width="180">
            <template #cell="{ record }">
              ¥{{ record.output_price.toFixed(4) }} / 百万tokens
            </template>
          </a-table-column>
          <a-table-column title="操作" width="100">
            <template #cell="{ record }">
              <a-button type="text" size="small" @click="editRate(record)">
                <icon-edit />
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
        :pagination="pagination"
        @page-change="handlePageChange"
      >
        <template #columns>
          <a-table-column title="时间" data-index="created_at" width="160" />
          <a-table-column title="用户" data-index="user" width="120" />
          <a-table-column title="模型" data-index="model" width="150" />
          <a-table-column title="输入Token" data-index="input_tokens" width="120" />
          <a-table-column title="输出Token" data-index="output_tokens" width="120" />
          <a-table-column title="总Token" width="120">
            <template #cell="{ record }">
              {{ record.input_tokens + record.output_tokens }}
            </template>
          </a-table-column>
          <a-table-column title="费用" width="120">
            <template #cell="{ record }">
              <span style="color: #f53f3f;">¥{{ record.cost.toFixed(6) }}</span>
            </template>
          </a-table-column>
        </template>
      </a-table>
    </a-card>

    <!-- 编辑费率弹窗 -->
    <a-modal 
      v-model:visible="editModalVisible" 
      title="编辑模型费率" 
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
    </a-modal>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
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
</style>
