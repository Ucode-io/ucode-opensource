package v1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/billing"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/fileupload"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/HugoSmits86/nativewebp"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Upload godoc
// @ID create_file
// @Security ApiKeyAuth
// @Router /v1/files/folder_upload [POST]
// @Summary Upload Folder
// @Description Upload Folder
// @Tags Files
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Param folder_name query string true "folder_name"
// @Success 200 {object} status_http.Response{data=models.Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UploadToFolder(c *gin.Context) {
	var (
		file models.File
	)

	if err := c.ShouldBind(&file); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var (
		folderName = c.DefaultQuery("folder_name", "Media")
		rationStr  = c.Query("ratio")
		ratio      float64
	)

	if rationStr != "" {
		ratio, _ = strconv.ParseFloat(rationStr, 64)
		if ratio <= 0 {
			ratio = 0
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

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
		if err = billing.CheckAssetSizeLimit(c.Request.Context(), h.companyServices, services, projectId.(string), resource.ResourceEnvironmentId, file.File.Size); err != nil {
			if errors.Is(err, billing.ErrAssetLimitExceeded) {
				h.HandleResponse(c, status_http.PaymentRequired, models.PaymentAssetLimit)
			} else {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
			}
			return
		}
	}

	title := file.File.Filename
	fName, _ := uuid.NewRandom()
	cleanFileName := strings.ReplaceAll(title, " ", "")
	finalFilename := fmt.Sprintf("%s_%s", fName.String(), cleanFileName)
	contentType := file.File.Header.Get("Content-Type")

	object, err := file.File.Open()
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	defer object.Close()

	var uploadReader io.Reader = object
	var uploadSize int64 = file.File.Size

	isImage := strings.HasPrefix(contentType, "image/")
	isSVG := strings.Contains(contentType, "svg")

	if isImage && !isSVG && ratio > 0 {
		img, imgFormat, err := image.Decode(object)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, "decode error: "+err.Error())
			return
		}

		img, err = cropImageByRatio(img, ratio)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		var buf bytes.Buffer
		// Сохраняем в том же формате, в котором пришло
		switch imgFormat {
		case "jpeg", "jpg":
			err = jpeg.Encode(&buf, img, nil)
			contentType = "image/jpeg"
		case "webp":
			err = nativewebp.Encode(&buf, img, nil)
			contentType = "image/webp"
		default:
			err = png.Encode(&buf, img)
			contentType = "image/png"
			imgFormat = "png"
		}

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, "encode error: "+err.Error())
			return
		}

		uploadReader = bytes.NewReader(buf.Bytes())
		uploadSize = int64(buf.Len())

		ext := filepath.Ext(cleanFileName)
		baseName := strings.TrimSuffix(cleanFileName, ext)
		finalFilename = fmt.Sprintf("%s_%s.%s", fName.String(), baseName, imgFormat)
	}

	driveUpload, driveConfigured, err := fileupload.NewGoogleDriveUploader(h.companyServices.Resource(), fileupload.GoogleDriveConfig{
		ClientID:           h.baseConf.GoogleDriveClientID,
		ClientSecret:       h.baseConf.GoogleDriveClientSecret,
		RedirectURI:        h.baseConf.GoogleDriveRedirectURI,
		ServiceAccountJSON: h.baseConf.GoogleDriveServiceAccountJSON,
		ParentFolderID:     h.baseConf.GoogleDriveParentFolderID,
		Visibility:         h.baseConf.GoogleDriveVisibility,
	}).UploadIfConfigured(
		c.Request.Context(),
		fileupload.GoogleDriveUploadRequest{
			ProjectID:     projectId.(string),
			EnvironmentID: environmentId.(string),
			FolderName:    folderName,
			FileName:      finalFilename,
			ContentType:   contentType,
			Size:          uploadSize,
			Reader:        uploadReader,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if driveConfigured {
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Create(c.Request.Context(), &obs.CreateFileRequest{
				Id:               fName.String(),
				Title:            title,
				Storage:          driveUpload.Storage,
				StorageType:      fileupload.GoogleDriveStorageType,
				FileNameDisk:     driveUpload.FileNameDisk,
				FileNameDownload: title,
				Link:             driveUpload.Link,
				FileSize:         uploadSize,
				ProjectId:        resource.ResourceEnvironmentId,
			})
			if err != nil {
				h.HandleResponse(c, status_http.BadRequest, err.Error())
				return
			}
			h.HandleResponse(c, status_http.Created, resp)

		case pb.ResourceType_POSTGRESQL:
			resp, err := services.GoObjectBuilderService().File().Create(c.Request.Context(), &nb.CreateFileRequest{
				Id:               fName.String(),
				Title:            title,
				Storage:          driveUpload.Storage,
				StorageType:      fileupload.GoogleDriveStorageType,
				FileNameDisk:     driveUpload.FileNameDisk,
				FileNameDownload: title,
				Link:             driveUpload.Link,
				FileSize:         uploadSize,
				ProjectId:        resource.ResourceEnvironmentId,
			})
			if err != nil {
				h.HandleResponse(c, status_http.BadRequest, err.Error())
				return
			}
			h.HandleResponse(c, status_http.Created, resp)
		}
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

	_, err = minioClient.PutObject(
		c.Request.Context(),
		resource.ResourceEnvironmentId,
		folderName+"/"+finalFilename,
		uploadReader,
		uploadSize,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Create(c.Request.Context(), &obs.CreateFileRequest{
			Id:               fName.String(),
			Title:            title,
			Storage:          folderName,
			StorageType:      fileupload.MinioStorageType,
			FileNameDisk:     finalFilename,
			FileNameDownload: title,
			Link:             resource.ResourceEnvironmentId + "/" + folderName + "/" + finalFilename,
			FileSize:         uploadSize,
			ProjectId:        resource.ResourceEnvironmentId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		h.HandleResponse(c, status_http.Created, resp)

	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().Create(c.Request.Context(), &nb.CreateFileRequest{
			Id:               fName.String(),
			Title:            title,
			Storage:          folderName,
			StorageType:      fileupload.MinioStorageType,
			FileNameDisk:     finalFilename,
			FileNameDownload: title,
			Link:             resource.ResourceEnvironmentId + "/" + folderName + "/" + finalFilename,
			FileSize:         uploadSize,
			ProjectId:        resource.ResourceEnvironmentId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}
		h.HandleResponse(c, status_http.Created, resp)
	}
}

func cropImageByRatio(img image.Image, ratio float64) (image.Image, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	targetWidth := width
	targetHeight := int(float64(width) / ratio)

	if targetHeight > height {
		targetHeight = height
		targetWidth = int(float64(height) * ratio)
	}

	x0 := (width - targetWidth) / 2
	y0 := (height - targetHeight) / 2
	x1 := x0 + targetWidth
	y1 := y0 + targetHeight

	subImg, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	})
	if !ok {
		return nil, errors.New("image doesn't support cropping")
	}

	return subImg.SubImage(image.Rect(x0, y0, x1, y1)), nil
}

// GetSingleFile godoc
// @Security ApiKeyAuth
// @ID get_file_by_id
// @Router /v1/files/{id} [GET]
// @Summary Get single variable
// @Description Get single variable
// @Tags Files
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=obs.File} "FileBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleFile(c *gin.Context) {
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
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().GetSingle(
			c.Request.Context(), &obs.FilePrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        fileID,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().GetSingle(
			c.Request.Context(), &nb.FilePrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        fileID,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)

	}

}

// UpdateFile godoc
// @Security ApiKeyAuth
// @ID update_file
// @Router /v1/files [PUT]
// @Summary Update file
// @Description Update file
// @Tags Files
// @Accept json
// @Produce json
// @Param variable body models.UpdateFileRequest true "UpdateFileRequestBody"
// @Success 200 {object} status_http.Response{data=obs.File} "File data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateFile(c *gin.Context) {
	var file models.UpdateFileRequest

	if err := c.ShouldBindJSON(&file); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Update(
			c.Request.Context(), &obs.File{
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
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().Update(
			c.Request.Context(), &nb.File{
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

}

// DeleteFile godoc
// @Security ApiKeyAuth
// @ID delete_file
// @Router /v1/files/{id} [DELETE]
// @Summary Delete file
// @Description Delete file
// @Tags Files
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteFile(c *gin.Context) {
	var path = c.Param("id")
	if util.IsValidUUID(path) {
		h.HandleResponse(c, status_http.BadRequest, "id is empty")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(ctx,
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

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	err = minioClient.RemoveObject(ctx, resource.ResourceEnvironmentId, path, minio.RemoveObjectOptions{})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, "Successfully deleted")
}

// DeleteFiles godoc
// @Security ApiKeyAuth
// @ID delete_files
// @Router /v1/files [DELETE]
// @Summary Delete files
// @Description Delete files
// @Tags Files
// @Accept json
// @Produce json
// @Param file body models.FileDeleteRequest true "DeleteFilesRequestBody"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteFiles(c *gin.Context) {
	var (
		file models.FileDeleteRequest
		res  = obs.File{}
	)

	if err := c.ShouldBindJSON(&file); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().GetSingle(
			c.Request.Context(), &obs.FilePrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        file.Objects[0].ObjectId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		res.Id = resp.Id
		res.Title = resp.Title
		res.Description = resp.Description
		res.Tags = resp.Tags
		res.Storage = resp.Storage
		res.FileNameDisk = resp.FileNameDisk
		res.FileNameDownload = resp.FileNameDownload
		res.Link = resp.Link
		res.FileSize = resp.FileSize
		res.ProjectId = resp.ProjectId

	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().GetSingle(
			c.Request.Context(), &nb.FilePrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				Id:        file.Objects[0].ObjectId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		res.Id = resp.Id
		res.Title = resp.Title
		res.Description = resp.Description
		res.Tags = resp.Tags
		res.Storage = resp.Storage
		res.FileNameDisk = resp.FileNameDisk
		res.FileNameDownload = resp.FileNameDownload
		res.Link = resp.Link
		res.FileSize = resp.FileSize
		res.ProjectId = resp.ProjectId

	}

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var delete_request []string

	for _, val := range file.Objects {
		delete_request = append(delete_request, val.ObjectId)
		err = minioClient.RemoveObject(c.Request.Context(), resource.ResourceEnvironmentId, res.Storage+"/"+val.ObjectName, minio.RemoveObjectOptions{})
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().Delete(
			c.Request.Context(), &obs.FileDeleteRequest{
				Ids:       delete_request,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().Delete(
			c.Request.Context(), &nb.FileDeleteRequest{
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
}

// GetAllFiles godoc
// @Security ApiKeyAuth
// @ID get_file_list
// @Router /v1/files [GET]
// @Summary Get file list
// @Description Get file list
// @Tags Files
// @Accept json
// @Produce json
// @Param filters query obs.GetAllFilesRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllFilesRequest} "FileBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllFiles(c *gin.Context) {
	var (
		limit, _   = strconv.Atoi(c.DefaultQuery("limit", "10"))
		offset, _  = strconv.Atoi(c.DefaultQuery("offset", "0"))
		folderName = normalizeGoogleDriveFolderName(c.DefaultQuery("folder_name", ""))
	)
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().GetList(
			c.Request.Context(), &obs.GetAllFilesRequest{
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
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().File().GetList(
			c.Request.Context(), &nb.GetAllFilesRequest{
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
}

func normalizeGoogleDriveFolderName(folderName string) string {
	return fileupload.NormalizeGoogleDriveFolderName(folderName)
}

// WordTemplate godoc
// @Security ApiKeyAuth
// @ID word_template
// @Router /v1/files/word-template [POST]
// @Summary Word template
// @Description Word template
// @Tags Files
// @Accept json
// @Produce json
// @Param variable body models.CommonMessage true "WordTemplateRequestBody"
// @Success 200 {object} status_http.Response{data=obs.WordTemplateResponse} "File data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) WordTemplate(c *gin.Context) {
	var file models.CommonMessage

	if err := c.ShouldBindJSON(&file); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(file.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).File().WordTemplate(
			c.Request.Context(), &obs.CommonMessage{
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}

}

// Upload godoc
// @ID upload_image
// @Security ApiKeyAuth
// @Param from-chat query string false "from-chat"
// @Router /v1/upload [POST]
// @Summary Upload
// @Description Upload
// @Tags file
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Success 200 {object} status_http.Response{data=models.Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) Upload(c *gin.Context) {
	var (
		file models.File
	)
	err := c.ShouldBind(&file)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	// fetch project/environment and service resource for actions
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
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

	_, err = minioClient.FPutObject(
		c.Request.Context(),
		resource.ResourceEnvironmentId,
		file.File.Filename,
		dst+"/"+file.File.Filename,
		minio.PutObjectOptions{ContentType: file.File.Header["Content-Type"][0]},
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

	h.HandleResponse(c, status_http.Created, models.Path{
		Filename: file.File.Filename,
		Hash:     fName.String(),
	})
}

// UploadFile godoc
// @Security ApiKeyAuth
// @ID upload_file
// @Router /v1/upload-file/{collection}/{object_id} [POST]
// @Summary Upload file
// @Description Upload file
// @Tags file
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "file"
// @Param collection path string true "collection"
// @Param object_id path string true "object_id"
// @Param tags query string false "tags"
// @Success 200 {object} status_http.Response{data=models.Path} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UploadFile(c *gin.Context) {
	var file models.File

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType == pb.ResourceType_POSTGRESQL {
		if err = billing.CheckAssetSizeLimit(c.Request.Context(), h.companyServices, services, projectId.(string), resource.ResourceEnvironmentId, file.File.Size); err != nil {
			if errors.Is(err, billing.ErrAssetLimitExceeded) {
				h.HandleResponse(c, status_http.PaymentRequired, models.PaymentAssetLimit)
			} else {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
			}
			return
		}
	}

	ContentTypeOfFile := file.File.Header.Get("Content-Type")
	object, err := file.File.Open()
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	defer object.Close()

	driveUpload, driveConfigured, err := fileupload.NewGoogleDriveUploader(h.companyServices.Resource(), fileupload.GoogleDriveConfig{
		ClientID:           h.baseConf.GoogleDriveClientID,
		ClientSecret:       h.baseConf.GoogleDriveClientSecret,
		RedirectURI:        h.baseConf.GoogleDriveRedirectURI,
		ServiceAccountJSON: h.baseConf.GoogleDriveServiceAccountJSON,
		ParentFolderID:     h.baseConf.GoogleDriveParentFolderID,
		Visibility:         h.baseConf.GoogleDriveVisibility,
	}).UploadIfConfigured(
		c.Request.Context(),
		fileupload.GoogleDriveUploadRequest{
			ProjectID:     projectId.(string),
			EnvironmentID: environmentId.(string),
			FileName:      file.File.Filename,
			ContentType:   ContentTypeOfFile,
			Size:          file.File.Size,
			Reader:        object,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if driveConfigured {
		splitedFileName := strings.Split(fileNameForObjectBuilder, ".")
		var tags []string
		if c.Query("tags") != "" {
			tags = strings.Split(c.Query("tags"), ",")
		}

		requestMap := make(map[string]any)
		requestMap["table_slug"] = c.Param("collection")
		requestMap["object_id"] = c.Param("object_id")
		requestMap["date"] = time.Now().Format(time.RFC3339)
		requestMap["tags"] = tags
		requestMap["size"] = int32(file.File.Size)
		requestMap["type"] = splitedFileName[len(splitedFileName)-1]
		requestMap["file_link"] = driveUpload.Link
		requestMap["name"] = fileNameForObjectBuilder
		structData, err := helper.ConvertMapToStruct(requestMap)
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		_, err = services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().Create(
			c.Request.Context(),
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

		h.HandleResponse(c, status_http.Created, models.Path{
			Filename: file.File.Filename,
			Hash:     fName.String(),
		})
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
	requestMap["object_id"] = c.Param("object_id")
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

	h.HandleResponse(c, status_http.Created, models.Path{
		Filename: file.File.Filename,
		Hash:     fName.String(),
	})
}
