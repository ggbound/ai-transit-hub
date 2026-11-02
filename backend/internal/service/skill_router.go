package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ai "github.com/your-org/ai-transit-hub/internal/ai"
	"github.com/your-org/ai-transit-hub/internal/models"
)

// SkillRouteDecision 需求→SKILL 语义路由决策（/v1/agent/dispatch）
type SkillRouteDecision struct {
	Skills []string `json:"skills"`           // 选中的技能名（已过滤为目录内存在的名称）
	Model  string   `json:"model,omitempty"`  // 建议的执行模型（provider/model，可为空）
	Reason string   `json:"reason,omitempty"` // 一句话路由理由（可观测）
}

// RouteSkills 用路由模型在技能目录上做语义路由：
// system prompt 给出目录（name/description/tags），要求模型只输出一个 JSON 决策对象。
// 返回的 Skills 已过滤为目录内存在的名称（保持模型给出的顺序）。
func RouteSkills(ctx context.Context, input string, catalog []models.Skill, adapter ai.Adapter) (*SkillRouteDecision, error) {
	var b strings.Builder
	b.WriteString("name | description | tags\n")
	for _, s := range catalog {
		b.WriteString(fmt.Sprintf("- %s | %s | %s\n", s.Name, s.Description, strings.Join(decodeSkillTags(s.Tags), ",")))
	}

	systemPrompt := fmt.Sprintf(`你是技能路由器。根据用户需求，从技能目录中选择最适合执行的技能，并可建议执行模型。

技能目录：
%s

规则：
1. 只输出一个 JSON 对象，不要输出任何其他文字、解释或代码围栏：
   {"skills": ["<name>"], "model": "", "reason": "<一句话理由>"}
2. skills 只能从技能目录的 name 中选择，可多选；没有合适技能时输出空数组。
3. model 为 provider/model 格式的执行模型建议；不确定时留空字符串。
4. reason 用一句话说明选择依据。`, b.String())

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: input},
	}
	resp, err := adapter.Chat(ctx, messages, map[string]interface{}{
		"temperature": 0,
		"max_tokens":  512,
	})
	if err != nil {
		return nil, err
	}

	valid := make(map[string]bool, len(catalog))
	for _, s := range catalog {
		valid[s.Name] = true
	}

	decision := &SkillRouteDecision{}
	if err := json.Unmarshal([]byte(extractJSONObject(extractChatContent(resp))), decision); err != nil {
		return nil, fmt.Errorf("router returned unparseable response: %w", err)
	}
	// 过滤目录内不存在的名称 + 去重
	seen := make(map[string]bool, len(decision.Skills))
	filtered := make([]string, 0, len(decision.Skills))
	for _, n := range decision.Skills {
		n = strings.TrimSpace(n)
		if n == "" || !valid[n] || seen[n] {
			continue
		}
		seen[n] = true
		filtered = append(filtered, n)
	}
	decision.Skills = filtered
	if len(decision.Skills) == 0 {
		return nil, fmt.Errorf("router matched no skill from catalog")
	}
	return decision, nil
}

// extractChatContent 提取标准 OpenAI 格式响应中首条消息的文本内容
func extractChatContent(resp map[string]interface{}) string {
	choices, _ := resp["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	first, _ := choices[0].(map[string]interface{})
	if first == nil {
		return ""
	}
	msg, _ := first["message"].(map[string]interface{})
	if msg == nil {
		return ""
	}
	content, _ := msg["content"].(string)
	return content
}

// extractJSONObject 提取文本中首个平衡的 JSON 对象（容忍代码围栏与前后缀文本）
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if esc {
			esc = false
			continue
		}
		switch {
		case ch == '\\' && inStr:
			esc = true
		case ch == '"':
			inStr = !inStr
		case inStr:
			// 字符串内字符不参与括号计数
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

// decodeSkillTags 解析技能 tags（JSON 数组；格式异常时忽略）
func decodeSkillTags(raw models.JSON) []string {
	var tags []string
	if len(raw) > 0 && json.Unmarshal(raw, &tags) == nil {
		return tags
	}
	return nil
}
