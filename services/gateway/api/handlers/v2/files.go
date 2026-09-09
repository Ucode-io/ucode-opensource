package v2

import (
	"context"
	"errors"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"log"
	"mime/multipart"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"github.com/gin-gonic/gin"

	"github.com/minio/minio-go/v7"
)

// minioStorageType marks files stored in the built-in MinIO bucket.
const minioStorageType = "minio"

type UploadResponse struct {
	Filename string `json:"filename"`
}

type File struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
}

type Path struct {
	Filename string `json:"filename"`
	Hash     string `json:"hash"`
}

// Upload godoc
// @ID v2_upload_image
// @Security ApiKeyAuth
// @Param from-chat query string false "from-chat"
// @Router /v2/files/import [POST]
// @Summary Upload
// @Description Upload
// @Tags file
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Success 200 {object} status_http.Response{data=Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) Upload(c *gin.Context) {
	var (
		file          File
		defaultBucket = "ucode"
	)
	err := c.ShouldBind(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	fName, _ := uuid.NewRandom()
	file.File.Filename = strings.ReplaceAll(file.File.Filename, " ", "")
	file.File.Filename = fmt.Sprintf("%s_%s", fName.String(), file.File.Filename)
	dst, _ := os.Getwd()

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: h.baseConf.MinioProtocol,
	})

	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	err = c.SaveUploadedFile(file.File, dst+"/"+file.File.Filename)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	splitedContentType := strings.Split(file.File.Header["Content-Type"][0], "/")
	if splitedContentType[0] != "image" && splitedContentType[0] != "video" {
		defaultBucket = "docs"
	}

	if c.Query("from-chat") == "to_telegram_bot" {
		defaultBucket = "telegram"
	}

	_, err = minioClient.FPutObject(
		context.Background(),
		defaultBucket,
		file.File.Filename,
		dst+"/"+file.File.Filename,
		minio.PutObjectOptions{
			ContentType: file.File.Header["Content-Type"][0]},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		err = os.Remove(dst + "/" + file.File.Filename)
		if err != nil {
			h.log.Error("cant remove file")
		}
		return
	}

	err = os.Remove(dst + "/" + file.File.Filename)
	if err != nil {
		h.log.Error("cant remove file")
	}

	h.HandleResponse(c, status_http.Created, Path{
		Filename: file.File.Filename,
		Hash:     fName.String(),
	})
}

// UploadFile godoc
// @Security ApiKeyAuth
// @ID v2_upload_file
// @Router /v2/upload-file/{collection}/{id} [POST]
// @Summary Upload file
// @Description Upload file
// @Tags file
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Param tags query string false "tags"
// @Success 200 {object} status_http.Response{data=Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UploadFile(c *gin.Context) {
	var file File
	err := c.ShouldBind(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	fileNameForObjectBuilder := file.File.Filename

	fName, _ := uuid.NewRandom()
	file.File.Filename = strings.ReplaceAll(file.File.Filename, " ", "")
	file.File.Filename = fmt.Sprintf("%s_%s", fName.String(), file.File.Filename)
	dst, _ := os.Getwd()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
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

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
	}

	ContentTypeOfFile := file.File.Header.Get("Content-Type")
	object, err := file.File.Open()
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	defer object.Close()

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: h.baseConf.MinioProtocol,
	})
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	err = c.SaveUploadedFile(file.File, dst+"/"+file.File.Filename)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	fileLink := "https://" + h.baseConf.MinioEndpoint + "/docs/" + file.File.Filename
	splitedFileName := strings.Split(fileNameForObjectBuilder, ".")
	f, err := os.Stat(dst + "/" + file.File.Filename)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	_, err = minioClient.FPutObject(
		context.Background(),
		"docs",
		file.File.Filename,
		dst+"/"+file.File.Filename,
		minio.PutObjectOptions{ContentType: ContentTypeOfFile},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		err = os.Remove(dst + "/" + file.File.Filename)
		if err != nil {
			h.log.Error("cant remove file")
		}
		return
	}
	err = os.Remove(dst + "/" + file.File.Filename)
	if err != nil {
		h.log.Error("cant remove file")
	}

	var tags []string
	if c.Query("tags") != "" {
		tags = strings.Split(c.Query("tags"), ",")
	}
	var requestMap = make(map[string]any)
	requestMap["table_slug"] = c.Param("collection")
	requestMap["object_id"] = c.Param("id")
	requestMap["date"] = time.Now().Format(time.RFC3339)
	requestMap["tags"] = tags
	requestMap["size"] = int32(f.Size())
	requestMap["type"] = splitedFileName[len(splitedFileName)-1]
	requestMap["file_link"] = fileLink
	requestMap["name"] = fileNameForObjectBuilder
	structData, err := helper.ConvertMapToStruct(requestMap)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	_, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
		context.Background(),
		&obs.CommonMessage{
			TableSlug: "file",
			Data:      structData,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, Path{
		Filename: file.File.Filename,
		Hash:     fName.String(),
	})
}

// Upload godoc
// @ID v2_create_file
// @Security ApiKeyAuth
// @Router /v2/files [POST]
// @Summary Upload Folder
// @Description Upload Folder
// @Tags V2Files
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Param folder_name query string true "folder_name"
// @Success 200 {object} status_http.Response{data=Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UploadToFolder(c *gin.Context) {
	var (
		file File
	)

	if file.File != nil {
		h.HandleResponse(c, status_http.BadRequest, "file is empty")
		return
	}

	folder_name := c.DefaultQuery("folder_name", "")

	err := c.ShouldBind(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
	}

	var title string = file.File.Filename

	fName, _ := uuid.NewRandom()
	file.File.Filename = strings.ReplaceAll(file.File.Filename, " ", "")
	file.File.Filename = fmt.Sprintf("%s_%s", fName.String(), file.File.Filename)
	object, err := file.File.Open()
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	defer object.Close()

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: h.baseConf.MinioProtocol,
	})

	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	_, err = minioClient.PutObject(
		context.Background(),
		resource.ResourceEnvironmentId,
		folder_name+"/"+file.File.Filename,
		object,
		file.File.Size,
		minio.PutObjectOptions{ContentType: file.File.Header["Content-Type"][0]},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		// err = os.Remove(dst + "/" + file.File.Filename)
		// if err != nil {
		// 	h.log.Error("cant remove file")
		// }
		return
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Create(context.Background(), &obs.CreateFileRequest{
		Id:               fName.String(),
		Title:            title,
		Storage:          folder_name,
		StorageType:      minioStorageType,
		FileNameDisk:     file.File.Filename,
		FileNameDownload: title,
		Link:             resource.ResourceEnvironmentId + "/" + folder_name + "/" + file.File.Filename,
		FileSize:         file.File.Size,
		ProjectId:        resource.ResourceEnvironmentId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	// err = os.Remove(dst + "/" + file.File.Filename)
	// if err != nil {
	// 	h.log.Error("cant remove file")
	// }

	h.HandleResponse(c, status_http.Created, resp)
}

// GetSingleFile godoc
// @Security ApiKeyAuth
// @ID v2_get_file_by_id
// @Router /v2/files/{id} [GET]
// @Summary Get single variable
// @Description Get single variable
// @Tags V2Files
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=obs.File} "FileBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetSingleFile(c *gin.Context) {
	fileID := c.Param("id")

	if !util.IsValidUUID(fileID) {
		h.HandleResponse(c, status_http.InvalidArgument, "variable id is an invalid uuid")
		return
	}

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

	resourse, err := h.companyServices.ServiceResource().GetSingle(
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
		projectId.(string),
		resourse.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GetBuilderServiceByType(resourse.NodeType).File().GetSingle(
		context.Background(),
		&obs.FilePrimaryKey{
			Id:        fileID,
			ProjectId: resourse.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateFile godoc
// @Security ApiKeyAuth
// @ID v2_update_file
// @Router /v2/files/{id}/upload [PUT]
// @Summary Update file
// @Description Update file
// @Tags V2Files
// @Accept json
// @Produce json
// @Param variable body models.UpdateFileRequest true "UpdateFileRequestBody"
// @Success 200 {object} status_http.Response{data=obs.File} "File data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateFile(c *gin.Context) {
	var file models.UpdateFileRequest

	if c.Param("id") != "" {
		file.Id = c.Param("id")
	}

	err := c.ShouldBindJSON(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Update(
		context.Background(),
		&obs.File{
			Id:               file.Id,
			Title:            file.Title,
			Description:      file.Description,
			Tags:             file.Tags,
			FileNameDownload: file.FileNameDownload,
			ProjectId:        resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteFile godoc
// @Security ApiKeyAuth
// @ID v2_delete_file
// @Router /v2/files/{id}/upload [DELETE]
// @Summary Delete file
// @Description Delete file
// @Tags V2Files
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteFile(c *gin.Context) {
	id := c.Param("id")

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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GetBuilderServiceByType(resource.NodeType).File().GetSingle(
		context.Background(),
		&obs.FilePrimaryKey{
			Id:        id,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Println(err)
	}

	ctx := context.Background()

	var delete_request []string

	delete_request = append(delete_request, id)
	err = minioClient.RemoveObject(ctx, resource.ResourceEnvironmentId, res.Storage+"/"+res.FileNameDisk, minio.RemoveObjectOptions{})
	if err != nil {
		log.Println(err)
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Delete(
		context.Background(),
		&obs.FileDeleteRequest{
			Ids:       delete_request,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, resp)
}

// DeleteFiles godoc
// @Security ApiKeyAuth
// @ID v2_delete_files
// @Router /v2/files [DELETE]
// @Summary Delete files
// @Description Delete files
// @Tags V2Files
// @Accept json
// @Produce json
// @Param file body models.FileDeleteRequest true "DeleteFilesRequestBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteFiles(c *gin.Context) {

	var file models.FileDeleteRequest

	err := c.ShouldBindJSON(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	res, err := services.GetBuilderServiceByType(resource.NodeType).File().GetSingle(
		context.Background(),
		&obs.FilePrimaryKey{
			Id:        file.Objects[0].ObjectId,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		log.Println(err)
	}

	ctx := context.Background()

	var delete_request []string

	for _, val := range file.Objects {
		delete_request = append(delete_request, val.ObjectId)
		err = minioClient.RemoveObject(ctx, resource.ResourceEnvironmentId, res.Storage+"/"+val.ObjectName, minio.RemoveObjectOptions{})
		if err != nil {
			log.Println(err)
		}
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Delete(
		context.Background(),
		&obs.FileDeleteRequest{
			Ids:       delete_request,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, resp)
}

// GetAllFiles godoc
// @Security ApiKeyAuth
// @ID v2_get_file_list
// @Router /v2/files [GET]
// @Summary Get file list
// @Description Get file list
// @Tags V2Files
// @Accept json
// @Produce json
// @Param filters query obs.GetAllFilesRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllFilesRequest} "FileBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllFiles(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	folderName := normalizeGoogleDriveFolderName(c.DefaultQuery("folder_name", ""))

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
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
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

	resp, err := services.GetBuilderServiceByType(resource.NodeType).File().GetList(
		context.Background(),
		&obs.GetAllFilesRequest{
			Search:     c.DefaultQuery("search", ""),
			Sort:       c.DefaultQuery("sort", ""),
			ProjectId:  resource.ResourceEnvironmentId,
			FolderName: folderName,
			Limit:      int32(limit),
			Offset:     int32(offset),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func normalizeGoogleDriveFolderName(folderName string) string {
	return strings.TrimSpace(folderName)
}
