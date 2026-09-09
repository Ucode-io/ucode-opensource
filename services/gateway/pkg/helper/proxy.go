package helper

import (
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
)

type MatchingData struct {
	ProjectId string
	EnvId     string
	Path      string
}

func FindUrlTo(res *company_service.GetListRedirectUrlRes, data MatchingData) (string, error) {
	for _, v := range res.GetRedirectUrls() {
		m := make(map[string]string)
		from := strings.Split(v.From, "/")
		to := v.To
		path := strings.Split(data.Path, "/")
		isEqual := true

		if len(path) != len(from) {
			continue
		}

		for i, el := range from {
			if len(el) >= 1 && el[0] == '{' && el[len(el)-1] == '}' {
				m[el] = path[i]
			} else {
				if el != path[i] {
					isEqual = false
					break
				}
			}
		}

		if isEqual {
			for i, el := range m {
				to = strings.Replace(to, i, el, 1)
			}
			return to, nil
		}
	}

	return data.Path, nil
}
