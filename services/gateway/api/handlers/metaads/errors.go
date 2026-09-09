package metaads

import "fmt"

type graphError struct {
	StatusCode int
	Code       int
	Subcode    int
	Type       string
	Message    string
	Transient  bool
}

func (e *graphError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("meta graph api error: status=%d code=%d subcode=%d type=%s message=%s", e.StatusCode, e.Code, e.Subcode, e.Type, e.Message)
	}
	return fmt.Sprintf("meta graph api error: status=%d message=%s", e.StatusCode, e.Message)
}

func (e *graphError) retryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500 || e.Transient
}
