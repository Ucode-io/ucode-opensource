package models

type UserId struct {
	UserId string `json:"user_id"`
}

type Language struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	ShortName  string `json:"short_name"`
	NativeName string `json:"native_name"`
}

type SetEmail struct {
	Email  string `json:"email"`
	UserId string `json:"user_id"`
}

type ResetPassword struct {
	Password string `json:"password"`
	UserId   string `json:"user_id"`
}

type ForgotPasswordResponse struct {
	UserId     string `json:"user_id"`
	EmailFound bool   `json:"email_found"`
	SmsId      string `json:"sms_id"`
	Email      string `json:"email"`
}

type Timezone struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
}

type ListLanguage struct {
	Language []*Language `json:"language"`
	Count    int         `json:"count"`
}

type ListTimezone struct {
	Timezone []*Timezone `json:"timezone"`
	Count    int         `json:"count"`
}

type UserProjectClientTypeRequest struct {
	UserId    string `json:"user_id"`
	ProjectId string `json:"project_id"`
}
type UserProjectClientTypeResponse struct {
	ClientTypeIds []string `json:"client_type_ids"`
}
