<template>
  <div class="api-docs-container">
    <div class="page-header">
      <div>
        <h1>API 文档</h1>
        <p>查看 OpenAI / Anthropic 兼容 API 与视频、图片生成接口的详细说明</p>
      </div>
    </div>

    <a-card class="content-card">
      <a-tabs v-model:activeKey="activeTab">
        <!-- OpenAI 兼容 API -->
        <a-tab-pane key="openai" title="OpenAI 兼容 API">
          <div class="doc-section">
            <a-alert type="info" :show-icon="true" class="mb-4">
              <template #title>认证方式</template>
              <div>
                <p>所有 OpenAI 兼容接口都需要在请求头中包含 API Key：</p>
                <code>Authorization: Bearer sk-xxxxxxxxxxxxxxxx</code>
              </div>
            </a-alert>

            <h3>POST /api/v1/chat/completions</h3>
            <p>创建对话补全，支持流式和非流式输出。完全兼容 OpenAI API 格式。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiChatParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiChatRequest }}</code></pre>
            </a-card>

            <h4>响应示例（非流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiChatResponse }}</code></pre>
            </a-card>

            <h4>响应示例（流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiStreamResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">GET /api/v1/models</h3>
            <p>获取可用的模型列表。</p>

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiModelsResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">POST /api/v1/embeddings</h3>
            <p>创建向量嵌入，支持文本、图像、视频等多模态输入。完全兼容 OpenAI API 格式，同时支持复杂的多模态输入结构。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiEmbeddingParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiEmbeddingRequest }}</code></pre>
            </a-card>

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiEmbeddingResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">POST /api/v1/images/generations</h3>
            <p>创建图片生成，支持文生图、图生图、多图生图、交互编辑等功能。完全兼容 OpenAI API 格式，同时支持火山引擎豆包等厂商的扩展参数。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiImageParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiImageRequest }}</code></pre>
            </a-card>

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiImageResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">POST /api/v1/videos/generations/tasks</h3>
            <p>创建视频生成，支持文生视频、图生视频、多图生视频等功能。参考火山引擎豆包API格式，支持多种视频生成参数。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiVideoParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiVideoRequest }}</code></pre>
            </a-card>

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiVideoResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">GET /api/v1/videos/generations/tasks/{task_id}</h3>
            <p>查询视频生成任务状态，获取任务处理进度和最终结果。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiVideoQueryParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiVideoQueryRequest }}</code></pre>
            </a-card>

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiVideoQueryResponse }}</code></pre>
            </a-card>

            <h3 class="mt-4">POST /api/v1/responses</h3>
            <p>创建 Responses 接口，支持流式和非流式输出，兼容 OpenAI Responses API 格式。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="openaiResponsesParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiResponsesRequest }}</code></pre>
            </a-card>

            <h4>响应示例（非流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiResponsesResponse }}</code></pre>
            </a-card>

            <h4>响应示例（流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ openaiResponsesStreamResponse }}</code></pre>
            </a-card>
          </div>
        </a-tab-pane>

        <!-- 视频与图片 API 详解 -->
        <a-tab-pane key="multimodal" title="视频与图片 API 详解">
          <div class="doc-section">
            <a-alert type="info" :show-icon="true" class="mb-4">
              <template #title>认证与模型格式</template>
              <div>
                <p>请求头需携带 API Key：<code>Authorization: Bearer sk-xxxxxxxxxxxxxxxx</code></p>
                <p>model 参数必须为 <b>provider/model_code</b> 格式（如 <code>doubao/doubao-seedance-2-0-260128</code>），缺少 provider 前缀将返回 400 <code>invalid_model_format</code>。视频与图片为真实计费接口，费用由模型单价与用量决定。</p>
              </div>
            </a-alert>

            <h4>接口总览</h4>
            <a-table :columns="endpointColumns" :data="multimodalEndpoints" :pagination="false" bordered size="small" />

            <!-- 1. 创建视频生成任务 -->
            <h3 class="mt-4">1. POST /api/v1/videos/generations/tasks（创建视频生成任务）</h3>
            <p>异步接口：提交后立即返回任务 ID，需通过查询接口轮询任务状态。支持文生视频、图生视频、首尾帧、全模态参考等模式；除下列参数外的其余字段将原样透传给上游厂商。</p>

            <h4>入参说明</h4>
            <a-table :columns="paramColumns" :data="videoCreateParams" :pagination="false" bordered />

            <h4>content 数组元素字段说明</h4>
            <a-table :columns="paramColumns" :data="videoContentItemParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ videoCreateRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明</h4>
            <a-table :columns="respColumns" :data="videoCreateRespFields" :pagination="false" bordered />

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ videoCreateResponse }}</code></pre>
            </a-card>

            <h4>本接口错误码（其余见第 4 节公共错误码）</h4>
            <a-table :columns="errColumns" :data="videoCreateErrors" :pagination="false" bordered size="small" />

            <!-- 2. 查询视频任务 -->
            <h3 class="mt-4">2. GET /api/v1/videos/generations/tasks/{task_id}（查询视频任务）</h3>
            <p>轮询查询视频生成任务状态。查询本身不计费，token 与费用在任务成功后回写创建时生成的账单记录。网关会在响应顶层补充 status / video_url 标准别名（仅当顶层缺失且可从嵌套结构提取时补充，不覆盖上游原始字段），客户端可统一从顶层读取。轮询建议采用阶梯间隔：0–30 秒每 5 秒一次，30–150 秒每 15 秒一次，之后每 30 秒一次。</p>

            <h4>入参说明（Path 与 Query）</h4>
            <a-table :columns="paramColumns" :data="videoQueryParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ videoQueryRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明</h4>
            <a-table :columns="respColumns" :data="videoQueryRespFields" :pagination="false" bordered />

            <h4>响应示例（成功终态）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ videoQueryResponse }}</code></pre>
            </a-card>

            <h4>本接口错误码（其余见第 4 节公共错误码）</h4>
            <a-table :columns="errColumns" :data="videoQueryErrors" :pagination="false" bordered size="small" />

            <!-- 3. 图片生成 -->
            <h3 class="mt-4">3. POST /api/v1/images/generations（图片生成）</h3>
            <p>同步接口（可开启 stream 以 SSE 流式返回），支持文生图、图生图、多图生图与序列生成；除下列参数外的其余字段将原样透传给上游厂商。</p>

            <h4>入参说明</h4>
            <a-table :columns="paramColumns" :data="imageGenParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ imageGenRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明</h4>
            <a-table :columns="respColumns" :data="imageGenRespFields" :pagination="false" bordered />

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ imageGenResponse }}</code></pre>
            </a-card>

            <!-- 4. 统一错误格式 -->
            <h3 class="mt-4">4. 统一错误响应格式</h3>
            <p>三个接口在参数校验失败或上游出错时，均返回 OpenAI 风格的错误信封：</p>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ errorEnvelope }}</code></pre>
            </a-card>

            <h4>错误信封字段</h4>
            <a-table :columns="respColumns" :data="errorEnvelopeFields" :pagination="false" bordered />

            <h4>公共错误码</h4>
            <a-table :columns="errColumns" :data="commonErrors" :pagination="false" bordered size="small" />
          </div>
        </a-tab-pane>

        <!-- Agent 任务 API -->
        <a-tab-pane key="agent" title="Agent 任务 API">
          <div class="doc-section">
            <a-alert type="info" :show-icon="true" class="mb-4">
              <template #title>关于 Agent 任务</template>
              <div>
                <p>Agent 任务是平台的虚拟 Agent 端点：提交任务后由服务端在隔离工作区内运行 <b>SKILL 工具循环</b>（读技能包文件 / 写产物 / 按需生成图片与视频），产出文件可通过资产下载端点取回。认证方式与其余 v1 接口一致：<code>Authorization: Bearer sk-xxxxxxxxxxxxxxxx</code>。</p>
                <p>两个创建入口：<code>POST /v1/agent/tasks</code> 显式指定技能与模型；<code>POST /v1/agent/dispatch</code> 只描述需求，由 LLM 在 enabled 技能目录上做<b>语义路由</b>。执行模型必须为对话型（type=text，依赖 function calling）。</p>
              </div>
            </a-alert>

            <h4>接口总览</h4>
            <a-table :columns="endpointColumns" :data="agentEndpoints" :pagination="false" bordered size="small" />

            <!-- 1. 创建 Agent 任务 -->
            <h3 class="mt-4">1. POST /v1/agent/tasks（创建 Agent 任务）</h3>
            <p>异步接口：校验输入并落库 pending 任务后立即返回（HTTP 202），任务在后台 goroutine 中执行。除 model 与 skills 外的所有字段均为用户输入数据。</p>

            <h4>入参说明</h4>
            <a-table :columns="paramColumns" :data="agentCreateParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentCreateRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明（202 响应）</h4>
            <a-table :columns="respColumns" :data="agentCreateRespFields" :pagination="false" bordered />

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentCreateResponse }}</code></pre>
            </a-card>

            <h4>本接口错误码（其余见第 5 节公共错误码）</h4>
            <a-table :columns="errColumns" :data="agentCreateErrors" :pagination="false" bordered size="small" />

            <!-- 2. 语义路由 dispatch -->
            <h3 class="mt-4">2. POST /v1/agent/dispatch（需求语义路由 + 异步执行）</h3>
            <p>只需描述需求 input；skills 与 model 均可选。未显式指定 skills 时，用路由模型（请求 model 优先，否则服务端 AGENT_ROUTER_MODEL）在 enabled 技能目录上做 LLM 语义路由；目录只有一个技能时直接命中不消耗路由 token。执行模型按四级回退确定：<b>请求 model &gt; 技能声明 model &gt; 路由建议 model &gt; 服务端 AGENT_ROUTER_MODEL</b>，全部为空返回 400 model_required。</p>

            <h4>入参说明</h4>
            <a-table :columns="paramColumns" :data="agentDispatchParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentDispatchRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明（202 响应）</h4>
            <a-table :columns="respColumns" :data="agentDispatchRespFields" :pagination="false" bordered />

            <h4>响应示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentDispatchResponse }}</code></pre>
            </a-card>

            <h4>本接口错误码（其余见第 5 节公共错误码）</h4>
            <a-table :columns="errColumns" :data="agentDispatchErrors" :pagination="false" bordered size="small" />

            <!-- 3. 查询任务 -->
            <h3 class="mt-4">3. GET /v1/agent/tasks/{task_id}（查询任务状态）</h3>
            <p>轮询任务状态与执行明细（仅任务所有者可见）。任务详情包含最终结果、技能使用、资产清单、token 用量与费用（已合并子模型调用 generate_image / generate_video 的消耗），以及按序排列的工具调用步骤 trace。</p>

            <h4>入参说明（Path）</h4>
            <a-table :columns="paramColumns" :data="agentShowParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentShowRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明（200 响应）</h4>
            <a-table :columns="respColumns" :data="agentShowRespFields" :pagination="false" bordered />

            <h4>steps 数组元素字段说明</h4>
            <a-table :columns="respColumns" :data="agentStepFields" :pagination="false" bordered />

            <h4>响应示例（completed 终态）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentShowResponse }}</code></pre>
            </a-card>

            <h4>本接口错误码</h4>
            <a-table :columns="errColumns" :data="agentShowErrors" :pagination="false" bordered size="small" />

            <h4 class="mt-4">任务终态判定与产物契约（长任务实测）</h4>
            <a-alert type="warning" :show-icon="true">
              <template #title>全部步骤成功也可能判为 partial —— 产物契约降级</template>
              <div>
                <p>技能可通过 SKILL.md 声明<b>必需产物清单</b>（outputs）。任务终态时服务端校验工作区实际产出，未满足契约时即使所有步骤 ok、最终总结完整，也会把 completed 降级为 partial，error_message 给出缺失明细（如 <code>missing required outputs: [note.md]</code>）。</p>
                <p><b>实测案例</b>：10 步长任务（10 次 write_asset 全部 ok、总结完整，36s / 47,736 tokens / ¥0.0179）因按指令产出 01-10 编号文件、未生成契约要求的 note.md 而被判 partial；在 input 中补充"最后写入 note.md（执行摘要）"后，同规模任务（11 步 / 58s）终态 completed。</p>
                <p><b>调用方建议</b>：</p>
                <ul>
                  <li>使用带产物契约的技能时，务必在 input 中明确要求产出契约文件（如 note.md），否则任务会因契约缺失降级 partial；</li>
                  <li>max_iterations 建议按预期步骤数 × 1.5 设置：实测每步 ≈ 一次 LLM 调用（3-6s，本地工具 0ms），10 步任务设 15，避免轮次耗尽抢在契约满足前结束；</li>
                  <li>partial 不代表失败：资产已产出可正常下载，按 error_message 提示补产契约文件重新提交即可获得 completed。</li>
                </ul>
              </div>
            </a-alert>

            <!-- 4. 下载资产 -->
            <h3 class="mt-4">4. GET /v1/agent/tasks/{task_id}/assets/{asset_path}（下载任务资产）</h3>
            <p>下载任务工作区产出的资产文件（assets[].rel_path 对应的文件），支持嵌套目录路径。返回文件二进制流而非 JSON，可直接用 curl -OJ 或浏览器下载。</p>

            <h4>入参说明（Path）</h4>
            <a-table :columns="paramColumns" :data="agentAssetParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ agentAssetRequest }}</code></pre>
            </a-card>

            <h4>输出参数说明</h4>
            <a-table :columns="respColumns" :data="agentAssetRespFields" :pagination="false" bordered />

            <h4>本接口错误码</h4>
            <a-table :columns="errColumns" :data="agentAssetErrors" :pagination="false" bordered size="small" />

            <!-- 5. 公共错误码 -->
            <h3 class="mt-4">5. 公共错误码</h3>
            <p>四个接口在鉴权与请求体校验失败时返回统一错误信封（结构与「视频与图片 API 详解」第 4 节相同）：</p>
            <a-table :columns="errColumns" :data="commonErrors" :pagination="false" bordered size="small" />
          </div>
        </a-tab-pane>

        <!-- Anthropic 兼容 API -->
        <a-tab-pane key="anthropic" title="Anthropic 兼容 API">
          <div class="doc-section">
            <a-alert type="info" :show-icon="true" class="mb-4">
              <template #title>认证方式</template>
              <div>
                <p>所有 Anthropic 兼容接口都需要在请求头中包含 API Key：</p>
                <code>x-api-key: sk-xxxxxxxxxxxxxxxx</code>
              </div>
            </a-alert>

            <h3>POST /api/v1/messages</h3>
            <p>发送消息请求，支持流式和非流式输出。兼容 Anthropic API 格式。</p>

            <h4>请求参数</h4>
            <a-table :columns="paramColumns" :data="anthropicMessageParams" :pagination="false" bordered />

            <h4>请求示例</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ anthropicRequest }}</code></pre>
            </a-card>

            <h4>响应示例（非流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ anthropicResponse }}</code></pre>
            </a-card>

            <h4>响应示例（流式）</h4>
            <a-card class="code-card" :bordered="false">
              <pre><code>{{ anthropicStreamResponse }}</code></pre>
            </a-card>
          </div>
        </a-tab-pane>

        <!-- API 密钥管理 -->
        <a-tab-pane key="keys" title="API 密钥管理">
          <div class="doc-section">
            <h3>API 密钥说明</h3>
            <p>API 密钥用于访问 OpenAI 和 Anthropic 兼容的对外接口。每个用户可以在个人中心创建和管理自己的 API 密钥。</p>

            <h4>创建 API 密钥</h4>
            <p>访问 <router-link to="/api-keys">API 密钥管理</router-link> 页面创建新的 API 密钥。</p>

            <h4>使用限制</h4>
            <ul>
              <li>每个 API 密钥都有速率限制</li>
              <li>可以设置密钥的有效期</li>
              <li>支持随时撤销密钥</li>
              <li>使用量会计入账户配额</li>
            </ul>

            <h4>安全提示</h4>
            <a-alert type="warning" :show-icon="true">
              <template #title>保护好您的 API Key</template>
              <div>
                <p>不要在客户端代码中暴露 API Key，也不要将其提交到公共代码仓库。建议在服务端调用 API。</p>
              </div>
            </a-alert>
          </div>
        </a-tab-pane>
      </a-tabs>
    </a-card>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import config from '@/config/api.config.js'

const activeTab = ref('openai')

const paramColumns = [
  { title: '参数名', dataIndex: 'name', width: 150 },
  { title: '类型', dataIndex: 'type', width: 100 },
  { title: '必填', dataIndex: 'required', width: 80 },
  { title: '说明', dataIndex: 'description' },
]

// OpenAI 参数
const openaiChatParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 gpt-4, claude-3-opus 等' },
  { name: 'messages', type: 'array', required: '是', description: '消息数组，包含 role 和 content' },
  { name: 'stream', type: 'boolean', required: '否', description: '是否启用流式输出，默认 false' },
  { name: 'stream_options', type: 'object', required: '否', description: '流式输出选项，{ "include_usage": true }' },
  { name: 'temperature', type: 'number', required: '否', description: '采样温度，0-2 之间，默认 1' },
  { name: 'max_tokens', type: 'integer', required: '否', description: '最大生成 token 数' },
  { name: 'top_p', type: 'number', required: '否', description: '核采样概率，0-1 之间' },
]

const openaiChatRequest = ref(`curl -X POST ${config.api.openai.chat} \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "你好！"}
    ],
    "stream": false
  }'`)

const openaiChatResponse = ref(`{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！有什么我可以帮助你的吗？"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 20,
    "total_tokens": 30
  }
}`)

const openaiStreamResponse = ref(`data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"你好"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"！"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]`)

const openaiModelsResponse = ref(`{
  "object": "list",
  "data": [
    {
      "id": "gpt-4",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    },
    {
      "id": "claude-3-opus",
      "object": "model",
      "created": 1677610602,
      "owned_by": "anthropic"
    },
    {
      "id": "text-embedding-3-small",
      "object": "model",
      "created": 1677610602,
      "owned_by": "openai"
    }
  ]
}`)

// OpenAI 向量嵌入参数
const openaiEmbeddingParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 text-embedding-3-small 等' },
  { name: 'input', type: 'string|array', required: '是', description: '输入文本或文本数组，也可以是图像数据' },
  { name: 'encoding_format', type: 'string', required: '否', description: '编码格式，可选 float 或 base64，默认 float' },
  { name: 'dimensions', type: 'integer', required: '否', description: '向量维度，仅支持部分模型' },
  { name: 'modality', type: 'string', required: '否', description: '模态类型，可选 text 或 image' },
]

const openaiEmbeddingRequest = ref(`# 简单文本示例
curl -X POST ${config.api.openai.base}/embeddings \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "text-embedding-3-small",
    "input": "你好，这是一段测试文本",
    "encoding_format": "float"
  }'

# 多模态示例（支持视频、图片、文本混合）
curl -X POST ${config.api.openai.base}/embeddings \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-embedding-vision-251215",
    "encoding_format": "float",
    "input": [
      {
        "type": "video_url",
        "video_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_video/ark_vlm_video_input.mp4"
        }
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/tower.png"
        }
      },
      {
        "type": "text",
        "text": "视频和图片里有什么"
      }
    ]
  }'`)

const openaiEmbeddingResponse = ref(`{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [
        0.0023064255,
        -0.009327292,
        0.0045678921,
        -0.0023456789,
        0.0012345678
      ]
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 5,
    "total_tokens": 5
  }
}`)

// OpenAI 图片生成参数
const openaiImageParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 doubao-seedream-5.0-pro-260628 等' },
  { name: 'prompt', type: 'string', required: '是', description: '提示词，描述想要生成的图片内容' },
  { name: 'image', type: 'string|array', required: '否', description: '参考图片URL或URL数组，用于图生图或多图生图' },
  { name: 'size', type: 'string', required: '否', description: '图片尺寸，如 1k, 1.5k, 2k, auto, 3k, 4k' },
  { name: 'output_format', type: 'string', required: '否', description: '输出格式，png 或 jpeg，默认 png' },
  { name: 'watermark', type: 'boolean', required: '否', description: '是否添加水印，默认 false' },
  { name: 'sequential_image_generation', type: 'string', required: '否', description: '序列生成模式，auto 或 disabled，多图生图时使用' },
  { name: 'sequential_image_generation_options', type: 'object', required: '否', description: '序列生成选项，如 { "max_images": 3 }' },
]

const openaiImageRequest = ref(`# 文生图示例
curl -X POST ${config.api.openai.base}/images/generations \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedream-5-0-pro-260628",
    "prompt": "充满活力的特写编辑肖像，模特眼神犀利，头戴雕塑感帽子，色彩拼接丰富，眼部焦点锐利，景深较浅，具有Vogue杂志封面的美学风格，采用中画幅拍摄，工作室灯光效果强烈",
    "size": "2k",
    "output_format": "png",
    "watermark": false
  }'

# 图生图示例
curl -X POST ${config.api.openai.base}/images/generations \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedream-5-0-pro-260628",
    "prompt": "保持模特姿势和液态服装的流动形状不变，将服装材质从银色金属改为完全透明的清水，透过液态水流，可以看到模特的皮肤细节，光影从反射变为折射",
    "image": "https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_5_image_to_image.png",
    "size": "2k",
    "output_format": "png",
    "watermark": false
  }'

# 多图生图示例
curl -X POST ${config.api.openai.base}/images/generations \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedream-5-0-260128",
    "prompt": "生成3张女孩和奶牛玩偶在游乐园开心地坐过山车的图片，涵盖早晨、中午、晚上",
    "image": [
      "https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_images_to_images_1.png",
      "https://ark-project.tos-cn-beijing.volces.com/doc_image/seedream4_images_to_images_2.png"
    ],
    "sequential_image_generation": "auto",
    "sequential_image_generation_options": {
      "max_images": 3
    },
    "size": "2k",
    "output_format": "png",
    "watermark": false
  }'`)

const openaiImageResponse = ref(`{
  "model": "doubao-seedream-5-0-pro-260628",
  "created": 1789448654,
  "data": [
    {
      "url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-5-0-pro/82c3671e7ee33ee0f9eed02771d89bf481e0de855906844987120/Image/2105959667/021789448609558ed0e184fb98d17720dee9f0ee33ee7e1763c28.png?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20260915%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20260915T050414Z&X-Tos-Expires=86400&X-Tos-Signature=6bd5a57074511a31fc8d7bfa086ee4a00d50a3dc02a2e570b368be3e753cd35f&X-Tos-SignedHeaders=host",
      "size": "1024x1024",
      "output_format": "png"
    }
  ],
  "usage": {
    "input_images": 1,
    "generated_images": 1,
    "output_tokens": 4096,
    "total_tokens": 4096
  }
}`)

// OpenAI 视频生成参数
const openaiVideoParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 doubao-seedance-1-0-pro-250528、doubao-seedance-1-0-lite-i2v-250428、doubao-seedance-1-5-pro-251215、doubao-seedance-2-5-260628 等' },
  { name: 'content', type: 'array', required: '是', description: '内容数组，支持文本、图片、视频等多模态输入，每个元素包含type和对应的内容' },
  { name: 'ratio', type: 'string', required: '否', description: '视频比例，如 16:9, 9:16, 1:1, adaptive 等，adaptive表示自适应' },
  { name: 'duration', type: 'integer', required: '否', description: '视频时长，如 5, 10, 15, 30 秒，默认 5' },
  { name: 'output_format', type: 'string', required: '否', description: '输出格式，如 mp4, mov, webm' },
  { name: 'watermark', type: 'boolean', required: '否', description: '是否添加水印，默认 false' },
  { name: 'generate_audio', type: 'boolean', required: '否', description: '是否生成音频，默认 false' },
  { name: 'omni_reference_task_type', type: 'string', required: '否', description: '全模态参考任务类型，如 reference，仅在多模态参考时使用' },
]

const openaiVideoRequest = ref(`# 文生视频示例
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedance-1-0-pro-250528",
    "content": [
      {
        "type": "text",
        "text": "写实风格，晴朗的蓝天之下，一大片白色的雏菊花田，镜头逐渐拉近，最终定格在一朵雏菊花的特写上，花瓣上有几颗晶莹的露珠"
      }
    ],
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
  }'

# 图生视频示例
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedance-1-0-lite-i2v-250428",
    "content": [
      {
        "type": "text",
        "text": "女孩抱着狐狸，女孩睁开眼，温柔地看向镜头，狐狸友善地抱着，镜头缓缓拉出，女孩的头发被风吹动"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/i2v_foxrgirl.png"
        }
      }
    ],
    "ratio": "adaptive",
    "duration": 5,
    "watermark": false
  }'

# 有声视频-首尾帧示例
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedance-1-5-pro-251215",
    "content": [
      {
        "type": "text",
        "text": "图中女孩对着镜头说“茄子”，360度环绕运镜"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/seepro_first_frame.jpeg"
        },
        "role": "first_frame"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/seepro_last_frame.jpeg"
        },
        "role": "last_frame"
      }
    ],
    "generate_audio": true,
    "ratio": "adaptive",
    "duration": 5,
    "watermark": false
  }'

# 有声视频首帧示例
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedance-1-5-pro-251215",
    "content": [
      {
        "type": "text",
        "text": "女孩抱着狐狸，女孩睁开眼，温柔地看向镜头，狐狸友善地抱着，镜头缓缓拉出，女孩的头发被风吹动，可以听到风声"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://ark-project.tos-cn-beijing.volces.com/doc_image/i2v_foxrgirl.png"
        }
      }
    ],
    "generate_audio": true,
    "ratio": "adaptive",
    "duration": 5,
    "watermark": false
  }'

# 全模态参考示例
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seedance-2-5-260628",
    "content": [
      {
        "type": "text",
        "text": "明亮多彩的广告片风格，果味饼干为主角，包含草莓、苹果、葡萄、橙子四种口味，草莓味参考@图像1，饼干与对应水果以强秩序感的几何阵列排布，整体画面干净、高级、节奏强。"
      },
      {
        "type": "image_url",
        "image_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/images/video-generation/seedance2.5_reference1.png"
        },
        "role": "reference_image"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference2.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference3.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference4.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference5.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference6.mp4"
        },
        "role": "reference_video"
      },
      {
        "type": "video_url",
        "video_url": {
          "url": "https://arkdocs.tos-cn-beijing.volces.com/videos/video-generation/seedance2.5_reference7.mp4"
        },
        "role": "reference_video"
      }
    ],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 15,
    "omni_reference_task_type": "reference",
    "output_format": "mov"
  }'`)

const openaiVideoResponse = ref(`{
  "id": "cgt-20260915101121-922bi"
}`)

// OpenAI 视频查询参数
const openaiVideoQueryParams = [
  { name: 'task_id', type: 'string', required: '是', description: '视频生成任务ID，从创建任务接口返回的id字段获取' },
]

const openaiVideoQueryRequest = ref(`curl -X GET ${config.api.openai.base}/videos/generations/tasks/cgt-20260915101121-922bi \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx"`)

const openaiVideoQueryResponse = ref(`{
  "id": "cgt-20260915101121-922bi",
  "model": "doubao-seedance-2-5-260628",
  "status": "succeeded",
  "content": {
    "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/doubao-seedance-2-5/2105959667/Video/ib229-12110151906202-tgc_18a953b2b391caffff00000000000000000000747271244987120/cgt-20260915101121-922bi.mp4?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20260915%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20260915T031904Z&X-Tos-Expires=86400&X-Tos-Max-Requests=100&X-Tos-Signature=25bd5a57074511a31fc8d7bfa086ee4a00d50a3dc02a2e570b368be3e753cd35f&X-Tos-SignedHeaders=host"
  },
  "usage": {
    "completion_tokens": 108900,
    "total_tokens": 108900
  },
  "created_at": 1789438281,
  "updated_at": 1789442348,
  "seed": 87334,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "framespersecond": 24,
  "service_tier": "default",
  "execution_expires_after": 172800,
  "generate_audio": true,
  "draft": false,
  "priority": 0,
  "output_format": "mp4"
}`)

// OpenAI Responses 参数
const openaiResponsesParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 gpt-4, claude-3-opus, doubao-seed-2-0-pro-260628 等' },
  { name: 'input', type: 'string|array', required: '是', description: '输入内容，可以是字符串或消息数组' },
  { name: 'stream', type: 'boolean', required: '否', description: '是否启用流式输出，默认 false' },
  { name: 'max_output_tokens', type: 'integer', required: '否', description: '最大输出 token 数' },
  { name: 'temperature', type: 'number', required: '否', description: '采样温度，0-2 之间，默认 1' },
  { name: 'top_p', type: 'number', required: '否', description: '核采样概率，0-1 之间' },
]

const openaiResponsesRequest = ref(`# 简单输入示例
curl -X POST ${config.api.openai.responses} \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seed-2-0-pro-260628",
    "input": "你好！",
    "max_output_tokens": 1024,
    "stream": false
  }'

# 消息数组示例
curl -X POST ${config.api.openai.responses} \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao-seed-2-0-pro-260628",
    "input": [
      {"role": "system", "content": "你是一个助手"},
      {"role": "user", "content": "你好！"}
    ],
    "max_output_tokens": 1024,
    "stream": true
  }'`)

const openaiResponsesResponse = ref(`{
  "id": "resp_021789525135711137f500080bf18603e76d17d12e7c43ed19398",
  "created_at": 1789525136,
  "model": "doubao-seed-2-0-pro-260628",
  "object": "response",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "你好！很高兴见到你。有什么我可以帮助你的吗？"
        }
      ],
      "status": "completed"
    }
  ],
  "status": "completed",
  "usage": {
    "input_tokens": 35,
    "output_tokens": 88,
    "total_tokens": 123,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens_details": {
      "reasoning_tokens": 71
    }
  }
}`)

const openaiResponsesStreamResponse = ref(`data: {"type":"response.created","response":{"id":"resp_021789525135711137f500080bf18603e76d17d12e7c43ed19398","created_at":1789525136,"model":"doubao-seed-2-0-pro-260628","object":"response","status":"in_progress"}}

data: {"type":"output_text.delta","delta":{"type":"output_text.delta","text":"你好"}}

data: {"type":"output_text.delta","delta":{"type":"output_text.delta","text":"！很高兴见到你"}}

data: {"type":"output_text.delta","delta":{"type":"output_text.delta","text":"。有什么我可以帮助你的吗"}}

data: {"type":"response.completed","response":{"id":"resp_021789525135711137f500080bf18603e76d17d12e7c43ed19398","created_at":1789525136,"model":"doubao-seed-2-0-pro-260628","object":"response","status":"completed","usage":{"input_tokens":35,"output_tokens":88,"total_tokens":123}},"sequence_number":99}

data: [DONE]`)

// ============ 视频与图片 API 详解（独立面板） ============

const endpointColumns = [
  { title: '接口', dataIndex: 'name', width: 170 },
  { title: '方法', dataIndex: 'method', width: 70 },
  { title: '路径', dataIndex: 'path' },
  { title: '计费方式', dataIndex: 'billing', width: 260 },
]

const multimodalEndpoints = [
  { name: '创建视频生成任务', method: 'POST', path: '/api/v1/videos/generations/tasks', billing: '异步任务；任务成功后按视频生成 token 计费' },
  { name: '查询视频生成任务', method: 'GET', path: '/api/v1/videos/generations/tasks/{task_id}', billing: '查询本身不计费；成功后回写任务账单' },
  { name: '图片生成', method: 'POST', path: '/api/v1/images/generations', billing: '同步；按生成图片的 token 计费' },
]

// 输出参数与错误信封字段共用的列定义
const respColumns = [
  { title: '字段', dataIndex: 'name', width: 200 },
  { title: '类型', dataIndex: 'type', width: 110 },
  { title: '说明', dataIndex: 'description' },
]

const errColumns = [
  { title: 'HTTP 状态码', dataIndex: 'status', width: 120 },
  { title: 'code', dataIndex: 'code', width: 220 },
  { title: '说明', dataIndex: 'description' },
]

// ---- 1. 创建视频生成任务 ----
const videoCreateParams = [
  { name: 'model', type: 'string', required: '是', description: '模型标识，provider/model_code 格式，如 doubao/doubao-seedance-2-0-260128、doubao/doubao-seedance-1-5-pro-251215 等' },
  { name: 'prompt', type: 'string', required: '与 content 二选一', description: '文生视频提示词（网关标准字段）。与 content 同时传入时返回 400 prompt_content_conflict；类型不是字符串时返回 400 invalid_prompt_type' },
  { name: 'content', type: 'array', required: '与 prompt 二选一', description: '豆包原生多模态内容数组（透传字段），用于图生视频、首尾帧、全模态参考等场景，元素结构见下表' },
  { name: 'ratio', type: 'string', required: '否', description: '视频比例：16:9、9:16、1:1、adaptive（自适应）等' },
  { name: 'duration', type: 'integer', required: '否', description: '视频时长（秒）：5、10、15 等，默认 5，以上游模型支持为准' },
  { name: 'output_format', type: 'string', required: '否', description: '输出格式：mp4、mov、webm，默认 mp4' },
  { name: 'watermark', type: 'boolean', required: '否', description: '是否添加水印，默认 false' },
  { name: 'generate_audio', type: 'boolean', required: '否', description: '是否生成音频，默认 false，仅部分模型支持' },
  { name: 'omni_reference_task_type', type: 'string', required: '否', description: '全模态参考任务类型，如 reference，仅全模态参考（seedance-2.5）场景使用' },
]

const videoContentItemParams = [
  { name: 'type', type: 'string', required: '是', description: '元素类型：text（提示词文本）/ image_url（参考图片）/ video_url（参考视频）' },
  { name: 'text', type: 'string', required: 'type=text 时必填', description: '提示词文本，描述运镜、画面与声音等内容' },
  { name: 'image_url', type: 'object', required: 'type=image_url 时必填', description: '图片引用，形如 { "url": "https://..." }' },
  { name: 'video_url', type: 'object', required: 'type=video_url 时必填', description: '视频引用，形如 { "url": "https://..." }' },
  { name: 'role', type: 'string', required: '否', description: '元素角色：first_frame（首帧）/ last_frame（尾帧）/ reference_image（参考图）/ reference_video（参考视频），全模态参考在提示词中以 @图像1 等标记引用' },
]

const videoCreateRequest = ref(`# content 多模态方式（图生视频 + 首帧）
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao/doubao-seedance-2-0-260128",
    "content": [
      { "type": "text", "text": "写实风格，晴朗的蓝天之下，一大片白色的雏菊花田，镜头逐渐拉近" },
      { "type": "image_url", "image_url": { "url": "https://example.com/first_frame.png" }, "role": "first_frame" }
    ],
    "ratio": "16:9",
    "duration": 5,
    "generate_audio": true,
    "watermark": false
  }'

# prompt 标准字段方式（文生视频，与 content 互斥）
curl -X POST ${config.api.openai.base}/videos/generations/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao/doubao-seedance-2-0-260128",
    "prompt": "一只橘猫在草地上奔跑，阳光明媚，5 秒镜头",
    "duration": 5
  }'`)

const videoCreateRespFields = [
  { name: 'id', type: 'string', description: '视频生成任务 ID（cgt- 前缀），请妥善保存，用于后续轮询查询' },
]

const videoCreateResponse = ref(`{
  "id": "cgt-20261025081840-np4kk"
}`)

const videoCreateErrors = [
  { status: '400', code: 'prompt_content_conflict', description: 'prompt 与 content 同时传入，两者语义互斥，必须只传其一' },
  { status: '400', code: 'invalid_prompt_type', description: 'prompt 字段存在但不是字符串' },
]

// ---- 2. 查询视频任务 ----
const videoQueryParams = [
  { name: 'task_id', type: 'string (path)', required: '是', description: '视频任务 ID，来自创建接口返回的 id 字段' },
  { name: 'record_id', type: 'string (query)', required: '否', description: '本平台 API 调用记录 ID；提供后任务成功时网关回写该记录与计费信息；非本人记录按 404 record_not_found 处理（防越权）' },
]

const videoQueryRequest = ref(`curl -X GET "${config.api.openai.base}/videos/generations/tasks/cgt-20261025081840-np4kk?record_id=485" \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx"`)

const videoQueryRespFields = [
  { name: 'id', type: 'string', description: '任务 ID，与请求的 task_id 一致' },
  { name: 'model', type: 'string', description: '生成使用的上游模型编码' },
  { name: 'status', type: 'string', description: '任务状态，常见取值：queued（排队中）/ running（生成中）/ succeeded（成功，终态）/ failed（失败，终态），其余以上游返回为准' },
  { name: 'video_url', type: 'string', description: '生成视频下载地址（顶层标准别名；上游嵌套在 content.video_url 时由网关归一化补充）。为厂商签名 URL，有效期有限，请及时转存' },
  { name: 'content', type: 'object', description: '上游原始结构，豆包返回 { "video_url": "..." }' },
  { name: 'usage', type: 'object', description: 'token 用量：completion_tokens / total_tokens（视频按生成时长折算 token 计费）' },
  { name: 'created_at', type: 'integer', description: '任务创建时间（Unix 秒）' },
  { name: 'updated_at', type: 'integer', description: '任务最近更新时间（Unix 秒）' },
  { name: 'seed', type: 'integer', description: '本次生成的随机种子' },
  { name: 'resolution', type: 'string', description: '分辨率，如 720p、1080p' },
  { name: 'ratio', type: 'string', description: '视频比例，如 16:9' },
  { name: 'duration', type: 'integer', description: '实际时长（秒）' },
  { name: 'framespersecond', type: 'integer', description: '帧率，如 24' },
  { name: 'service_tier', type: 'string', description: '服务层级，如 default' },
  { name: 'execution_expires_after', type: 'integer', description: '任务结果保留时长（秒），如 172800（48 小时）' },
  { name: 'generate_audio', type: 'boolean', description: '是否生成了音频' },
  { name: 'draft', type: 'boolean', description: '是否草稿模式' },
  { name: 'priority', type: 'integer', description: '任务优先级' },
  { name: 'output_format', type: 'string', description: '输出格式，如 mp4' },
]

const videoQueryResponse = ref(`{
  "id": "cgt-20261025081840-np4kk",
  "model": "doubao-seedance-2-0-260128",
  "status": "succeeded",
  "content": {
    "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/.../cgt-20261025081840-np4kk.mp4?X-Tos-Expires=86400&X-Tos-Signature=..."
  },
  "video_url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/.../cgt-20261025081840-np4kk.mp4?X-Tos-Expires=86400&X-Tos-Signature=...",
  "usage": {
    "completion_tokens": 108900,
    "total_tokens": 108900
  },
  "created_at": 1789438281,
  "updated_at": 1789442348,
  "seed": 87334,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "framespersecond": 24,
  "service_tier": "default",
  "execution_expires_after": 172800,
  "generate_audio": true,
  "draft": false,
  "priority": 0,
  "output_format": "mp4"
}`)

const videoQueryErrors = [
  { status: '404', code: 'task_not_found', description: '任务不存在（含上游返回 404 的场景）' },
  { status: '404', code: 'record_not_found', description: 'record_id 对应记录不存在或不属于当前用户' },
  { status: '500', code: 'model_unresolved', description: '无法确定任务对应的模型或厂商（网关侧配置缺失）' },
  { status: '500', code: 'adapter_creation_failed', description: '适配器创建失败' },
  { status: '500', code: 'task_query_failed', description: '上游任务查询失败' },
]

// ---- 3. 图片生成 ----
const imageGenParams = [
  { name: 'model', type: 'string', required: '是', description: '模型标识，provider/model_code 格式，如 doubao/doubao-seedream-4-5-251128、doubao/doubao-seedream-5-0-pro-260628 等' },
  { name: 'prompt', type: 'string', required: '是', description: '提示词，描述想要生成的图片内容' },
  { name: 'image', type: 'string|array', required: '否', description: '参考图片 URL 或 URL 数组，用于图生图或多图生图' },
  { name: 'size', type: 'string', required: '否', description: '图片尺寸：1k、1.5k、2k、3k、4k、auto，默认以上游模型为准；响应中 data[].size 为实际像素（如 1024x1024）' },
  { name: 'output_format', type: 'string', required: '否', description: '输出格式：png 或 jpeg，默认 png' },
  { name: 'watermark', type: 'boolean', required: '否', description: '是否添加水印，默认 false' },
  { name: 'sequential_image_generation', type: 'string', required: '否', description: '序列生成模式：auto 或 disabled，多图组图生成时使用' },
  { name: 'sequential_image_generation_options', type: 'object', required: '否', description: '序列生成选项，如 { "max_images": 3 }' },
  { name: 'stream', type: 'boolean', required: '否', description: '是否以 SSE 流式返回，默认 false（同步 JSON）' },
]

const imageGenRequest = ref(`curl -X POST ${config.api.openai.base}/images/generations \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "doubao/doubao-seedream-4-5-251128",
    "prompt": "一只戴圣诞帽的橘猫，简约扁平插画风格",
    "size": "2k",
    "output_format": "png",
    "watermark": false
  }'`)

const imageGenRespFields = [
  { name: 'model', type: 'string', description: '实际使用的上游模型编码' },
  { name: 'created', type: 'integer', description: '生成时间（Unix 秒）' },
  { name: 'data', type: 'array', description: '生成结果数组，每张图片一个元素' },
  { name: 'data[].url', type: 'string', description: '图片下载地址（厂商签名 URL，有效期有限，请及时转存）' },
  { name: 'data[].size', type: 'string', description: '实际像素尺寸，如 1024x1024' },
  { name: 'data[].output_format', type: 'string', description: '图片格式，如 png' },
  { name: 'usage', type: 'object', description: '用量统计：input_images（输入参考图数量）/ generated_images（生成图片数量）/ output_tokens / total_tokens（图片按 token 计费）' },
]

const imageGenResponse = ref(`{
  "model": "doubao-seedream-4-5-251128",
  "created": 1789448654,
  "data": [
    {
      "url": "https://ark-acg-cn-beijing.tos-cn-beijing.volces.com/.../021789448609558ed0e184fb98d17720dee9f0ee33ee7e1763c28.png?X-Tos-Expires=86400&X-Tos-Signature=...",
      "size": "1024x1024",
      "output_format": "png"
    }
  ],
  "usage": {
    "input_images": 1,
    "generated_images": 1,
    "output_tokens": 4096,
    "total_tokens": 4096
  }
}`)

// ---- 4. 统一错误响应格式 ----
const errorEnvelope = ref(`{
  "error": {
    "message": "'prompt' and 'content' are mutually exclusive: 'prompt' is the standard field, 'content' is the native passthrough field; use exactly one of them",
    "type": "invalid_request_error",
    "param": "prompt",
    "code": "prompt_content_conflict"
  }
}`)

const errorEnvelopeFields = [
  { name: 'error.message', type: 'string', description: '人类可读的错误描述' },
  { name: 'error.type', type: 'string', description: '错误类型：invalid_request_error（请求参数问题）/ api_error（网关或上游内部错误）' },
  { name: 'error.param', type: 'string', description: '出错的参数名，可能缺失' },
  { name: 'error.code', type: 'string', description: '机器可读的错误码，见上方各接口错误码与公共错误码表' },
]

const commonErrors = [
  { status: '401', code: 'missing_api_key', description: '请求头缺少 Authorization: Bearer <API Key>' },
  { status: '401', code: 'invalid_api_key', description: 'API Key 无效或已被撤销' },
  { status: '400', code: 'invalid_body', description: '请求体缺失或不是合法 JSON' },
  { status: '400', code: 'invalid_model_format', description: 'model 缺少 provider/ 前缀，必须为 provider/model_code 格式' },
  { status: '404', code: 'provider_not_found', description: 'model 中 provider 部分不存在或未启用' },
  { status: '404', code: 'model_not_found', description: 'model_code 部分不存在、未启用或不属于该 provider' },
]

// ============ Agent 任务 API（独立面板） ============

const agentEndpoints = [
  { name: '创建 Agent 任务', method: 'POST', path: '/v1/agent/tasks', billing: '按执行模型对话 token 计费；子模型调用（图片/视频）独立落账' },
  { name: '需求语义路由 + 执行', method: 'POST', path: '/v1/agent/dispatch', billing: '路由消耗 + 执行 token 计费' },
  { name: '查询 Agent 任务', method: 'GET', path: '/v1/agent/tasks/{task_id}', billing: '查询不计费' },
  { name: '下载任务资产', method: 'GET', path: '/v1/agent/tasks/{task_id}/assets/{asset_path}', billing: '不计费' },
]

// ---- 1. 创建 Agent 任务 ----
const agentCreateParams = [
  { name: 'model', type: 'string', required: '是', description: '执行模型，provider/model_code 格式；必须为对话型模型（type=text，Agent 循环依赖 function calling），否则 400 model_not_supported' },
  { name: 'skills', type: 'array<string>', required: '是（至少 1 个）', description: '要装载的 SKILL 名称列表（SKILL.md frontmatter 的 name），仅 enabled 技能；不存在或禁用时 400 skills_not_found' },
  { name: 'input', type: 'string', required: '是', description: '用户输入数据（任务描述/自由文本），作为任务正文落库' },
  { name: 'max_iterations', type: 'integer', required: '否', description: 'Agent 循环轮次上限，默认 100；≤0 归一为 100，最大 100（超出截断，防失控扣费）。长任务建议按预期步骤数 × 1.5 设置：实测 10 步任务耗 10 轮、每步约一次 LLM 调用（3-6s）' },
  { name: 'options', type: 'object', required: '否', description: '透传给执行模型的额外参数（如 temperature）' },
]

const agentCreateRequest = ref(`curl -X POST ${config.api.openai.base}/agent/tasks \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "volcengine/doubao-seed-1-8-251228",
    "skills": ["podcast-video-producer"],
    "input": "把下面的大纲做成一集播客视频：……",
    "max_iterations": 12,
    "options": { "temperature": 0.7 }
  }'`)

const agentCreateRespFields = [
  { name: 'task_id', type: 'string', description: '任务 ID（agt_ 前缀），用于轮询与资产下载' },
  { name: 'status', type: 'string', description: '初始状态，固定为 pending' },
  { name: 'poll_url', type: 'string', description: '轮询地址 /v1/agent/tasks/{task_id}' },
]

const agentCreateResponse = ref(`HTTP/1.1 202 Accepted

{
  "task_id": "agt_20261025120000_1a2b3c4d5e6f7788",
  "status": "pending",
  "poll_url": "/v1/agent/tasks/agt_20261025120000_1a2b3c4d5e6f7788"
}`)

const agentCreateErrors = [
  { status: '400', code: 'model_required', description: '缺失 model（param=model）' },
  { status: '400', code: 'skills_required', description: 'skills 缺失或为空数组（至少 1 个）' },
  { status: '400', code: 'input_required', description: '缺失 input（任务描述/自由文本）' },
  { status: '400', code: 'invalid_json', description: '请求体不是合法 JSON 或字段类型不匹配（param=body）' },
  { status: '400', code: 'invalid_model_format', description: 'model 未带 provider/ 前缀' },
  { status: '400', code: 'skills_not_found', description: '技能名称不存在或未启用，报错信息列出缺失名称' },
  { status: '404', code: 'model_not_found', description: 'model 在模型目录中不存在（如 doubao/not-exist-model）' },
  { status: '400', code: 'model_not_supported', description: '模型存在但不是对话型（type≠text），Agent 循环依赖 function calling' },
  { status: '500', code: 'record_create_failed', description: '计费初始记录创建失败，任务置为 failed' },
]

// ---- 2. 需求语义路由 dispatch ----
const agentDispatchParams = [
  { name: 'input', type: 'string', required: '是', description: '用户需求描述（自由文本），用于语义路由与任务正文' },
  { name: 'model', type: 'string', required: '否', description: 'provider/model_code；提供时优先用作路由模型与执行模型。执行模型四级回退：请求 model > 技能声明 model > 路由建议 model > 服务端 AGENT_ROUTER_MODEL 配置，全空返回 400 model_required' },
  { name: 'skills', type: 'array<string>', required: '否', description: '显式指定 SKILL 名称（提供时跳过 LLM 语义路由直接装载）' },
  { name: 'max_iterations', type: 'integer', required: '否', description: '同创建接口：默认 100，上限 100' },
  { name: 'options', type: 'object', required: '否', description: '透传给执行模型的额外参数' },
]

const agentDispatchRequest = ref(`curl -X POST ${config.api.openai.base}/agent/dispatch \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "input": "帮我生成一张赛博朋克风格的城市夜景图，并配一段宣传文案",
    "max_iterations": 10
  }'
# 不传 skills/model：由路由模型在 enabled 技能目录上做 LLM 语义路由`)

const agentDispatchRespFields = [
  { name: 'task_id', type: 'string', description: '任务 ID（agt_ 前缀）' },
  { name: 'status', type: 'string', description: '初始状态，固定为 pending' },
  { name: 'poll_url', type: 'string', description: '轮询地址 /v1/agent/tasks/{task_id}' },
  { name: 'routed', type: 'object', description: '语义路由决策：skills（选中的技能名数组）/ model（最终执行模型 provider/model）/ reason（一句话路由理由）。显式传入 skills 时同样返回，reason 为 explicit skills provided' },
]

const agentDispatchResponse = ref(`HTTP/1.1 202 Accepted

{
  "task_id": "agt_20261025120100_9f8e7d6c5b4a3210",
  "status": "pending",
  "poll_url": "/v1/agent/tasks/agt_20261025120100_9f8e7d6c5b4a3210",
  "routed": {
    "skills": ["podcast-video-producer"],
    "model": "volcengine/doubao-seed-1-8-251228",
    "reason": "需求涉及视频制作与文案生成"
  }
}`)

const agentDispatchErrors = [
  { status: '400', code: 'input_required', description: '缺失 input（用户需求描述）' },
  { status: '400', code: 'invalid_json', description: '请求体不是合法 JSON 或字段类型不匹配（param=body）' },
  { status: '400', code: 'no_skills_available', description: '平台当前没有 enabled 技能，无法路由' },
  { status: '400', code: 'model_required', description: '四级回退后仍无执行模型：未传 model、技能未声明、路由未建议且服务端未配置 AGENT_ROUTER_MODEL' },
  { status: '400', code: 'router_model_missing', description: '未传 model 且服务端未配置 AGENT_ROUTER_MODEL，无法路由' },
  { status: '400', code: 'skills_not_found', description: '显式 skills 中存在不存在或未启用的名称' },
  { status: '404', code: 'model_not_found', description: '回退确定的执行模型在模型目录中不存在' },
  { status: '400', code: 'model_not_supported', description: '执行模型存在但 type≠text，Agent 循环依赖 function calling' },
  { status: '502', code: 'router_failed', description: '语义路由失败（路由模型无可用 Key、适配器创建失败或上游调用出错）' },
]

// ---- 3. 查询任务 ----
const agentShowParams = [
  { name: 'task_id', type: 'string (path)', required: '是', description: '任务 ID（agt_ 前缀）；仅任务所有者可见，否则 404' },
]

const agentShowRequest = ref(`curl "${config.api.openai.base}/agent/tasks/agt_20261025120000_1a2b3c4d5e6f7788" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx"`)

const agentShowRespFields = [
  { name: 'id', type: 'integer', description: '任务内部自增 ID（数据库主键，与对外 task_id 不同）' },
  { name: 'task_id', type: 'string', description: '对外任务 ID（agt_ 前缀）' },
  { name: 'user_id', type: 'integer', description: '所属用户 ID' },
  { name: 'api_key_id', type: 'integer', description: '发起任务的 API Key ID，可能缺失' },
  { name: 'model_id', type: 'integer', description: '执行模型的内部 ID' },
  { name: 'provider_id', type: 'integer', description: '执行模型厂商的内部 ID' },
  { name: 'task', type: 'string', description: '任务正文（创建时的 input）' },
  { name: 'skill_ids', type: 'array<integer>', description: '装载技能的内部 ID 数组，可能缺失' },
  { name: 'max_iterations', type: 'integer', description: '轮次上限（归一化后的实际值）' },
  { name: 'status', type: 'string', description: '任务状态：pending（排队）/ running（执行中）/ completed（成功）/ partial（有资产产出但无最终总结：循环耗尽或必需产物缺失时降级）/ failed（失败）' },
  { name: 'iterations', type: 'integer', description: '已执行的 Agent 循环轮次' },
  { name: 'result', type: 'string', description: '模型最终回答（completed 时），可能缺失' },
  { name: 'error_message', type: 'string', description: '失败/降级原因（partial 时为缺失必需产物明细），可能缺失' },
  { name: 'skills_used', type: 'array<object>', description: '实际使用的技能：id / name / loaded_files（通过工具实际读取的包内文件列表），可能缺失' },
  { name: 'assets', type: 'array<object>', description: '工作区资产清单：name（文件名）/ size（字节）/ mime（MIME 类型）/ rel_path（工作区内相对路径，用于下载端点），可能缺失' },
  { name: 'prompt_tokens', type: 'integer', description: '提示 token 用量（已合并子模型调用 generate_image/generate_video 的用量）' },
  { name: 'completion_tokens', type: 'integer', description: '补全 token 用量（已合并子模型调用用量）' },
  { name: 'total_tokens', type: 'integer', description: '总 token 用量（执行模型 + 子模型）' },
  { name: 'cost', type: 'number', description: '本次任务总费用（元）' },
  { name: 'started_at', type: 'string', description: '开始执行时间（ISO 8601），可能缺失' },
  { name: 'completed_at', type: 'string', description: '完成时间（ISO 8601），可能缺失' },
  { name: 'created_at / updated_at', type: 'string', description: '记录创建/更新时间（ISO 8601）' },
  { name: 'steps', type: 'array<object>', description: '工具调用步骤 trace（按 seq 升序），元素结构见下表' },
]

const agentStepFields = [
  { name: 'id', type: 'integer', description: '步骤记录自增 ID' },
  { name: 'task_id', type: 'string', description: '所属任务 ID' },
  { name: 'seq', type: 'integer', description: '任务内步序（跨轮累计，1 起）；seq=0 为语义路由 step（tool=skill_router，output_excerpt 为路由决策 JSON）' },
  { name: 'iteration', type: 'integer', description: '所属 Agent 循环轮次（路由 step 为 0）' },
  { name: 'tool', type: 'string', description: '工具名：read_skill_file（读技能包文件）/ execute_script（受控执行脚本）/ write_asset（写产物）/ generate_image / generate_video（按需子模型调用）/ skill_router（路由决策）' },
  { name: 'args_excerpt', type: 'string', description: '工具调用参数摘要，可能缺失' },
  { name: 'ok', type: 'boolean', description: '工具调用是否成功' },
  { name: 'error', type: 'string', description: '失败时的错误信息，可能缺失' },
  { name: 'output_excerpt', type: 'string', description: '工具输出摘要，可能缺失' },
  { name: 'duration_ms', type: 'integer', description: '工具调用耗时（毫秒）' },
  { name: 'created_at', type: 'string', description: '步骤记录时间（ISO 8601）' },
]

const agentShowResponse = ref(`{
  "id": 7,
  "task_id": "agt_20261025120000_1a2b3c4d5e6f7788",
  "user_id": 2,
  "api_key_id": 1,
  "model_id": 12,
  "provider_id": 3,
  "task": "把下面的大纲做成一集播客视频：……",
  "skill_ids": [5],
  "max_iterations": 12,
  "status": "completed",
  "iterations": 6,
  "result": "视频已完成，共 3 个镜头……（最终总结）",
  "skills_used": [
    { "id": 5, "name": "podcast-video-producer", "loaded_files": ["SKILL.md", "assets/intro.md"] }
  ],
  "assets": [
    { "name": "episode.mp4", "size": 10485760, "mime": "video/mp4", "rel_path": "productions/episode.mp4" }
  ],
  "prompt_tokens": 18200,
  "completion_tokens": 4300,
  "total_tokens": 22500,
  "cost": 0.0876,
  "started_at": "2026-10-25T12:00:01+08:00",
  "completed_at": "2026-10-25T12:03:44+08:00",
  "created_at": "2026-10-25T12:00:00+08:00",
  "updated_at": "2026-10-25T12:03:44+08:00",
  "steps": [
    {
      "id": 1, "task_id": "agt_20261025120000_1a2b3c4d5e6f7788",
      "seq": 0, "iteration": 0, "tool": "skill_router", "ok": true,
      "output_excerpt": "{\\"skills\\":[\\"podcast-video-producer\\"],\\"reason\\":\\"...\\"}",
      "duration_ms": 820, "created_at": "2026-10-25T12:00:00+08:00"
    },
    {
      "id": 2, "task_id": "agt_20261025120000_1a2b3c4d5e6f7788",
      "seq": 1, "iteration": 1, "tool": "write_asset", "args_excerpt": "{\\"name\\":\\"BRIEF.md\\"}",
      "ok": true, "output_excerpt": "productions/hero/BRIEF.md",
      "duration_ms": 12, "created_at": "2026-10-25T12:00:20+08:00"
    }
  ]
}`)

const agentShowErrors = [
  { status: '404', code: 'task_not_found', description: '任务不存在或不属于当前用户（仅本人可见）' },
]

// ---- 4. 下载资产 ----
const agentAssetParams = [
  { name: 'task_id', type: 'string (path)', required: '是', description: '任务 ID；仅任务所有者可下载' },
  { name: 'asset_path', type: 'string (path)', required: '是', description: '资产相对路径（assets[].rel_path），支持嵌套目录，如 productions/hero/BRIEF.md；不允许路径穿越与非法字符，违规返回 400 invalid_asset_name' },
]

const agentAssetRequest = ref(`curl -OJ "${config.api.openai.base}/agent/tasks/agt_20261025120000_1a2b3c4d5e6f7788/assets/productions/episode.mp4" \\
  -H "Authorization: Bearer sk-xxxxxxxxxxxxxxxx"`)

const agentAssetRespFields = [
  { name: '（响应体）', type: 'binary', description: '资产文件二进制流而非 JSON；Content-Type 按文件 MIME 设置，支持 HTTP Range 断点续传，文件名取自磁盘文件名（可直接 curl -OJ 保存）' },
]

const agentAssetErrors = [
  { status: '400', code: 'invalid_asset_name', description: '资产路径非法（路径穿越、以点开头、含非法字符或超过长度限制）' },
  { status: '404', code: 'task_not_found', description: '任务不存在或不属于当前用户' },
  { status: '404', code: 'asset_not_found', description: '资产文件不存在或是目录' },
]

// Anthropic 参数
const anthropicMessageParams = [
  { name: 'model', type: 'string', required: '是', description: '模型代码，如 claude-3-opus 等' },
  { name: 'messages', type: 'array', required: '是', description: '消息数组，包含 role 和 content' },
  { name: 'max_tokens', type: 'integer', required: '否', description: '最大生成 token 数' },
  { name: 'temperature', type: 'number', required: '否', description: '采样温度，0-1 之间' },
  { name: 'top_p', type: 'number', required: '否', description: '核采样概率，0-1 之间' },
  { name: 'stream', type: 'boolean', required: '否', description: '是否启用流式输出，默认 false' },
]

const anthropicRequest = ref(`curl -X POST ${config.api.anthropic.messages} \\
  -H "Content-Type: application/json" \\
  -H "x-api-key: sk-xxxxxxxxxxxxxxxx" \\
  -d '{
    "model": "claude-3-opus",
    "messages": [
      {"role": "user", "content": "你好！"}
    ],
    "max_tokens": 1024
  }'`)

const anthropicResponse = ref(`{
  "id": "msg_01XgYf8h2jQJ",
  "type": "message",
  "role": "assistant",
  "model": "claude-3-opus",
  "content": [
    {
      "type": "text",
      "text": "你好！很高兴见到你。有什么我可以帮助你的吗？"
    }
  ],
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 10,
    "output_tokens": 25
  }
}`)

const anthropicStreamResponse = ref(`{"type":"message_start","message":{"id":"msg_01XgYf8h2jQJ","type":"message","role":"assistant","model":"claude-3-opus","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}
{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好！"}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"很高兴见到你。"}}
{"type":"content_block_stop","index":0}
{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null,"usage":{"output_tokens":25}},"usage":{"input_tokens":10,"output_tokens":25}}
{"type":"message_stop"}`)
</script>

<style scoped>
.api-docs-container {
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

.doc-section {
  padding: 16px 0;
}

.doc-section h3 {
  margin-top: 24px;
  margin-bottom: 12px;
  color: #1d2129;
  font-weight: 600;
}

.doc-section h4 {
  margin-top: 20px;
  margin-bottom: 12px;
  color: #4e5969;
}

.doc-section p {
  color: #86909c;
  margin-bottom: 16px;
}

.code-card {
  background: #f5f5f5;
  margin: 16px 0;
}

.code-card pre {
  margin: 0;
  padding: 16px;
  overflow-x: auto;
}

.code-card code {
  font-family: 'Fira Code', Consolas, monospace;
  font-size: 13px;
  color: #1d2129;
}

.mb-4 {
  margin-bottom: 16px;
}

.mt-4 {
  margin-top: 24px;
}
</style>
