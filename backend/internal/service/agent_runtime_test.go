package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// --- checkNpmPackage（system 模式 npm: 依赖预检）---

// TestCheckNpmPackageNpmMissing 宿主无 npm：报错须给出两条出路（装 Node.js / 切 venv 模式）
func TestCheckNpmPackageNpmMissing(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	got := checkNpmPackage("typescript")
	assertContains(t, got, `cannot verify npm package "typescript"`)
	assertContains(t, got, "npm not available on host")
	assertContains(t, got, "install Node.js/npm on the server")
	assertContains(t, got, "AGENT_PY_ENV_MODE=venv")
}

// TestCheckNpmPackageEmptyName 空包名优先短路
func TestCheckNpmPackageEmptyName(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	if got := checkNpmPackage(""); got != "empty package name" {
		t.Errorf("checkNpmPackage(\"\") = %q, want %q", got, "empty package name")
	}
}

// TestCheckNpmPackageNotInstalled fake npm ls 退出 1：报包未全局安装
func TestCheckNpmPackageNotInstalled(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "npm", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", bin)
	got := checkNpmPackage("typescript")
	assertContains(t, got, `npm package "typescript" is not installed globally`)
}

// TestCheckNpmPackageInstalled fake npm ls 退出 0：校验通过
func TestCheckNpmPackageInstalled(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "npm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	if got := checkNpmPackage("typescript"); got != "" {
		t.Errorf("checkNpmPackage = %q, want empty", got)
	}
}

// --- hostEnvironmentProfile 的 Node 执行环境画像行 ---

// TestHostEnvironmentProfileNodeMissing 宿主无 node：画像须含 node 不可用行（附出路）与基础探测事实
func TestHostEnvironmentProfileNodeMissing(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	r := &AgentRuntime{pyenv: newTestPyEnvManager(t, "")}
	profile := r.hostEnvironmentProfile(filepath.Join(t.TempDir(), "task-1"))
	assertContains(t, profile, "node: 未检测到")
	assertContains(t, profile, "Node 执行环境: 宿主机未安装 Node.js")
	assertContains(t, profile, "请改用 Python 脚本")
}

// TestHostEnvironmentProfileSelfHostedNode 宿主无 node 但自托管运行时已引导：
// 画像如实报告引导产物与可用性，不再重复"请改用 Python 脚本"的负面出路
func TestHostEnvironmentProfileSelfHostedNode(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	m := newTestPyEnvManager(t, "")
	nodeBin := installFakeNode(t, m)
	r := &AgentRuntime{pyenv: m}
	profile := r.hostEnvironmentProfile(filepath.Join(t.TempDir(), "task-1"))
	assertContains(t, profile, "已自动引导自托管运行时（"+nodeBin+"）")
	assertContains(t, profile, "node 脚本与 npm: 依赖声明照常可用")
	assertNotContains(t, profile, "请改用 Python 脚本")
}

// TestHostEnvironmentProfileNodeModulesPresent node_modules 已建：画像列出预装包，且不出现"未安装"提示（两分支互斥）
func TestHostEnvironmentProfileNodeModulesPresent(t *testing.T) {
	bin := fakeBinDir(t)
	fakeBin(t, bin, "node", "#!/bin/sh\necho v20.0.0\n")
	fakeBin(t, bin, "npm", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)

	m := newTestPyEnvManager(t, "")
	// 预置任务 node_modules（模拟 precheck 已构建完成）
	nm := m.TaskNodeModulesDir("task-1")
	if err := mkdirAll(filepath.Join(nm, "express")); err != nil {
		t.Fatalf("prepare node_modules: %v", err)
	}
	r := &AgentRuntime{pyenv: m}
	profile := r.hostEnvironmentProfile(filepath.Join(t.TempDir(), "task-1"))
	assertContains(t, profile, "任务级 node_modules")
	assertContains(t, profile, "express")
	assertContains(t, profile, "NODE_PATH 仅对 require 生效")
	assertNotContains(t, profile, "宿主机未安装 Node.js")
}

// TestHostEnvironmentProfileSystemMode system 模式（pyenv=nil）同样有 node 缺失提示
func TestHostEnvironmentProfileSystemMode(t *testing.T) {
	t.Setenv("PATH", fakeBinDir(t))
	r := &AgentRuntime{}
	profile := r.hostEnvironmentProfile(filepath.Join(t.TempDir(), "task-1"))
	assertContains(t, profile, "Node 执行环境: 宿主机未安装 Node.js")
}

// mkdirAll 测试内铺目录的小包装
func mkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// --- streamAccumulator（agent chat 流式分片 → 非流式响应还原）---

// feedChunks 按真实 SSE 解析产物喂分片：JSON 字符串 → map → absorb
func feedChunks(t *testing.T, acc *streamAccumulator, chunks ...string) {
	t.Helper()
	for _, raw := range chunks {
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &chunk); err != nil {
			t.Fatalf("bad chunk fixture %q: %v", raw, err)
		}
		acc.absorb(chunk)
	}
}

// firstChoice 从还原响应中取 choices[0]（测试便捷）
func firstChoice(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	choices, ok := resp["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		t.Fatalf("resp has no choices: %v", resp)
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		t.Fatalf("invalid choice format: %v", choices[0])
	}
	return choice
}

// TestStreamAccumulatorContentAndUsage content 分片拼接、id/model 捕获、usage 块捕获
// （include_usage 末块 choices 为空）、[DONE] 标记跳过且不计数、finish_reason 捕获
func TestStreamAccumulatorContentAndUsage(t *testing.T) {
	acc := &streamAccumulator{}
	feedChunks(t, acc,
		`{"id":"chatcmpl-1","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant","content":"你"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"好"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		`{"id":"chatcmpl-1","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`,
		`{"done":true}`,
	)
	resp := acc.response()

	if got := resp["object"]; got != "chat.completion" {
		t.Errorf("object = %v, want chat.completion", got)
	}
	if got := resp["id"]; got != "chatcmpl-1" {
		t.Errorf("id = %v, want chatcmpl-1", got)
	}
	if got := resp["model"]; got != "gpt-test" {
		t.Errorf("model = %v, want gpt-test", got)
	}
	usage, ok := resp["usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("usage missing or wrong type: %v", resp["usage"])
	}
	if usage["total_tokens"].(float64) != 18 {
		t.Errorf("usage.total_tokens = %v, want 18", usage["total_tokens"])
	}
	if acc.chunks != 4 {
		t.Errorf("chunks = %d, want 4（[DONE] 标记不计数）", acc.chunks)
	}
	choice := firstChoice(t, resp)
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop", choice["finish_reason"])
	}
	msg := choice["message"].(map[string]interface{})
	if msg["content"] != "你好" {
		t.Errorf("content = %v, want 你好", msg["content"])
	}
	if _, ok := msg["tool_calls"]; ok {
		t.Errorf("纯文本回复不应携带 tool_calls")
	}
}

// TestStreamAccumulatorToolCallsMerge tool_calls 分片按上游 index 合并：
// arguments 逐段拼接、id/name 取首个非空（后续重复值不覆盖）、不同 index 各占一槽、
// 缺 index 兜底为新槽、finish_reason 缺失时按内容推断为 tool_calls
func TestStreamAccumulatorToolCallsMerge(t *testing.T) {
	acc := &streamAccumulator{}
	feedChunks(t, acc,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-a","type":"function","function":{"name":"write_asset","arguments":"{\"na"}}]}}]}`,
		// 中途重复的 id/name（异常上游）不得覆盖已合并的值
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-zz","function":{"name":"other","arguments":""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"me\":\"index.html\"}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call-b","function":{"name":"run_cmd","arguments":"{\"cmd\""}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":":\"ls\"}"}}]}}]}`,
		// 缺 index：规范要求携带，防御性兜底为新槽
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call-c","function":{"name":"noop","arguments":"{}"}}]}}]}`,
	)
	resp := acc.response()

	choice := firstChoice(t, resp)
	if choice["finish_reason"] != "tool_calls" {
		t.Errorf("finish_reason = %v, want tool_calls（按内容推断）", choice["finish_reason"])
	}

	raw, ok := choice["message"].(map[string]interface{})["tool_calls"]
	if !ok {
		t.Fatalf("message.tool_calls missing")
	}
	blob, _ := json.Marshal(raw)
	var calls []ai.ToolCall
	if err := json.Unmarshal(blob, &calls); err != nil {
		t.Fatalf("unmarshal tool_calls: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("len(tool_calls) = %d, want 3", len(calls))
	}
	if calls[0].ID != "call-a" || calls[0].Type != "function" || calls[0].Function.Name != "write_asset" {
		t.Errorf("call[0] = %+v", calls[0])
	}
	if calls[0].Function.Arguments != `{"name":"index.html"}` {
		t.Errorf("call[0].arguments = %q, want 拼接结果", calls[0].Function.Arguments)
	}
	if calls[1].ID != "call-b" || calls[1].Function.Name != "run_cmd" || calls[1].Function.Arguments != `{"cmd":"ls"}` {
		t.Errorf("call[1] = %+v", calls[1])
	}
	if calls[2].ID != "call-c" || calls[2].Function.Name != "noop" || calls[2].Function.Arguments != `{}` {
		t.Errorf("call[2] = %+v", calls[2])
	}
}

// TestStreamAccumulatorEmptyResponse 仅 [DONE]/nil 分片时也产出合法空响应：
// finish_reason 推断为 stop、content 显式置 nil、不携带 tool_calls/usage/id/model
func TestStreamAccumulatorEmptyResponse(t *testing.T) {
	acc := &streamAccumulator{}
	acc.absorb(nil) // 防御：nil 分片不 panic
	feedChunks(t, acc, `{"done":true}`)
	resp := acc.response()

	if acc.chunks != 0 {
		t.Errorf("chunks = %d, want 0", acc.chunks)
	}
	choice := firstChoice(t, resp)
	if choice["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v, want stop（推断）", choice["finish_reason"])
	}
	msg := choice["message"].(map[string]interface{})
	if c, ok := msg["content"]; !ok || c != nil {
		t.Errorf("content = %v (present=%v), want 显式 nil", c, ok)
	}
	if _, ok := msg["tool_calls"]; ok {
		t.Errorf("空响应不应携带 tool_calls")
	}
	for _, key := range []string{"usage", "id", "model"} {
		if _, ok := resp[key]; ok {
			t.Errorf("空响应不应携带 %s", key)
		}
	}
}

// TestStreamAccumulatorDownstreamContract 流式还原响应走 RunTask 下游真实消费路径：
// firstChoiceMessage 提取 → choice["tool_calls"] 经 json 往返解成 []ai.ToolCall →
// arguments 反序列化为参数表 → assistant 原样回传 + tool 回执组回 ai.Message 后经
// JSON 往返（buildRequestData 序列化路径）不丢字段
func TestStreamAccumulatorDownstreamContract(t *testing.T) {
	acc := &streamAccumulator{}
	feedChunks(t, acc,
		`{"id":"chatcmpl-9","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"我来写入文件"},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"write_asset","arguments":"{\"name\":\"a.html\",\"content\":\"<p>hi</p>\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`,
	)
	resp := acc.response()

	choice, err := firstChoiceMessage(resp)
	if err != nil {
		t.Fatalf("firstChoiceMessage: %v", err)
	}
	var toolCalls []ai.ToolCall
	if raw, ok := choice["tool_calls"]; ok && raw != nil {
		blob, _ := json.Marshal(raw)
		if err := json.Unmarshal(blob, &toolCalls); err != nil {
			t.Fatalf("parse tool_calls: %v", err)
		}
	}
	if len(toolCalls) != 1 || toolCalls[0].ID != "call-1" || toolCalls[0].Function.Name != "write_asset" {
		t.Fatalf("toolCalls = %+v", toolCalls)
	}
	var args map[string]string
	if err := json.Unmarshal([]byte(toolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("arguments 不是合法 JSON: %v", err)
	}
	if args["name"] != "a.html" {
		t.Errorf("arguments.name = %q, want a.html", args["name"])
	}

	content := choice["content"]
	outMsgs := []ai.Message{
		{Role: "assistant", Content: content, ToolCalls: toolCalls},
		{Role: "tool", Content: "written", ToolCallID: toolCalls[0].ID},
	}
	blob, err := json.Marshal(outMsgs)
	if err != nil {
		t.Fatalf("marshal messages: %v", err)
	}
	var back []ai.Message
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	if back[0].Content != "我来写入文件" {
		t.Errorf("assistant content = %v", back[0].Content)
	}
	if len(back[0].ToolCalls) != 1 || back[0].ToolCalls[0].ID != "call-1" {
		t.Errorf("assistant tool_calls 往返后丢失: %+v", back[0].ToolCalls)
	}
	if back[1].ToolCallID != "call-1" {
		t.Errorf("tool 回执 ToolCallID = %q, want call-1", back[1].ToolCallID)
	}
}

// --- 渐进披露：技能注入策略与 load_skill 工具 ---

// TestBuildSystemPromptSkillInjection 单技能全文直注；多技能仅注入 metadata 清单（正文须由模型 load_skill 装载）
func TestBuildSystemPromptSkillInjection(t *testing.T) {
	r := &AgentRuntime{}
	skills := []models.Skill{
		{ID: 1, Name: "alpha", Version: "1.0", Description: "首个技能简介", Content: "ALPHA_BODY"},
		{ID: 2, Name: "beta", Version: "2.0", Description: "第二个技能简介", Content: "BETA_BODY"},
	}

	single := r.buildSystemPrompt(skills[:1], "/tmp/ws", 5)
	assertContains(t, single, `<skill name="alpha" version="1.0">`)
	assertContains(t, single, "ALPHA_BODY")
	if strings.Contains(single, "可用技能（仅列出名称与简介") {
		t.Errorf("单技能任务不应注入 metadata 清单提示")
	}

	multi := r.buildSystemPrompt(skills, "/tmp/ws", 5)
	assertContains(t, multi, "- alpha（v1.0）：首个技能简介")
	assertContains(t, multi, "- beta（v2.0）：第二个技能简介")
	assertContains(t, multi, "load_skill")
	if strings.Contains(multi, "ALPHA_BODY") || strings.Contains(multi, "BETA_BODY") {
		t.Errorf("多技能任务不应注入任何 SKILL.md 正文")
	}
}

// TestExecuteToolLoadSkill load_skill 装载正文并记录 BodyLoaded；未知技能名报 TOOL ERROR；超长正文截断
func TestExecuteToolLoadSkill(t *testing.T) {
	r := &AgentRuntime{}
	skills := []models.Skill{{ID: 7, Name: "alpha", Version: "1.0", Content: "ALPHA_BODY"}}
	result := &AgentRunResult{SkillsUsed: []AgentSkillUsage{{ID: 7, Name: "alpha"}}}

	call := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "load_skill", Arguments: `{"skill":"alpha"}`}}
	got := r.executeTool(context.Background(), call, skills, "", result)
	assertContains(t, got, "已装载技能")
	assertContains(t, got, "ALPHA_BODY")
	if !result.SkillsUsed[0].BodyLoaded {
		t.Errorf("load_skill 成功后 BodyLoaded 应为 true")
	}

	miss := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "load_skill", Arguments: `{"skill":"ghost"}`}}
	got = r.executeTool(context.Background(), miss, skills, "", result)
	assertContains(t, got, "TOOL ERROR")

	big := strings.Repeat("x", maxSkillFileReadSize+10)
	skillsBig := []models.Skill{{ID: 8, Name: "big", Version: "2.0", Content: big}}
	resultBig := &AgentRunResult{SkillsUsed: []AgentSkillUsage{{ID: 8, Name: "big"}}}
	bigCall := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "load_skill", Arguments: `{"skill":"big"}`}}
	got = r.executeTool(context.Background(), bigCall, skillsBig, "", resultBig)
	assertContains(t, got, "[TRUNCATED")
	if !strings.Contains(got, strings.Repeat("x", maxSkillFileReadSize)) {
		t.Errorf("截断结果应包含前 %d 字节正文", maxSkillFileReadSize)
	}
	if strings.Contains(got, big) {
		t.Errorf("截断结果不应包含完整正文")
	}
}

// TestToolDefinitionsLoadSkill load_skill 工具注册：必填参数、渐进披露文案、
// read_skill_file/list_skill_files 的描述与 skill 参数应与「先 load_skill 装载主文档」的新机制一致
func TestToolDefinitionsLoadSkill(t *testing.T) {
	r := &AgentRuntime{}
	skills := []models.Skill{{ID: 1, Name: "alpha", Version: "1.0", Description: "首个技能简介", Content: "ALPHA_BODY"}}
	tools := r.toolDefinitions(skills)

	var loadSkill, readSkillFile, listSkillFiles map[string]interface{}
	for _, tl := range tools {
		f, _ := tl["function"].(map[string]interface{})
		switch {
		case f == nil:
		case f["name"] == "load_skill":
			loadSkill = f
		case f["name"] == "read_skill_file":
			readSkillFile = f
		case f["name"] == "list_skill_files":
			listSkillFiles = f
		}
	}
	if loadSkill == nil {
		t.Fatalf("toolDefinitions 未注册 load_skill")
	}
	if readSkillFile == nil || listSkillFiles == nil {
		t.Fatalf("read_skill_file/list_skill_files 缺失")
	}

	params := loadSkill["parameters"].(map[string]interface{})
	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "skill" {
		t.Errorf("load_skill required = %v, want [skill]", required)
	}
	assertContains(t, loadSkill["description"].(string), "装载指定技能的完整说明文档")
	assertContains(t, loadSkill["description"].(string), "无需调用")

	// read_skill_file 描述应引导先 load_skill；skill 参数描述不再声称 <skill name=...> 直注
	assertContains(t, readSkillFile["description"].(string), "load_skill")
	readProps := readSkillFile["parameters"].(map[string]interface{})["properties"].(map[string]interface{})
	assertContains(t, readProps["skill"].(map[string]interface{})["description"].(string), "技能清单")
	listProps := listSkillFiles["parameters"].(map[string]interface{})["properties"].(map[string]interface{})
	assertContains(t, listProps["skill"].(map[string]interface{})["description"].(string), "技能清单")

	// write_asset 描述应声明生成类产物已自动落盘、勿重写
	var writeAsset map[string]interface{}
	for _, tl := range tools {
		if f, _ := tl["function"].(map[string]interface{}); f != nil && f["name"] == "write_asset" {
			writeAsset = f
		}
	}
	if writeAsset == nil {
		t.Fatalf("toolDefinitions 未注册 write_asset")
	}
	assertContains(t, writeAsset["description"].(string), "不要再用本工具重写")
}

func TestExecuteToolWriteAssetOverwriteProtection(t *testing.T) {
	r := &AgentRuntime{}
	wsDir := t.TempDir()
	result := &AgentRunResult{}

	// 已有非空 png（模拟 generate_image 落盘成品）：空 content 与文本 content 均应拒绝覆盖
	existing := filepath.Join(wsDir, "poster.png")
	if err := os.WriteFile(existing, []byte(strings.Repeat("IM", 128)), 0o644); err != nil {
		t.Fatalf("预置文件失败: %v", err)
	}

	empty := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "write_asset", Arguments: `{"name":"poster.png","content":""}`}}
	got := r.executeTool(context.Background(), empty, []models.Skill{}, wsDir, result)
	assertContains(t, got, "TOOL ERROR")
	assertContains(t, got, "refusing to overwrite")
	if data, err := os.ReadFile(existing); err != nil || len(data) != 256 {
		t.Fatalf("已有文件应保持原样不被清空: size=%d err=%v", len(data), err)
	}

	text := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "write_asset", Arguments: `{"name":"poster.png","content":"fake text"}`}}
	got = r.executeTool(context.Background(), text, []models.Skill{}, wsDir, result)
	assertContains(t, got, "TOOL ERROR")
	assertContains(t, got, "generated asset")

	// 视频产物同理：generate_video 落盘后的 mp4 不允许被 write_asset 覆盖
	if err := os.WriteFile(filepath.Join(wsDir, "video.mp4"), []byte(strings.Repeat("MP", 128)), 0o644); err != nil {
		t.Fatalf("预置 video.mp4 失败: %v", err)
	}
	vid := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "write_asset", Arguments: `{"name":"video.mp4","content":""}`}}
	got = r.executeTool(context.Background(), vid, []models.Skill{}, wsDir, result)
	assertContains(t, got, "TOOL ERROR")
	assertContains(t, got, "generated asset")

	// 已有非空文本文件：正常内容允许覆盖更新
	if err := os.WriteFile(filepath.Join(wsDir, "note.md"), []byte("v1"), 0o644); err != nil {
		t.Fatalf("预置 note.md 失败: %v", err)
	}
	upd := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "write_asset", Arguments: `{"name":"note.md","content":"v2"}`}}
	got = r.executeTool(context.Background(), upd, []models.Skill{}, wsDir, result)
	assertContains(t, got, "OK: asset note.md saved (2 bytes)")

	// 新建空文件不受保护影响（目标不存在）
	newEmpty := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "write_asset", Arguments: `{"name":"empty.txt","content":""}`}}
	got = r.executeTool(context.Background(), newEmpty, []models.Skill{}, wsDir, result)
	assertContains(t, got, "OK: asset empty.txt saved (0 bytes)")
}

// TestMarkSkillBodyLoaded markSkillBodyLoaded 按 ID 置位、幂等且不误伤其他条目
func TestMarkSkillBodyLoaded(t *testing.T) {
	result := &AgentRunResult{SkillsUsed: []AgentSkillUsage{
		{ID: 1, Name: "alpha"},
		{ID: 2, Name: "beta"},
	}}
	markSkillBodyLoaded(result, 1)
	if !result.SkillsUsed[0].BodyLoaded {
		t.Errorf("ID=1 应被置为 BodyLoaded=true")
	}
	if result.SkillsUsed[1].BodyLoaded {
		t.Errorf("ID=2 不应被误置")
	}
	markSkillBodyLoaded(result, 1) // 幂等
	markSkillBodyLoaded(result, 99)
	if len(result.SkillsUsed) != 2 {
		t.Errorf("未知 ID 不应新增条目, len = %d", len(result.SkillsUsed))
	}
}

// TestExecuteToolUnknownTool 未知工具名走 default 兜底，报 TOOL ERROR
func TestExecuteToolUnknownTool(t *testing.T) {
	r := &AgentRuntime{}
	call := ai.ToolCall{Type: "function", Function: ai.ToolCallFunc{Name: "no_such_tool", Arguments: `{}`}}
	got := r.executeTool(context.Background(), call, nil, "", &AgentRunResult{})
	assertContains(t, got, "TOOL ERROR")
	assertContains(t, got, "no_such_tool")
}
