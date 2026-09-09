package models

import (
	"reflect"
	"testing"
)

func TestApplyMobileCapabilityKeywordOverride(t *testing.T) {
	plan := &ArchitectPlan{
		ProjectType:        "mobile",
		MobileCapabilities: []MobileCapability{MobileCapabilityPushNotifications, MobileCapability("unsupported")},
	}

	ApplyMobileCapabilityKeywordOverride(plan, "Use Face ID and KYC passport verification with reminders")

	expected := []MobileCapability{
		MobileCapabilityCamera,
		MobileCapabilityLocalNotifications,
		MobileCapabilityPushNotifications,
		MobileCapabilityBiometricAuth,
		MobileCapabilityIdentityVerification,
	}
	if !reflect.DeepEqual(plan.MobileCapabilities, expected) {
		t.Fatalf("unexpected capabilities: got %v want %v", plan.MobileCapabilities, expected)
	}
}

func TestApplyMobileCapabilityKeywordOverrideClearsNonMobileCapabilities(t *testing.T) {
	plan := &ArchitectPlan{
		ProjectType:        "webapp",
		MobileCapabilities: []MobileCapability{MobileCapabilityCamera},
	}

	ApplyMobileCapabilityKeywordOverride(plan, "web app with camera")

	if plan.MobileCapabilities != nil {
		t.Fatalf("expected non-mobile capabilities to be cleared, got %v", plan.MobileCapabilities)
	}
}

func TestApplyMobileCapabilityKeywordOverrideHandlesGenericNativeRequest(t *testing.T) {
	plan := &ArchitectPlan{ProjectType: "mobile"}

	ApplyMobileCapabilityKeywordOverride(plan, "Use camera, real face identification, fingerprint, and notifications")

	expected := []MobileCapability{
		MobileCapabilityCamera,
		MobileCapabilityPushNotifications,
		MobileCapabilityBiometricAuth,
		MobileCapabilityIdentityVerification,
	}
	if !reflect.DeepEqual(plan.MobileCapabilities, expected) {
		t.Fatalf("unexpected capabilities: got %v want %v", plan.MobileCapabilities, expected)
	}
}
