package v2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/doc_generator_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"maps"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// CreateDocxTemplate godoc
// @Security ApiKeyAuth
// @ID create_docx_template
// @Router /v2/docx-template [POST]
// @Summary Create docx template
// @Description Create docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param template body nb.CreateDocxTemplateRequest true "CreateDocxTemplateReq"
// @Success 201 {object} status_http.Response{data=nb.DocxTemplate} "DocxTemplate data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateDocxTemplate(c *gin.Context) {
	var (
		docxTemplate nb.CreateDocxTemplateRequest
	)
	if err := c.ShouldBindJSON(&docxTemplate); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	docxTemplate.ResourceId = resource.GetResourceEnvironmentId()

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	docxTemplate.ProjectId = resource.ResourceEnvironmentId

	fileUUID := uuid.New().String()
	docxFileName := fileUUID + ".docx"
	pdfFileName := fileUUID + ".pdf"

	if docxTemplate.FileUrl != "" {
		client := &http.Client{}

		req, err := http.NewRequest("GET", docxTemplate.FileUrl, nil)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.HandleResponse(c, status_http.GRPCError, "error getting docx")
			return
		}

		out, err := os.Create(docxFileName)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer out.Close()

		if _, err = io.Copy(out, resp.Body); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
		}

		dst, _ := os.Getwd()

		fileData, err := os.ReadFile(dst + "/" + docxFileName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading saved document"})
			return
		}
		base64FileData := base64.StdEncoding.EncodeToString(fileData)

		payload := map[string]any{
			"Parameters": []map[string]any{
				{
					"Name": "File",
					"FileValue": map[string]any{
						"Name": "output.docx",
						"Data": base64FileData,
					},
				},
				{
					"Name":  "StoreFile",
					"Value": true,
				},
			},
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error marshaling payload"})
			return
		}

		convertResp, err := http.Post(
			config.ConvertDocxToPdfUrl+config.ConvertDocxToPdfSecret,
			"application/json",
			bytes.NewBuffer(payloadBytes),
		)
		if err != nil || convertResp.StatusCode != http.StatusOK {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error converting document to PDF error: %v", err)})
			return
		}
		defer convertResp.Body.Close()

		var convertApiResponse models.ConvertAPIResponse
		if err = json.NewDecoder(convertResp.Body).Decode(&convertApiResponse); err != nil || len(convertApiResponse.Files) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing conversion response"})
			return
		}

		pdfUrl := ""
		if len(convertApiResponse.Files) > 0 {
			pdfUrl = convertApiResponse.Files[0].Url
		}

		req, err = http.NewRequest("GET", pdfUrl, nil)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

		resp, err = client.Do(req)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.HandleResponse(c, status_http.BadRequest, "error getting pdf")
			return
		}

		pdfOut, err := os.Create(pdfFileName)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if _, err = io.Copy(pdfOut, resp.Body); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
			return
		}

		if err = pdfOut.Close(); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
			return
		}

		minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
			Secure: h.baseConf.MinioProtocol,
		})
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		defaultBucket := "docs"

		if _, err = minioClient.FPutObject(c.Request.Context(), defaultBucket, docxFileName, dst+"/"+docxFileName, minio.PutObjectOptions{}); err != nil {
			err = os.Remove(dst + "/" + docxFileName)
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if err = os.Remove(dst + "/" + docxFileName); err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
		}

		if _, err = minioClient.FPutObject(c.Request.Context(), defaultBucket, pdfFileName, dst+"/"+pdfFileName, minio.PutObjectOptions{}); err != nil {
			err = os.Remove(dst + "/" + pdfFileName)
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if err = os.Remove(dst + "/" + pdfFileName); err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
		}

		docxTemplate.FileUrl = h.baseConf.MinioEndpoint + "/" + defaultBucket + "/" + docxFileName
		docxTemplate.PdfUrl = h.baseConf.MinioEndpoint + "/" + defaultBucket + "/" + pdfFileName
	}

	res, err := services.GoObjectBuilderService().DocxTemplate().Create(
		c.Request.Context(),
		&docxTemplate,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, res)
}

// GetSingleDocxTemplate godoc
// @Security ApiKeyAuth
// @ID get_single_docx_template
// @Router /v2/docx-template/{docx-template-id} [GET]
// @Summary Get single docx template
// @Description Get single docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param docx-template-id path string true "docx-template-id"
// @Success 200 {object} status_http.Response{data=nb.DocxTemplate} "DocxTemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetSingleDocxTemplate(c *gin.Context) {
	docxTemplateId := c.Param("docx-template-id")

	if !util.IsValidUUID(docxTemplateId) {
		h.HandleResponse(c, status_http.InvalidArgument, "docx template id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GoObjectBuilderService().DocxTemplate().GetByID(
		c.Request.Context(),
		&nb.DocxTemplatePrimaryKey{
			Id:        docxTemplateId,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// UpdateDocxTemplate godoc
// @Security ApiKeyAuth
// @ID update_docx_template
// @Router /v2/docx-template [PUT]
// @Summary Update docx template
// @Description Update docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param docx_template body nb.DocxTemplate true "UpdateDocxTemplateReqBody"
// @Success 200 {object} status_http.Response{data=nb.DocxTemplate} "DocxTemplate data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateDocxTemplate(c *gin.Context) {
	var (
		docxTemplate nb.DocxTemplate
	)

	if err := c.ShouldBindJSON(&docxTemplate); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	docxTemplate.ProjectId = resource.ResourceEnvironmentId

	fileUUID := uuid.New().String()
	docxFileName := fileUUID + ".docx"
	pdfFileName := fileUUID + ".pdf"

	if docxTemplate.FileUrl != "" {
		client := &http.Client{}

		req, err := http.NewRequest("GET", docxTemplate.FileUrl, nil)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.HandleResponse(c, status_http.BadRequest, "error getting docx")
			return
		}

		out, err := os.Create(docxFileName)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer out.Close()

		if _, err = io.Copy(out, resp.Body); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
			return
		}

		dst, _ := os.Getwd()

		fileData, err := os.ReadFile(dst + "/" + docxFileName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error reading saved document"})
			return
		}
		base64FileData := base64.StdEncoding.EncodeToString(fileData)

		payload := map[string]any{
			"Parameters": []map[string]any{
				{
					"Name": "File",
					"FileValue": map[string]any{
						"Name": "output.docx",
						"Data": base64FileData,
					},
				},
				{
					"Name":  "StoreFile",
					"Value": true,
				},
			},
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error marshaling payload"})
			return
		}

		convertResp, err := http.Post(
			config.ConvertDocxToPdfUrl+config.ConvertDocxToPdfSecret,
			"application/json",
			bytes.NewBuffer(payloadBytes),
		)
		if err != nil || convertResp.StatusCode != http.StatusOK {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error converting document to PDF error: %v", err)})
			return
		}
		defer convertResp.Body.Close()

		var convertApiResponse models.ConvertAPIResponse
		if err = json.NewDecoder(convertResp.Body).Decode(&convertApiResponse); err != nil || len(convertApiResponse.Files) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing conversion response"})
			return
		}

		pdfUrl := ""
		if len(convertApiResponse.Files) > 0 {
			pdfUrl = convertApiResponse.Files[0].Url
		}

		req, err = http.NewRequest("GET", pdfUrl, nil)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36")

		resp, err = client.Do(req)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			h.HandleResponse(c, status_http.BadRequest, "error getting pdf")
			return
		}

		pdfOut, err := os.Create(pdfFileName)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if _, err = io.Copy(pdfOut, resp.Body); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
			return
		}

		if err = pdfOut.Close(); err != nil {
			h.HandleResponse(c, status_http.GrpcStatusToHTTP["Internal"], err.Error())
			return
		}

		minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
			Secure: h.baseConf.MinioProtocol,
		})
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		defaultBucket := "docs"

		if _, err = minioClient.FPutObject(c.Request.Context(), defaultBucket, docxFileName, dst+"/"+docxFileName, minio.PutObjectOptions{}); err != nil {
			err = os.Remove(dst + "/" + docxFileName)
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if err = os.Remove(dst + "/" + docxFileName); err != nil {
			h.log.Error("Error removing file", logger.Error(err))
		}

		if _, err = minioClient.FPutObject(c.Request.Context(), defaultBucket, pdfFileName, dst+"/"+pdfFileName, minio.PutObjectOptions{}); err != nil {
			err = os.Remove(dst + "/" + pdfFileName)
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		if err = os.Remove(dst + "/" + pdfFileName); err != nil {
			h.log.Error("Error removing file", logger.Error(err))
		}

		docxTemplate.FileUrl = h.baseConf.MinioEndpoint + "/" + defaultBucket + "/" + docxFileName
		docxTemplate.PdfUrl = h.baseConf.MinioEndpoint + "/" + defaultBucket + "/" + pdfFileName
	}

	res, err := services.GoObjectBuilderService().DocxTemplate().Update(
		c.Request.Context(),
		&docxTemplate,
	)

	if err != nil {
		h.log.Error("error in update docx template", logger.Error(err))
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// DeleteDocxTemplate godoc
// @Security ApiKeyAuth
// @ID delete_docx_template
// @Router /v2/docx-template/{docx-template-id} [DELETE]
// @Summary Delete docx template
// @Description Delete docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param docx-template-id path string true "docx-template-id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteDocxTemplate(c *gin.Context) {
	docxTemplateId := c.Param("docx-template-id")

	if !util.IsValidUUID(docxTemplateId) {
		h.HandleResponse(c, status_http.InvalidArgument, "docx template id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}
	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GoObjectBuilderService().DocxTemplate().Delete(
		c.Request.Context(),
		&nb.DocxTemplatePrimaryKey{
			Id:        docxTemplateId,
			ProjectId: resource.GetResourceEnvironmentId(),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, res)
}

// GetListDocxTemplate godoc
// @Security ApiKeyAuth
// @ID get_list_docx_template
// @Router /v2/docx-template [GET]
// @Summary Get List docx template
// @Description Get List docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param table-slug query string true "table-slug"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 200 {object} status_http.Response{data=nb.GetAllDocxTemplateResponse} "DocxTemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetListDocxTemplate(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GoObjectBuilderService().DocxTemplate().GetAll(
		c.Request.Context(),
		&nb.GetAllDocxTemplateRequest{
			ProjectId:  resource.GetResourceEnvironmentId(),
			TableSlug:  c.Query("table-slug"),
			Limit:      int32(limit),
			Offset:     int32(offset),
			ResourceId: resource.GetResourceEnvironmentId(),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// GetAllFieldsDocxTemplate godoc
// @Security ApiKeyAuth
// @ID get_all_fields_docx_template
// @Router /v2/docx-template/fields/list [GET]
// @Summary Get All fields docx template
// @Description Get All fields docx template
// @Tags Template
// @Accept json
// @Produce json
// @Param table-slug query string true "table-slug"
// @Success 200 {object} status_http.Response{data=nb.CommonMessage} "DocxTemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllFieldsDocxTemplate(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GoObjectBuilderService().ObjectBuilder().GetAllFieldsForDocx(
		c.Request.Context(),
		&nb.CommonMessage{
			TableSlug: c.DefaultQuery("table-slug", ""),
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// ConvertConstructorDocxToPdf godoc
// @Security ApiKeyAuth
// @ID convert_contructor_docx_to_pdf
// @Router /v2/docx-constructor/convert/pdf [POST]
// @Summary Generate PDF from docx-constructor template
// @Description Generate PDF from docx-constructor template
// @Tags Template
// @Accept json
// @Produce json
// @Param link query string true "link"
// @Param request body models.DocxTemplateVariables true "Variables"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) ConvertDocxToPdf(c *gin.Context) {
	link := c.Query("link")

	if link == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "link is required")
		return
	}

	request := models.DocxTemplateVariables{
		Data: make(map[string]any),
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid body data")
		return
	}

	additionalFields := make(map[string]any)
	for key, value := range request.Data {
		if strings.Contains(key, "_id") {
			additionalFields[key] = value
		}
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_TEMPLATE_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	objectRequest := models.CommonMessage{
		Data: map[string]any{
			"additional_fields": additionalFields,
		},
	}

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}
	respGetAll, err := services.GoObjectBuilderService().ObjectBuilder().GetAllForDocx(
		c.Request.Context(),
		&nb.CommonMessage{
			TableSlug: request.TableSlug,
			Data:      structData,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	mapV2, err := helper.ConvertStructToMap(respGetAll.Data)
	if err != nil {
		h.log.Error("error converting struct to map resp to respNew", logger.Error(err))
	}

	maps.Copy(mapV2, request.Data)

	structData, err = helper.ConvertMapToStruct(mapV2)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	res, err := services.DocGeneratorService().DocumentGenerator().GenerateDocument(c.Request.Context(), &doc_generator_service.GenerateDocumentRequest{
		Data: structData,
		Link: link,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	c.Header("Content-Disposition", "inline; filename=file.pdf")
	c.Header("Content-Type", "application/pdf")

	_, err = io.Copy(c.Writer, bytes.NewReader(res.PdfContent))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to send file")
		return
	}

	c.Status(http.StatusOK)
}

// ConvertHtmlToDocxOrPdf godoc
// @Security ApiKeyAuth
// @ID convert_html_to_docx_or_pdf
// @Router /v2/html/convert [POST]
// @Summary Convert HTML to DOCX or PDF
// @Description Convert HTML content to DOCX or PDF format
// @Tags Template
// @Accept json
// @Produce json
// @Param request body models.HtmlConvertRequest true "HTML Convert Request"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) ConvertHtmlToDocxOrPdf(c *gin.Context) {
	var request models.HtmlConvertRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if request.HtmlContent == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "html content is required")
		return
	}

	if request.OutputFormat != "docx" && request.OutputFormat != "pdf" && request.OutputFormat != "png" {
		h.HandleResponse(c, status_http.InvalidArgument, "format must be 'docx' or 'pdf'")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		resource.GetProjectId(),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// Convert HTML content to desired format
	convertResponse, err := services.DocGeneratorService().DocumentGenerator().ConvertHtml(c.Request.Context(), &doc_generator_service.ConvertHtmlRequest{
		Data:         []byte(request.HtmlContent),
		InputFormat:  "html",
		OutputFormat: request.OutputFormat,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if !convertResponse.Success {
		h.HandleResponse(c, status_http.BadRequest, convertResponse.ErrorMessage)
		return
	}

	// Set response headers based on output format
	fileName := fmt.Sprintf("file.%s", request.OutputFormat)

	switch request.OutputFormat {
	case "pdf":
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileName))
		c.Header("Content-Type", "application/pdf")
	case "docx":
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	}

	_, err = io.Copy(c.Writer, bytes.NewReader(convertResponse.Data))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to send file")
		return
	}

	c.Status(http.StatusOK)
}
