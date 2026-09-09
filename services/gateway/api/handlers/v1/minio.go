package v1

import (
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cast"
)

type BucketRequest struct {
	Name string `json:"bucket_name"`
}

type BucketResponse struct {
	Size float64 `json:"size"`
}

// BucketSize godoc
// @ID bucket_size
// @Security ApiKeyAuth
// @Router /v1/minio/bucket-size [POST]
// @Summary Get Bucket size
// @Description Provide a bucket name, retrieve the total size of all files in the bucket in megabytes
// @Tags Minio
// @Accept json
// @Produce json
// @Param object body BucketRequest true "BucketSizeRequestBody"
// @Success 200 {object} status_http.Response{data=BucketResponse} "Path"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) BucketSize(c *gin.Context) {
	var bucket BucketRequest

	if err := c.ShouldBind(&bucket); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var (
		bucketName    = bucket.Name
		minioEndpoint = h.baseConf.MinioEndpoint
		minioAccess   = h.baseConf.MinioAccessKeyID
		minioSecret   = h.baseConf.MinioSecretAccessKey
		useSSL        = h.baseConf.MinioProtocol

		totalSize int64
	)

	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccess, minioSecret, ""),
		Secure: useSSL,
	})
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	objectCh := minioClient.ListObjects(c.Request.Context(), bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			h.log.Error("Error listing objects")
			return
		}
		totalSize += object.Size
	}

	totalSizeMB := float64(totalSize) / (1024 * 1024)
	h.HandleResponse(c, status_http.OK, BucketResponse{Size: cast.ToFloat64(fmt.Sprintf("%.2f", totalSizeMB))})
}
