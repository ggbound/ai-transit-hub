# AI模型中转站项目分析文档

## 1. 项目概述

### 1.1 项目背景
本项目旨在构建一个统一的AI模型调度管理系统，包含两个主要部分：
- **后台管理系统**：管理接入各家AI模型的调度配置
- **员工使用平台**：公司员工内部注册及使用AI服务的平台

两个系统共享数据和接口调用能力，形成一个完整的AI模型中转站。

### 1.2 项目目标
- 统一管理多家AI模型提供商的接口
- 提供标准化的API调用方式
- 实现用户权限管理和计费统计
- 支持灵活的模型调度和负载均衡
- 提供友好的用户界面和管理后台
- **完善的操作日志记录系统**：管理后台和用户端所有非查询操作均需记录，包括登录注册等关键操作
- **完整的审计追踪能力**：支持操作日志的查询、分析和追溯
- **流式输出支持**：提供流畅的逐字输出体验，提升用户交互感受
- **工具调用（Function Calling）支持**：支持模型调用外部工具，扩展AI能力边界

## 2. 主流AI模型API接口分析

### 2.1 OpenAI API协议（行业标准）

#### 2.1.1 核心端点
- **聊天补全**：`/v1/chat/completions`
- **文本补全**：`/v1/completions`
- **图像生成**：`/v1/images/generations`
- **嵌入**：`/v1/embeddings`
- **语音转录**：`/v1/audio/transcriptions`

#### 2.1.2 请求参数
```json
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {
      "role": "system",
      "content": "你是一个有帮助的AI助手"
    },
    {
      "role": "user",
      "content": "你好"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 2000,
  "top_p": 1.0,
  "stream": false,
  "frequency_penalty": 0,
  "presence_penalty": 0
}
```

#### 2.1.3 响应格式
```json
{
  "id": "chatcmpl-12345",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-3.5-turbo",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！很高兴能帮助你。"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 56,
    "completion_tokens": 31,
    "total_tokens": 87
  }
}
```

#### 2.1.4 流式输出（Streaming）
**请求参数：**
- `stream: true` - 启用流式输出

**流式响应格式（Server-Sent Events）：**
```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"role":"assistant","content":"你"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-3.5-turbo","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

**流式输出特点：**
- 逐字/逐Token返回，减少用户等待时间
- 实时展示生成过程，提升用户体验
- 支持中途取消生成
- 最后返回usage统计信息

#### 2.1.5 工具调用（Function Calling）
**工具定义格式：**
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_current_weather",
        "description": "获取当前天气",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "城市名称，如北京、上海"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

**工具调用响应：**
```json
{
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_123",
            "type": "function",
            "function": {
              "name": "get_current_weather",
              "arguments": "{\"location\":\"北京\",\"unit\":\"celsius\"}"
            }
          }
        ]
      },
      "finish_reason": "tool_calls"
    }
  ]
}
```

**工具结果返回格式：**
```json
{
  "role": "tool",
  "tool_call_id": "call_123",
  "content": "{\"temperature\": 25, \"weather\": \"晴天\", \"humidity\": 60}"
}
```

**工具调用流程：**
1. 用户发送请求，附带可用工具定义
2. 模型决定是否需要调用工具
3. 如需要，返回工具调用请求
4. 服务端执行工具，获取结果
5. 将工具结果返回给模型
6. 模型基于工具结果生成最终回复

### 2.2 主流AI模型提供商对比

| 提供商 | 模型系列 | API协议 | 流式输出 | 工具调用 | 特点 |
|--------|----------|---------|---------|---------|------|
| OpenAI | GPT-3.5/GPT-4 | OpenAI标准 | ✅ | ✅ | 综合能力最强，生态完善 |
| Anthropic | Claude | OpenAI兼容 | ✅ | ✅ | 长文本处理优秀，安全性高 |
| Google | Gemini | 自定义 | ✅ | ✅ | 多模态能力强 |
| 阿里云 | Qwen系列 | OpenAI兼容 | ✅ | ✅ | 中文优化，稳定可靠 |
| DeepSeek | DeepSeek-V3 | OpenAI兼容 | ✅ | ✅ | 性价比极高 |
| 智谱AI | GLM系列 | OpenAI兼容 | ✅ | ✅ | 国产化，清华技术 |
| 百度 | ERNIE系列 | OpenAI兼容 | ✅ | ✅ | 中文能力强 |
| 讯飞 | 星火 | 自定义 | ✅ | ✅ | 语音能力强 |

### 2.3 API调用关键概念

#### Token计算
- 输入Token：消息历史+用户问题
- 输出Token：模型生成的回复
- 收费通常按输入/输出Token分别计费

#### 流式输出
- 通过`stream: true`启用
- 逐字返回结果，提升用户体验
- 使用Server-Sent Events (SSE)传输

#### 工具调用
- 允许模型调用外部函数/API
- 支持多轮工具调用
- 可扩展AI能力（如获取实时数据、执行操作等）

#### 鉴权方式
- 主要使用API Key在Authorization Header中传递
- 格式：`Authorization: Bearer sk-xxxxx`

## 3. 系统架构设计

### 3.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                         外部AI模型                           │
│  OpenAI │ Anthropic │ 阿里云 │ DeepSeek │ 其他...            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      API网关/适配器层                        │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ OpenAI适配器 │  │ Claude适配器 │  │ Qwen适配器  │  ...    │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      核心服务层                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ 统一API服务  │  │ 调度服务    │  │ 工具执行服务  │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ 计费服务    │  │ 用户服务    │  │ 权限服务    │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
│  ┌─────────────┐                                        │
│  │ 日志服务    │                                        │
│  └─────────────┘                                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      数据层                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ 用户数据库 │  │ 计费数据库 │  │ 配置数据库 │  │ 日志数据库 │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
│  ┌──────────┐  ┌──────────┐                                    │
│  │ Redis缓存 │  │ 消息队列 │                                    │
│  └──────────┘  └──────────┘                                    │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────────┐   ┌─────────────────────────────┐
│        员工使用平台          │   │       后台管理系统          │
│  - 用户注册/登录            │   │  - 模型配置管理             │
│  - 模型选择                 │   │  - 密钥管理                 │
│  - 对话功能                 │   │  - 用户管理                 │
│  - 历史记录                 │   │  - 计费统计                 │
│  - 个人中心                 │   │  - 监控告警                 │
└─────────────────────────────┘   └─────────────────────────────┘
```

### 3.2 核心技术组件

#### 3.2.1 API适配器层
- **职责**：
  - 将统一请求格式转换为各模型厂商的格式
  - 处理各厂商流式输出的差异，统一为标准SSE格式
  - 转换不同格式的工具调用（如Claude的tool_use、Gemini的function calling）
  - 处理响应，统一转换为OpenAI兼容格式
  
- **设计模式**：适配器模式
- **支持动态注册新适配器**
- **流式输出处理**：
  - 支持各厂商流式协议差异
  - 统一输出为标准SSE格式
  - 支持流式中断和取消
  
- **工具调用处理**：
  - 统一工具定义格式
  - 转换不同厂商的工具调用响应
  - 处理工具结果返回格式

#### 3.2.2 调度服务
- 职责：根据配置选择合适的模型和密钥
- 功能：负载均衡、故障转移、降级策略
- 策略：按权重分配、按响应时间选择、按成本优化

#### 3.2.3 计费服务
- 职责：计算每次调用的费用
- 支持多币种（USD/CNY）
- 支持预算预警

#### 3.2.4 工具执行服务
- **职责**：
  - 管理和执行预定义工具
  - 处理工具调用请求
  - 工具权限控制和安全验证
  
- **核心功能**：
  - 工具注册和管理
  - 工具执行和结果返回
  - 工具调用权限控制
  - 工具调用日志记录
  - 支持自定义工具开发
  
- **内置工具示例**：
  - 天气查询
  - 时间/日期获取
  - 数据检索
  - 代码执行（受控环境）
  - 文件操作（受限）
  
- **安全机制**：
  - 工具白名单
  - 参数验证
  - 执行超时控制
  - 资源使用限制
  - 审计日志

#### 3.2.5 日志审计服务
- **职责**：统一记录和管理所有系统操作日志
- **核心功能**：
  - 操作日志自动记录（所有非查询操作）
  - 日志分类和结构化存储
  - 异步日志写入（通过消息队列）
  - 日志查询和检索
  - 日志分析和统计
- **记录范围**：
  - 用户认证相关：注册、登录、登出、密码修改
  - API密钥相关：创建、删除、启用、禁用、重新生成
  - 用户信息：个人信息修改、头像上传
  - 对话操作：创建会话、删除会话、收藏/取消收藏
  - 工具调用：工具名称、参数、执行结果
  - 后台管理：所有管理操作（增删改）
- **记录内容**：操作者、操作类型、操作时间、IP地址、User-Agent、操作参数、操作结果、详细信息

### 3.3 技术栈建议

| 层级 | 技术选型 | 说明 |
|------|----------|------|
| 后端框架 | Go / Node.js / Python | Go适合高性能，Node.js全栈，Python生态好 |
| 数据库 | PostgreSQL + Redis | 关系型数据存PostgreSQL，缓存和会话用Redis |
| 前端框架 | React / Vue.js | 现代化前端框架 |
| 消息队列 | RabbitMQ / Kafka | 异步处理日志和计费 |
| API网关 | Nginx / Kong | 入口路由和负载均衡 |
| 部署 | Docker + Kubernetes | 容器化部署，便于扩展 |

## 4. 后台管理系统功能规划

### 4.1 仪表盘
- 总调用量统计
- 费用统计（今日/本周/本月）
- 各模型使用占比
- 实时请求监控
- 错误率统计

### 4.2 模型管理
- 模型列表（支持多种提供商）
- 模型配置（名称、描述、价格、状态）
- 模型启用/禁用
- 模型版本管理
- **模型能力配置**：
  - 流式输出支持配置
  - 工具调用支持配置
  - 上下文窗口大小配置
  - 功能开关控制

### 4.3 工具管理
- **工具列表**：
  - 查看所有可用工具
  - 工具启用/禁用
  - 工具分类管理
  
- **工具配置**：
  - 工具名称、描述
  - 工具参数定义
  - 工具权限配置（哪些用户/角色可用）
  - 工具执行权限（内部/外部）
  
- **自定义工具**：
  - 支持上传自定义工具代码
  - 工具测试功能
  - 工具版本管理
  
- **工具监控**：
  - 工具调用统计
  - 执行耗时监控
  - 错误率统计
  
### 4.4 密钥管理
- API密钥配置
- 密钥分组（支持多个密钥按权重分配）
- 密钥状态管理（启用/禁用）
- 使用量监控
- 熔断机制配置

### 4.5 用户管理
- 用户列表
- 用户创建/编辑/删除
- 用户权限分配
- 用户配额管理
- 用户状态管理

### 4.6 计费管理
- 费率配置
- 计费规则设置
- 预算预警配置
- 账单生成
- 费用明细查询

### 4.7 日志与审计
- **操作日志查询**
  - 按用户筛选日志
  - 按操作类型筛选
  - 按时间范围查询
  - 按操作结果筛选（成功/失败）
  - 支持关键词搜索
  - 日志列表分页展示
  
- **日志详情查看**
  - 操作完整信息展示
  - 请求参数详情（敏感信息脱敏）
  - 响应结果详情
  - IP地址和设备信息
  - 操作前后数据对比（如适用）
  
- **日志统计分析**
  - 操作类型分布统计
  - 用户活跃度统计
  - 操作成功率统计
  - 异常操作趋势分析
  - 可视化图表展示
  
- **日志管理**
  - 日志导出功能（CSV/Excel）
  - 日志归档策略
  - 日志保留期限配置
  - 敏感操作标记
  
- **审计追踪**
  - 特定用户操作轨迹追踪
  - 关键操作审计记录
  - 异常操作告警

### 4.8 排行榜管理
- 排行榜配置
  - 排行榜启用/禁用
  - 显示范围设置（部门/全公司）
  - 排行榜数量设置（Top N）
  - 统计时间范围配置
- 排行榜数据查看
  - 查看实时排行榜
  - 历史排行榜数据查询
  - 数据导出功能
- 排行榜规则设置
  - 排名规则配置（按Token/按费用/按调用次数）
  - 数据脱敏设置（隐藏用户真实姓名）

### 4.9 系统配置
- 系统参数设置
- 通知配置（邮件/钉钉/企业微信）
- 安全设置（IP白名单、访问频率限制）

## 5. 员工使用平台功能规划

### 5.1 用户认证模块
- **用户注册**
  - 邮箱注册（支持验证码验证）
  - 企业账号集成（如LDAP、SSO）
  - 注册信息填写（姓名、部门、工号等）
  - 账号激活流程
  
- **用户登录**
  - 邮箱/密码登录
  - 支持记住登录状态
  - 登录失败次数限制
  
- **密码管理**
  - 忘记密码（邮箱验证码重置）
  - 密码修改
  - 密码强度要求

- **个人信息管理**
  - 基本信息编辑
  - 头像上传
  - 邮箱验证

### 5.2 API密钥管理模块
- **API Key生成**
  - 一键生成个人专属API Key
  - 支持生成多个Key（测试/生产等用途）
  - Key命名和备注功能
  
- **API Key管理**
  - Key列表展示（创建时间、最后使用时间、状态）
  - Key启用/禁用
  - Key删除
  - Key重新生成（轮换）
  
- **Key权限与配额**
  - 可访问模型列表配置
  - 调用频率限制
  - Token配额设置
  - 密钥使用统计

### 5.3 API文档模块
- **接口文档展示**
  - 统一API接口说明
  - 请求/响应参数详解
  - 支持的模型列表
  - 接口调用示例（多种编程语言）
  
- **交互式文档**
  - 在线API文档（类似Swagger/OpenAPI）
  - 参数说明和示例值
  - 响应示例展示

### 5.4 接口测试工具模块
- **在线测试控制台**
  - 选择要测试的API端点
  - 可视化参数配置界面
  - 支持自定义请求头
  - 支持JSON格式请求体编辑
  
- **测试功能**
  - 一键发送测试请求
  - 实时显示响应结果
  - 响应状态码和耗时显示
  - 请求/响应详情查看
  - 测试历史记录
  
- **代码示例生成**
  - 根据测试配置自动生成代码片段
  - 支持多种语言（Python、JavaScript、Java等）
  - 一键复制代码

### 5.5 数据统计与排行榜模块
- **个人使用统计**
  - Token使用量统计（今日/本周/本月/累计）
  - 调用次数统计
  - 费用统计（按模型、按时间）
  - 使用趋势图表
  
- **排行榜展示**
  - Token使用总量排行榜（Top N）
  - 支持按时间范围筛选（日/周/月）
  - 排行榜可视化展示
  - 个人排名位置高亮
  - 可配置排行榜可见范围（仅部门/全公司）

### 5.6 对话功能
- **基础对话**：
  - 模型选择
  - 新建对话
  - 多轮对话
  - 对话历史记录
  - 对话收藏
  - 对话分享
  
- **流式输出**：
  - 逐字实时展示
  - 打字机效果
  - 生成进度指示
  - 支持中途取消生成
  
- **工具调用**：
  - 工具选择（可选启用哪些工具）
  - 工具调用过程可视化展示
  - 工具执行结果展示
  - 支持查看工具调用详情（参数、结果）
  - 多轮工具调用流程展示
  
- **对话设置**：
  - 流式输出开关
  - 工具选择配置
  - 系统提示词自定义

### 5.7 个人中心
- 使用统计（调用次数、费用）
- 消费记录
- 余额查询（如适用）
- 个人设置

### 5.8 个人操作日志模块
- **我的操作日志**
  - 查看个人操作历史记录
  - 按操作类型筛选（登录、API密钥管理、对话等）
  - 按时间范围筛选
  - 日志列表分页展示
  
- **日志详情查看**
  - 操作时间
  - 操作类型
  - 操作结果
  - IP地址
  - 设备信息
  
- **日志导出**
  - 个人操作日志导出功能

### 5.9 帮助与文档
- 快速入门指南
- 常见问题
- 使用教程

## 6. 数据库设计概要

### 6.1 核心数据表

#### 6.1.1 用户相关
- **users（用户表）**
  - id, username, email, password_hash, department, employee_id, avatar, status, created_at, updated_at
  
- **user_api_keys（用户API密钥表）**
  - id, user_id, key_name, api_key, key_hash, status, quota_limit, rate_limit, last_used_at, created_at, updated_at

#### 6.1.2 模型相关
- **models（模型表）**
  - id, provider, model_name, display_name, description, input_price, output_price, status, created_at
  - **新增字段**：supports_streaming (BOOLEAN), supports_tool_calling (BOOLEAN), context_window (INTEGER)
  
- **provider_api_keys（提供商API密钥表）**
  - id, provider, api_key, key_name, weight, status, usage_count, created_at

#### 6.1.3 对话相关
- **chat_sessions（会话表）**
  - id, user_id, model_id, title, created_at, updated_at
  - **新增字段**：stream_enabled (BOOLEAN), enabled_tools (JSON), system_prompt (TEXT)
  
- **chat_messages（消息表）**
  - id, session_id, role, content, tokens, created_at
  - **新增字段**：tool_calls (JSON), tool_call_id (TEXT), finish_reason (TEXT)

#### 6.1.4 统计相关
- **usage_logs（使用日志表）**
  - id, user_id, api_key_id, model_id, prompt_tokens, completion_tokens, total_tokens, cost, request_time, response_time, status, error_message
  
- **user_usage_stats（用户使用统计表）**
  - id, user_id, date, total_tokens, prompt_tokens, completion_tokens, total_cost, call_count
  
- **leaderboard_snapshots（排行榜快照表）**
  - id, snapshot_time, period_type (daily/weekly/monthly), data_json, created_at

#### 6.1.5 日志审计相关
- **operation_logs（操作日志表）**
  - id, user_id, username, operation_type, operation_module, operation_action
  - ip_address, user_agent, request_params (JSON), response_result (JSON)
  - status (success/failed), error_message, before_data (JSON), after_data (JSON)
  - created_at, trace_id
  
- **operation_types（操作类型字典表）**
  - id, type_code, type_name, description, module_name, is_sensitive

#### 6.1.6 工具相关
- **tools（工具表）**
  - id, name, display_name, description, tool_type, parameters (JSON), enabled, created_at, updated_at
  
- **tool_versions（工具版本表）**
  - id, tool_id, version, code, created_at
  
- **tool_execution_logs（工具执行日志表）**
  - id, user_id, tool_id, parameters (JSON), result (JSON), status, execution_time, created_at

#### 6.1.7 排行榜相关
- **leaderboard_configs（排行榜配置表）**
  - id, period_type (daily/weekly/monthly), enabled, display_count, visibility_scope (department/company), ranking_rule (tokens/cost/calls), created_at, updated_at

#### 6.1.8 系统相关
- **system_configs（系统配置表）**
  - id, config_key, config_value, description, updated_at

### 6.2 关键索引
- 用户ID索引（users.id, user_api_keys.user_id, usage_logs.user_id, operation_logs.user_id, tool_execution_logs.user_id）
- 模型ID索引（models.id, usage_logs.model_id）
- 会话ID索引（chat_sessions.id, chat_messages.session_id）
- 时间范围索引（usage_logs.request_time, user_usage_stats.date, operation_logs.created_at, tool_execution_logs.created_at）
- API Key索引（user_api_keys.api_key）
- 排行榜快照时间索引
- 操作日志索引（operation_logs.operation_type, operation_logs.status）
- 工具ID索引（tools.id, tool_execution_logs.tool_id, tool_versions.tool_id）

## 7. 安全考虑

### 7.1 认证与授权
- JWT Token认证
- 角色权限控制（RBAC）
- API Key权限范围限制

### 7.2 数据安全
- API密钥加密存储
- 敏感数据加密传输（HTTPS）
- 数据备份策略

### 7.3 访问控制
- IP白名单
- 请求频率限制
- 审计日志

### 7.4 日志安全
- **日志完整性保护**
  - 操作日志不可修改、不可删除
  - 日志记录完整性校验
  - 防止日志篡改
  
- **敏感信息保护**
  - 日志中密码、密钥等敏感信息自动脱敏
  - 个人信息加密存储
  - 日志访问权限控制（仅管理员可查看完整日志）
  
- **日志存储安全**
  - 日志数据定期备份
  - 日志保留期限管理
  - 归档日志加密存储

## 8. 扩展性考虑

### 8.1 水平扩展
- 无状态服务设计
- 数据库读写分离
- Redis集群

### 8.2 插件化设计
- 适配器可插拔
- 支持自定义调度策略
- 支持自定义计费规则

## 9. 实施路线建议

### 第一阶段（MVP）
- 基础架构搭建
- 接入1-2家主流模型（如OpenAI和DeepSeek）
- 基础用户管理
- 简单的对话界面
- 基础计费统计
- **操作日志记录系统**：核心操作日志记录功能（登录注册、API密钥管理、用户信息修改等）
- **基础日志查询功能**：后台和用户端的日志查看
- **流式输出支持**：核心流式输出功能，支持逐字展示

### 第二阶段
- 接入更多模型提供商
- 完善后台管理功能
- 实现调度策略
- 用户配额管理
- 预算预警
- **工具调用基础支持**：
  - 工具管理后台
  - 基础内置工具（天气、时间等）
  - 工具调用执行引擎
  - 用户端工具调用可视化展示

### 第三阶段
- **高级流式输出功能**：
  - 流式取消功能
  - 流式输出优化和性能提升
  
- **高级工具调用功能**：
  - 自定义工具支持
  - 多轮工具调用
  - 工具权限精细化控制
  
- **性能优化**
- **监控告警系统**
- **数据可视化**

---

*文档创建日期：2026-07-30*
