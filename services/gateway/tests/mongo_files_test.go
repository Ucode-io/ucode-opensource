package tests

var (
	UploadUrl     = "https://api.admin.u-code.io/v1/files/folder_upload?folder_name=Media"
	GenerateFile2 = "/main_test.go"
)

// func TestFileUpload(t *testing.T) {
// 	currentDir, err := os.Getwd()
// 	assert.NoError(t, err)

// 	uploadResp, err := UploadFile(UploadUrl, currentDir+GenerateFile2, UcodeApi.Config().AppId)
// 	assert.NoError(t, err)
// 	assert.NotEmpty(t, uploadResp)

// 	_, err = UcodeApi.DoRequest(BaseUrl+"/v1/files/"+uploadResp.Data.ID, http.MethodDelete, map[string]any{},
// 		map[string]string{
// 			"Resource-Id":    ResourceId,
// 			"Environment-Id": EnvironmentId,
// 			"X-API-KEY":      UcodeApi.Config().AppId,
// 			"Authorization":  "API-KEY",
// 		},
// 	)
// 	assert.NoError(t, err)
// }

// func DoRequestV2(url string, method string, body bytes.Buffer, appId string, writer *multipart.Writer) ([]byte, error) {
// 	req, err := http.NewRequest("POST", url, &body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	req.Header.Set("Content-Type", writer.FormDataContentType())
// 	req.Header.Set("authorization", "API-KEY")
// 	req.Header.Set("X-API-KEY", appId)

// 	client := &http.Client{}
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	respByte, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return respByte, nil
// }

// func UploadFile(url, filePath, apiKey string) (FileResponse, error) {
// 	var (
// 		err        error
// 		file       *os.File
// 		fileResp   FileResponse
// 		fileBuffer bytes.Buffer
// 		writer     *multipart.Writer
// 	)

// 	file, err = os.Open(filePath)
// 	if err != nil {
// 		return FileResponse{}, err
// 	}
// 	defer file.Close()

// 	writer = multipart.NewWriter(&fileBuffer)
// 	part, err := writer.CreateFormFile("file", file.Name())
// 	if err != nil {
// 		return FileResponse{}, err
// 	}

// 	_, err = io.Copy(part, file)
// 	if err != nil {
// 		return FileResponse{}, err
// 	}

// 	err = writer.Close()
// 	if err != nil {
// 		return FileResponse{}, err
// 	}

// 	resp, err := DoRequestV2(url, "POST", fileBuffer, apiKey, writer)
// 	if err != nil {
// 		return FileResponse{}, err
// 	}

// 	err = json.Unmarshal(resp, &fileResp)
// 	if err != nil {
// 		return FileResponse{}, err
// 	}

// 	return fileResp, nil
// }

// type FileResponse struct {
// 	Status      string `json:"status"`
// 	Description string `json:"description"`
// 	Data        struct {
// 		ID               string `json:"id"`
// 		Title            string `json:"title"`
// 		Storage          string `json:"storage"`
// 		FileNameDisk     string `json:"file_name_disk"`
// 		FileNameDownload string `json:"file_name_download"`
// 		Link             string `json:"link"`
// 		FileSize         int    `json:"file_size"`
// 	} `json:"data"`
// 	CustomMessage string `json:"custom_message"`
// }
