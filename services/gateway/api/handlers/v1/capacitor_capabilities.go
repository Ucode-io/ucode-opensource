package v1

import (
	"encoding/json"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const capacitorCameraBridge = `import { Camera, CameraResultType, CameraSource } from '@capacitor/camera';
import { isNativePlatform } from '@/lib/capacitor';

export async function takePhoto(): Promise<string | undefined> {
  if (!isNativePlatform) return pickBrowserImage();
  const photo = await Camera.getPhoto({
    quality: 85,
    resultType: CameraResultType.Uri,
    source: CameraSource.Camera,
  });
  return photo.webPath ?? photo.path;
}

export async function choosePhoto(): Promise<string | undefined> {
  if (!isNativePlatform) return pickBrowserImage();
  const photo = await Camera.getPhoto({
    quality: 85,
    resultType: CameraResultType.Uri,
    source: CameraSource.Photos,
  });
  return photo.webPath ?? photo.path;
}

function pickBrowserImage(): Promise<string | undefined> {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.onchange = () => {
      const file = input.files?.[0];
      resolve(file ? URL.createObjectURL(file) : undefined);
    };
    input.click();
  });
}
`

const capacitorLocalNotificationsBridge = `import { LocalNotifications } from '@capacitor/local-notifications';
import { isNativePlatform } from '@/lib/capacitor';

export async function requestNotificationPermission(): Promise<boolean> {
  if (isNativePlatform) {
    const permission = await LocalNotifications.requestPermissions();
    return permission.display === 'granted';
  }
  if (!('Notification' in window)) return false;
  return (await Notification.requestPermission()) === 'granted';
}

export async function scheduleLocalNotification(title: string, body: string, at: Date): Promise<boolean> {
  if (isNativePlatform) {
    const permission = await LocalNotifications.checkPermissions();
    if (permission.display !== 'granted' && !(await requestNotificationPermission())) return false;
    await LocalNotifications.schedule({
      notifications: [{ id: Math.floor(Math.random() * 2147483647), title, body, schedule: { at } }],
    });
    return true;
  }
  if (!('Notification' in window) || Notification.permission !== 'granted') return false;
  window.setTimeout(() => new Notification(title, { body }), Math.max(0, at.getTime() - Date.now()));
  return true;
}
`

const capacitorBiometricBridge = `import { BiometricAuth } from '@aparajita/capacitor-biometric-auth';
import { isNativePlatform } from '@/lib/capacitor';

// Whether THIS device can use Face ID / Touch ID / fingerprint. Gate biometric UI on this.
export async function isBiometricAvailable(): Promise<boolean> {
  if (!isNativePlatform) return false;
  try {
    const info = await BiometricAuth.checkBiometry();
    return info.isAvailable;
  } catch {
    return false;
  }
}

// Prompts the REAL native Face ID / Touch ID / fingerprint. Resolves true on success.
export async function authenticateBiometric(reason = 'Authenticate to continue'): Promise<boolean> {
  if (!isNativePlatform) {
    // Biometric runs only on a real device; confirm in the browser preview so the flow continues.
    return window.confirm(reason + '\n\n(Biometric unlock runs on a real device.)');
  }
  try {
    await BiometricAuth.authenticate({ reason, cancelTitle: 'Cancel', allowDeviceCredential: true });
    return true;
  } catch {
    return false;
  }
}
`

type capacitorCapabilityManifest struct {
	Capabilities        []models.MobileCapability `json:"capabilities"`
	RuntimeReady        []models.MobileCapability `json:"runtime_ready"`
	BuildWorkerRequired []models.MobileCapability `json:"build_worker_required"`
}

func applyCapacitorCapabilities(files []models.ProjectFile, dependencies map[string]any, capabilities []models.MobileCapability) []models.ProjectFile {
	manifest := newCapacitorCapabilityManifest(capabilities)
	for _, capability := range manifest.RuntimeReady {
		switch capability {
		case models.MobileCapabilityCamera:
			dependencies["@capacitor/camera"] = capacitorPackageVersion
			files = upsertProjectFile(files, models.ProjectFile{Path: "src/lib/mobile/camera.ts", Content: capacitorCameraBridge})
		case models.MobileCapabilityLocalNotifications:
			dependencies["@capacitor/local-notifications"] = capacitorPackageVersion
			files = upsertProjectFile(files, models.ProjectFile{Path: "src/lib/mobile/localNotifications.ts", Content: capacitorLocalNotificationsBridge})
		case models.MobileCapabilityBiometricAuth:
			dependencies[capacitorBiometricPackage] = capacitorBiometricPackageVersion
			files = upsertProjectFile(files, models.ProjectFile{Path: "src/lib/mobile/biometric.ts", Content: capacitorBiometricBridge})
		}
	}

	content, _ := json.MarshalIndent(manifest, "", "  ")
	return upsertProjectFile(files, models.ProjectFile{Path: "mobile.capabilities.json", Content: string(content) + "\n"})
}

func newCapacitorCapabilityManifest(capabilities []models.MobileCapability) capacitorCapabilityManifest {
	manifest := capacitorCapabilityManifest{
		Capabilities:        models.CanonicalMobileCapabilities(capabilities),
		RuntimeReady:        make([]models.MobileCapability, 0),
		BuildWorkerRequired: make([]models.MobileCapability, 0),
	}
	for _, capability := range manifest.Capabilities {
		manifest.BuildWorkerRequired = append(manifest.BuildWorkerRequired, capability)
		switch capability {
		case models.MobileCapabilityCamera, models.MobileCapabilityLocalNotifications, models.MobileCapabilityBiometricAuth:
			manifest.RuntimeReady = append(manifest.RuntimeReady, capability)
		}
	}
	return manifest
}
