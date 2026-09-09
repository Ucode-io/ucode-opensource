package config

import (
	"time"
)

const (
	DatabaseQueryTimeLayout  string = `'YYYY-MM-DD"T"HH24:MI:SS"."MS"Z"TZ'`
	DatabaseTimeLayout       string = time.RFC3339
	TimeLayoutItems          string = "02.01.2006 15:04"
	ErrTheSameId             string = "cannot use the same uuid for 'id' and 'parent_id' fields"
	ErrRpcNodFoundAndNoRows  string = "rpc error: code = NotFound desc = no rows in result set"
	ErrNoRows                string = "no rows in result set"
	ErrObjectType            string = "object type error: code =  NodFound"
	ErrEnvNodFound           string = "No .env file found"
	ErrAuthInfo              string = "this table is auth table. Auth information not fully given"
	ErrInvalidUserId         string = "this user is not created in auth service"
	ErrCannotDelete          string = "you can only delete from login table"
	BcryptHashPasswordLength        = 60

	MenuParentId string = "c57eedc3-a954-4262-a0af-376c65b5a284"

	// Relation Types
	MANY2DYNAMIC string = "Many2Dynamic"
	MANY2MANY    string = "Many2Many"
	RECURSIVE    string = "Recursive"
	MANY2ONE     string = "Many2One"
	ONE2ONE      string = "One2One"
	ONE2MANY     string = "One2Many"

	// Filed Types
	INCREMENT_ID string = "INCREMENT_ID"
	PERSON       string = "PERSON"

	// Table Slugs
	CLIENT_TYPE       string = "client_type"
	ROLE              string = "role"
	USER              string = "user"
	PERSON_TABLE_SLUG string = "person"
	TEMPLATE          string = "template"

	// FieldSlugs
	CLIENT_TYPE_ID string = "client_type_id"
	ROLE_ID        string = "role_id"
	LAST_ACTIVITY  string = "last_activity"

	// Tree
	GUID string = "guid"
	PATH string = "path"
	ID   string = "_id"

	GRPC_MAX_CALL_SEND_MSG_SIZE = 100 * 1024 * 1024
	GRPC_MAX_CALL_RECV_MSG_SIZE = 100 * 1024 * 1024

	DateTimeWithZone = "2006-01-02 15:04:05 -0700 MST"

	FORMULA_FRONT = "FORMULA_FRONTEND"

	MAX_EXCEL_LIMIT = 10_000

	// Fare (billing) types
	FARE_TABLES string = "tables"

	FUNCTIONS_LOG_EXPIERE_DAY = 10

	VERSION_HISTORY_EXPIRE_DAY = 7
)

var (
	MENU_TYPES = map[string]bool{
		"TABLE":               true,
		"LAYOUT":              true,
		"FOLDER":              true,
		"MICROFRONTEND":       true,
		"FAVOURITE":           true,
		"HIDE":                true,
		"WEBPAGE":             true,
		"PIVOT":               true,
		"REPORT_SETTING":      true,
		"LINK":                true,
		"MINIO_FOLDER":        true,
		"GOOGLE_DRIVE_FOLDER": true,
		"WIKI":                true,
		"WIKI_FOLDER":         true,
	}

	SKIPPED_RELATION_TYPES = map[string]bool{
		"Many2Many":    true,
		"Many2Dynamic": true,
	}

	STATIC_MENU_IDS = map[string]bool{
		"c57eedc3-a954-4262-a0af-376c65b5a284": true, //root
		"c57eedc3-a954-4262-a0af-376c65b5a282": true, //favorite
		"c57eedc3-a954-4262-a0af-376c65b5a280": true, //admin
		"c57eedc3-a954-4262-a0af-376c65b5a278": true, //analytics
		"c57eedc3-a954-4262-a0af-376c65b5a276": true, //pivot
		"c57eedc3-a954-4262-a0af-376c65b5a274": true, //report setting
		"7c26b15e-2360-4f17-8539-449c8829003f": true, //saved pivot
		"e96b654a-1692-43ed-89a8-de4d2357d891": true, //history pivot
		"a8de4296-c8c3-48d6-bef0-ee17057733d6": true, //admin => user and permission
		"d1b3b349-4200-4ba9-8d06-70299795d5e6": true, //admin => database
		"f7d1fa7d-b857-4a24-a18c-402345f65df8": true, //admin => code
		"f313614f-f018-4ddc-a0ce-10a1f5716401": true, //admin => resource
		"db4ffda3-7696-4f56-9f1f-be128d82ae68": true, //admin => api
		"3b74ee68-26e3-48c8-bc95-257ca7d6aa5c": true, // profile setting
		"8a6f913a-e3d4-4b73-9fc0-c942f343d0b9": true, //files menu id
		"744d63e6-0ab7-4f16-a588-d9129cf959d1": true, //wiki menu id
		"9e988322-cffd-484c-9ed6-460d8701551b": true, // users menu id
	}

	STRING_TYPES = []string{
		"SINGLE_LINE", "MULTI_LINE",
		"PICK_LIST", "DATE",
		"LOOKUP", "EMAIL",
		"PHOTO", "PHONE", "UUID", "DATE_TIME",
		"TIME", "INCREMENT_ID", "RANDOM_NUMBERS", "PASSWORD",
		"FILE", "CODABAR", "INTERNATIONAL_PHONE", "DATE_TIME_WITHOUT_TIME_ZONE",
	}

	STATIC_TABLE_IDS = []string{
		"65a7936b-f3db-4401-afef-8eee77b68da3", //view_permission
		"1b066143-9aad-4b28-bd34-0032709e463b", //global_permission
		"08a391b2-1c78-4f3e-b84a-9d745e7d528f", //menu_permission
		"eca81c06-c4fc-4242-8dc9-ecca575e1762", // user_login_table
		"c2f225b6-b6d9-4201-aa25-e648a4c1ff29", //custom_error
		"6b99e876-b4d8-440c-b2e2-a961530690f8", //doctors
		"961a3201-65a4-452a-a8e1-7c7ba137789c", //field_permission
		"5db33db7-4524-4414-b65a-b6b8e5bba345", //test_login
		"5af2bfb2-6880-42ad-80c8-690e24a2523e", //action_permission
		"53edfff0-2a31-4c73-b230-06a134afa50b", //client_platform
		"4c1f5c95-1528-4462-8d8c-cd377c23f7f7", //automatic_filters
		"25698624-5491-4c39-99ec-aed2eaf07b97", //record_permission
		"074fcb3b-038d-483d-b390-ca69490fc4c3", //view_relation_permission
		"d267203c-1c23-4663-a721-7a845d4b98ad", //setting.languages
		"bba3dddc-5f20-449c-8ec8-37bef283c766", //setting.timezones
		"b1896ed7-ba00-46ae-ae53-b424d2233589", //file
		"08972256-30fb-4d75-b8cf-940d8c4fc8ac", //template
		"373e9aae-315b-456f-8ec3-0851cad46fbf", //project
		"2546e042-af2f-4cef-be7c-834e6bde951c", //user
		"0ade55f8-c84d-42b7-867f-6418e1314e28", //connections
		"5ca6860a-29d0-4a65-904d-e8a81525ad4e", //docx_template
	}

	SkipFields = map[string]bool{
		"guid": true,
	}

	Ftype = map[string]bool{
		"INCREMENT_NUMBER": true,
		"INCREMENT_ID":     true,
		"MANUAL_STRING":    true,
		"RANDOM_UUID":      true,
		"RANDOM_TEXT":      true,
		"RANDOM_NUMBER":    true,
		"PASSWORD":         true,
	}

	GetList2TableSlug = map[string]bool{
		CLIENT_TYPE: true,
		ROLE:        true,
		USER:        true,
		TEMPLATE:    true,
	}

	PersonTable = map[string]bool{
		PERSON_TABLE_SLUG: true,
	}

	McProjectAllowedOrder = map[string]string{
		"created_at": "mp.created_at",
		"updated_at": "mp.updated_at",
		"title":      "mp.title",
	}

	ChatAllowedOrder = map[string]string{
		"created_at":   "c.created_at",
		"updated_at":   "c.updated_at",
		"title":        "c.title",
		"total_tokens": "c.total_tokens",
	}

	AgentAllowedOrder = map[string]string{
		"created_at": "a.created_at",
		"updated_at": "a.updated_at",
		"name":       "a.name",
	}

	AgentRunAllowedOrder = map[string]string{
		"created_at":  "created_at",
		"finished_at": "finished_at",
		"status":      "status",
	}

	ProjectFolderAllowedOrder = map[string]string{
		"created_at":   "pf.created_at",
		"updated_at":   "pf.updated_at",
		"label":        "pf.label",
		"order_number": "pf.order_number",
	}

	CustomEndpointAllowedOrder = map[string]string{
		"name":       "name",
		"method":     "method",
		"created_at": "created_at",
		"updated_at": "updated_at",
	}
)
