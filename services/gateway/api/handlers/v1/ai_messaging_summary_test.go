package v1

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestBuildProjectSummaryForMobile(t *testing.T) {
	summary := buildProjectSummary(&models.ArchitectPlan{
		ProjectName: "Pocket CRM",
		ProjectType: mobileProjectType,
	}, []models.ProjectFile{
		{Path: "src/App.tsx"},
		{Path: "src/pages/HomePage.tsx"},
		{Path: "src/pages/DealsPage.tsx"},
	}, 12)

	for _, expected := range []string{
		"устанавливаемое мобильное приложение готово",
		"2 экранов",
		"Home · Deals",
		"Vite · Capacitor " + capacitorRuntimeVersion,
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("expected summary to contain %q, got:\n%s", expected, summary)
		}
	}
	if strings.Contains(summary, "Expo") || strings.Contains(summary, "React Native") {
		t.Fatalf("Capacitor mobile summary must not mention Expo/React Native:\n%s", summary)
	}
}
