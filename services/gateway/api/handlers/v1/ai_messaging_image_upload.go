package v1

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/fileupload"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"golang.org/x/sync/errgroup"
)

const imageDownloadTimeout = 20 * time.Second

// uploadImagePool downloads all Unsplash photos and uploads them to the project's MinIO bucket.
// Returns a rebuilt ImagePoolResult with ucode CDN URLs. Never fatal — falls back to Unsplash on error.
func (p *ChatProcessor) uploadImagePool(ctx context.Context, resourceEnvId string, pool helper.ImagePoolResult) helper.ImagePoolResult {
	if pool.Err != nil || len(pool.Photos) == 0 || resourceEnvId == "" {
		return pool
	}

	// folderName := helper.SanitizeFolderName(pool.ProjectName)
	folderName := "Media"

	minioClient, err := minio.New(p.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(p.baseConf.MinioAccessKeyID, p.baseConf.MinioSecretAccessKey, ""),
		Secure: p.baseConf.MinioProtocol,
	})
	if err != nil {
		log.Printf("[cdn-upload] minio client error: %v", err)
		return pool
	}

	// Create MINIO_FOLDER menu entry — same logic as HandlerV3.CreateMenu PostgreSQL path
	// attrs, _ := helperFunc.ConvertMapToStruct(map[string]any{
	// 	"label_ru": folderName,
	// 	"label_en": folderName,
	// })
	// _, menuErr := p.service.GoObjectBuilderService().Menu().Create(ctx, &nb.CreateMenuRequest{
	// 	Label:      folderName,
	// 	Icon:       "",
	// 	ParentId:   config.MainFolderID,
	// 	Type:       "MINIO_FOLDER",
	// 	ProjectId:  resourceEnvId,
	// 	NewRouter:  true,
	// 	Attributes: attrs,
	// })
	// if menuErr != nil {
	// 	log.Printf("[cdn-upload] MINIO_FOLDER menu warning (continuing): %v", menuErr)
	// }

	cdnURLs := make([]string, len(pool.Photos))
	for i, ph := range pool.Photos {
		cdnURLs[i] = ph.URLHero // default fallback: keep Unsplash URL
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(6)

	for i, photo := range pool.Photos {
		g.Go(func() error {
			imgBytes, contentType, err := downloadImage(photo.URLHero)
			if err != nil {
				log.Printf("[cdn-upload] photo %d download failed: %v", i, err)
				return nil
			}

			fileID := uuid.NewString()
			filename := fmt.Sprintf("%s_img_%02d.jpg", fileID, i)
			objectPath := folderName + "/" + filename

			// Upload to MinIO — same as HandlerV1.UploadToFolder
			_, err = minioClient.PutObject(
				gCtx,
				resourceEnvId,
				objectPath,
				bytes.NewReader(imgBytes),
				int64(len(imgBytes)),
				minio.PutObjectOptions{ContentType: contentType},
			)
			if err != nil {
				log.Printf("[cdn-upload] photo %d minio upload failed: %v", i, err)
				return nil
			}

			// Register in files table — same as HandlerV1.UploadToFolder PostgreSQL path
			link := resourceEnvId + "/" + objectPath
			_, _ = p.service.GoObjectBuilderService().File().Create(gCtx, &nb.CreateFileRequest{
				Id:               fileID,
				Title:            filename,
				Storage:          folderName,
				StorageType:      fileupload.MinioStorageType,
				FileNameDisk:     filename,
				FileNameDownload: filename,
				Link:             link,
				FileSize:         int64(len(imgBytes)),
				ProjectId:        resourceEnvId,
			})

			cdnURLs[i] = "https://" + p.baseConf.MinioEndpoint + "/" + link
			log.Printf("[cdn-upload] photo %d → %s", i, cdnURLs[i])
			return nil
		})
	}
	g.Wait()

	// Rebuild photos with CDN URLs (single URL for all sizes — CDN has no resize transform)
	newPhotos := make([]helper.UnsplashPhoto, len(pool.Photos))
	for i, ph := range pool.Photos {
		url := cdnURLs[i]
		newPhotos[i] = helper.UnsplashPhoto{
			URLHero:      url,
			URLCard:      url,
			URLThumb:     url,
			Photographer: ph.Photographer,
		}
	}

	block, thumbURLs := helper.BuildPoolBlock(pool.ProjectName, pool.Keywords, newPhotos)

	log.Printf("[cdn-upload] done: %d photos in bucket=%s folder=%s", len(newPhotos), resourceEnvId, folderName)

	return helper.ImagePoolResult{
		Block:       block,
		Keywords:    pool.Keywords,
		Count:       len(newPhotos),
		ThumbURLs:   thumbURLs,
		Photos:      newPhotos,
		ProjectName: pool.ProjectName,
	}
}

func downloadImage(imageURL string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imageDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	return data, ct, nil
}
