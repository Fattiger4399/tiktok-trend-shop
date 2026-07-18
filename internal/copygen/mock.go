package copygen

import (
	"context"
	"fmt"
	"strings"
)

// MockProvider is a deterministic Provider for development and tests: the
// same input always yields the same output. It composes drafts heuristically
// from the product profile across three fixed angles (痛点型/卖点型/场景型).
type MockProvider struct{}

func (MockProvider) Name() string { return "mock" }

// Model reports no concrete model for the mock provider.
func (MockProvider) Model() string { return "" }

func (MockProvider) Generate(_ context.Context, input GenInput) ([]VariantDraft, error) {
	count := input.VariantCount
	if count <= 0 {
		count = 3
	}
	title := strings.TrimSpace(input.ProductTitle)
	if title == "" {
		title = "这款产品"
	}
	display := title
	if brand := strings.TrimSpace(input.Brand); brand != "" {
		display = brand + " " + title
	}
	point := "品质与性价比兼具"
	if len(input.SellingPoints) > 0 && strings.TrimSpace(input.SellingPoints[0]) != "" {
		point = strings.TrimSpace(input.SellingPoints[0])
	}
	style := strings.TrimSpace(input.Style)
	if style == "" {
		style = "种草"
	}
	focus := strings.TrimSpace(input.Focus)
	if focus == "" {
		focus = point
	}
	usage := strings.TrimSpace(input.Usage)
	if usage == "" {
		usage = "短视频带货"
	}

	angles := []struct {
		name string
		hook string
		body string
	}{
		{
			name: "痛点型",
			hook: "还在为选不到合适的" + title + "发愁？",
			body: display + "针对日常使用痛点设计，" + focus + "的表现让人放心。",
		},
		{
			name: "卖点型",
			hook: display + "凭什么被反复回购？",
			body: "核心卖点是" + point + "，以" + style + "风格呈现，适合" + usage + "。",
		},
		{
			name: "场景型",
			hook: "把" + display + "带进日常的一天",
			body: "真实使用场景实测：" + point + "。" + sceneSuffix(input),
		},
	}
	drafts := make([]VariantDraft, 0, count)
	for i := 0; i < count; i++ {
		angle := angles[i%len(angles)]
		drafts = append(drafts, VariantDraft{
			Hook:    angle.hook,
			Body:    angle.body,
			Caption: fmt.Sprintf("%s｜%s", angle.name, display),
			Hashtags: []string{
				"#好物推荐",
				"#" + angle.name,
				"#" + strings.ReplaceAll(style, " ", ""),
			},
		})
	}
	return drafts, nil
}

func (MockProvider) Prefill(_ context.Context, input PrefillInput) (PrefillSuggestion, error) {
	suggestion := PrefillSuggestion{
		Usage: "短视频带货",
		Style: "真实测评",
		Focus: "整体品质",
	}
	if len(input.SellingPoints) > 0 && strings.TrimSpace(input.SellingPoints[0]) != "" {
		suggestion.Focus = strings.TrimSpace(input.SellingPoints[0])
	}
	if strings.TrimSpace(input.ReviewSummary) != "" {
		suggestion.Style = "痛点共鸣"
	}
	notes := []string{}
	if brand := strings.TrimSpace(input.Brand); brand != "" {
		notes = append(notes, "突出品牌 "+brand)
	}
	if len(input.SellingPoints) > 1 {
		notes = append(notes, fmt.Sprintf("覆盖 %d 个卖点", len(input.SellingPoints)))
	}
	suggestion.Notes = strings.Join(notes, "；")
	return suggestion, nil
}

// sceneSuffix folds the review summary into the 场景型 body when available.
func sceneSuffix(input GenInput) string {
	summary := strings.TrimSpace(input.ReviewSummary)
	if summary == "" {
		return "评论区反馈整体稳定。"
	}
	return "评论摘要：" + summary
}
