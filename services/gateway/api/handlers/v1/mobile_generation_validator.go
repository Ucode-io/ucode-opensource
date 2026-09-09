package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func validateMobileGeneratedProject(files []models.ProjectFile) []ValidationError {
	var validationErrors []ValidationError
	contentByPath := make(map[string]string, len(files))
	for _, file := range files {
		contentByPath[file.Path] = file.Content
	}

	for _, path := range []string{
		"package.json",
		"index.html",
		"capacitor.config.ts",
		"mobile.capabilities.json",
		"vite.config.ts",
		"src/App.tsx",
		"src/main.tsx",
		"src/lib/capacitor.ts",
	} {
		if _, exists := contentByPath[path]; !exists {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     path,
				Message:  "required Capacitor mobile project file is missing",
			})
		}
	}

	capabilities, manifestErrors := validateCapacitorCapabilityManifest(contentByPath["mobile.capabilities.json"])
	validationErrors = append(validationErrors, manifestErrors...)
	validationErrors = append(validationErrors, validateCapacitorPackageJSON(contentByPath["package.json"], capabilities)...)
	validationErrors = append(validationErrors, validateCapacitorIndexHTML(contentByPath["index.html"])...)
	validationErrors = append(validationErrors, validateCapacitorConfig(contentByPath["capacitor.config.ts"])...)
	validationErrors = append(validationErrors, validateCapacitorSource(files, capabilities)...)
	validationErrors = append(validationErrors, validateCapacitorCapabilityFiles(contentByPath, capabilities)...)
	if appContent := contentByPath["src/App.tsx"]; appContent != "" {
		switch {
		case strings.Contains(appContent, "BrowserRouter"):
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "src/App.tsx",
				Message:  "Capacitor mobile app must use HashRouter so bundled routes survive native WebView reloads",
			})
		case !strings.Contains(appContent, "HashRouter"):
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "src/App.tsx",
				Message:  "Capacitor mobile app is missing HashRouter",
			})
		}
	}
	return validationErrors
}

// mobileContractErrorCount returns the count of fatal (error-severity) Capacitor
// contract violations — missing entry files, BrowserRouter instead of HashRouter,
// React Native/Expo or unapproved Capacitor imports, server.url, etc. Mobile gates
// ONLY on these. UI-quality and manifest-completeness findings are best-effort, the
// same as the webapp this wraps, so a mobile app is exactly as shippable as a webapp.
func mobileContractErrorCount(files []models.ProjectFile) int {
	count := 0
	for _, e := range validateMobileGeneratedProject(files) {
		if e.Severity == "error" {
			count++
		}
	}
	return count
}

func validateCapacitorIndexHTML(content string) []ValidationError {
	if content == "" {
		return nil
	}

	var validationErrors []ValidationError
	for _, required := range []string{"viewport-fit=cover", `id="root"`, `src="/src/main.tsx"`} {
		if !strings.Contains(content, required) {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "index.html",
				Message:  fmt.Sprintf("Capacitor index.html must contain %q", required),
			})
		}
	}
	return validationErrors
}

func validateCapacitorPackageJSON(content string, capabilities []models.MobileCapability) []ValidationError {
	if content == "" {
		return nil
	}

	var packageJSON struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Engines         map[string]string `json:"engines"`
	}
	if err := json.Unmarshal([]byte(content), &packageJSON); err != nil {
		return []ValidationError{{Severity: "error", File: "package.json", Message: "invalid JSON: " + err.Error()}}
	}

	var validationErrors []ValidationError
	for _, dependency := range []string{
		"@capacitor/app",
		"@capacitor/core",
		"@capacitor/haptics",
		"@capacitor/keyboard",
		"@capacitor/status-bar",
	} {
		if packageJSON.Dependencies[dependency] == "" {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "package.json",
				Message:  fmt.Sprintf("missing Capacitor dependency %q", dependency),
			})
		}
	}
	for _, capability := range capabilities {
		dependency := capacitorCapabilityPackage(capability)
		if dependency != "" && packageJSON.Dependencies[dependency] == "" {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "package.json",
				Message:  fmt.Sprintf("missing Capacitor capability dependency %q", dependency),
			})
		}
	}
	for _, dependency := range []string{"@capacitor/android", "@capacitor/cli", "@capacitor/ios"} {
		if packageJSON.DevDependencies[dependency] == "" {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "package.json",
				Message:  fmt.Sprintf("missing Capacitor development dependency %q", dependency),
			})
		}
	}
	for section, dependencies := range map[string]map[string]string{
		"dependencies":    packageJSON.Dependencies,
		"devDependencies": packageJSON.DevDependencies,
	} {
		for dependency := range dependencies {
			if strings.HasPrefix(dependency, "@capacitor/") && !isApprovedCapacitorPackage(dependency, capabilities) {
				validationErrors = append(validationErrors, ValidationError{
					Severity: "error",
					File:     "package.json",
					Message:  fmt.Sprintf("%s contains unapproved Capacitor package %q", section, dependency),
				})
			}
		}
	}
	for _, script := range []string{"build", "cap:sync", "cap:add:android", "cap:add:ios"} {
		if packageJSON.Scripts[script] == "" {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "package.json",
				Message:  fmt.Sprintf("missing Capacitor build script %q", script),
			})
		}
	}
	if packageJSON.Engines["node"] != capacitorNodeVersion {
		validationErrors = append(validationErrors, ValidationError{
			Severity: "error",
			File:     "package.json",
			Message:  fmt.Sprintf("Capacitor %s requires engines.node %q", capacitorRuntimeVersion, capacitorNodeVersion),
		})
	}
	return validationErrors
}

func validateCapacitorConfig(content string) []ValidationError {
	if content == "" {
		return nil
	}

	var validationErrors []ValidationError
	for _, required := range []string{"appId:", "appName:", "webDir:", fmt.Sprintf("%q", capacitorWebDir)} {
		if !strings.Contains(content, required) {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     "capacitor.config.ts",
				Message:  fmt.Sprintf("Capacitor config must contain %s", required),
			})
		}
	}
	if regexp.MustCompile(`(?m)\burl\s*:`).MatchString(content) {
		validationErrors = append(validationErrors, ValidationError{
			Severity: "error",
			File:     "capacitor.config.ts",
			Message:  "server.url is forbidden for production mobile projects; bundle local web assets",
		})
	}
	return validationErrors
}

func validateCapacitorSource(files []models.ProjectFile, capabilities []models.MobileCapability) []ValidationError {
	var validationErrors []ValidationError
	capacitorImport := regexp.MustCompile(`@capacitor/[a-z0-9-]+`)
	for _, file := range files {
		if strings.HasPrefix(file.Path, "ios/") || strings.HasPrefix(file.Path, "android/") {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     file.Path,
				Message:  "native platform directories must be created by the Capacitor build worker, not generated by AI",
			})
		}
		if !strings.HasSuffix(file.Path, ".ts") && !strings.HasSuffix(file.Path, ".tsx") && file.Path != "package.json" {
			continue
		}
		for _, forbidden := range []string{"react-native", "expo-constants", "expo-status-bar", "@expo/", "from 'expo'", `from "expo"`} {
			if strings.Contains(file.Content, forbidden) {
				validationErrors = append(validationErrors, ValidationError{
					Severity: "error",
					File:     file.Path,
					Message:  fmt.Sprintf("contains unsupported Expo/React Native dependency %q; mobile uses React/Vite + Capacitor", forbidden),
				})
			}
		}
		if file.Path == "package.json" {
			continue
		}
		for _, dependency := range capacitorImport.FindAllString(file.Content, -1) {
			if !isApprovedCapacitorSourceImport(file.Path, dependency, capabilities) {
				validationErrors = append(validationErrors, ValidationError{
					Severity: "error",
					File:     file.Path,
					Message:  fmt.Sprintf("contains unapproved Capacitor import %q", dependency),
				})
			}
		}
	}
	return validationErrors
}

func isApprovedCapacitorPackage(packageName string, capabilities []models.MobileCapability) bool {
	switch packageName {
	case "@capacitor/android",
		"@capacitor/app",
		"@capacitor/cli",
		"@capacitor/core",
		"@capacitor/haptics",
		"@capacitor/ios",
		"@capacitor/keyboard",
		"@capacitor/status-bar":
		return true
	}
	for _, capability := range capabilities {
		if capacitorCapabilityPackage(capability) == packageName {
			return true
		}
	}
	return false
}

func isApprovedCapacitorSourceImport(path, packageName string, capabilities []models.MobileCapability) bool {
	if !isApprovedCapacitorPackage(packageName, capabilities) {
		return false
	}
	switch packageName {
	case "@capacitor/camera":
		return path == "src/lib/mobile/camera.ts"
	case "@capacitor/local-notifications":
		return path == "src/lib/mobile/localNotifications.ts"
	default:
		return true
	}
}

func validateCapacitorCapabilityManifest(content string) ([]models.MobileCapability, []ValidationError) {
	if content == "" {
		return nil, nil
	}

	var manifest capacitorCapabilityManifest
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return nil, []ValidationError{{
			Severity: "error",
			File:     "mobile.capabilities.json",
			Message:  "invalid JSON: " + err.Error(),
		}}
	}

	canonical := models.CanonicalMobileCapabilities(manifest.Capabilities)
	if len(canonical) != len(manifest.Capabilities) {
		return canonical, []ValidationError{{
			Severity: "error",
			File:     "mobile.capabilities.json",
			Message:  "contains duplicate or unsupported mobile capabilities",
		}}
	}
	expected := newCapacitorCapabilityManifest(canonical)
	if !equalMobileCapabilities(manifest.RuntimeReady, expected.RuntimeReady) ||
		!equalMobileCapabilities(manifest.BuildWorkerRequired, expected.BuildWorkerRequired) {
		return canonical, []ValidationError{{
			Severity: "error",
			File:     "mobile.capabilities.json",
			Message:  "runtime_ready or build_worker_required does not match declared capabilities",
		}}
	}
	return canonical, nil
}

func validateCapacitorCapabilityFiles(contentByPath map[string]string, capabilities []models.MobileCapability) []ValidationError {
	var validationErrors []ValidationError
	for _, capability := range capabilities {
		path := capacitorCapabilityWrapper(capability)
		if path != "" && contentByPath[path] == "" {
			validationErrors = append(validationErrors, ValidationError{
				Severity: "error",
				File:     path,
				Message:  fmt.Sprintf("required wrapper for mobile capability %q is missing", capability),
			})
		}
	}
	return validationErrors
}

func capacitorCapabilityPackage(capability models.MobileCapability) string {
	switch capability {
	case models.MobileCapabilityCamera:
		return "@capacitor/camera"
	case models.MobileCapabilityLocalNotifications:
		return "@capacitor/local-notifications"
	case models.MobileCapabilityBiometricAuth:
		return capacitorBiometricPackage
	default:
		return ""
	}
}

func capacitorCapabilityWrapper(capability models.MobileCapability) string {
	switch capability {
	case models.MobileCapabilityCamera:
		return "src/lib/mobile/camera.ts"
	case models.MobileCapabilityLocalNotifications:
		return "src/lib/mobile/localNotifications.ts"
	case models.MobileCapabilityBiometricAuth:
		return "src/lib/mobile/biometric.ts"
	default:
		return ""
	}
}

func equalMobileCapabilities(left, right []models.MobileCapability) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
