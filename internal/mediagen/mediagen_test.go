package mediagen_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tiktok-trend-shop/internal/mediagen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/store"
)

// openTestDB returns a fresh SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open("sqlite:///" + filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func strPtr(value string) *string { return &value }

// mustProduct creates a product with a detail snapshot carrying a brand and
// four selling points (one more than the prompt includes).
func mustProduct(t *testing.T, db *sql.DB) product.Product {
	t.Helper()
	repo := product.NewRepository(db)
	p, err := repo.UpsertProduct(context.Background(), product.ProductInput{
		Title: "Desk Lamp", Region: "US", Provider: "manual",
	})
	if err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	if _, err := repo.AddDetailSnapshot(context.Background(), product.DetailInput{
		ProductID:     p.ID,
		Provider:      "manual",
		Brand:         strPtr("Glow"),
		SellingPoints: []string{"dimmable", "flicker-free", "usb-c charging", "lightweight"},
	}); err != nil {
		t.Fatalf("add detail snapshot: %v", err)
	}
	return p
}

// newService builds a mediagen service with a fast poll interval for tests.
func newService(db *sql.DB, provider mediagen.Provider, storageRoot string) *mediagen.Service {
	svc := mediagen.NewService(
		product.NewRepository(db),
		mediagen.NewAssetStore(db, storageRoot),
		mediagen.NewRepository(db),
		provider,
	)
	svc.PollInterval = 5 * time.Millisecond
	return svc
}

// waitTask polls the task list until the task reaches a terminal state.
func waitTask(t *testing.T, svc *mediagen.Service, productID, taskID string, timeout time.Duration) mediagen.ImageGenTask {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tasks, err := svc.ListTasks(context.Background(), productID)
		if err != nil {
			t.Fatalf("list tasks: %v", err)
		}
		for _, task := range tasks {
			if task.ID == taskID &&
				(task.Status == mediagen.StatusSucceeded || task.Status == mediagen.StatusFailed) {
				return task
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach a terminal state within %s", taskID, timeout)
	return mediagen.ImageGenTask{}
}

// fakeComfy is an httptest double of the ComfyUI REST API.
type fakeComfy struct {
	server *httptest.Server

	mu        sync.Mutex
	workflows []map[string]any
	png       []byte
}

func newFakeComfy(t *testing.T) *fakeComfy {
	t.Helper()
	f := &fakeComfy{png: mediagen.MockPNG()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /system_stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"system":{}}`)
	})
	mux.HandleFunc("POST /prompt", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		var workflow map[string]any
		if err := json.Unmarshal(body["prompt"], &workflow); err != nil {
			http.Error(w, "invalid workflow", http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.workflows = append(f.workflows, workflow)
		promptID := fmt.Sprintf("pid-%d", len(f.workflows))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"prompt_id":%q}`, promptID)
	})
	mux.HandleFunc("GET /history/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{%q:{"outputs":{"60":{"images":[{"filename":"tkshop_00001_.png","subfolder":"","type":"output"}]}}}}`,
			r.PathValue("id"))
	})
	mux.HandleFunc("GET /view", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filename") == "" {
			http.Error(w, "filename required", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(f.png)
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

// lastWorkflow returns the most recently submitted workflow.
func (f *fakeComfy) lastWorkflow() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.workflows[len(f.workflows)-1]
}

// nodeInputs returns the inputs object of one workflow node.
func nodeInputs(t *testing.T, workflow map[string]any, node string) map[string]any {
	t.Helper()
	entry, ok := workflow[node].(map[string]any)
	if !ok {
		t.Fatalf("workflow node %s missing", node)
	}
	inputs, ok := entry["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("workflow node %s has no inputs", node)
	}
	return inputs
}

func TestGenerateLifecycleWithFakeComfyUI(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	fake := newFakeComfy(t)
	storageRoot := filepath.Join(t.TempDir(), "assets")
	svc := newService(db, mediagen.NewComfyUIProvider(fake.server.URL, 5*time.Second), storageRoot)

	if status := svc.Health(ctx); !status.Available || status.Provider != "comfyui" {
		t.Fatalf("expected available comfyui health got %#v", status)
	}

	task, err := svc.Generate(ctx, p.ID, "", "", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if task.Status != mediagen.StatusQueued {
		t.Fatalf("expected queued task got %q", task.Status)
	}
	if task.Usage != "main" || task.Style != "clean" || task.Quality != "fast" {
		t.Fatalf("expected default brief got usage=%q style=%q quality=%q", task.Usage, task.Style, task.Quality)
	}
	if task.Width != 1024 || task.Height != 1024 {
		t.Fatalf("expected 1024x1024 got %dx%d", task.Width, task.Height)
	}
	wantPrompt := "product marketing photo of Glow Desk Lamp, dimmable, flicker-free, usb-c charging, main use, clean style, professional e-commerce photography"
	if task.Prompt != wantPrompt {
		t.Fatalf("unexpected prompt\nwant: %s\ngot:  %s", wantPrompt, task.Prompt)
	}

	final := waitTask(t, svc, p.ID, task.ID, 5*time.Second)
	if final.Status != mediagen.StatusSucceeded {
		t.Fatalf("expected succeeded got %q (last_error=%v)", final.Status, final.LastError)
	}
	if final.ComfyPromptID == nil || *final.ComfyPromptID != "pid-1" {
		t.Fatalf("expected comfy prompt id pid-1 got %v", final.ComfyPromptID)
	}
	if final.AssetID == nil || final.CompletedAt == nil {
		t.Fatalf("expected asset id and completed_at on task %#v", final)
	}

	// The submitted workflow carries the rendered parameters.
	workflow := fake.lastWorkflow()
	if text := nodeInputs(t, workflow, "20")["text"]; text != wantPrompt {
		t.Fatalf("workflow prompt mismatch: %v", text)
	}
	if steps := nodeInputs(t, workflow, "41")["steps"]; steps != float64(4) {
		t.Fatalf("expected fast preset 4 steps got %v", steps)
	}
	if cfg := nodeInputs(t, workflow, "42")["cfg"]; cfg != float64(1) {
		t.Fatalf("expected fast preset cfg 1 got %v", cfg)
	}
	if width := nodeInputs(t, workflow, "30")["width"]; width != float64(1024) {
		t.Fatalf("expected width 1024 got %v", width)
	}
	if seed := nodeInputs(t, workflow, "40")["noise_seed"]; seed != float64(task.Seed) {
		t.Fatalf("expected workflow seed %d got %v", task.Seed, seed)
	}

	// The image is persisted as a content-addressed asset on disk and in the DB.
	assets, err := svc.ListImages(ctx, p.ID)
	if err != nil {
		t.Fatalf("list images: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset got %d", len(assets))
	}
	asset := assets[0]
	if asset.ID != *final.AssetID {
		t.Fatalf("task asset id %q does not match asset %q", *final.AssetID, asset.ID)
	}
	if asset.Kind != mediagen.KindGeneratedImage || asset.Backend != "local" || asset.ContentType != "image/png" {
		t.Fatalf("unexpected asset %#v", asset)
	}
	sum := sha256.Sum256(fake.png)
	if asset.Checksum != hex.EncodeToString(sum[:]) {
		t.Fatalf("unexpected checksum %q", asset.Checksum)
	}
	wantURI := "/assets/generated-image/" + asset.Checksum[:12] + ".png"
	if asset.URI != wantURI {
		t.Fatalf("expected uri %q got %q", wantURI, asset.URI)
	}
	if asset.ByteSize != int64(len(fake.png)) {
		t.Fatalf("expected byte size %d got %d", len(fake.png), asset.ByteSize)
	}
	if asset.ProductID == nil || *asset.ProductID != p.ID {
		t.Fatalf("expected product id %q got %v", p.ID, asset.ProductID)
	}
	if asset.Metadata["task_id"] != task.ID {
		t.Fatalf("expected task id in asset metadata got %#v", asset.Metadata)
	}
	onDisk, err := os.ReadFile(filepath.Join(storageRoot, "generated-image", asset.Checksum[:12]+".png"))
	if err != nil {
		t.Fatalf("read stored image: %v", err)
	}
	if !bytes.Equal(onDisk, fake.png) {
		t.Fatalf("stored image bytes mismatch")
	}
}

func TestGenerateHQQualityPreset(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	fake := newFakeComfy(t)
	svc := newService(db, mediagen.NewComfyUIProvider(fake.server.URL, 5*time.Second), filepath.Join(t.TempDir(), "assets"))

	task, err := svc.Generate(ctx, p.ID, "scene", "studio", "hq")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	final := waitTask(t, svc, p.ID, task.ID, 5*time.Second)
	if final.Status != mediagen.StatusSucceeded {
		t.Fatalf("expected succeeded got %q (last_error=%v)", final.Status, final.LastError)
	}
	workflow := fake.lastWorkflow()
	if steps := nodeInputs(t, workflow, "41")["steps"]; steps != float64(20) {
		t.Fatalf("expected hq preset 20 steps got %v", steps)
	}
	if cfg := nodeInputs(t, workflow, "42")["cfg"]; cfg != float64(5) {
		t.Fatalf("expected hq preset cfg 5 got %v", cfg)
	}
}

func TestGenerateWithMockProvider(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	storageRoot := filepath.Join(t.TempDir(), "assets")
	svc := newService(db, &mediagen.MockProvider{}, storageRoot)

	status := svc.Health(ctx)
	if !status.Available || status.Provider != "mock" {
		t.Fatalf("expected available mock health got %#v", status)
	}

	task, err := svc.Generate(ctx, p.ID, "detail", "studio", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	final := waitTask(t, svc, p.ID, task.ID, 5*time.Second)
	if final.Status != mediagen.StatusSucceeded {
		t.Fatalf("expected succeeded got %q (last_error=%v)", final.Status, final.LastError)
	}
	if final.ComfyPromptID == nil || !strings.HasPrefix(*final.ComfyPromptID, "mock_prompt_") {
		t.Fatalf("expected mock prompt id got %v", final.ComfyPromptID)
	}
	assets, err := svc.ListImages(ctx, p.ID)
	if err != nil {
		t.Fatalf("list images: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset got %d", len(assets))
	}
	onDisk, err := os.ReadFile(filepath.Join(storageRoot, "generated-image", assets[0].Checksum[:12]+".png"))
	if err != nil {
		t.Fatalf("read stored image: %v", err)
	}
	if !bytes.Equal(onDisk, mediagen.MockPNG()) {
		t.Fatalf("expected mock png bytes")
	}
	if !bytes.HasPrefix(onDisk, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("stored bytes are not a PNG")
	}
}

func TestGenerateSubmitFailureMarksTaskFailed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	p := mustProduct(t, db)
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/system_stats" {
			fmt.Fprint(w, `{}`)
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	svc := newService(db, mediagen.NewComfyUIProvider(failing.URL, 5*time.Second), filepath.Join(t.TempDir(), "assets"))

	task, err := svc.Generate(ctx, p.ID, "", "", "")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	final := waitTask(t, svc, p.ID, task.ID, 5*time.Second)
	if final.Status != mediagen.StatusFailed {
		t.Fatalf("expected failed got %q", final.Status)
	}
	if final.LastError == nil || *final.LastError == "" {
		t.Fatalf("expected last_error on failed task %#v", final)
	}
	assets, err := svc.ListImages(ctx, p.ID)
	if err != nil {
		t.Fatalf("list images: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected no assets on failure got %d", len(assets))
	}
}

func TestHealthUnavailable(t *testing.T) {
	db := openTestDB(t)
	svc := newService(db, mediagen.NewComfyUIProvider("http://127.0.0.1:1", 300*time.Millisecond), filepath.Join(t.TempDir(), "assets"))
	status := svc.Health(context.Background())
	if status.Provider != "comfyui" {
		t.Fatalf("expected comfyui provider got %q", status.Provider)
	}
	if status.Available {
		t.Fatalf("expected unavailable health")
	}
	if status.Detail == "" {
		t.Fatalf("expected failure detail")
	}
}

func TestGenerateProductNotFound(t *testing.T) {
	svc := newService(openTestDB(t), &mediagen.MockProvider{}, filepath.Join(t.TempDir(), "assets"))
	if _, err := svc.Generate(context.Background(), "prod_missing", "", "", ""); err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows got %v", err)
	}
}
