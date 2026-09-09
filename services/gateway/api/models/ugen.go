package models

type (
	UgenProjectResp struct {
		Id            string `json:"id"`
		Title         string `json:"title"`
		Logo          string `json:"logo"`
		IsUgen        bool   `json:"is_ugen"`
		EnvironmentId string `json:"environment_id"`
	}

	UgenCompanyResp struct {
		Id              string            `json:"id"`
		Name            string            `json:"name"`
		Logo            string            `json:"logo"`
		HasPersonalFork bool              `json:"has_personal_fork"`
		Projects        []UgenProjectResp `json:"projects"`
	}

	UgenUserProjectsResp struct {
		Companies []UgenCompanyResp `json:"companies"`
	}
	UgenProjectData struct {
		Id            string
		Title         string
		Logo          string
		IsUgen        bool
		EnvironmentId string
	}
)
