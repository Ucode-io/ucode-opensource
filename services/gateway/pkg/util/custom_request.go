package util

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const Header = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"

func DoRequest(url string, method string, body any) (responseModel models.InvokeFunctionResponse, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(time.Second * 30),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(respByte, &responseModel)
	if err != nil {
		return
	}

	return
}

// DoXMLRequest function for alaflab integration
func DoXMLRequest(url string, method string, body any) (responseModel models.InvokeFunctionResponse, err error) {
	data, err := xml.MarshalIndent(&body, " ", "  ")
	if err != nil {
		return
	}

	data = []byte(Header + string(data))

	client := &http.Client{
		Timeout: time.Duration(30 * time.Second),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	err = json.Unmarshal(respByte, &responseModel)

	return
}

func DoRequestCheckCodeServer(url string, method string, body any) (status int, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}
	client := &http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	status = resp.StatusCode
	defer resp.Body.Close()

	return
}

func DoDynamicRequest(url string, headers map[string]string, method string, body any) (respByte []byte, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(time.Second * 30),
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	respByte, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}
