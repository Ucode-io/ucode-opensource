package config

import (
	"errors"
	"time"

	"github.com/ucode-io/ratelimiter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Default Configs
	DatabaseQueryTimeLayout   string        = `'YYYY-MM-DD"T"HH24:MI:SS"."MS"Z"TZ'`
	DatabaseTimeLayout        string        = time.RFC3339
	AccessTokenExpiresInTime  time.Duration = 1 * 60 * 24 * time.Minute
	RefreshTokenExpiresInTime time.Duration = 30 * 24 * 60 * time.Minute

	REDIS_EXPIRY_TIME         time.Duration = 3 * time.Minute
	HAS_ACCESS_USER_CACHE_TTL time.Duration = 1 * time.Minute

	// Minimum interval between session.last_activity_at writes per session,
	// so per-request access checks don't turn into per-request UPDATEs.
	SESSION_LAST_ACTIVITY_THRESHOLD time.Duration = 1 * time.Minute

	ProjectID             string = "f5955c82-f264-4655-aeb4-86fd1c642cb6"
	AdminClientPlatformID string = "7d4a4c38-dd84-4902-b744-0488b80a4c01"
	AdminClientName       string = "ADMIN"
	OpenFaaSPlatformID    string = "7d4a4c38-dd84-4902-b744-0488b80a4c04"

	// Project statuses that block write access. Reads are still allowed; writes are
	// rejected with a clear, status-specific message.
	InactiveStatus          string = "inactive"
	InsufficientFundsStatus string = "insufficient_funds"
	BlockedStatus           string = "blocked"

	DefaultOtp string = "208071"

	// Service Configs
	LOW_NODE_TYPE        string = "LOW"
	HIGH_NODE_TYPE       string = "HIGH"
	ENTER_PRICE_TYPE     string = "ENTER_PRICE"
	ObjectBuilderService string = "BUILDER_SERVICE"

	READ   string = "read"
	WRITE  string = "write"
	UPDATE string = "update"
	DELETE string = "delete"

	// Login Strategy
	WithGoogle   string = "google"
	Default      string = "default"
	WithPhone    string = "phone"
	WithApple    string = "apple"
	WithEmail    string = "email"
	WithLogin    string = "login"
	WithFirebase string = "firebase"

	K8SNamespace string = "cp-region-type-id"
	LanguageId   string = "e2d68f08-8587-4136-8cd4-c26bf1b9cda1"
	NativeName   string = "English"
	ShortName    string = "en"

	// Errors
	UserProjectIdConstraint   string = "user_project_idx_unique"
	DuplicateUserProjectError string = "user with this client_type already exists in the project"
	PermissionDenied          string = "Permission denied"
	ProjectInactiveError      string = "Your project is inactive"
	InvalidPhoneError         string = "Неверный номер телефона, он должен содержать двенадцать цифр и +"
	InvalidOTPError           string = "invalid number of otp"
	InvalidRecipientError     string = "Invalid recipient type"
	ProjectIdError            string = "cant get project_id"
	EnvironmentIdError        string = "cant get environment_id"
	InvalidEmailError         string = "Email is not valid"
	EmailSettingsError        string = "email settings not found"
	SessionExpired            string = "Session has been expired"
	InvalidUsername           string = "invalid username"

	// User Status
	UserStatusBlocked  string = "BLOCKED"
	UserStatusInactive string = "INACTIVE"
	UserStatusActive   string = "ACTIVE"

	// Fare (billing) types
	FARE_API_KEYS string = "api_keys"
	FARE_USERS    string = "users_count"
	FARE_BUILDERS string = "builders"

	UGEN_FREE_FARE_ID = "07d8a364-ebb2-4291-a452-f44b335cb031"

	UgenSuperAdminUserId string = "c12c163c-38ee-4b37-8854-1dc9285fc3f8"

	// Commit Types
	COMMIT_TYPE_TABLE string = "TABLE"
	SMS_TEXT          string = "Code"
	EMAIL_REGEX       string = "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"
)

// ProjectStatusMessages maps each blocking project status to the user-facing
// message shown when access is refused.
var ProjectStatusMessages = map[string]string{
	InactiveStatus:          "Your project is inactive. Please contact support to reactivate it.",
	InsufficientFundsStatus: "Your project is suspended due to insufficient balance. Please top up your balance to continue.",
	BlockedStatus:           "Your project has been blocked. Please contact support.",
}

// IsProjectStatusBlocking reports whether a project status forbids write access.
func IsProjectStatusBlocking(projectStatus string) bool {
	_, blocking := ProjectStatusMessages[projectStatus]
	return blocking
}

// BlockingStatusMessage translates a PermissionDenied error from the access gate
// into its user-facing message, returning false for any other error. The gate
// signals a blocking project status by carrying the raw status string in the
// gRPC error message.
func BlockingStatusMessage(err error) (string, bool) {
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		return "", false
	}
	message, blocking := ProjectStatusMessages[st.Message()]
	return message, blocking
}

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrEmailRequired    = errors.New("email required for register company")
	ErrUserAlradyMember = errors.New("user is already member")
	ErrInactiveClientId = errors.New("client id is an inactive")
	ErrInvalidUsername  = errors.New("invalid username")

	RegisterTypes = map[string]bool{
		"google":  true,
		"default": true,
		"phone":   true,
		"apple":   true,
		"email":   true,
	}

	DEFAULT_OTPS = map[string]bool{
		"1221":   true,
		"78281":  true,
		"4231":   true,
		"123456": true,
	}

	HashTypes = map[string]int{
		"argon":  1,
		"bcrypt": 2,
	}

	Path = map[string]bool{
		"object":       true,
		"object-slim":  true,
		"items":        true,
		"many-to-many": false,
	}

	RateLimitCfg = []*ratelimiter.LeakyBucket{
		{
			Method:         "POST",
			Path:           "/v2/send-code",
			RequestLimit:   5,
			Interval:       "minute",
			Type:           "body",
			KeyField:       "recipient",
			AllowOnFailure: true,
			NotAllowMsg:    "send-code request limit exceeded",
			NotAllowCode:   "TOO_MANY_REQUESTS",
		},
	}

	SystemTableSlugs = map[string]bool{
		"connections": true,
	}
)
