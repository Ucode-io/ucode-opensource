package helper

var VIEW_TYPES = map[string]string{
	"TABLE":            "TABLE",
	"CALENDAR":         "CALENDAR",
	"CALENDAR HOUR":    "CALENDAR HOUR",
	"GANTT":            "GANTT",
	"TREE":             "TREE",
	"BOARD":            "BOARD",
	"FINANCE CALENDAR": "FINANCE CALENDAR",
}

var (
	FIELD_TYPES = map[string]string{
		"SINGLE_LINE":                 "VARCHAR",
		"MULTI_LINE":                  "VARCHAR",
		"PICK_LIST":                   "VARCHAR",
		"LOOKUP":                      "UUID",
		"EMAIL":                       "VARCHAR",
		"PHOTO":                       "VARCHAR",
		"PHONE":                       "VARCHAR",
		"UUID":                        "VARCHAR",
		"INCREMENT_ID":                "VARCHAR",
		"RANDOM_NUMBERS":              "VARCHAR",
		"PASSWORD":                    "VARCHAR",
		"FILE":                        "VARCHAR",
		"CODABAR":                     "VARCHAR",
		"INTERNATION_PHONE":           "VARCHAR",
		"FORMULA_FRONTEND":            "VARCHAR",
		"TIME":                        "VARCHAR",
		"DATE_TIME":                   "TIMESTAMP",
		"DATE_TIME_WITHOUT_TIME_ZONE": "TIMESTAMP",
		"DATE":                        "DATE",
		"NUMBER":                      "FLOAT",
		"FLOAT":                       "FLOAT",
		"FLOAT_NOLIMIT":               "FLOAT",
		"FORMULA":                     "FLOAT",
		"CHECKBOX":                    "BOOL",
		"SWITCH":                      "BOOL",
		"MULTISELECT":                 "TEXT[]",
		"LOOKUPS":                     "UUID[]",
		"DYNAMIC":                     "TEXT[]",
		"LANGUAGE_TYPE":               "TEXT[]",
		"MULTI_IMAGE":                 "TEXT[]",
		"MULTI_FILE":                  "TEXT[]",
		"MONEY":                       "TEXT[]",
		"INCREMENT_NUMBER":            "SERIAL",
		"MAP":                         "VARCHAR",
		"JSON":                        "VARCHAR",
		"COLOR":                       "VARCHAR",
		"ICON":                        "VARCHAR",
		"VIDEO":                       "VARCHAR",
		"CODE":                        "VARCHAR",
		"RANDOM_UUID":                 "VARCHAR",
		"ARRAY":                       "TEXT[]",
	}
)

var (
	TRACKED_TABLES_FIELD_TYPES = map[string]string{
		"character varying": "SINGLE_LINE",
		"varchar":           "SINGLE_LINE",
		"text":              "SINGLE_LINE",
		"enum":              "SINGLE_LINE",
		"bytea":             "SINGLE_LINE",
		"citext":            "SINGLE_LINE",

		"jsonb": "JSON",
		"json":  "JSON",

		"smallint":         "FLOAT",
		"integer":          "FLOAT",
		"bigint":           "FLOAT",
		"numeric":          "FLOAT",
		"decimal":          "FLOAT",
		"real":             "FLOAT",
		"double precision": "FLOAT",
		"smallserial":      "FLOAT",
		"serial":           "FLOAT",
		"bigserial":        "FLOAT",
		"money":            "FLOAT",
		"int2":             "FLOAT",
		"int4":             "FLOAT",
		"int8":             "FLOAT",

		"timestamp":                   "DATE_TIME",
		"timestamptz":                 "DATE_TIME",
		"timestamp without time zone": "DATE_TIME_WITHOUT_TIME_ZONE",
		"timestamp with time zone":    "DATE_TIME",
		"date":                        "DATE",

		"boolean": "CHECKBOX",

		"uuid": "UUID",
	}
)

var (
	NUMERIC_TYPES = map[string]bool{
		"NUMBER": true,
		"FLOAT":  true,
	}
)

var (
	TYPE_REGEXP = map[string]string{
		"VARCHAR":   "^.{0,255}$",
		"DATE":      `^\d{4}-\d{2}-\d{2}$`,
		"TIME":      `^\d{2}:\d{2}(:\d{2})?$`,
		"TIMESTAMP": `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2})?$`,
		"FLOAT":     `^-?\d+(\.\d+)?$`,
		"BOOL":      "^(true|false)$",
		"TEXT[]":    "^{.*}$",
	}
)

var (
	TYPE_DEFAULT = map[string]any{
		"VARCHAR":   "",
		"DATE":      "CURRENT_DATE",
		"TIME":      "CURRENT_TIME",
		"TIMESTAMP": "CURRENT_TIMESTAMP",
		"FLOAT":     0.0,
		"BOOL":      false,
		"TEXT[]":    "{}",
	}
)

func GetCustomToPostgres(pgType string) string {
	if customType, ok := TRACKED_TABLES_FIELD_TYPES[pgType]; ok {
		return customType
	}

	return "TEXT[]"
}

func GetDataType(t string) string {
	val, ok := FIELD_TYPES[t]
	if !ok {
		return "VARCHAR"
	}

	return val
}

func GetRegExp(t string) string {
	val, ok := TYPE_REGEXP[t]
	if !ok {
		return "^.{0,255}$"
	}

	return val
}

func GetDefault(t string) any {
	val, ok := TYPE_DEFAULT[t]
	if !ok {
		return ""
	}

	return val
}
