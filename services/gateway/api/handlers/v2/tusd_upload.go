package v2

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/aws/aws-sdk-go/aws"
	awscredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	tusd "github.com/tus/tusd/pkg/handler"
	"github.com/tus/tusd/pkg/s3store"
)

func (h *HandlerV2) MovieUpload(c *gin.Context) {
	projectID, ok := c.Get("project_id")
	projectIDString, isString := projectID.(string)
	if !ok || !isString || !util.IsValidUUID(projectIDString) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentID, ok := c.Get("environment_id")
	environmentIDString, isString := environmentID.(string)
	if !ok || !isString || !util.IsValidUUID(environmentIDString) {
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectIDString,
			EnvironmentId: environmentIDString,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	bucketName := resource.GetResourceEnvironmentId()
	if !util.IsValidUUID(bucketName) {
		h.HandleResponse(c, status_http.InternalServerError, "builder resource environment id is invalid")
		return
	}
	h.log.Info("tusd upload bucket resolved",
		logger.String("project_id", projectIDString),
		logger.String("environment_id", environmentIDString),
		logger.String("resource_environment_id", bucketName),
		logger.String("upload_id", c.Param("any")),
	)

	handler, err := h.movieUploadHandler(bucketName)
	if err != nil {
		h.log.Error("error while starting movie upload handler")
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	http.StripPrefix("/v2/upload-file/", handler).ServeHTTP(c.Writer, c.Request)
}

func (h *HandlerV2) movieUploadHandler(bucketName string) (*tusd.Handler, error) {
	h.tusdMu.Lock()
	defer h.tusdMu.Unlock()

	if handler, ok := h.tusdHandlers[bucketName]; ok {
		h.log.Info("tusd upload handler reused", logger.String("bucket", bucketName))
		return handler, nil
	}
	if h.tusdHandlers == nil {
		h.tusdHandlers = make(map[string]*tusd.Handler)
	}

	s3Config := aws.NewConfig().
		WithRegion("us-east-1").
		WithCredentials(awscredentials.NewStaticCredentials(
			h.baseConf.MinioAccessKeyID,
			h.baseConf.MinioSecretAccessKey,
			"")).
		WithEndpoint(h.baseConf.MinioEndpoint).
		WithS3ForcePathStyle(true)

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return nil, err
	}

	s3Client := s3.New(sess)
	s3Store := s3store.New(bucketName, s3Client)

	composer := tusd.NewStoreComposer()
	s3Store.UseIn(composer)

	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:                h.baseConf.HTTPBaseURL + "/v2/upload-file/",
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		NotifyUploadProgress:    true,
		NotifyTerminatedUploads: true,
		RespectForwardedHeaders: true,
	})

	if err != nil {
		return nil, err
	}

	h.tusdHandlers[bucketName] = handler
	h.log.Info("tusd upload handler created", logger.String("bucket", bucketName))
	go h.eventHandler(handler, bucketName, s3Client)
	return handler, nil
}

func (h *HandlerV2) eventHandler(handler *tusd.Handler, bucketName string, s3Client *s3.S3) {
	go func() {
		for event := range handler.CompleteUploads {
			objectKey := event.Upload.Storage["Key"]
			if objectKey == "" {
				h.log.Error("tusd upload completed without an object key", logger.String("bucket", bucketName), logger.String("upload_id", event.Upload.ID))
				continue
			}

			fileName := event.Upload.MetaData["filename"]
			contentType := uploadContentType(event.Upload.MetaData["filetype"], fileName)
			disposition := uploadContentDisposition(fileName)
			if err := setObjectContentType(context.Background(), s3Client, bucketName, objectKey, contentType, disposition); err != nil {
				h.log.Error("tusd upload content type update failed",
					logger.Error(err),
					logger.String("bucket", bucketName),
					logger.String("object_key", objectKey),
					logger.String("content_type", contentType),
				)
				continue
			}

			h.log.Info("tusd upload completed",
				logger.String("bucket", bucketName),
				logger.String("upload_id", event.Upload.ID),
				logger.String("object_key", objectKey),
				logger.String("content_type", contentType),
			)
		}
	}()
}

// uploadContentType resolves the object content type from the tus `filetype`
// metadata, falling back to the `filename` extension and then to a neutral type.
func uploadContentType(fileType, fileName string) string {
	if mediaType, ok := parseMediaType(fileType); ok {
		return mediaType
	}

	if ext := filepath.Ext(strings.TrimSpace(fileName)); ext != "" {
		if mediaType, ok := parseMediaType(mime.TypeByExtension(strings.ToLower(ext))); ok {
			return mediaType
		}
	}

	return "application/octet-stream"
}

// parseMediaType reports whether value is a well formed `type/subtype` media
// type and returns it without parameters.
func parseMediaType(value string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return "", false
	}

	mainType, subType, ok := strings.Cut(mediaType, "/")
	if !ok || mainType == "" || subType == "" {
		return "", false
	}

	return mediaType, true
}

// uploadContentDisposition keeps the object inline but restores the original
// file name for downloads, because the object key is a generated tusd id.
func uploadContentDisposition(fileName string) string {
	name := filepath.Base(strings.TrimSpace(strings.ReplaceAll(fileName, "\\", "/")))
	if name == "." || name == "/" || name == "" {
		return "inline"
	}

	disposition := mime.FormatMediaType("inline", map[string]string{"filename": name})
	if disposition == "" {
		return "inline"
	}

	return disposition
}

func setObjectContentType(ctx context.Context, s3Client *s3.S3, bucketName, objectKey, contentType, contentDisposition string) error {
	head, err := s3Client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return err
	}

	_, err = s3Client.CopyObjectWithContext(ctx, &s3.CopyObjectInput{
		Bucket:             aws.String(bucketName),
		Key:                aws.String(objectKey),
		CopySource:         aws.String(bucketName + "/" + url.PathEscape(objectKey)),
		MetadataDirective:  aws.String(s3.MetadataDirectiveReplace),
		Metadata:           head.Metadata,
		ContentType:        aws.String(contentType),
		ContentDisposition: aws.String(contentDisposition),
		CacheControl:       head.CacheControl,
		ContentEncoding:    head.ContentEncoding,
		ContentLanguage:    head.ContentLanguage,
	})
	return err
}

func (h *HandlerV2) Tusd() *tusd.Handler {
	ResourceEnvironmentId := "b8199457-9a0e-4260-bcca-75b7bc55c1f9"
	s3Config := aws.NewConfig().
		WithRegion("us-east-1").
		WithCredentials(awscredentials.NewStaticCredentials(
			h.baseConf.MinioAccessKeyID,
			h.baseConf.MinioSecretAccessKey,
			"")).
		WithEndpoint(h.baseConf.MinioEndpoint).
		WithS3ForcePathStyle(true)

	sess, err := session.NewSession(s3Config)
	if err != nil {
		h.log.Error("error while starting movie upload handler")
		return nil
	}
	s3Store := s3store.New(ResourceEnvironmentId, s3.New(sess))

	composer := tusd.NewStoreComposer()
	s3Store.UseIn(composer)

	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:                "/v1/tusd/",
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		NotifyUploadProgress:    true,
		NotifyTerminatedUploads: true,
		RespectForwardedHeaders: true,
	})

	if err != nil {
		h.log.Error("err while tusd new handler")
		return nil
	}

	go func() {
		for {
			select {
			case event := <-handler.UploadProgress:
				h.log.Info("tusd upload progress", logger.String("bucket", ResourceEnvironmentId), logger.String("upload_id", event.Upload.ID))
			case event := <-handler.CompleteUploads:
				h.log.Info("tusd upload completed", logger.String("bucket", ResourceEnvironmentId), logger.String("upload_id", event.Upload.ID))
			case event := <-handler.TerminatedUploads:
				h.log.Info("tusd upload terminated", logger.String("bucket", ResourceEnvironmentId), logger.String("upload_id", event.Upload.ID))
			}
		}
	}()

	return handler
}
