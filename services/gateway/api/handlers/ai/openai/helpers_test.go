package openai

import "testing"

func TestBuildContentPartsUsesHighDetailForUIScreenshot(t *testing.T) {
	parts := buildContentParts("inspect this CRM card", []string{"data:image/png;base64,eA=="})
	if len(parts) != 2 {
		t.Fatalf("got %d content parts, want 2", len(parts))
	}
	if parts[0].ImageURL == nil {
		t.Fatal("image content part is missing image_url")
	}
	if parts[0].ImageURL.Detail != "high" {
		t.Fatalf("got image detail %q, want high", parts[0].ImageURL.Detail)
	}
}
