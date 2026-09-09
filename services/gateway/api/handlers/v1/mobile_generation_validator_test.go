package v1

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestValidateMobileGeneratedProjectAcceptsCapacitorProject(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", nil)
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}

	if validationErrors := validateMobileGeneratedProject(files); len(validationErrors) != 0 {
		t.Fatalf("expected valid Capacitor project, got errors: %+v", validationErrors)
	}
}

func TestValidateMobileGeneratedProjectRejectsMissingCapacitorConfig(t *testing.T) {
	validationErrors := validateMobileGeneratedProject(append([]models.ProjectFile(nil), GetTemplateScaffold()...))

	for _, validationError := range validationErrors {
		if validationError.File == "capacitor.config.ts" && strings.Contains(validationError.Message, "missing") {
			return
		}
	}
	t.Fatalf("expected missing capacitor.config.ts error, got: %+v", validationErrors)
}

func TestValidateMobileGeneratedProjectRequiresHashRouter(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", nil)
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}
	appIndex := projectFileIndex(files, "src/App.tsx")
	files[appIndex].Content = strings.ReplaceAll(files[appIndex].Content, "HashRouter", "MemoryRouter")

	validationErrors := validateMobileGeneratedProject(files)
	for _, validationError := range validationErrors {
		if validationError.File == "src/App.tsx" && strings.Contains(validationError.Message, "missing HashRouter") {
			return
		}
	}
	t.Fatalf("expected missing HashRouter error, got: %+v", validationErrors)
}

func TestValidateMobileGeneratedProjectRejectsExpoAndRemoteServer(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", nil)
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "src/pages/HomePage.tsx",
		Content: `import Constants from 'expo-constants'; export function HomePage() { return null; }`,
	})
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "capacitor.config.ts",
		Content: `const config = { appId: 'run.ucode.app', appName: 'App', webDir: 'build', server: { url: 'https://example.com' } }; export default config;`,
	})

	validationErrors := validateMobileGeneratedProject(files)
	messages := make([]string, 0, len(validationErrors))
	for _, validationError := range validationErrors {
		messages = append(messages, validationError.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{"server.url is forbidden", "unsupported Expo/React Native dependency"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q error, got: %s", expected, joined)
		}
	}
}

func TestValidateMobileGeneratedProjectRejectsUnapprovedCapacitorPlugin(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", nil)
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "src/pages/HomePage.tsx",
		Content: `import { Camera } from '@capacitor/camera'; export function HomePage() { return null; }`,
	})

	validationErrors := validateMobileGeneratedProject(files)
	for _, validationError := range validationErrors {
		if strings.Contains(validationError.Message, `unapproved Capacitor import "@capacitor/camera"`) {
			return
		}
	}
	t.Fatalf("expected unapproved Capacitor plugin error, got: %+v", validationErrors)
}

func TestValidateMobileGeneratedProjectAcceptsDeclaredCameraWrapper(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", []models.MobileCapability{models.MobileCapabilityCamera})
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}

	if validationErrors := validateMobileGeneratedProject(files); len(validationErrors) != 0 {
		t.Fatalf("expected valid camera capability project, got errors: %+v", validationErrors)
	}
}

func TestValidateMobileGeneratedProjectRejectsDirectCameraImport(t *testing.T) {
	files := append([]models.ProjectFile(nil), GetTemplateScaffold()...)
	files, err := applyCapacitorScaffold(files, "Pocket CRM", "project-12345678", []models.MobileCapability{models.MobileCapabilityCamera})
	if err != nil {
		t.Fatalf("apply Capacitor scaffold: %v", err)
	}
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "src/pages/HomePage.tsx",
		Content: `import { Camera } from '@capacitor/camera'; export function HomePage() { return null; }`,
	})

	validationErrors := validateMobileGeneratedProject(files)
	for _, validationError := range validationErrors {
		if strings.Contains(validationError.Message, `unapproved Capacitor import "@capacitor/camera"`) {
			return
		}
	}
	t.Fatalf("expected direct camera import error, got: %+v", validationErrors)
}
