package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const (
	unsplashBase    = "https://api.unsplash.com"
	unsplashTimeout = 10 * time.Second
	imgCacheMax     = 200
)

var unsplashClient = &http.Client{Timeout: unsplashTimeout}

type UnsplashPhoto struct {
	URLHero      string // 1600×900
	URLCard      string // 800×600
	URLThumb     string // 400×300
	Photographer string
}

var (
	imgCacheMu  sync.Mutex
	imgCacheMap = make(map[string][]UnsplashPhoto, imgCacheMax)
)

func imgCacheKey(keywords []string) string {
	sorted := make([]string, len(keywords))
	copy(sorted, keywords)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}

func imgixURL(raw string, w, h int) string {
	sep := "&"
	if !strings.Contains(raw, "?") {
		sep = "?"
	}
	return fmt.Sprintf("%s%sw=%d&h=%d&fit=crop&auto=format&q=80", raw, sep, w, h)
}

func extractKeywords(plan *models.ArchitectPlan) []string {
	kw := plan.ImageKeywords
	if len(kw) > 4 {
		kw = kw[:4]
	}
	return kw
}

func searchUnsplash(ctx context.Context, accessKey, query string, count int) ([]UnsplashPhoto, error) {
	if count > 30 {
		count = 30
	}
	params := url.Values{
		"query":          {query},
		"per_page":       {fmt.Sprintf("%d", count)},
		"orientation":    {"landscape"},
		"content_filter": {"high"},
		"order_by":       {"relevant"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		unsplashBase+"/search/photos?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Client-ID "+accessKey)

	resp, err := unsplashClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unsplash %d: %s", resp.StatusCode, string(b))
	}

	var data struct {
		Results []struct {
			URLs struct {
				Raw string `json:"raw"`
			} `json:"urls"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	out := make([]UnsplashPhoto, 0, len(data.Results))
	for _, p := range data.Results {
		out = append(out, UnsplashPhoto{
			URLHero:      imgixURL(p.URLs.Raw, 1600, 900),
			URLCard:      imgixURL(p.URLs.Raw, 800, 600),
			URLThumb:     imgixURL(p.URLs.Raw, 400, 300),
			Photographer: p.User.Name,
		})
	}
	log.Printf("[unsplash] search %q → %d photos", query, len(out))
	return out, nil
}

func fetchPhotos(ctx context.Context, accessKey string, keywords []string) ([]UnsplashPhoto, error) {
	key := imgCacheKey(keywords)

	imgCacheMu.Lock()
	if cached, ok := imgCacheMap[key]; ok {
		imgCacheMu.Unlock()
		return cached, nil
	}
	imgCacheMu.Unlock()

	const needed = 12
	var allResults [][]UnsplashPhoto

	// Fetch up to 15 photos for EACH keyword separately to ensure high quality matches
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		photos, err := searchUnsplash(ctx, accessKey, kw, 15)
		if err == nil && len(photos) > 0 {
			allResults = append(allResults, photos)
		}
	}

	var photos []UnsplashPhoto
	seen := make(map[string]bool)

	// Round-robin selection to mix diverse concepts
	for i := 0; i < 15; i++ {
		for _, group := range allResults {
			if i < len(group) {
				p := group[i]
				if !seen[p.URLHero] {
					seen[p.URLHero] = true
					photos = append(photos, p)
					if len(photos) == needed {
						break
					}
				}
			}
		}
		if len(photos) == needed {
			break
		}
	}

	// Fallback if absolutely no photos were found
	if len(photos) == 0 {
		return nil, fmt.Errorf("0 results for %v", keywords)
	}

	imgCacheMu.Lock()
	if len(imgCacheMap) >= imgCacheMax {
		imgCacheMap = make(map[string][]UnsplashPhoto, imgCacheMax)
	}
	imgCacheMap[key] = photos
	imgCacheMu.Unlock()

	return photos, nil
}

type ImagePoolResult struct {
	Block       string          // formatted prompt block to append to apiConfig; empty on failure
	Keywords    []string        // search terms that were used
	Count       int             // number of photos fetched
	ThumbURLs   []string        // first 6 thumb URLs for mock_data image injection
	Photos      []UnsplashPhoto // raw photos list — used by uploadImagePool for CDN re-upload
	ProjectName string          // project name — used when rebuilding block after CDN upload
	Err         error           // non-nil when the API call failed; generation continues either way
}

// BuildPoolBlock rebuilds the IMAGE POOL prompt block and thumb URL list from a photo slice.
func BuildPoolBlock(projectName string, keywords []string, photos []UnsplashPhoto) (block string, thumbURLs []string) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "IMAGE POOL — USE THESE EXACT URLs\n")
	fmt.Fprintf(&sb, "════════════════════════════════════════\n")
	fmt.Fprintf(&sb, "Pre-fetched for \"%s\" · query: %s\n", projectName, strings.Join(keywords, " "))
	fmt.Fprintf(&sb, "NEVER invent photo IDs. NEVER use placeholder.com / picsum.photos.\n\n")

	fmt.Fprintf(&sb, "HERO (1600×900) — hero sections, full-bleed banners:\n")
	for i, p := range photos {
		if i >= 3 {
			break
		}
		fmt.Fprintf(&sb, "  • %s", p.URLHero)
		if p.Photographer != "" {
			fmt.Fprintf(&sb, "  [%s]", p.Photographer)
		}
		fmt.Fprintf(&sb, "\n")
	}

	fmt.Fprintf(&sb, "\nCARD (800×600) — feature cards, section images:\n")
	for i, p := range photos {
		if i < 3 {
			continue
		}
		fmt.Fprintf(&sb, "  • %s\n", p.URLCard)
	}

	fmt.Fprintf(&sb, "\nTHUMB (400×300) — table rows, small cards, list images:\n")
	for i, p := range photos {
		if i >= 6 {
			break
		}
		fmt.Fprintf(&sb, "  • %s\n", p.URLThumb)
	}

	fmt.Fprintf(&sb, "════════════════════════════════════════\n")

	thumbURLs = make([]string, 0, 6)
	for i, ph := range photos {
		if i >= 6 {
			break
		}
		thumbURLs = append(thumbURLs, ph.URLThumb)
	}
	return sb.String(), thumbURLs
}

func FetchImagePool(ctx context.Context, accessKey string, plan *models.ArchitectPlan) ImagePoolResult {
	keywords := extractKeywords(plan)
	log.Printf("[unsplash] project=%q keywords=%v", plan.ProjectName, keywords)

	if len(keywords) == 0 {
		return ImagePoolResult{Err: fmt.Errorf("no keywords extracted from plan")}
	}

	photos, err := fetchPhotos(ctx, accessKey, keywords)
	if err != nil {
		log.Printf("[unsplash] API error: %v", err)
		return ImagePoolResult{Keywords: keywords, Err: err}
	}
	if len(photos) == 0 {
		return ImagePoolResult{Keywords: keywords, Err: fmt.Errorf("0 results for %v", keywords)}
	}

	log.Printf("[unsplash] ✅ %d photos fetched", len(photos))

	block, thumbURLs := BuildPoolBlock(plan.ProjectName, keywords, photos)

	return ImagePoolResult{
		Block:       block,
		Keywords:    keywords,
		Count:       len(photos),
		ThumbURLs:   thumbURLs,
		Photos:      photos,
		ProjectName: plan.ProjectName,
	}
}
