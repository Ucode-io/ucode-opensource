package v1

import (
	"fmt"
	"strings"

	chat_prompts2 "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func capacitorPromptAddendum(capabilities []models.MobileCapability) string {
	var builder strings.Builder
	builder.WriteString(chat_prompts2.PromptCapacitorMobileAddendum)
	builder.WriteString("\nAPPROVED MOBILE CAPABILITIES:\n")
	if len(capabilities) == 0 {
		builder.WriteString("- None requested. Do not add native plugin behavior.\n")
		return builder.String()
	}

	for _, capability := range models.CanonicalMobileCapabilities(capabilities) {
		switch capability {
		case models.MobileCapabilityCamera:
			builder.WriteString("- camera: import takePhoto/choosePhoto from '@/lib/mobile/camera'.\n")
		case models.MobileCapabilityLocalNotifications:
			builder.WriteString("- local_notifications: import requestNotificationPermission/scheduleLocalNotification from '@/lib/mobile/localNotifications'.\n")
		case models.MobileCapabilityPushNotifications:
			builder.WriteString("- push_notifications: build-worker requirement only; create UI/preferences, but do not import or fake a push plugin.\n")
		case models.MobileCapabilityBiometricAuth:
			builder.WriteString("- biometric_auth: import isBiometricAvailable/authenticateBiometric from '@/lib/mobile/biometric'. Gate the biometric option on isBiometricAvailable() and call authenticateBiometric() on the unlock/login action — this triggers REAL Face ID / Touch ID / fingerprint on device (browser preview falls back to a confirm). NEVER render a fake or static biometric button, and never import the biometric plugin directly.\n")
		case models.MobileCapabilityIdentityVerification:
			builder.WriteString("- identity_verification: external backend/provider requirement; create capture/upload UI using the approved camera wrapper, but do not fake verification.\n")
		default:
			fmt.Fprintf(&builder, "- %s: unsupported; do not implement.\n", capability)
		}
	}
	return builder.String()
}

func usesWebAppGenerator(projectType string) bool {
	return projectType == "webapp" || projectType == mobileProjectType
}
