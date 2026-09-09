package models

import "testing"

func TestApplyProjectTypeKeywordOverride(t *testing.T) {
	tests := []struct {
		name        string
		projectType string
		prompt      string
		want        string
	}{
		{
			name:        "selected mobile app",
			projectType: "webapp",
			prompt:      "Selected platform: mobile-app",
			want:        "mobile",
		},
		{
			name:        "explicit Android mobile app",
			projectType: "webapp",
			prompt:      "Build an Android app for booking appointments",
			want:        "mobile",
		},
		{
			name:        "explicit Capacitor app",
			projectType: "web",
			prompt:      "Build an installable app with Capacitor",
			want:        "mobile",
		},
		{
			name:        "selected web app",
			projectType: "mobile",
			prompt:      "Selected platform: web-app",
			want:        "webapp",
		},
		{
			name:        "selected web app wins over original bare mobile phrase",
			projectType: "mobile",
			prompt:      "Build me a mobile app\nSelected platform: web-app",
			want:        "webapp",
		},
		{
			name:        "bare mobile app falls back to mobile",
			projectType: "web",
			prompt:      "Build me a mobile app",
			want:        "mobile",
		},
		{
			name:        "bare mobile app for domain falls back to mobile",
			projectType: "landing",
			prompt:      "Build me a mobile app for food delivery",
			want:        "mobile",
		},
		{
			name:        "marketing page for native app stays landing",
			projectType: "landing",
			prompt:      "Build a landing page for our Android app",
			want:        "landing",
		},
		{
			name:        "admin web app stays admin",
			projectType: "admin_panel",
			prompt:      "Build a web app admin panel to manage inventory",
			want:        "admin_panel",
		},
		{
			name:        "explicit landing page corrects web guess",
			projectType: "web",
			prompt:      "Make landing page according to this thesecrettrading.de website. Make 1 to 1 logic and design.",
			want:        "landing",
		},
		{
			name:        "russian landing request corrects web guess",
			projectType: "web",
			prompt:      "Сделай лендинг 1 в 1 как на сайте example.com",
			want:        "landing",
		},
		{
			name:        "landing signal does not downgrade webapp",
			projectType: "webapp",
			prompt:      "Build a landing page for our product",
			want:        "webapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &ArchitectPlan{ProjectType: tt.projectType}

			ApplyProjectTypeKeywordOverride(plan, tt.prompt)

			if plan.ProjectType != tt.want {
				t.Fatalf("ProjectType = %q, want %q", plan.ProjectType, tt.want)
			}
		})
	}
}
