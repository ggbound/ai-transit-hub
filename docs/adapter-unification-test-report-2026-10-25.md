# 适配器统一化改造测试报告

- **日期**：2026-10-25
- **范围**：[backend/internal/ai](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai) 包内适配器公共能力与逻辑的统一化改造（Go 结构体嵌入 + 死代码清理 + 工厂合一），共 4 步实施与验证
- **改造基线**：commit `48d07d9`（ac1ee7f 的父提交）
- **关联提交**（每步独立 commit，可单独回滚）：
  - Step 1 `ac1ee7f`：五个纯委托适配器嵌入 GenericAdapter
  - Step 2 `153bdba`：豆包/千问适配器嵌入 GenericAdapter
  - Step 3 `22066f1`：包内重复清理（parseJSON / BaseAdapter / stringsContains / 千问请求构建）
  - Step 4 `dbee185`：工厂两入口 switch 合一
- **关联文档**：
  - 上游回归报告：[v1-compatibility-regression-full-report-r01-r40-2026-10-24.md](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/v1-compatibility-regression-full-report-r01-r40-2026-10-24.md)
  - R12 回归报告（套件用法参考）：[r12-polling-optimization-test-report-2026-10-24.md](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/docs/r12-polling-optimization-test-report-2026-10-24.md)

---

## 1. 执行摘要

4 步改造全部落地，每步独立编译验证 + 最终免费回归验证，**回归 31/33 通过、跳过 2（真实计费用例）、失败 0**，改造前后对外行为与接口面保持不变，另顺带修复 3 个既有问题。

| 项目 | 结果 |
|---|---|
| 实施规模 | 10 个文件，+318 / -807 行（净 **-489 行**，全包 2650 → 2161 行，**-18.5%**） |
| 对外接口面 | `Adapter` 接口 15 方法、`New*Adapter` 构造器签名、包级 `PreparePassthroughRequest` 均零变化 |
| 静态验证 | 每步 `go build ./...` + `go vet ./internal/ai/` 一次通过；终态 `gofmt -l` 干净 |
| 回归验证（免费） | 31/33 通过 / 跳过 2 计费用例 / 失败 0 |
| 回归验证（计费实测） | **35/35 全通过 / 跳过 0 / 失败 0**（`--include-video --include-image`，exit=0） |
| 本轮计费成本 | 视频 ¥5.0094（108900 tokens）+ 图片 ¥0.5063 ≈ **¥5.52**；视频与 R08/R09/R12 历史记录**精确同价**（计费确定性再次确认） |
| 顺带修复 | ① `stringsContains` 等长误匹配 bug；② 千问丢弃 `tool_choice`；③ 千问 `modelConfig` nil panic 隐患 |
| 行为保持 | 覆写方法逐行保留厂商差异；Generic 提升方法内部自调用不受外层覆写影响（Go 非虚方法语义） |
| 回滚方案 | 4 个独立 commit，`git revert` 逐级回退即可，无数据库/配置变更 |

---

## 2. 背景与目标

### 2.1 改造前的问题清单

对 `internal/ai` 包 10 个文件体检发现四类问题：

1. **纯委托样板重复**：openai/deepseek/claude/hunyuan/zhipu 五个适配器与 GenericAdapter 逐字节一致（各 114 行 × 5 ≈ 570 行重复）；豆包/千问中亦存在纯转发方法（getters、toGenericAdapter、VideosGenerations/GetVideoTask 纯委托、buildHeaders/buildUrl 重复实现）。
2. **死代码**：[adapter_interface.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_interface.go) 中 `BaseAdapter`（原 L143-168）全仓零外部使用（仅 5 处自引用），且与实际采用的 `GenericAdapter` 嵌入模式形成两套并行方案，误导后续维护。
3. **隐藏 bug**：豆包内自实现的 `stringsContains` 在 `len(s) == len(substr)` 时**无条件返回 true**（等长必匹配），多模态嵌入端点判定存在误路由风险；详见 §4.1。
4. **工厂双 switch**：[adapter_factory.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_factory.go) 的 `CreateAdapter` 与 `CreateAdapterWithApiKey` 各维护一份近乎相同的 switch，新增适配器需改两处。

### 2.2 硬约束（不破坏原有功能与接口请求）

- 外部 8 个调用文件（v1_pipeline / chat / embeddings / videos / openai / responses / agent_runtime / video_task_poller）**全部只依赖 `ai.Adapter` 接口**，全仓零类型断言 → 接口 15 方法签名不可变；
- 包级函数 `ai.PreparePassthroughRequest` 被 openai.go / responses.go 直接调用 → 不可动；
- 工厂按 `New*Adapter(provider, apiKey string, modelConfig)` 签名创建 → 构造器签名不可变；
- **构造器语义差异必须保留**：具名适配器无条件包装 `&models.ProviderApiKey{...Status: 1}`（含 apiKey 为空时），`NewGenericAdapter` 仅在 `apiKey != ""` 时包装；
- 豆包/千问与 Generic 的**真实差异覆写**（如豆包 ChatStream usage 提升、千问 `input.texts` 原生 embeddings 格式）必须逐行保留。

---

## 3. 实施内容（4 步明细）

### 3.1 Step 1（`ac1ee7f`）：五个纯委托适配器嵌入化

| 文件 | 前 | 后 | Δ |
|---|---|---|---|
| openai / deepseek / claude / hunyuan / zhipu_adapter.go | 各 114 | 各 30 | 各 -84 |
| 小计 | 570 | 150 | **-420** |

统一模板（以 openai 为例，其余仅类名差异）：

```go
type OpenAIAdapter struct {
    *GenericAdapter   // 公共能力全部经方法集提升继承
}

func NewOpenAIAdapter(provider *models.AiProvider, apiKey string, modelConfig *models.AiModel) *OpenAIAdapter {
    providerKey := &models.ProviderApiKey{AiProviderID: provider.ID, ApiKey: apiKey, Status: 1}
    return &OpenAIAdapter{GenericAdapter: &GenericAdapter{
        client: upstreamHTTPClient, provider: provider,
        apiKey: providerKey, modelConfig: modelConfig,
    }}
}
```

### 3.2 Step 2（`153bdba`）：豆包/千问嵌入化

| 文件 | 前 | 后 | Δ | 保留的厂商覆写 |
|---|---|---|---|---|
| doubao_adapter.go | 330 | 384 | +54 | ChatStream / Embeddings(+multimodalEmbeddings) / VideosGenerations / TransformRequest / TransformResponse / ExtractTokenUsage / PassthroughRequest / PassthroughStreamRequest |
| qwen_adapter.go | 374 | 320 | -54 | Chat / ChatStream / Embeddings / ImagesGenerations / TransformRequest / TransformResponse / ExtractTokenUsage |

- 删除：纯委托 Chat/ImagesGenerations/GetVideoTask（豆包）、VideosGenerations/GetVideoTask/透传转发（千问）、双方 getters / toGenericAdapter / buildHeaders / buildUrl；
- 覆写方法内部对 Generic 能力的调用改为**显式 `a.GenericAdapter.X(...)`**，杜绝嵌入自递归；
- 豆包 `VideosGenerations`（Seedance 完整参数格式）与 `PassthroughRequest`（prompt→content）两条调用链补注释声明**不可合并**；
- 注：豆包 +54 行为覆写注释与显式调用引入的可读性开销，千问 -54 为删除纯委托回退，合计净零。

### 3.3 Step 3（`22066f1`）：包内重复清理

| # | 修改 | 依据 |
|---|---|---|
| 1 | [adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter.go)：删除 `parseJSONCustomHeaders`，唯一调用点改用 `parseJSON` | 两函数逐字节相同（后者仅多 1 处调用） |
| 2 | [adapter_interface.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/adapter_interface.go)：删除死代码 `BaseAdapter`（4 公开字段 + 3 getter）及 `net/http` import | 全仓 grep 零外部引用 |
| 3 | [doubao_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/doubao_adapter.go)：L64 多模态端点判定改 `strings.Contains`，删除 `stringsContains`/`containsSubstring` 自实现 | 修复等长误匹配 bug，见 §4.1 |
| 4 | [qwen_adapter.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/ai/qwen_adapter.go)：删除 `buildOpenAIRequest`，Chat/ChatStream 改用提升的 `buildRequestData` | 见 §4.2 合并前逐行 diff 结论 |

### 3.4 Step 4（`dbee185`）：工厂 switch 合一

抽取共用辅助函数（未识别类型返回 nil，由调用方决定回退）：

```go
func createByAdapterClass(adapterClass string, provider *models.AiProvider,
    apiKey string, modelConfig *models.AiModel) Adapter {
    switch adapterClass {
    case AdapterClassDoubao, AdapterClassVolc, "doubao/volcengine":
        return NewDoubaoAdapter(provider, apiKey, modelConfig)
    // ...openai/deepseek/claude/hunyuan/zhipu 同理...
    case AdapterClassGeneric, "":
        return NewGenericAdapter(provider, apiKey, modelConfig)
    default:
        return nil
    }
}
```

两个入口回退策略原样保留：`CreateAdapter` → `matchAdapterByProviderCode`（按 provider code 匹配 + 日志）；`CreateAdapterWithApiKey` → 直接 `NewGenericAdapter`。空串与 `"generic"` 在两入口的最终归宿（Generic）与改造前逐一等价。

### 3.5 行数变化总表（git 实测）

| 文件 | 改造前 | 改造后 | Δ |
|---|---:|---:|---:|
| adapter.go | 1030 | 1033 | +3 |
| adapter_interface.go | 151 | 140 | -11 |
| adapter_factory.go | 195 | 199 | +4 |
| doubao_adapter.go | 330 | 373 | +43 |
| qwen_adapter.go | 374 | 266 | -108 |
| openai / deepseek / claude / hunyuan / zhipu_adapter.go | 114×5 | 30×5 | -84×5 |
| **合计** | **2650** | **2161** | **-489（-18.5%）** |

---

## 4. 行为保持性论证（核心）

### 4.1 顺带修复的 3 个既有问题（明确标注的行为改进）

| # | 问题 | 修复 | 影响面与实测 |
|---|---|---|---|
| 1 | `stringsContains` 原实现 `len(s)==len(substr)` 时无条件返回 true（**等长必匹配**） | 改用标准库 `strings.Contains` | 仅影响豆包多模态嵌入端点判定（L64：`contains "multimodal" || "vision"`）。语义即"包含"判定，等长但不同内容的端点（如恰好 10 字符的路径）此前会误入 multimodal 通道。**实测**：R01-R06 全部经 vision 嵌入模型（`doubao-embedding-vision-251215`）路由正确，R18 元数据 vision 识别正常 |
| 2 | 千问 `buildOpenAIRequest` 仅复制 temperature/top_p/max_tokens/tools，**丢弃 `tool_choice`** | 复用 Generic `buildRequestData`（optionalParams 透传） | agent_runtime 在 chatOptions 注入 `tool_choice:"auto"`（[agent_runtime.go](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/backend/internal/service/agent_runtime.go) L198），改造前千问 agent 环会静默丢参，与其他 8 个 Generic 系适配器不一致；合并后对齐。注意：当前无活跃 provider 使用千问 adapter_class（见 §5.5 覆盖矩阵），线上零影响 |
| 3 | 千问 `buildOpenAIRequest` 取 `a.modelConfig.Code` 无 nil 防护（modelConfig 为 nil 时 panic） | Generic `buildRequestData` 含 `a.modelConfig != nil` 判断 | 纯健壮性提升 |

### 4.2 千问合并 buildRequestData 的逐行 diff 结论

两函数消息转换段（role/content、ToolCalls、ToolCallID 回传）**逐字节相同**，差异及处置：

| 差异点 | 千问原实现 | Generic | 合并后行为 | 结论 |
|---|---|---|---|---|
| model 回退 | `a.modelConfig.Code`（无防护） | `else if a.modelConfig != nil` | nil 时跳过而非 panic | 更安全 |
| 可选参数 | 仅 float64 类型的 temperature/top_p | optionalParams 列表按原值透传 | float64 值在线上完全一致；int/其他类型从"静默丢弃"变为"透传" | 对齐 Generic 全家行为 |
| max_tokens | float64→int 截断 / int 直通 | 原值透传 | 整数经 Go json.Marshal 均输出 `4096` 形态，线上无差异；小数输入（非法）由上游拒绝 | 无害 |
| stream/tool_choice 从 options 透传 | 不透传 | 透传 | 调用方实测（chat.go options 仅含 `model`；agent_runtime 注入 `tool_choice`）→ 唯一实际变化是 tool_choice 不再被丢 | 对齐性修复，见 §4.1 #2 |

### 4.3 Go 方法集语义保障（为何继承不会破坏覆写）

1. **覆写规则**：嵌入 `*GenericAdapter` 后，豆包/千问的自有覆写方法优先于提升方法；覆写内部必须显式 `a.GenericAdapter.X(...)`（写 `a.X(...)` 会自递归）——Step 2 全部按此写法，每步 `go vet` 佐证。
2. **非虚方法**：提升方法内部的自调用（如 Generic `Chat` 内调 `TransformResponse`）始终分派到 `*GenericAdapter` 自身版本，**不会**被外层覆写劫持。这正是千问 `Chat` 必须保留覆写的原因：若继承 Generic `Chat`，其内部 `TransformResponse` 走 identity 而非千问的 `transformChatResponse`，会破坏响应转换。
3. **构造器语义保留**：豆包/千问/五个具名适配器构造器**无条件**包装 `ProviderApiKey`（含空 apiKey），`NewGenericAdapter` 条件包装——与改造前逐字一致。

### 4.4 外部调用面核验（改造前后一致）

| 依赖点 | 核验方式 | 结果 |
|---|---|---|
| `Adapter` 接口 15 方法 | 全仓 grep：8 个外部文件全部经接口调用，零类型断言 | 接口未动，外部零改动 |
| `ai.PreparePassthroughRequest` | openai.go L169、responses.go L45 包级调用 | 函数未动 |
| 工厂签名 `New*Adapter(provider, apiKey, modelConfig)` | 8 个适配器构造器逐一核对 | 签名未动 |
| `UpstreamError` 错误契约 | R22/R25/R26/R27 错误信封回归 | 全部通过 |

---

## 5. 验证过程与结果

### 5.1 静态验证（每步执行）

| 步骤 | 验证命令 | 结果 |
|---|---|---|
| Step 1-4 每步 | `go build ./... && go vet ./internal/ai/` | 一次通过（BUILD_OK） |
| 终态 | `gofmt -l internal/ai/` | 无输出（格式干净） |
| 终态 | 工作区 `git status -- internal/ai/` | 干净，无未提交残留 |

### 5.2 回归环境

- **后端**：`/tmp/ath-server` 于 Step 4 提交后由 `go build -o ./cmd/server` **重建并重启**（旧进程为 10-24 构建，不重启则回归验证的是旧代码），健康检查 200；
- **套件**：[scripts/regression_full_suite.py](file:///Users/mac-yueyue0516/data/new_version/GitHub/ai-transit-hub/scripts/regression_full_suite.py)（免费模式，真实计费用例默认跳过）；
- **模型清单**：32 个（Doubao 类 22 + Generic 类 2 + openai 类 8）。

### 5.3 回归结果明细（31 通过 / 2 跳过 / 0 失败，exit=0）

**A. 共享管线正向（8 用例全过）**

| 用例 | 关键断言（实测值） |
|---|---|
| R31 chat 非流式 | 200；usage total_tokens=65；model=doubao-seed-2-0-mini-260428 |
| R32 chat 流式 | 200；`[DONE]` 结束；增量块 55 个 |
| R33a/R33b messages 非流式 | Anthropic 格式完整（type/role/stop_reason）；Bearer 与 X-Api-Key 双认证等价；usage input_tokens=9 |
| R34 messages 流式 | message_start / content_block_delta / message_stop 事件序列完整 |
| R35a/R35b responses | object=response、status=incomplete(length) 方言透传；流式以 response.completed 结束 |
| R38 | X-Api-Key + `/api/v1/models` 备用前缀：32 模型 |

**B. P3 能力元数据（6 用例全过）**：R16（32 模型 compatibility 分段）、R17（豆包 text 无 messages 端点、ZhiyunTB 含 responses）、R18（vision 嵌入 multimodal_content[] 声明）、R19（video 契约：endpoints/exclusive_fields/别名）、R20（Anthropic 极简 models）、R21（两次 /v1/models 缓存序列化一致）

**C. P4 错误信封矩阵（8 用例全过）**：R22（401 三连 missing/invalid_api_key）、R23（404 unknown_url）、R24（非 v1 前缀 gin 默认 404）、R25a（chat 缺 messages → 上游 `MissingParameter` 原文透传）、R25b（messages 缺 max_tokens → 上游不强制、200 合法）、R26（provider/model_not_found）、R27（>10MB → 413）、R28（/api/v1 前缀信封一致）

**D. P1 embeddings 专项（6 用例全过）**——直接命中豆包覆写路径：R01（单文本→1 条，维度 2048）、R02（数组→2 条，index [0,1]）、R03（3 条 + usage 累加，倍率 3.00）、R04（单输入路径）、R05（空串→上游原文 400）、R06（10 条并发上限 8 合并稳定无乱序）

**E. P2 校验与错误（3 用例全过）**：R10（prompt/content 冲突 400）、R11（prompt 类型错 400）、R14（task_not_found 404）

**计费用例**：免费轮默认跳过；已按用户要求追加实测（见 §5.4），**全部通过**。

### 5.4 计费用例实测（--include-video --include-image，35/35 全通过）

#### F1. P2 视频闭环（豆包 VideosGenerations + 继承 GetVideoTask）

| 用例 | 关键断言（实测值） |
|---|---|
| R08 prompt-only 创建 | 200；任务 ID `cgt-20261025081840-np4kk` |
| R13 running 立即查询 | 200；status=running；无 content 归一化污染（键列表干净）、顶层无 video_url:null |
| R12 轮询至终态 + 别名 | 终态 succeeded；顶层 video_url 与 content.video_url 别名一致；duration=5 ratio=16:9 720p fps=24 |

R12 阶梯轮询时刻表（继承 R12 优化后的 5/15/30s 策略，本次实测再次验证）：

```
[  0s]-[ 30s] 每 5s 一查（快速失败段，7 次）
[ 45s]-[121s] 每 15s 一查（稳定期，6 次）
[136s] status=succeeded   闭环耗时 136s / 轮询 14 次 / 终态滞后 <=30s
```

#### F2. images（豆包 ImagesGenerations，继承路径）

| 用例 | 关键断言（实测值） |
|---|---|
| R36 images 生成 | 200；data 非空（1 项）；首项含 url/size |

#### 记账对账

| record_id | 模型 | tokens | 费用 | 对账结论 |
|---|---|---|---|---|
| 485 | seedance 视频（model_id=40） | 108900 | **¥5.0094** | 与 R08/R09 历史及 R12 报告记录（108900/¥5.0094）**精确同价**，同参数计费确定性再次确认 |
| 486 | seedream 图片（model_id=18） | 16384 | ¥0.5063 | images 计费正常 |
| 484/483 | 嵌入（免费段） | 230/0 | ¥0.000000 | — |

本轮计费用例总成本：¥5.0094 + ¥0.5063 ≈ **¥5.52**。

### 5.5 全量回归（免费+计费）对 Go 适配器路径的覆盖矩阵

| Go 适配器 | 回归用例实测 | 保障手段 |
|---|---|---|
| DoubaoAdapter（22 模型，主力） | Chat（继承）R31；ChatStream（覆写）R32；Embeddings+multimodal 检测（覆写，含 strings.Contains 修复点）R01-R06/R18；PassthroughRequest（multimodalEmbeddings 调用链）R01-R06；VideosGenerations（覆写）R08；GetVideoTask（继承）R13/R12；ImagesGenerations（继承）R36 | 实测 |
| GenericAdapter（kimi-k2.5、qwen3.7-plus） | responses/messages 正向与错误矩阵 | 实测 |
| OpenAIAdapter（8 模型） | messages 类用例（R25b/R33 等，全继承路径） | 实测 |
| QwenAdapter | 无活跃 provider 使用 qwen adapter_class；其 Chat/ChatStream/Embeddings/ImagesGenerations 覆写均未被套件直达（R36 走豆包 seedream） | 编译+vet + §4.2/4.3 逐行等价性论证 |
| Claude / Hunyuan / Zhipu / DeepSeekAdapter | 无活跃 provider 使用对应 adapter_class | 编译+vet + 嵌入语义论证（30 行模板文件） |
| 豆包 PassthroughStreamRequest（流式透传覆写） | 套件未直达该路径 | 代码审查确认覆写逐行保留 + 编译 |

---

## 6. 风险与回滚

### 6.1 残余风险

| 风险 | 等级 | 说明与缓解 |
|---|---|---|
| 千问覆写路径无线上实测 | 低 | 当前库内无 qwen adapter_class 的活跃 provider；若未来启用，建议先以单模型冒烟（Chat/ChatStream/Embeddings/图片） |
| 豆包 PassthroughStreamRequest（流式透传）套件未直达 | 低 | 覆写代码逐行保留并经编译/vet；现网流式透传为独立包级路径（PreparePassthroughRequest），适配器覆写仅做请求体转换 |
| strings.Contains 修复改变等长端点行为 | 极低 | 属 bug 修复方向；现网全部多模态嵌入经 vision 模型实测路由正确 |

> 计费链路已实测通过（见 §5.4）：视频/图片用例 35/35 全回归通过，视频记账与历史四次精确同价，不再列为残余风险。

### 6.2 回滚方案

4 个 commit 相互独立、无数据库/配置变更，支持逐级回退：

```bash
git revert dbee185   # 仅回退工厂合一
git revert 22066f1   # 回退包内重复清理
git revert 153bdba   # 回退豆包/千问嵌入化
git revert ac1ee7f   # 回退五适配器嵌入化
```

---

## 7. 结论

适配器统一化改造按批准的 4 步方案全部完成：**净 -489 行（-18.5%）**，`Adapter` 接口与构造器签名零变化，豆包/千问真实厂商差异覆写逐行保留，工厂两入口回退策略等价。每步静态验证一次通过，另修复 3 个既有问题（等长误匹配 bug、千问 tool_choice 丢弃、nil panic 隐患）。

回归验证分两轮：免费用例 **31/33 通过、0 失败**（跳过 R08/R13/R12/R36 计费用例）；随后开启 `--include-video --include-image` 计费实测 **35/35 全通过、跳过 0、失败 0**——视频任务 `cgt-20261025081840-np4kk` 闭环 136s/14 次轮询、R12 阶梯轮询时刻表生效、任务无污染，记账对账视频 108900 tokens/¥5.0094 与历史 R08/R09 及 R12 报告记录**精确同价**（计费确定性再次确认），图片 ¥0.5063，本轮总成本 ≈**¥5.52**。残余风险仅剩无活跃 provider 的千问覆写路径与套件未直达的豆包 PassthroughStreamRequest 路径，均已给出缓解建议。改造达成"公共能力统一、行为与接口请求零破坏"的目标。
