package v1

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestApplyCapacitorScaffoldAddsRuntimeContract(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", nil)
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}

	contentByPath := make(map[string]string, len(files))
	for _, file := range files {
		contentByPath[file.Path] = file.Content
	}

	for _, expected := range []string{
		`"@capacitor/core": "^8.0.0"`,
		`"@capacitor/android": "^8.0.0"`,
		`"cap:sync": "npm run build && npx cap sync"`,
		`"node": ">=22.0.0"`,
	} {
		if !strings.Contains(contentByPath["package.json"], expected) {
			t.Fatalf("expected package.json to contain %q", expected)
		}
	}
	for _, expected := range []string{`appId: "run.ucode.pocketcrm12345678"`, `webDir: "build"`} {
		if !strings.Contains(contentByPath["capacitor.config.ts"], expected) {
			t.Fatalf("expected capacitor.config.ts to contain %q", expected)
		}
	}
	if contentByPath["src/lib/capacitor.ts"] != capacitorBridge {
		t.Fatal("expected canonical Capacitor bridge")
	}
	if contentByPath["mobile.capabilities.json"] == "" {
		t.Fatal("expected mobile capability manifest")
	}
	for _, expected := range []string{"viewport-fit=cover", `id="root"`, `src="/src/main.tsx"`} {
		if !strings.Contains(contentByPath["index.html"], expected) {
			t.Fatalf("expected index.html to contain %q", expected)
		}
	}
	if !strings.Contains(contentByPath["src/App.tsx"], "HashRouter") ||
		strings.Contains(contentByPath["src/App.tsx"], "BrowserRouter") {
		t.Fatal("expected Capacitor scaffold to normalize App.tsx to HashRouter")
	}
}

func TestApplyCapacitorScaffoldAddsApprovedCapabilityWrappers(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", []models.MobileCapability{
		models.MobileCapabilityLocalNotifications,
		models.MobileCapabilityIdentityVerification,
	})
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}

	contentByPath := make(map[string]string, len(files))
	for _, file := range files {
		contentByPath[file.Path] = file.Content
	}
	for _, dependency := range []string{`"@capacitor/camera": "^8.0.0"`, `"@capacitor/local-notifications": "^8.0.0"`} {
		if !strings.Contains(contentByPath["package.json"], dependency) {
			t.Fatalf("expected package.json to contain %q", dependency)
		}
	}
	for _, path := range []string{"src/lib/mobile/camera.ts", "src/lib/mobile/localNotifications.ts"} {
		if contentByPath[path] == "" {
			t.Fatalf("expected capability wrapper %s", path)
		}
	}
	for _, capability := range []string{`"camera"`, `"local_notifications"`, `"identity_verification"`} {
		if !strings.Contains(contentByPath["mobile.capabilities.json"], capability) {
			t.Fatalf("expected capability manifest to contain %s", capability)
		}
	}
}

func TestBuildCapacitorAppID(t *testing.T) {
	if got := buildCapacitorAppID("123 CRM!", ""); got != "run.ucode.app123crm" {
		t.Fatalf("buildCapacitorAppID() = %q", got)
	}
	if got := buildCapacitorAppID(strings.Repeat("Project", 10), "project-12345678"); len(got) > 58 {
		t.Fatalf("buildCapacitorAppID() is too long: %q", got)
	}
}
