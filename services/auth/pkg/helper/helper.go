package helper

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	digits = "0123456789"
	all    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		digits
)

func ReplaceQueryParams(namedQuery string, params map[string]any) (string, []any) {
	var (
		i    = 1
		args []any
	)

	for k, v := range params {
		if k != "" && strings.Contains(namedQuery, ":"+k) {
			namedQuery = strings.ReplaceAll(namedQuery, ":"+k, "$"+strconv.Itoa(i))
			args = append(args, v)
			i++
		}
	}

	return namedQuery, args
}

func ReplaceSQL(old, searchPattern string) string {
	tmpCount := strings.Count(old, searchPattern)
	for m := 1; m <= tmpCount; m++ {
		old = strings.Replace(old, searchPattern, "$"+strconv.Itoa(m), 1)
	}
	return old
}

func ConvertMapToStruct(inputMap map[string]any) (*structpb.Struct, error) {
	marshledInputMap, err := json.Marshal(inputMap)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshledInputMap, outputStruct)

	return outputStruct, err
}

func ConvertRequestToSturct(inputRequest any) (*structpb.Struct, error) {
	marshelledInputInterface, err := json.Marshal(inputRequest)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshelledInputInterface, outputStruct)
	return outputStruct, err
}

func ConvertStructToResponse(inputStruct *structpb.Struct) (map[string]any, error) {
	marshelledInputStruct, err := protojson.Marshal(inputStruct)
	outputMap := make(map[string]any, 0)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshelledInputStruct, &outputMap)
	return outputMap, err
}

func ConverPhoneNumberToMongoPhoneFormat(input string) string {
	//input +998995677777
	input = input[4:]

	// input  = 995677777
	changedEl := input[:2]

	input = "(" + changedEl + ") " + input[2:5] + "-" + input[5:7] + "-" + input[7:]

	// input = (99) 567-77-77
	return input
}

func GenerateSecretKey(length int) string {
	buf := make([]byte, length)

	for i := 0; i < length; i++ {
		buf[i] = all[cryptoRandSecure(int64(len(all)))]
	}

	return string(buf)
}

func cryptoRandSecure(max int64) int64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		log.Println(err)
	}
	return nBig.Int64()
}

func ParsePsqlTypeToEnum(arg string) error {
	enum := map[string]int32{
		"PHONE":     0,
		"EMAIL":     1,
		"LOGIN":     2,
		"PHONE_OTP": 3,
		"EMAIL_OTP": 4,
		"LOGIN_PWD": 5,
		"PHONE_PWD": 6,
		"EMAIL_PWD": 7,
		"GOOGLE":    8,
		"APPLE":     9,
	}
	_, ok := enum[arg]
	if !ok {
		return errors.New("incorrect auth type")
	}
	return nil
}

func GetURLWithTableSlug(c *gin.Context) string {
	url := c.FullPath()
	if strings.Contains(url, ":table_slug") {
		tableSlug := c.Param("table_slug")
		url = strings.Replace(url, ":table_slug", tableSlug, -1)
	} else if strings.Contains(url, ":collection") && strings.Contains(url, "/items") {
		tableSlug := c.Param("collection")
		url = strings.Replace(url, ":collection", tableSlug, -1)
	}

	return url
}

func MarshalToStruct(data any, resp any) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = json.Unmarshal(js, resp)
	if err != nil {
		return err
	}

	return nil
}

func EmailValidation(email string) bool {
	emailRegex := regexp.MustCompile(config.EMAIL_REGEX)
	checkEmail := emailRegex.MatchString(email)

	return checkEmail
}

func GetStringFromMap(body map[string]any, key string) string {
	if value, ok := body[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}
func AnyToString(value any, exist bool) string {
	if !exist {
		return ""
	}

	return value.(string)
}
