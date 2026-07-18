package copygen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// OpenAIProvider talks to any OpenAI-compatible chat/completions endpoint
// (DeepSeek, 通义千问 compatible mode, etc.). Models are asked to answer with
// strict JSON which is then parsed into drafts.
type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIProvider builds a provider for an OpenAI-compatible endpoint. The
// base URL is the API root (e.g. https://api.openai.com/v1), timeout bounds
// each HTTP call.
func NewOpenAIProvider(baseURL, apiKey, model string, timeout time.Duration) *OpenAIProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &OpenAIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (p *OpenAIProvider) Name() string { return "openai" }

// Model reports the configured model name.
func (p *OpenAIProvider) Model() string { return p.model }

func (p *OpenAIProvider) Generate(ctx context.Context, input GenInput) ([]VariantDraft, error) {
	count := input.VariantCount
	if count <= 0 {
		count = 3
	}
	system := "你是一名资深的电商短视频文案策划。" +
		"只输出严格 JSON，不要输出任何其他文字，格式：" +
		`{"variants":[{"hook":"开头钩子","body":"正文","caption":"发布文案","hashtags":["#标签"]}]}` +
		"，hashtags 为字符串数组。"
	user := fmt.Sprintf("请为以下商品生成 %d 条不同角度的宣传文案变体（如痛点型/卖点型/场景型）。\n%s",
		count, describeProduct(input.ProductTitle, input.Brand, input.SellingPoints, input.Specs, input.ReviewSummary)+
			fmt.Sprintf("投放用途：%s\n文案风格：%s\n强调重点：%s\n备注：%s\n",
				emptyDash(input.Usage), emptyDash(input.Style), emptyDash(input.Focus), emptyDash(input.Notes)))
	content, err := p.chat(ctx, system, user)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Variants []VariantDraft `json:"variants"`
	}
	if err := json.Unmarshal([]byte(stripCodeFence(content)), &parsed); err != nil {
		return nil, fmt.Errorf("copygen: parse LLM variants response: %w (content: %.200s)", err, content)
	}
	if len(parsed.Variants) == 0 {
		return nil, fmt.Errorf("copygen: LLM returned no variants (content: %.200s)", content)
	}
	return parsed.Variants, nil
}

func (p *OpenAIProvider) Prefill(ctx context.Context, input PrefillInput) (PrefillSuggestion, error) {
	system := "你是一名资深的电商投放策划。" +
		"只输出严格 JSON，不要输出任何其他文字，格式：" +
		`{"usage":"投放用途","style":"文案风格","focus":"强调重点","notes":"备注"}。`
	user := "请根据以下商品档案，为物料需求单草拟 usage/style/focus/notes 四个字段。\n" +
		describeProduct(input.ProductTitle, input.Brand, input.SellingPoints, input.Specs, input.ReviewSummary)
	content, err := p.chat(ctx, system, user)
	if err != nil {
		return PrefillSuggestion{}, err
	}
	var suggestion PrefillSuggestion
	if err := json.Unmarshal([]byte(stripCodeFence(content)), &suggestion); err != nil {
		return PrefillSuggestion{}, fmt.Errorf("copygen: parse LLM prefill response: %w (content: %.200s)", err, content)
	}
	return suggestion, nil
}

// chat performs one chat/completions call and returns the first message
// content.
func (p *OpenAIProvider) chat(ctx context.Context, system, user string) (string, error) {
	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("copygen: LLM request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("copygen: read LLM response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("copygen: LLM endpoint returned %s: %.200s", resp.Status, string(raw))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("copygen: decode LLM response: %w (body: %.200s)", err, string(raw))
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("copygen: LLM response has no choices (body: %.200s)", string(raw))
	}
	return decoded.Choices[0].Message.Content, nil
}

// describeProduct renders the product profile as deterministic prompt lines.
func describeProduct(title, brand string, sellingPoints []string, specs map[string]any, reviewSummary string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "商品标题：%s\n品牌：%s\n", emptyDash(title), emptyDash(brand))
	if len(sellingPoints) > 0 {
		fmt.Fprintf(&b, "卖点：%s\n", strings.Join(sellingPoints, "；"))
	}
	if len(specs) > 0 {
		keys := make([]string, 0, len(specs))
		for key := range specs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		pairs := make([]string, 0, len(keys))
		for _, key := range keys {
			pairs = append(pairs, fmt.Sprintf("%s=%v", key, specs[key]))
		}
		fmt.Fprintf(&b, "规格：%s\n", strings.Join(pairs, "；"))
	}
	if strings.TrimSpace(reviewSummary) != "" {
		fmt.Fprintf(&b, "评论摘要：%s\n", reviewSummary)
	}
	return b.String()
}

// stripCodeFence removes a surrounding ```json ... ``` fence if the model
// added one despite the instructions.
func stripCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
	}
	return trimmed
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "无"
	}
	return value
}
