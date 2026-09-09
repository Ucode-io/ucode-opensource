package util

import "errors"

func CheckEnum(str string, enums []string) error {

	for _, enum := range enums {
		if enum == str {
			return nil
		}
	}
	return errors.New("wrong login type")
}
