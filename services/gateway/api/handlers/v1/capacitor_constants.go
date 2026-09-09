package v1

const (
	mobileProjectType       = "mobile"
	mobileRuntime           = "capacitor"
	capacitorRuntimeVersion = "8"
	capacitorPackageVersion = "^8.0.0"
	capacitorNodeVersion    = ">=22.0.0"
	capacitorWebDir         = "build"

	// Biometric (Face ID / Touch ID / fingerprint) is a THIRD-PARTY Capacitor plugin
	// with its own version line — NOT pinned to Capacitor core. VERIFY this version
	// resolves for your Capacitor 8 build worker; it is the one value not checkable here.
	capacitorBiometricPackage        = "@aparajita/capacitor-biometric-auth"
	capacitorBiometricPackageVersion = "^9.0.0"
)
