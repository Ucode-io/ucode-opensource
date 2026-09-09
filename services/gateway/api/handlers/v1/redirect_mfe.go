package v1

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"strings"
	"time"

	cs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

	"github.com/gin-gonic/gin"
)

const (
	mfeShortLinkRedisPrefix = "mfe:s:"
	mfeShortLinkRedisTTL    = 30 * 24 * time.Hour
	mfeSlugLength           = 6
	mfeSlugAlphabet         = "abcdefghijklmnopqrstuvwxyz0123456789"
)

func mfeShortURL(base, slug string) string {
	return strings.TrimRight(base, "/") + "/p/" + slug
}

func normalizeURL(u string) string {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return "https://" + u
}

// generateSlug returns a random mfeSlugLength-character base36 string.
// Uses crypto/rand; collisions handled by the caller via retry.
func generateSlug() (string, error) {
	b := make([]byte, mfeSlugLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = mfeSlugAlphabet[b[i]%byte(len(mfeSlugAlphabet))]
	}
	return string(b), nil
}

func (h *HandlerV1) RedirectShortURL(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()

	if h.centralRedis != nil {
		if url, err := h.centralRedis.Get(ctx, mfeShortLinkRedisPrefix+slug).Result(); err == nil {
			c.Redirect(http.StatusMovedPermanently, normalizeURL(url))
			return
		}
	}

	link, err := h.companyServices.MfeShortLink().GetBySlug(ctx, &cs.MfeShortLinkSlugReq{Slug: slug})
	if err != nil {
		log.Printf("[SHORT_LINK] GetBySlug slug=%s err=%v", slug, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "short link not found"})
		return
	}
	if link.GetUrl() == "" {
		log.Printf("[SHORT_LINK] GetBySlug slug=%s returned empty url", slug)
		c.JSON(http.StatusNotFound, gin.H{"error": "short link not found"})
		return
	}

	targetURL := normalizeURL(link.GetUrl())

	if h.centralRedis != nil {
		go func() {
			_ = h.centralRedis.Set(context.Background(), mfeShortLinkRedisPrefix+slug, targetURL, mfeShortLinkRedisTTL).Err()
		}()
	}

	c.Redirect(http.StatusMovedPermanently, targetURL)
}
