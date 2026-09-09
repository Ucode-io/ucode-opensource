package helper

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func ChangeHostname(data []byte) ([]byte, error) {
	var isChangedByHost = map[string]bool{}

	if err := json.Unmarshal(data, &isChangedByHost); err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	isChangedByHost[hostname] = true

	data, err = json.Marshal(isChangedByHost)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func ReplaceQueryParams(namedQuery string, params map[string]any) (string, []any) {
	var (
		i    int = 1
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

func ConvertMapToStruct(inputMap map[string]any) (*structpb.Struct, error) {
	marshledInputMap, err := json.Marshal(inputMap)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshledInputMap, outputStruct)

	return outputStruct, err
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

func Convert[T any, U any](in T) (U, error) {
	var out U

	data, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	err = json.Unmarshal(data, &out)
	return out, err
}
