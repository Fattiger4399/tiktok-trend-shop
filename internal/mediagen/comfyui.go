package mediagen

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed workflow/text_to_image.json
var textToImageWorkflow string

// ComfyUIProvider talks to a ComfyUI REST API: it queues the embedded
// text-to-image workflow via POST /prompt, polls GET /history/{prompt_id} for
// completion and downloads the rendered outputs through GET /view.
type ComfyUIProvider struct {
	baseURL string
	timeout time.Duration
	client  *http.Client
}

// NewComfyUIProvider builds a provider for a ComfyUI endpoint. The timeout
// bounds each HTTP call and is also reported as the generation cycle budget.
func NewComfyUIProvider(baseURL string, timeout time.Duration) *ComfyUIProvider {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &ComfyUIProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (p *ComfyUIProvider) Name() string { return "comfyui" }

// Timeout reports the configured timeout.
func (p *ComfyUIProvider) Timeout() time.Duration { return p.timeout }

// Available pings GET /system_stats.
func (p *ComfyUIProvider) Available(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/system_stats", nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("mediagen: ComfyUI unreachable: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mediagen: ComfyUI /system_stats returned %s", resp.Status)
	}
	return nil
}

// Submit renders the workflow template with the job parameters and queues it,
// returning the ComfyUI prompt_id.
func (p *ComfyUIProvider) Submit(ctx context.Context, input JobInput) (string, error) {
	workflow, err := RenderWorkflow(input)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{"prompt": json.RawMessage(workflow)})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/prompt", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mediagen: ComfyUI submit failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("mediagen: read ComfyUI submit response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mediagen: ComfyUI /prompt returned %s: %.200s", resp.Status, string(raw))
	}
	var decoded struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("mediagen: decode ComfyUI submit response: %w (body: %.200s)", err, string(raw))
	}
	if decoded.PromptID == "" {
		return "", fmt.Errorf("mediagen: ComfyUI submit returned no prompt_id (body: %.200s)", string(raw))
	}
	return decoded.PromptID, nil
}

// outputImageRef is one image file reference inside a history outputs entry.
type outputImageRef struct {
	Filename  string `json:"filename"`
	Subfolder string `json:"subfolder"`
	Type      string `json:"type"`
}

// Fetch reports whether the prompt finished. Once the history entry carries
// outputs it downloads every non-temp image through GET /view.
func (p *ComfyUIProvider) Fetch(ctx context.Context, promptID string) (bool, [][]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/history/"+url.PathEscape(promptID), nil)
	if err != nil {
		return false, nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false, nil, fmt.Errorf("mediagen: ComfyUI history request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return false, nil, fmt.Errorf("mediagen: read ComfyUI history: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return false, nil, fmt.Errorf("mediagen: ComfyUI /history returned %s: %.200s", resp.Status, string(raw))
	}
	var history map[string]struct {
		Outputs map[string]struct {
			Images []outputImageRef `json:"images"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal(raw, &history); err != nil {
		return false, nil, fmt.Errorf("mediagen: decode ComfyUI history: %w (body: %.200s)", err, string(raw))
	}
	entry, ok := history[promptID]
	if !ok || len(entry.Outputs) == 0 {
		return false, nil, nil
	}
	var refs []outputImageRef
	for _, node := range entry.Outputs {
		for _, ref := range node.Images {
			if ref.Type == "temp" {
				continue
			}
			refs = append(refs, ref)
		}
	}
	images := make([][]byte, 0, len(refs))
	for _, ref := range refs {
		data, err := p.download(ctx, ref)
		if err != nil {
			return false, nil, err
		}
		images = append(images, data)
	}
	return true, images, nil
}

// download fetches one rendered image through GET /view.
func (p *ComfyUIProvider) download(ctx context.Context, ref outputImageRef) ([]byte, error) {
	query := url.Values{}
	query.Set("filename", ref.Filename)
	if ref.Subfolder != "" {
		query.Set("subfolder", ref.Subfolder)
	}
	imageType := ref.Type
	if imageType == "" {
		imageType = "output"
	}
	query.Set("type", imageType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/view?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mediagen: ComfyUI view request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("mediagen: read ComfyUI image: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mediagen: ComfyUI /view returned %s: %.200s", resp.Status, string(raw))
	}
	return raw, nil
}

// RenderWorkflow substitutes the template placeholders with the job parameters
// and returns the workflow as a JSON string. The prompt is escaped for
// embedding inside a JSON string literal; numeric slots are plain digits.
func RenderWorkflow(input JobInput) (string, error) {
	seed := input.Seed
	if seed < 0 {
		seed = -seed
	}
	replacements := map[string]string{
		"{{PROMPT}}": jsonStringContent(input.Prompt),
		"{{SEED}}":   strconv.FormatInt(seed, 10),
		"{{WIDTH}}":  strconv.Itoa(input.Width),
		"{{HEIGHT}}": strconv.Itoa(input.Height),
		"{{STEPS}}":  strconv.Itoa(input.Steps),
		"{{CFG}}":    strconv.FormatFloat(input.CFG, 'f', -1, 64),
	}
	rendered := textToImageWorkflow
	for placeholder, value := range replacements {
		rendered = strings.ReplaceAll(rendered, placeholder, value)
	}
	if strings.Contains(rendered, "{{") {
		return "", fmt.Errorf("mediagen: workflow template has unreplaced placeholders")
	}
	if !json.Valid([]byte(rendered)) {
		return "", fmt.Errorf("mediagen: rendered workflow is not valid JSON")
	}
	return rendered, nil
}

// jsonStringContent escapes s so it can be embedded inside a JSON string
// literal (the template already carries the surrounding quotes).
func jsonStringContent(s string) string {
	encoded, err := json.Marshal(s)
	if err != nil || len(encoded) < 2 {
		return s
	}
	return string(encoded[1 : len(encoded)-1])
}
