package askquestion

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestImagePasteStore_AddAndGet(t *testing.T) {
	store := NewImagePasteStore()

	img := PastedContent{ID: "img_1", Type: "image", Content: "base64data", MediaType: "image/png"}
	store.Add("Q1", img)

	items := store.Get("Q1")
	if len(items) != 1 {
		t.Fatalf("expected 1 image, got %d", len(items))
	}
	if items[0].ID != "img_1" {
		t.Fatalf("expected img_1, got %s", items[0].ID)
	}
}

func TestImagePasteStore_Remove(t *testing.T) {
	store := NewImagePasteStore()
	store.Add("Q1", PastedContent{ID: "img_1", Type: "image"})
	store.Add("Q1", PastedContent{ID: "img_2", Type: "image"})

	store.Remove("Q1", "img_1")
	items := store.Get("Q1")
	if len(items) != 1 {
		t.Fatalf("expected 1 image after remove, got %d", len(items))
	}
	if items[0].ID != "img_2" {
		t.Fatalf("expected img_2, got %s", items[0].ID)
	}
}

func TestImagePasteStore_All(t *testing.T) {
	store := NewImagePasteStore()
	store.Add("Q1", PastedContent{ID: "img_1"})
	store.Add("Q2", PastedContent{ID: "img_2"})

	all := store.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 total images, got %d", len(all))
	}
}

func TestImagePasteStore_HasImages(t *testing.T) {
	store := NewImagePasteStore()
	if store.HasImages() {
		t.Fatal("should not have images initially")
	}
	store.Add("Q1", PastedContent{ID: "img_1"})
	if !store.HasImages() {
		t.Fatal("should have images after add")
	}
}

func TestDetectBase64Image_DataURI(t *testing.T) {
	data := "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, 200))
	img := DetectBase64Image(data)
	if img == nil {
		t.Fatal("should detect data URI image")
	}
	if img.MediaType != "image/png" {
		t.Fatalf("expected image/png, got %s", img.MediaType)
	}
	if img.Type != "image" {
		t.Fatalf("expected type 'image', got %s", img.Type)
	}
}

func TestDetectBase64Image_RawBase64(t *testing.T) {
	// Create a large enough valid base64 string (>500 chars after encoding)
	raw := base64.StdEncoding.EncodeToString(make([]byte, 400))
	img := DetectBase64Image(raw)
	if img == nil {
		t.Fatal("should detect raw base64 image")
	}
	if img.MediaType != "image/png" {
		t.Fatalf("expected default image/png, got %s", img.MediaType)
	}
}

func TestDetectBase64Image_NotImage(t *testing.T) {
	img := DetectBase64Image("hello world")
	if img != nil {
		t.Fatal("should not detect plain text as image")
	}

	img = DetectBase64Image("short")
	if img != nil {
		t.Fatal("should not detect short string as image")
	}
}

func TestRenderImageAttachments(t *testing.T) {
	if RenderImageAttachments(nil) != "" {
		t.Error("should return empty for nil")
	}
	if RenderImageAttachments([]PastedContent{}) != "" {
		t.Error("should return empty for empty slice")
	}

	result := RenderImageAttachments([]PastedContent{{ID: "1"}})
	if result != "(1 image attached)" {
		t.Errorf("expected '(1 image attached)', got %q", result)
	}

	result = RenderImageAttachments([]PastedContent{{ID: "1"}, {ID: "2"}, {ID: "3"}})
	if !strings.Contains(result, "3 images") {
		t.Errorf("expected '3 images attached', got %q", result)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}
	if !strings.HasPrefix(id1, "img_") {
		t.Errorf("expected img_ prefix, got %q", id1)
	}
}
