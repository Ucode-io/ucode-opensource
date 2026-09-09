package helper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

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

func ConvertMapToStruct(inputMap map[string]any) (*structpb.Struct, error) {
	marshledInputMap, err := json.Marshal(inputMap)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshledInputMap, outputStruct)

	return outputStruct, err
}

func GetURLWithTableSlug(c *gin.Context) (string, string) {
	url := c.FullPath()
	if strings.Contains(url, ":collection") {
		tableSlug := c.Param("collection")
		return url, tableSlug
	}

	return url, ""
}

func ReplaceQueryParams(namedQuery string, params map[string]any) (string, []any) {
	var (
		i    int = 1
		args     = make([]any, 0, len(params))
	)

	for k, v := range params {
		if k != "" {
			oldsize := len(namedQuery)
			namedQuery = strings.ReplaceAll(namedQuery, ":"+k, "$"+strconv.Itoa(i))

			if oldsize != len(namedQuery) {
				args = append(args, v)
				i++
			}
		}
	}

	return namedQuery, args
}

func ConvertPbToAnotherPb(data *pbObject.V2LoginResponse) *pb.V2LoginResponse {
	res := &pb.V2LoginResponse{}
	res.UserId = data.UserId
	res.LoginTableSlug = data.LoginTableSlug
	tables := make([]*pb.Table, 0, len(data.ClientType.Tables))
	for _, v := range data.ClientType.Tables {
		table := &pb.Table{}
		table.Data = v.Data
		table.Icon = v.Icon
		table.Label = v.Label
		table.Slug = v.Slug
		table.ViewLabel = v.ViewLabel
		table.ViewSlug = v.ViewSlug
		tables = append(tables, table)
	}

	res.ClientType = &pb.ClientType{
		Id:           data.ClientType.Guid,
		Name:         data.ClientType.Name,
		ConfirmBy:    pb.ConfirmStrategies(data.ClientType.ConfirmBy),
		SelfRegister: data.ClientType.SelfRegister,
		SelfRecover:  data.ClientType.SelfRecover,
		ProjectId:    data.ClientType.ProjectId,
		Tables:       tables,
	}

	res.ClientPlatform = &pb.ClientPlatform{
		Id:        data.ClientPlatform.Guid,
		Name:      data.ClientPlatform.Name,
		ProjectId: data.ClientPlatform.ProjectId,
		Subdomain: data.ClientPlatform.Subdomain,
	}
	permissions := make([]*pb.RecordPermission, 0, len(data.Permissions))
	for _, v := range data.Permissions {
		permission := &pb.RecordPermission{}
		permission.ClientTypeId = v.ClientTypeId
		permission.Id = v.Guid
		permission.Read = v.Read
		permission.Write = v.Write
		permission.Delete = v.Delete
		permission.Update = v.Update
		permission.RoleId = v.RoleId
		permission.TableSlug = v.TableSlug
		permissions = append(permissions, permission)
	}
	res.Permissions = permissions
	res.Role = &pb.Role{
		Id:               data.Role.Guid,
		ClientTypeId:     data.Role.ClientTypeId,
		Name:             data.Role.Name,
		ClientPlatformId: data.Role.ClientPlatformId,
		ProjectId:        data.Role.ProjectId,
	}
	return res
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

func ConvertStructToResponse(inputStruct *structpb.Struct) (map[string]any, error) {
	marshelledInputStruct, err := protojson.Marshal(inputStruct)
	outputMap := make(map[string]any, 0)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshelledInputStruct, &outputMap)
	return outputMap, err
}

func DeleteKeys(m map[string]any, keysToDelete ...string) {
	for _, key := range keysToDelete {
		delete(m, key)
	}
}

func GetWeekdayRange(start, end time.Time) []string {
	var weekdays []string

	// Make sure start is at the beginning of the day
	start = start.Truncate(24 * time.Hour)

	// Iterate over each day between start and end
	for current := start; !current.After(end); current = current.Add(24 * time.Hour) {
		// Check if the current day is a weekday (Monday to Friday)
		if current.Weekday() >= time.Sunday && current.Weekday() <= time.Saturday {
			weekdays = append(weekdays, current.Format("Monday"))
		}
	}

	return weekdays
}

func RemoveDuplicateStrings(arr []string) []string {
	// Use a map to track unique values
	uniqueMap := make(map[string]bool)
	var uniqueArr []string

	// Iterate over the array
	for _, val := range arr {
		// Check if the value is already in the map
		if _, exists := uniqueMap[val]; !exists {
			// If not, add it to the map and append to the unique array
			uniqueMap[val] = true
			uniqueArr = append(uniqueArr, strings.ToLower(val))
		}
	}

	return uniqueArr
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func ContainsLike(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(e, a) {
			return true
		}
	}
	return false
}

// InterfaceToMap converts an any to a map[string]any
func InterfaceToMap(data any) (map[string]any, error) {
	result := make(map[string]any)

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ListFiles(folderPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relativePath, err := filepath.Rel(folderPath, path)
			if err != nil {
				return err
			}
			files = append(files, strings.ReplaceAll(relativePath, "\\", "/"))
		}
		return nil
	})
	return files, err
}

func ConvertStructToMap(s *structpb.Struct) (map[string]any, error) {

	newMap := make(map[string]any)

	body, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}, err
	}
	if err := json.Unmarshal(body, &newMap); err != nil {
		return map[string]any{}, err
	}

	return newMap, nil
}
