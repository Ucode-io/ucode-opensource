package models

import "strings"

type MobileCapability string

const (
	MobileCapabilityCamera               MobileCapability = "camera"
	MobileCapabilityLocalNotifications   MobileCapability = "local_notifications"
	MobileCapabilityPushNotifications    MobileCapability = "push_notifications"
	MobileCapabilityBiometricAuth        MobileCapability = "biometric_auth"
	MobileCapabilityIdentityVerification MobileCapability = "identity_verification"
)

// ApplyMobileCapabilityKeywordOverride keeps architect output canonical and
// deterministically restores native requirements explicitly present in the prompt.
func ApplyMobileCapabilityKeywordOverride(plan *ArchitectPlan, userPrompt string) {
	if plan == nil || plan.ProjectType != "mobile" {
		if plan != nil {
			plan.MobileCapabilities = nil
		}
		return
	}

	enabled := make(map[MobileCapability]bool)
	for _, capability := range plan.MobileCapabilities {
		enabled[capability] = true
	}

	prompt := strings.ToLower(userPrompt)
	addCapabilityOnSignal(enabled, prompt, MobileCapabilityCamera,
		"camera", "take photo", "take a photo", "profile photo", "selfie", "scan document", "upload photo")
	addCapabilityOnSignal(enabled, prompt, MobileCapabilityLocalNotifications,
		"local notification", "local notifications", "reminder", "reminders", "remind me")
	addCapabilityOnSignal(enabled, prompt, MobileCapabilityPushNotifications,
		"push notification", "push notifications", "fcm", "apns", "notify users")
	addCapabilityOnSignal(enabled, prompt, MobileCapabilityBiometricAuth,
		"face id", "touch id", "fingerprint", "biometric", "biometrics")
	addCapabilityOnSignal(enabled, prompt, MobileCapabilityIdentityVerification,
		"identity verification", "id verification", "verify identity", "kyc", "liveness", "passport verification",
		"document verification", "face verification", "face identification", "facial recognition")
	if strings.Contains(prompt, "notification") &&
		!enabled[MobileCapabilityLocalNotifications] &&
		!enabled[MobileCapabilityPushNotifications] {
		enabled[MobileCapabilityPushNotifications] = true
	}

	if enabled[MobileCapabilityIdentityVerification] {
		enabled[MobileCapabilityCamera] = true
	}

	plan.MobileCapabilities = canonicalMobileCapabilities(enabled)
}

func CanonicalMobileCapabilities(capabilities []MobileCapability) []MobileCapability {
	enabled := make(map[MobileCapability]bool, len(capabilities))
	for _, capability := range capabilities {
		enabled[capability] = true
	}
	return canonicalMobileCapabilities(enabled)
}

func addCapabilityOnSignal(enabled map[MobileCapability]bool, prompt string, capability MobileCapability, signals ...string) {
	for _, signal := range signals {
		if strings.Contains(prompt, signal) {
			enabled[capability] = true
			return
		}
	}
}

func canonicalMobileCapabilities(enabled map[MobileCapability]bool) []MobileCapability {
	if enabled[MobileCapabilityIdentityVerification] {
		enabled[MobileCapabilityCamera] = true
	}
	capabilities := make([]MobileCapability, 0, len(enabled))
	for _, capability := range []MobileCapability{
		MobileCapabilityCamera,
		MobileCapabilityLocalNotifications,
		MobileCapabilityPushNotifications,
		MobileCapabilityBiometricAuth,
		MobileCapabilityIdentityVerification,
	} {
		if enabled[capability] {
			capabilities = append(capabilities, capability)
		}
	}
	return capabilities
}
