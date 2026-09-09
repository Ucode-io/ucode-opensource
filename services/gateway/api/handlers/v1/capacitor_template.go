package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// Capacitor wraps the generated React/Vite webapp in native iOS and Android
// containers. The LLM generates only web source; a trusted build worker creates
// ios/ and android/ with `npx cap add` and refreshes them with `npx cap sync`.

const capacitorBridge = `import { App } from '@capacitor/app';
import { Capacitor } from '@capacitor/core';
import { Haptics, ImpactStyle } from '@capacitor/haptics';
import { Keyboard } from '@capacitor/keyboard';
import { StatusBar, Style } from '@capacitor/status-bar';

export const isNativePlatform = Capacitor.isNativePlatform();

export async function initializeNativeShell() {
  if (!isNativePlatform) return;
  await StatusBar.setStyle({ style: Style.Dark });
  if (Capacitor.getPlatform() === 'ios') {
    await Keyboard.setAccessoryBarVisible({ isVisible: true });
  }
}

export async function hapticTap() {
  if (!isNativePlatform) return;
  await Haptics.impact({ style: ImpactStyle.Light });
}

export function listenForAndroidBackButton(handler: () => void) {
  if (!isNativePlatform) return;
  return App.addListener('backButton', handler);
}
`

func applyCapacitorScaffold(files []models.ProjectFile, projectName, projectID string, capabilities []models.MobileCapability) ([]models.ProjectFile, error) {
	packageIndex := projectFileIndex(files, "package.json")
	if packageIndex == -1 {
		return nil, fmt.Errorf("capacitor scaffold: package.json is missing")
	}

	var packageJSON map[string]any
	if err := json.Unmarshal([]byte(files[packageIndex].Content), &packageJSON); err != nil {
		return nil, fmt.Errorf("capacitor scaffold: decode package.json: %w", err)
	}

	scripts := packageJSONSection(packageJSON, "scripts")
	scripts["cap:sync"] = "npm run build && npx cap sync"
	scripts["cap:add:android"] = "npx cap add android"
	scripts["cap:add:ios"] = "npx cap add ios"
	scripts["cap:open:android"] = "npx cap open android"
	scripts["cap:open:ios"] = "npx cap open ios"

	dependencies := packageJSONSection(packageJSON, "dependencies")
	for _, name := range []string{
		"@capacitor/app",
		"@capacitor/core",
		"@capacitor/haptics",
		"@capacitor/keyboard",
		"@capacitor/status-bar",
	} {
		dependencies[name] = capacitorPackageVersion
	}
	files = applyCapacitorCapabilities(files, dependencies, capabilities)

	devDependencies := packageJSONSection(packageJSON, "devDependencies")
	for _, name := range []string{"@capacitor/android", "@capacitor/cli", "@capacitor/ios"} {
		devDependencies[name] = capacitorPackageVersion
	}
	engines := packageJSONSection(packageJSON, "engines")
	engines["node"] = capacitorNodeVersion

	var content bytes.Buffer
	encoder := json.NewEncoder(&content)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(packageJSON); err != nil {
		return nil, fmt.Errorf("capacitor scaffold: encode package.json: %w", err)
	}
	files[packageIndex].Content = content.String()

	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "capacitor.config.ts",
		Content: buildCapacitorConfig(projectName, projectID),
	})
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "index.html",
		Content: buildCapacitorIndexHTML(projectName),
	})
	files = upsertProjectFile(files, models.ProjectFile{
		Path:    "src/lib/capacitor.ts",
		Content: capacitorBridge,
	})
	files = applyCapacitorHashRouter(files)
	return files, nil
}

func buildCapacitorConfig(projectName, projectID string) string {
	return fmt.Sprintf(`import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
  appId: %q,
  appName: %q,
  webDir: %q,
  server: {
    androidScheme: 'https',
  },
};

export default config;
`, buildCapacitorAppID(projectName, projectID), projectName, capacitorWebDir)
}

func buildCapacitorIndexHTML(projectName string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
    <meta name="theme-color" content="#ffffff" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`, html.EscapeString(projectName))
}

func buildCapacitorAppID(projectName, projectID string) string {
	appSegment := asciiAlphaNumeric(projectName)
	if len(appSegment) > 40 {
		appSegment = appSegment[:40]
	}
	projectSuffix := asciiAlphaNumeric(projectID)
	if len(projectSuffix) > 8 {
		projectSuffix = projectSuffix[len(projectSuffix)-8:]
	}
	appSegment += projectSuffix
	if appSegment == "" || appSegment[0] < 'a' || appSegment[0] > 'z' {
		appSegment = "app" + appSegment
	}
	return "run.ucode." + appSegment
}

func asciiAlphaNumeric(value string) string {
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			result.WriteRune(char)
		}
	}
	return result.String()
}

func packageJSONSection(packageJSON map[string]any, key string) map[string]any {
	if section, ok := packageJSON[key].(map[string]any); ok {
		return section
	}
	section := make(map[string]any)
	packageJSON[key] = section
	return section
}

func projectFileIndex(files []models.ProjectFile, path string) int {
	for index, file := range files {
		if file.Path == path {
			return index
		}
	}
	return -1
}

func upsertProjectFile(files []models.ProjectFile, file models.ProjectFile) []models.ProjectFile {
	if index := projectFileIndex(files, file.Path); index >= 0 {
		files[index] = file
		return files
	}
	return append(files, file)
}

func applyCapacitorHashRouter(files []models.ProjectFile) []models.ProjectFile {
	appIndex := projectFileIndex(files, "src/App.tsx")
	if appIndex == -1 {
		return files
	}
	replacer := strings.NewReplacer(
		"createBrowserRouter", "createHashRouter",
		"BrowserRouter", "HashRouter",
		"MemoryRouter", "HashRouter",
	)
	files[appIndex].Content = replacer.Replace(files[appIndex].Content)
	return files
}
