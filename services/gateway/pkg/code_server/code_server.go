package code_server

import (
	"bytes"
	"errors"
	"log"
	"os/exec"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

type CodeServer struct {
	Id             string
	Name           string
	HelmRepoAdd    string
	HelmRepoUpdate string
	HelmInstall    string
	HelmUninstall  string
}

func CreateCodeServerV2(data CodeServer) error {
	var (
		stderr bytes.Buffer
		stdout bytes.Buffer
	)

	if len(data.HelmRepoAdd) < 2 || len(data.HelmRepoUpdate) < 2 || len(data.HelmInstall) < 2 {
		return errors.New("invalid helm command")
	}
	var (
		helmRepoAdd    = strings.Split(strings.ReplaceAll(data.HelmRepoAdd, "  ", " "), " ")
		helmRepoUpdate = strings.Split(strings.ReplaceAll(data.HelmRepoUpdate, "  ", " "), " ")
		cmd            = exec.Command(helmRepoAdd[0], helmRepoAdd[1:]...)
	)

	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		log.Println("error [helmRepoAdd]", err)
		return errors.New("error while adding repo:" + stderr.String())
	}

	cmd = exec.Command(helmRepoUpdate[0], helmRepoUpdate[1:]...)

	err = cmd.Run()
	if err != nil {
		return errors.New("error while update repo:" + stderr.String())
	}

	data.HelmInstall = strings.ReplaceAll(data.HelmInstall, "{id}", data.Id)
	data.HelmInstall = strings.ReplaceAll(data.HelmInstall, "{name}", data.Name)

	var helmInstall = strings.Split(strings.ReplaceAll(data.HelmInstall, "  ", " "), " ")

	cmd = exec.Command(helmInstall[0], helmInstall[1:]...)

	err = cmd.Run()
	isErr := !strings.HasPrefix(stderr.String(), "WARNING:")
	if err != nil && isErr {
		return errors.New("error while install code server:" + stderr.String())
	}

	return nil
}

func DeleteCodeServerByPath(path string, cfg config.BaseConfig) error {
	var (
		stdout bytes.Buffer
		cmd    = exec.Command("helm", "uninstall", path, "-n", "test")
		err    = cmd.Run()
		stderr bytes.Buffer
	)

	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err != nil {
		return err
	}

	return nil
}
