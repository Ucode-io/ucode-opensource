package models

import (
	"regexp"
	"strings"
)

// appKeywordRe matches explicit responsive-web app wording.
// This is the STRONG signal — strong enough to override even an admin_panel guess.
var appKeywordRe = regexp.MustCompile(`(?i)\b(web[\s-]?app|webapp|web application|responsive web app|pwa)\b`)

// appForRe matches product phrasing like "app for tracking ..." / "app to manage ...".
// This is a WEAK, ambiguous signal: "an app to manage orders" is almost always an admin
// tool, not a consumer app. So appForRe only upgrades web/landing → webapp and is
// NEVER allowed to downgrade an admin_panel into a sidebar-less mobile app.
var appForRe = regexp.MustCompile(`(?i)\bapp\s+(for|to)\b`)

// marketingSignals indicate the user explicitly wants a marketing site / promo page,
// in which case we must NOT override to webapp even if the word "app" appears
// (e.g. "build a landing page for my app").
var marketingSignals = []string{
	"landing page", "landing-page",
	"promo page", "promotional page", "promo site",
	"marketing site", "marketing website", "website for",
	"one-page", "one page site", "single page site", "single-page site",
	"coming soon",
	"лендинг", "одностраничн",
}

// landingSignals are the subset of marketing wording that explicitly names a
// SINGLE-PAGE landing. When present, an architect "web" guess is corrected to
// "landing": the architect prompt biases toward "web" whenever a real
// multi-page site is referenced ("landing page like <site>.com"), but an
// explicit landing request must win.
var landingSignals = []string{
	"landing page", "landing-page",
	"one-page", "one page site", "single page site", "single-page site",
	"лендинг", "одностраничн",
}

// adminSignals indicate the user EXPLICITLY wants an internal back-office / admin tool.
// When present we keep "admin_panel" even if app-wording also appears
// (e.g. "an app for our staff to manage orders — an admin panel").
var adminSignals = []string{
	"admin panel", "admin-panel", "admin console", "admin dashboard",
	"back office", "back-office", "backoffice",
	"management system", "management dashboard", "management panel",
	"control panel", "internal tool", "internal dashboard",
	"staff portal", "for staff", "for our staff", "for administrators",
	"erp", "cms",
}

// managementIntentRe catches business-data MANAGEMENT framing ("manage orders",
// "inventory", "warehouse", "back office", CRUD) that means an admin_panel even when the
// user also says "app". A management tool rendered as a sidebar-less mobile webapp is the
// worst regression, so this guard protects an admin_panel classification from being
// downgraded. ("manag\w*" covers manage/managing/management/manager.)
var managementIntentRe = regexp.MustCompile(`(?i)\b(manag\w*|inventory|warehouse|back[\s-]?office|crud)\b`)

// ApplyProjectTypeKeywordOverride deterministically corrects the architect's
// project-type classification when the user explicitly selected an installable
// mobile app or named a responsive web app.
//
// Behavior:
//   - Backs off entirely on explicit marketing-site signals
//     (so "build a landing page for my app" stays a landing page).
//   - Explicit installable/mobile-store signals always map to mobile.
//   - "mobile app" / "mobile application" maps to mobile (an explicit installable-mobile signal;
//     the router treats it the same and does not ask the platform-types question).
//   - web / landing → webapp on non-ambiguous app wording (explicit "web app" OR weak "app for/to").
//   - admin_panel → webapp ONLY on EXPLICIT web-app wording,
//     never on the weak "app for/to" phrasing, and only when there is NO admin or
//     management framing. This keeps real admin/management tools as admin panels
//     (with their sidebar) instead of turning them into sidebar-less mobile apps.
func ApplyProjectTypeKeywordOverride(plan *ArchitectPlan, userPrompt string) {
	if plan == nil {
		return
	}

	lower := strings.ToLower(userPrompt)

	// Respect explicit marketing/promo intent — do not override these to
	// webapp/mobile. An explicit single-page landing request additionally
	// corrects an architect "web" guess back to "landing".
	for _, s := range marketingSignals {
		if strings.Contains(lower, s) {
			if plan.ProjectType == "web" {
				for _, ls := range landingSignals {
					if strings.Contains(lower, ls) {
						plan.ProjectType = "landing"
						break
					}
				}
			}
			return
		}
	}

	if strings.Contains(lower, "web-app") {
		plan.ProjectType = "webapp"
		return
	}
	if hasInstallableMobileSignal(lower) {
		plan.ProjectType = "mobile"
		return
	}
	if strings.Contains(lower, "mobile app") || strings.Contains(lower, "mobile application") {
		plan.ProjectType = "mobile"
		return
	}
	if plan.ProjectType == "webapp" {
		return
	}

	hasExplicitApp := appKeywordRe.MatchString(lower)
	hasAppForTo := appForRe.MatchString(lower)
	if !hasExplicitApp && !hasAppForTo {
		return
	}

	switch plan.ProjectType {
	case "web", "landing":
		// Core rescue: a consumer app misclassified as a website. Any app-wording upgrades it.
		plan.ProjectType = "webapp"
	case "admin_panel":
		// Conservative: the architect detected internal/management intent. Only override on
		// EXPLICIT web-app wording, never the weak "app for/to", and only when there is
		// no admin/management framing at all.
		if !hasExplicitApp {
			return
		}
		for _, s := range adminSignals {
			if strings.Contains(lower, s) {
				return // user really wants an admin panel — keep it (with its sidebar)
			}
		}
		if managementIntentRe.MatchString(lower) {
			return // "manage X / inventory / back office" → admin panel, not a mobile app
		}
		plan.ProjectType = "webapp"
	case "mobile":
		if hasExplicitApp {
			plan.ProjectType = "webapp"
		}
	}
}

func hasInstallableMobileSignal(prompt string) bool {
	signals := []string{
		"mobile-app",
		"ios app",
		"ios application",
		"android app",
		"android application",
		"installable app",
		"native app",
		"capacitor",
		"app store",
		"play store",
		"play market",
		// Legacy explicit runtime requests still select the mobile product surface;
		// generation itself is standardized on Capacitor.
		"react native",
		"expo app",
		"expo go",
	}
	for _, signal := range signals {
		if strings.Contains(prompt, signal) {
			return true
		}
	}
	return false
}
