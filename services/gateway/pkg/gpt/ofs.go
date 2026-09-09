package gpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func CreateFunction(req *models.CreateFunctionAI) ([]*models.CreateVersionHistoryRequest, error) {

	var (
		resource   = req.Resource
		services   = req.Service
		respLogReq = []*models.CreateVersionHistoryRequest{}
	)

	funcPath, funcId, err := CreateOfs(req.FunctionName, req.Token)
	if err != nil {
		return respLogReq, err
	}

	repoURL := fmt.Sprintf("https://gitlab.udevs.io/ucode_functions_group/%s.git", funcPath)
	directory := fmt.Sprintf("./%s", funcPath)

	accessToken := req.GitlabToken

	gitlab, err := git.PlainClone(directory, false, &git.CloneOptions{
		URL: repoURL,
		Auth: &ghttp.BasicAuth{
			Username: "token",
			Password: accessToken,
		},
	})
	if err != nil {
		return respLogReq, err
	}

	curDir, err := os.Getwd()
	if err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir + "/" + funcPath); err != nil {
		return respLogReq, err
	}

	if err := exec.Command("make", "gen-function").Run(); err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir + "/" + funcPath + "/" + funcPath); err != nil {
		return respLogReq, err
	}

	err = os.Remove("handler_test.go")
	if err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir); err != nil {
		return respLogReq, err
	}

	apiKeys, err := services.AuthService().ApiKey().GetList(context.Background(), &auth_service.GetListReq{
		EnvironmentId: req.EnvironmentId,
		ProjectId:     resource.ProjectId,
	})
	if err != nil {
		return respLogReq, err
	}

	var appId string
	if len(apiKeys.Data) > 0 {
		appId = apiKeys.Data[0].AppId
	} else {
		return respLogReq, err
	}

	code, err := GetOfsCode(models.Message{
		Role:    "user",
		Content: req.Prompt,
	})
	if err != nil {
		return respLogReq, err
	}

	code = strings.ReplaceAll(code, "```", "")
	code = strings.ReplaceAll(code, "go", "")
	code = strings.ReplaceAll(code, "lang", "")

	if err := os.Chdir(curDir + "/template"); err != nil {
		return respLogReq, err
	}

	temp, err := os.ReadFile("ofs.txt")
	if err != nil {
		return respLogReq, err
	}

	tempYml, err := os.ReadFile("ci-yml.txt")
	if err != nil {
		return respLogReq, err
	}

	newCode := fmt.Sprintf(string(temp), appId, code, "%s", "%s", "%s")

	if err := os.Chdir(curDir + "/" + funcPath + "/" + funcPath); err != nil {
		return respLogReq, err
	}

	file, err := os.OpenFile("handler.go", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return respLogReq, err
	}
	defer file.Close()

	_, err = file.Write([]byte(newCode))
	if err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir + "/" + funcPath); err != nil {
		return respLogReq, err
	}

	file, err = os.OpenFile(".gitlab-ci.yml", os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return respLogReq, err
	}
	defer file.Close()

	_, err = file.Write([]byte(tempYml))
	if err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir + "/"); err != nil {
		return respLogReq, err
	}

	w, err := gitlab.Worktree()
	if err != nil {
		return respLogReq, err
	}

	err = w.AddWithOptions(&git.AddOptions{All: true})
	if err != nil {
		return respLogReq, err
	}

	status, err := w.Status()
	if err != nil {
		return respLogReq, err
	}

	if status.IsClean() {
		return respLogReq, fmt.Errorf("nothing to commit")
	}

	commit, err := w.Commit("Add function from gpt", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "GPT",
			Email: "shokhrukh.safarov@udevs.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		return respLogReq, err
	}

	_, err = gitlab.CommitObject(commit)
	if err != nil {
		return respLogReq, err
	}

	err = gitlab.Push(&git.PushOptions{
		Auth: &ghttp.BasicAuth{
			Username: "token",
			Password: accessToken,
		},
	})
	if err != nil {
		return respLogReq, err
	}

	if err := os.Chdir(curDir + "/"); err != nil {
		return respLogReq, err
	}

	err = os.RemoveAll(funcPath)
	if err != nil {
		return respLogReq, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:

		tables, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetTablesByLabel(context.Background(), &obs.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		table := tables.Tables[0]

		_, err = services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Create(
			context.Background(),
			&obs.CreateCustomEventRequest{
				Icon:       "arrows-up-down-left-right.svg",
				Label:      funcPath,
				EventPath:  funcId,
				TableSlug:  table.Slug,
				ActionType: req.ActionType,
				Method:     req.Method,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			return respLogReq, err
		}
	case pb.ResourceType_POSTGRESQL:

		tables, err := services.GoObjectBuilderService().Table().GetTablesByLabel(context.Background(), &nb.GetTablesByLabelReq{
			ProjectId: resource.ResourceEnvironmentId,
			Label:     req.Table,
		})
		if err != nil {
			return respLogReq, err
		}

		table := tables.Tables[0]

		_, err = services.GoObjectBuilderService().CustomEvent().Create(
			context.Background(),
			&nb.CreateCustomEventRequest{
				Icon:       "arrows-up-down-left-right.svg",
				Label:      funcPath,
				EventPath:  funcId,
				TableSlug:  table.Slug,
				ActionType: req.ActionType,
				Method:     req.Method,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			return respLogReq, err
		}
	}

	return respLogReq, nil
}

func CreateOfs(name, token string) (string, string, error) {
	body := []byte(fmt.Sprintf(`{
			"description": "%s",
			"name": "%s",
			"resource_id": "ucode_gitlab",
			"type": "FUNCTION",
			"path": "%s"
		}`, name, name, name))

	client := &http.Client{
		Timeout: time.Duration(10 * time.Second),
	}

	request, err := http.NewRequest("POST", "https://api.admin.u-code.io/v2/function", bytes.NewBuffer(body))
	if err != nil {
		return "", "", err
	}

	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := client.Do(request)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	response := OfsCreateResp{}

	if err := json.Unmarshal(respByte, &response); err != nil {
		return "", "", err
	}

	if response.Status != "CREATED" {
		return "", "", fmt.Errorf("error while create ofs")
	}

	return response.Data.Path, response.Data.Id, nil
}

type OfsCreateResp struct {
	Status string `json:"status"`
	Data   Data   `json:"data"`
}

type Data struct {
	Id   string `json:"id"`
	Path string `json:"path"`
}
