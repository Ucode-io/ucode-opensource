package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmlstd "html"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"golang.org/x/net/html"
)

const referenceContextMarker = "REFERENCE SITE CONTEXT - AUTHORITATIVE"

var (
	httpReferenceURLRe    = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"'\]\)]+`)
	bareReferenceURLRe    = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}(?:/[^\s<>"'\]\)]+)?`)
	cssColorRe            = regexp.MustCompile(`(?i)(#[0-9a-f]{3,8}\b|rgba?\([^)]+\)|hsla?\([^)]+\))`)
	fontFamilyRe          = regexp.MustCompile(`(?is)font-family\s*:\s*([^;}{]+)`)
	googleFontFamilyRe    = regexp.MustCompile(`(?i)[?&]family=([^&:]+)`)
	referenceWhitespaceRe = regexp.MustCompile(`\s+`)
)

var fetchReferenceSiteHTMLForPrompt = fetchReferenceSiteHTML

type referenceCaptureViewport struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type referenceCaptureRequest struct {
	URL       string                     `json:"url"`
	Viewports []referenceCaptureViewport `json:"viewports"`
	Extract   []string                   `json:"extract"`
	// FullPage asks the render service for full-page screenshots instead of a
	// viewport-height clip — a clone needs the below-the-fold sections too.
	FullPage bool `json:"full_page"`
}

func prepareReferencePrompt(ctx context.Context, conf config.BaseConfig, prompt string, imageURLs []string) (string, []string, *models.ReferenceSiteContext, string) {
	if strings.Contains(prompt, referenceContextMarker) {
		return prompt, imageURLs, nil, ""
	}

	rawURLs := extractReferenceURLs(prompt)
	if len(rawURLs) == 0 || !hasReferenceCloneIntent(prompt) {
		return prompt, imageURLs, nil, ""
	}
	rawURLs = reduceSameHostReferenceURLs(rawURLs)
	if len(rawURLs) > 1 {
		return prompt, imageURLs, nil, "I found multiple website links in your prompt. Please send one exact public URL to clone so I can reproduce the right design."
	}

	targetURL, err := normalizeReferenceURL(rawURLs[0])
	if err != nil {
		log.Printf("[reference-capture] blocked reference URL %q: %v", rawURLs[0], err)
		return prompt, imageURLs, nil, "I could not use that website URL as a visual reference. Please send one public http(s) website URL that is reachable from the internet."
	}

	ref, err := getReferenceSiteContext(ctx, conf, targetURL)
	if err != nil {
		log.Printf("[reference] failed url=%s: %v", targetURL, err)
		return prompt, imageURLs, nil, fmt.Sprintf("I could not access %s to capture its design. Please send a reachable public URL, or try again after the site allows access.", targetURL)
	}

	screenshotURLs := referenceScreenshotURLs(ref)
	if ref.URL == "" {
		ref.URL = targetURL
	}
	if !promptWantsSingleLanding(prompt) {
		crawlReferenceSubpages(ctx, ref, prompt)
	}
	log.Printf("[reference] captured url=%s final=%s screenshots=%d colors=%d fonts=%d sections=%d assets=%d subpages=%d warnings=%d",
		targetURL, ref.FinalURL, len(ref.Screenshots), len(ref.Colors), len(ref.Fonts), len(ref.Sections), len(ref.Assets), len(ref.Pages), len(ref.Warnings))

	enrichedPrompt := prompt + "\n\n" + buildReferenceSitePromptBlock(ref)
	return enrichedPrompt, appendUniqueStrings(imageURLs, screenshotURLs...), ref, ""
}

func getReferenceSiteContext(ctx context.Context, conf config.BaseConfig, targetURL string) (*models.ReferenceSiteContext, error) {
	if conf.ReferenceCaptureEnabled && strings.TrimSpace(conf.ReferenceCaptureURL) != "" {
		ref, err := captureReferenceSite(ctx, conf, targetURL)
		if err == nil && len(referenceScreenshotURLs(ref)) > 0 {
			return ref, nil
		}
		if err != nil {
			log.Printf("[reference-capture] render service failed, falling back to HTML/CSS: %v", err)
		} else {
			log.Printf("[reference-capture] render service returned no screenshots, falling back to HTML/CSS")
		}
	}

	ref, err := fetchReferenceSiteHTMLForPrompt(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	ref.Warnings = appendUniqueStrings(ref.Warnings,
		"HTML/CSS-only extraction was used; exact responsive layout, spacing, animations, and JS-rendered content may be incomplete.",
	)
	return ref, nil
}

func extractReferenceURLs(prompt string) []string {
	seen := make(map[string]bool)
	var out []string

	add := func(raw string) {
		raw = cleanReferenceURLCandidate(raw)
		if raw == "" {
			return
		}
		normalized, err := normalizeReferenceURL(raw)
		if err != nil {
			// Keep the raw candidate so the caller can return a user-facing URL error
			// instead of silently ignoring clone intent.
			normalized = raw
		}
		key := strings.ToLower(normalized)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, normalized)
	}

	for _, match := range httpReferenceURLRe.FindAllString(prompt, -1) {
		add(match)
	}

	for _, loc := range bareReferenceURLRe.FindAllStringIndex(prompt, -1) {
		if loc[0] > 0 && prompt[loc[0]-1] == '@' {
			continue
		}
		if loc[0] >= 3 && strings.EqualFold(prompt[loc[0]-3:loc[0]], "://") {
			continue
		}
		if isLikelyAssetFilename(prompt[loc[0]:loc[1]]) {
			continue
		}
		add(prompt[loc[0]:loc[1]])
	}

	return out
}

// assetFilenameSuffixes are "TLDs" that are really file extensions. The bare-domain
// regex accepts any alphabetic TLD, so words like "hero.png" or "index.tsx" in a
// prompt would otherwise count as extra reference URLs and trigger the
// one-URL-only refusal.
var assetFilenameSuffixes = map[string]bool{
	"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true, "webp": true,
	"ico": true, "css": true, "js": true, "ts": true, "tsx": true, "jsx": true,
	"json": true, "html": true, "htm": true, "pdf": true, "txt": true, "md": true,
	"woff": true, "ttf": true, "otf": true, "mp4": true, "webm": true, "zip": true,
	"csv": true, "xml": true, "yml": true, "yaml": true, "env": true,
}

func isLikelyAssetFilename(candidate string) bool {
	candidate = cleanReferenceURLCandidate(candidate)
	host := candidate
	if idx := strings.IndexAny(host, "/?#"); idx != -1 {
		host = host[:idx]
	}
	dot := strings.LastIndex(host, ".")
	if dot == -1 {
		return false
	}
	return assetFilenameSuffixes[strings.ToLower(host[dot+1:])]
}

// reduceSameHostReferenceURLs collapses multiple URLs that point at the same
// site (e.g. "site.de" and "site.de/about") to the shortest one, so a prompt
// mentioning two paths of one site is not rejected as "multiple websites".
func reduceSameHostReferenceURLs(urls []string) []string {
	if len(urls) < 2 {
		return urls
	}
	host := ""
	shortest := ""
	for _, raw := range urls {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			return urls
		}
		h := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
		if host == "" {
			host = h
		} else if h != host {
			return urls
		}
		if shortest == "" || len(raw) < len(shortest) {
			shortest = raw
		}
	}
	return []string{shortest}
}

func hasReferenceCloneIntent(prompt string) bool {
	lower := strings.ToLower(prompt)
	signals := []string{
		"1 to 1", "1:1", "one to one",
		"clone", "copy", "replicate", "recreate",
		"same design", "same logic", "same layout", "same style",
		"according to", "based on this", "based on the", "reference",
		"like this website", "like this site", "make like",
		// Russian — most Ugen users write prompts in Russian.
		"1 в 1", "один в один", "точь-в-точь", "точно как", "точно такой",
		"клон", "клониру", "скопиру", "скопир", "копию", "копия сайта",
		"такой же дизайн", "такой же сайт", "так же как", "также как",
		"как на сайте", "как у сайта", "как этот сайт", "по образцу", "по примеру",
		"повтори дизайн", "повтори сайт", "срисуй", "референс",
		// Uzbek (latin).
		"xuddi shu", "bir xil", "nusxa", "shu saytdek", "saytga o'xshash", "o‘xshash qilib",
	}
	for _, signal := range signals {
		if strings.Contains(lower, signal) {
			return true
		}
	}
	return false
}

func isReferenceClonePrompt(prompt string) bool {
	return hasReferenceCloneIntent(prompt) && len(extractReferenceURLs(prompt)) > 0
}

func cleanReferenceURLCandidate(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimLeft(raw, "\"'(<[{")
	raw = strings.TrimRight(raw, "\"')>]}.,;!")
	return raw
}

func normalizeReferenceURL(raw string) (string, error) {
	raw = cleanReferenceURLCandidate(raw)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	parsed.User = nil
	parsed.Fragment = ""

	host := strings.ToLower(parsed.Hostname())
	if err = validatePublicReferenceHost(host); err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("missing host")
	}

	return parsed.String(), nil
}

func validatePublicReferenceHost(host string) error {
	if host == "" {
		return fmt.Errorf("missing host")
	}

	blockedNames := map[string]bool{
		"localhost": true,
	}
	if blockedNames[host] {
		return fmt.Errorf("blocked host")
	}
	for _, suffix := range []string{".localhost", ".local", ".internal"} {
		if strings.HasSuffix(host, suffix) {
			return fmt.Errorf("blocked internal host")
		}
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() {
			return fmt.Errorf("blocked private IP")
		}
		return nil
	}

	if net.ParseIP(host) != nil {
		return fmt.Errorf("blocked unsupported IP")
	}
	if !strings.Contains(host, ".") {
		return fmt.Errorf("blocked non-public host")
	}
	return nil
}

func captureReferenceSite(ctx context.Context, conf config.BaseConfig, targetURL string) (*models.ReferenceSiteContext, error) {
	if !conf.ReferenceCaptureEnabled || strings.TrimSpace(conf.ReferenceCaptureURL) == "" {
		return nil, fmt.Errorf("reference capture service is not configured")
	}

	endpoint := strings.TrimSpace(conf.ReferenceCaptureURL)
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.Scheme == "" || parsedEndpoint.Host == "" {
		return nil, fmt.Errorf("invalid reference capture URL")
	}

	timeout := time.Duration(conf.ReferenceCaptureTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	payload := referenceCaptureRequest{
		URL: targetURL,
		Viewports: []referenceCaptureViewport{
			{Name: "desktop", Width: 1440, Height: 1200},
			{Name: "mobile", Width: 390, Height: 1200},
		},
		Extract:  []string{"screenshots", "text", "colors", "fonts", "assets", "sections"},
		FullPage: true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if conf.ReferenceCaptureAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+conf.ReferenceCaptureAPIKey)
		req.Header.Set("X-API-Key", conf.ReferenceCaptureAPIKey)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("capture service %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var ref models.ReferenceSiteContext
	if err = json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return nil, err
	}
	return &ref, nil
}

func fetchReferenceSiteHTML(ctx context.Context, targetURL string) (*models.ReferenceSiteContext, error) {
	client := newReferenceHTTPClient(15 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "UgenReferenceExtractor/1.0 (+https://u-code.io)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("reference page %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	baseURL := resp.Request.URL.String()

	ref := extractReferenceSiteFromHTML(targetURL, baseURL, string(body), "")

	var cssBuilder strings.Builder
	for _, href := range extractStylesheetLinks(string(body), baseURL, 6) {
		css, cssErr := fetchReferenceCSS(ctx, client, href)
		if cssErr != nil {
			ref.Warnings = append(ref.Warnings, "Could not fetch stylesheet: "+href)
			continue
		}
		cssBuilder.WriteString("\n")
		cssBuilder.WriteString(css)
	}
	if cssBuilder.Len() > 0 {
		ref = extractReferenceSiteFromHTML(targetURL, baseURL, string(body), cssBuilder.String())
	}

	if len(ref.Sections) == 0 && ref.Title == "" && ref.Description == "" {
		return nil, fmt.Errorf("no useful HTML context extracted")
	}
	return ref, nil
}

func newReferenceHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if err = validatePublicReferenceHost(strings.ToLower(host)); err != nil {
				return nil, err
			}
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			var publicIP string
			for _, addr := range addrs {
				if isPublicReferenceAddr(addr.IP) {
					publicIP = addr.IP.String()
					break
				}
			}
			if publicIP == "" {
				return nil, fmt.Errorf("host resolved to no public IP")
			}
			dialer := &net.Dialer{Timeout: timeout}
			return dialer.DialContext(ctx, network, net.JoinHostPort(publicIP, port))
		},
		ResponseHeaderTimeout: timeout,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			_, err := normalizeReferenceURL(req.URL.String())
			return err
		},
	}
}

func isPublicReferenceAddr(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	return !(addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast())
}

func fetchReferenceCSS(ctx context.Context, client *http.Client, cssURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cssURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "UgenReferenceExtractor/1.0 (+https://u-code.io)")
	req.Header.Set("Accept", "text/css,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("css %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func extractReferenceSiteFromHTML(sourceURL, finalURL, htmlText, cssText string) *models.ReferenceSiteContext {
	ref := &models.ReferenceSiteContext{
		URL:      sourceURL,
		FinalURL: finalURL,
	}

	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		ref.Warnings = append(ref.Warnings, "Could not parse HTML; using regex fallback.")
		ref.Colors = rankColorsByFrequency(htmlText+"\n"+cssText, 12)
		ref.Fonts = extractFontsFromCSS(htmlText+"\n"+cssText, 8)
		return ref
	}

	var inlineCSS strings.Builder
	var sections []models.ReferenceSection
	var assets []models.ReferenceAsset

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "title":
				if ref.Title == "" {
					ref.Title = normalizeReferenceText(nodeText(n), 180)
				}
			case "meta":
				name := strings.ToLower(attr(n, "name"))
				property := strings.ToLower(attr(n, "property"))
				content := normalizeReferenceText(attr(n, "content"), 260)
				if content != "" {
					if ref.Description == "" && (name == "description" || property == "og:description") {
						ref.Description = content
					}
					if ref.Title == "" && property == "og:title" {
						ref.Title = normalizeReferenceText(content, 180)
					}
				}
			case "style":
				inlineCSS.WriteString("\n")
				inlineCSS.WriteString(nodeText(n))
			case "img":
				if assetURL := absolutePublicURL(attr(n, "src"), finalURL); assetURL != "" && len(assets) < 16 {
					assets = append(assets, models.ReferenceAsset{Type: "image", URL: assetURL, Alt: normalizeReferenceText(attr(n, "alt"), 120)})
				}
			case "section", "header", "main", "article":
				if section := extractReferenceSection(n); section.Heading != "" || section.Copy != "" || section.CTA != "" {
					sections = append(sections, section)
				}
			}

			if style := attr(n, "style"); style != "" {
				inlineCSS.WriteString("\n")
				inlineCSS.WriteString(style)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	if len(sections) == 0 {
		sections = extractHeadingSections(doc)
	}

	allStyles := htmlText + "\n" + cssText + "\n" + inlineCSS.String()
	ref.Colors = rankColorsByFrequency(allStyles, 12)
	ref.Fonts = extractFontsFromCSS(allStyles, 8)
	ref.Sections = limitReferenceSections(sections, 16)
	ref.Assets = limitReferenceAssets(assets, 16)
	ref.NavLinks = extractNavLinks(doc, finalURL, 14)
	for _, link := range ref.NavLinks {
		ref.Navigation = append(ref.Navigation, link.Label)
	}
	if ref.FinalURL == "" {
		ref.FinalURL = sourceURL
	}
	return ref
}

// rankColorsByFrequency returns the most-used color literals first. Taking the
// first N literals in source order surfaces reset/utility colors instead of the
// brand palette, so occurrence count is the better dominance signal available
// without rendering.
func rankColorsByFrequency(styles string, max int) []string {
	counts := make(map[string]int)
	firstSeen := make(map[string]int)
	for i, match := range cssColorRe.FindAllString(styles, -1) {
		key := strings.ToLower(strings.TrimSpace(match))
		if key == "" {
			continue
		}
		if _, ok := firstSeen[key]; !ok {
			firstSeen[key] = i
		}
		counts[key]++
	}

	ranked := make([]string, 0, len(counts))
	for key := range counts {
		ranked = append(ranked, key)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if counts[ranked[i]] != counts[ranked[j]] {
			return counts[ranked[i]] > counts[ranked[j]]
		}
		return firstSeen[ranked[i]] < firstSeen[ranked[j]]
	})
	return limitStrings(ranked, max)
}

// extractNavLinks collects the site's top-level navigation links (label +
// resolved same-host URL) — the real page inventory for multi-page clones, the
// clone questionnaire's page options, and the crawl targets for subpages.
func extractNavLinks(doc *html.Node, baseURL string, max int) []models.ReferenceNavLink {
	var links []models.ReferenceNavLink
	seen := make(map[string]bool)

	var collectAnchors func(*html.Node)
	collectAnchors = func(n *html.Node) {
		if n == nil || len(links) >= max {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "a") {
			label := normalizeReferenceText(nodeText(n), 60)
			key := strings.ToLower(label)
			if label != "" && !seen[key] {
				seen[key] = true
				links = append(links, models.ReferenceNavLink{
					Label: label,
					URL:   absolutePublicURL(attr(n, "href"), baseURL),
				})
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collectAnchors(child)
		}
	}

	var findNav func(*html.Node)
	findNav = func(n *html.Node) {
		if n == nil || len(links) >= max {
			return
		}
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			role := strings.ToLower(attr(n, "role"))
			if tag == "nav" || role == "navigation" {
				collectAnchors(n)
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			findNav(child)
		}
	}
	findNav(doc)
	return links
}

// promptWantsSingleLanding reports an explicit single-page landing request —
// those clone only the referenced page, so subpage crawling and the clone
// questionnaire are skipped.
func promptWantsSingleLanding(prompt string) bool {
	lower := strings.ToLower(prompt)
	for _, s := range []string{"landing", "лендинг", "одностраничн", "one-page", "one page", "single page", "single-page"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

const maxReferenceSubpages = 5

// fetchReferencePageForCrawl fetches one page of the reference site and
// extracts its structure (no stylesheet follow-up — structure and copy only).
// Overridable in tests.
var fetchReferencePageForCrawl = func(ctx context.Context, pageURL string) (*models.ReferenceSiteContext, error) {
	client := newReferenceHTTPClient(10 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "UgenReferenceExtractor/1.0 (+https://u-code.io)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("subpage %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}
	return extractReferenceSiteFromHTML(pageURL, resp.Request.URL.String(), string(body), ""), nil
}

// crawlReferenceSubpages fetches the reference site's own subpages (same host,
// taken from its navigation) so every cloned page gets its own captured
// evidence instead of being invented in home-page style. Nav links whose label
// appears in the prompt (i.e. pages the user selected in the questionnaire)
// are crawled first. Best-effort: failures become warnings, never errors.
func crawlReferenceSubpages(ctx context.Context, ref *models.ReferenceSiteContext, prompt string) {
	if ref == nil || len(ref.Pages) > 0 {
		return
	}

	navLinks := ref.NavLinks
	if len(navLinks) == 0 {
		// Render-service captures carry no nav hrefs — fetch the home page
		// HTML just for its navigation.
		if home, err := fetchReferencePageForCrawl(ctx, referenceHomeURL(ref)); err == nil {
			navLinks = home.NavLinks
		}
	}
	if len(navLinks) == 0 {
		return
	}

	base, err := url.Parse(referenceHomeURL(ref))
	if err != nil {
		return
	}
	host := strings.TrimPrefix(strings.ToLower(base.Hostname()), "www.")

	promptLower := strings.ToLower(prompt)
	sort.SliceStable(navLinks, func(i, j int) bool {
		iSelected := navLinks[i].Label != "" && strings.Contains(promptLower, strings.ToLower(navLinks[i].Label))
		jSelected := navLinks[j].Label != "" && strings.Contains(promptLower, strings.ToLower(navLinks[j].Label))
		return iSelected && !jSelected
	})

	crawlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	seenPaths := map[string]bool{"": true}
	for _, link := range navLinks {
		if len(ref.Pages) >= maxReferenceSubpages {
			break
		}
		if link.URL == "" {
			continue
		}
		parsed, err := url.Parse(link.URL)
		if err != nil {
			continue
		}
		if strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.") != host {
			continue
		}
		pathKey := strings.TrimSuffix(parsed.Path, "/")
		if seenPaths[pathKey] {
			continue
		}
		seenPaths[pathKey] = true

		page, err := fetchReferencePageForCrawl(crawlCtx, link.URL)
		if err != nil {
			ref.Warnings = appendUniqueStrings(ref.Warnings, "Could not crawl subpage: "+link.URL)
			continue
		}
		ref.Pages = append(ref.Pages, models.ReferencePage{
			Label:    link.Label,
			URL:      link.URL,
			Title:    page.Title,
			Sections: limitReferenceSections(page.Sections, 8),
		})
	}
	if len(ref.Pages) > 0 {
		log.Printf("[reference] crawled %d subpage(s) for multi-page clone", len(ref.Pages))
	}
}

func referenceHomeURL(ref *models.ReferenceSiteContext) string {
	if ref.FinalURL != "" {
		return ref.FinalURL
	}
	return ref.URL
}

func extractStylesheetLinks(htmlText, baseURL string, max int) []string {
	doc, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil
	}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || len(links) >= max {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "link") {
			rel := strings.ToLower(attr(n, "rel"))
			if strings.Contains(rel, "stylesheet") {
				if href := absolutePublicURL(attr(n, "href"), baseURL); href != "" {
					links = append(links, href)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return appendUniqueStrings(nil, links...)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur == nil {
			return
		}
		if cur.Type == html.ElementNode {
			tag := strings.ToLower(cur.Data)
			if tag == "script" || tag == "style" || tag == "noscript" {
				return
			}
		}
		if cur.Type == html.TextNode {
			if text := strings.TrimSpace(cur.Data); text != "" {
				parts = append(parts, text)
			}
		}
		for child := cur.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return normalizeReferenceText(strings.Join(parts, " "), 1000)
}

func normalizeReferenceText(s string, max int) string {
	s = htmlstd.UnescapeString(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	s = referenceWhitespaceRe.ReplaceAllString(strings.TrimSpace(s), " ")
	if max > 0 {
		s = truncateReferenceText(s, max)
	}
	return s
}

func absolutePublicURL(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(strings.ToLower(raw), "data:") || strings.HasPrefix(strings.ToLower(raw), "javascript:") {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(u)
	normalized, err := normalizeReferenceURL(resolved.String())
	if err != nil {
		return ""
	}
	return normalized
}

func extractReferenceSection(n *html.Node) models.ReferenceSection {
	var (
		section    models.ReferenceSection
		paragraphs []string
	)
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur == nil {
			return
		}
		if cur.Type == html.ElementNode {
			tag := strings.ToLower(cur.Data)
			switch tag {
			case "h1", "h2", "h3":
				if section.Heading == "" {
					section.Heading = normalizeReferenceText(nodeText(cur), 140)
				}
			case "p":
				// Keep up to three paragraphs — one is too thin for "1 to 1" copy.
				if len(paragraphs) < 3 {
					if text := normalizeReferenceText(nodeText(cur), 300); text != "" {
						paragraphs = append(paragraphs, text)
					}
				}
			case "a", "button":
				if section.CTA == "" {
					section.CTA = normalizeReferenceText(nodeText(cur), 90)
				}
			}
		}
		for child := cur.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	section.Copy = normalizeReferenceText(strings.Join(paragraphs, " ¶ "), 600)

	layoutParts := []string{strings.ToLower(n.Data)}
	if id := attr(n, "id"); id != "" {
		layoutParts = append(layoutParts, "id="+id)
	}
	if className := attr(n, "class"); className != "" {
		layoutParts = append(layoutParts, "class="+truncateReferenceText(className, 100))
	}
	section.Layout = strings.Join(layoutParts, " ")
	return section
}

func extractHeadingSections(doc *html.Node) []models.ReferenceSection {
	var sections []models.ReferenceSection
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || len(sections) >= 12 {
			return
		}
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "h1" || tag == "h2" || tag == "h3" {
				heading := normalizeReferenceText(nodeText(n), 140)
				if heading != "" {
					sections = append(sections, models.ReferenceSection{Heading: heading, Layout: tag})
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return sections
}

func extractFontsFromCSS(cssText string, max int) []string {
	var fonts []string
	for _, match := range googleFontFamilyRe.FindAllStringSubmatch(cssText, -1) {
		if len(match) < 2 {
			continue
		}
		family, err := url.QueryUnescape(match[1])
		if err != nil {
			family = match[1]
		}
		family = strings.ReplaceAll(family, "+", " ")
		fonts = append(fonts, family)
	}

	for _, match := range fontFamilyRe.FindAllStringSubmatch(cssText, -1) {
		if len(match) < 2 {
			continue
		}
		for _, raw := range strings.Split(match[1], ",") {
			font := cleanFontFamily(raw)
			if font != "" {
				fonts = append(fonts, font)
			}
		}
	}
	return limitStrings(fonts, max)
}

func cleanFontFamily(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, `"'`)
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if raw == "" || strings.HasPrefix(lower, "var(") || strings.HasPrefix(lower, "--") {
		return ""
	}
	generic := map[string]bool{
		"serif": true, "sans-serif": true, "monospace": true, "system-ui": true,
		"inherit": true, "initial": true, "unset": true, "ui-sans-serif": true,
	}
	if generic[lower] {
		return ""
	}
	return raw
}

func limitReferenceSections(values []models.ReferenceSection, max int) []models.ReferenceSection {
	out := make([]models.ReferenceSection, 0, max)
	seen := make(map[string]bool)
	for _, section := range values {
		section.Heading = normalizeReferenceText(section.Heading, 140)
		section.Copy = normalizeReferenceText(section.Copy, 600)
		section.CTA = normalizeReferenceText(section.CTA, 90)
		section.Layout = normalizeReferenceText(section.Layout, 140)
		key := strings.ToLower(section.Heading + "|" + section.Copy + "|" + section.CTA)
		if key == "||" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, section)
		if len(out) == max {
			break
		}
	}
	return out
}

func limitReferenceAssets(values []models.ReferenceAsset, max int) []models.ReferenceAsset {
	out := make([]models.ReferenceAsset, 0, max)
	seen := make(map[string]bool)
	for _, asset := range values {
		asset.URL = strings.TrimSpace(asset.URL)
		if asset.URL == "" || seen[asset.URL] {
			continue
		}
		seen[asset.URL] = true
		asset.Type = normalizeReferenceText(asset.Type, 40)
		asset.Alt = normalizeReferenceText(asset.Alt, 120)
		out = append(out, asset)
		if len(out) == max {
			break
		}
	}
	return out
}

// referenceCaptureProgressMessage tells the user HOW the reference was captured.
// Screenshot capture and the HTML/CSS fallback produce very different clone
// fidelity; reporting them identically makes design drift undiagnosable.
func referenceCaptureProgressMessage(ref *models.ReferenceSiteContext) string {
	if ref != nil && len(ref.Screenshots) > 0 {
		return "Захватил референс сайта (скриншоты + структура) для точного клонирования"
	}
	return "Референс захвачен только по HTML/CSS — скриншоты недоступны, точность клонирования будет ниже"
}

func referenceScreenshotURLs(ref *models.ReferenceSiteContext) []string {
	if ref == nil {
		return nil
	}
	urls := make([]string, 0, len(ref.Screenshots))
	for _, shot := range ref.Screenshots {
		if strings.TrimSpace(shot.URL) != "" {
			urls = append(urls, strings.TrimSpace(shot.URL))
		}
	}
	return urls
}

func applyReferenceContextToPlan(plan *models.ArchitectPlan, ref *models.ReferenceSiteContext, prompt string) {
	if plan == nil || ref == nil {
		return
	}

	plan.CloneMode = true
	plan.Reference = ref
	plan.ImageKeywords = nil

	if plan.Design.DesignInspiration == "" || !strings.Contains(strings.ToLower(plan.Design.DesignInspiration), "reference") {
		plan.Design.DesignInspiration = "Reference site clone: match captured site evidence"
	}

	if shouldForceStaticReferenceClone(prompt, plan) {
		plan.Tables = nil
		plan.Relations = nil
		plan.ClientTypes = nil
	}

	refBlock := buildReferenceSitePromptBlock(ref)
	if !strings.Contains(plan.UIStructure, referenceContextMarker) {
		if strings.TrimSpace(plan.UIStructure) != "" {
			plan.UIStructure += "\n\n"
		}
		plan.UIStructure += refBlock
	}
}

// functionalCloneStems match the clone questionnaire's functionality option
// labels (the frontend sends selected option LABELS back as text) plus generic
// wording — any hit means the user wants a working product, so backend tables
// must be kept.
var functionalCloneStems = []string{
	// English questionnaire labels + generic terms.
	"working auth", "login / registration", "content from database",
	"cart & checkout", "checkout flow", "working forms", "search & filters",
	"cart", "checkout", "registration", "real data",
	// Russian questionnaire labels + generic terms.
	"рабочая авторизац", "логин / регистрац", "контент из базы",
	"корзина и оформление", "рабочие формы", "поиск и фильтры",
	"корзин", "оформление заказа", "авторизац", "регистрац", "логин",
	"база данных", "базы данных",
}

// staticCloneStems match the questionnaire's explicit "static copy" choice.
var staticCloneStems = []string{
	"static design copy", "статичная копия", "только дизайн", "no backend", "без бэкенда",
}

func shouldForceStaticReferenceClone(prompt string, plan *models.ArchitectPlan) bool {
	if plan == nil {
		return false
	}
	projectType := strings.ToLower(plan.ProjectType)
	if projectType != "" && projectType != "landing" && projectType != "web" {
		return false
	}

	lower := " " + strings.ToLower(prompt) + " "

	// Questionnaire answers are deterministic: selected functionality keeps the
	// backend even when "static copy" was also (contradictorily) ticked.
	for _, stem := range functionalCloneStems {
		if strings.Contains(lower, stem) {
			return false
		}
	}
	for _, stem := range staticCloneStems {
		if strings.Contains(lower, stem) {
			return true
		}
	}

	dynamicSignals := []string{
		" admin ", " admin panel ", " dashboard ", " database ", " db ",
		" crud ", " login ", " sign in ", " signup ", " register ",
		" api ", " table ", " manage ", " management ",
		" web app ", " webapp ", " mobile app ", " application ",
		" backend ", " crm ", " erp ",
	}
	for _, signal := range dynamicSignals {
		if strings.Contains(lower, signal) {
			return false
		}
	}
	return true
}

func shouldSkipBackendForReferencePrompt(prompt string, ref *models.ReferenceSiteContext) bool {
	if ref == nil {
		return false
	}
	return shouldForceStaticReferenceClone(prompt, &models.ArchitectPlan{ProjectType: "landing"})
}

func buildReferenceSitePromptBlock(ref *models.ReferenceSiteContext) string {
	if ref == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("====================================\n")
	sb.WriteString(referenceContextMarker + "\n")
	sb.WriteString("====================================\n")
	sb.WriteString("Clone mode is active because the user asked to reproduce a real website URL.\n")
	if len(ref.Screenshots) > 0 {
		sb.WriteString("The screenshots attached to this request are the primary visual source of truth.\n")
	} else {
		sb.WriteString("No screenshots are attached. Use the extracted HTML/CSS/text/colors/fonts/assets below as the available source of truth.\n")
		sb.WriteString("This is less precise than a browser screenshot, so preserve extracted structure and styling evidence and avoid inventing unrelated design.\n")
	}
	sb.WriteString("Do NOT invent a new design direction, archetype, palette, stock-image mood, page structure, or extra sections.\n")
	sb.WriteString("Reproduce the captured site's section order, copy, typography feel, colors, imagery, CTA placement, and visible brand style as closely as possible from the evidence.\n")
	sb.WriteString("For a pure landing/website clone, do NOT add database CRUD pages, dashboards, login screens, or API-driven sections unless the user explicitly requested those product features — e.g. selected working login/registration, cart & checkout, database content, forms, or search in the questionnaire answers. When such features ARE selected, build them as working functionality wired to the generated backend while still cloning the reference design.\n\n")

	fmt.Fprintf(&sb, "Source URL: %s\n", ref.URL)
	if ref.FinalURL != "" && ref.FinalURL != ref.URL {
		fmt.Fprintf(&sb, "Final URL: %s\n", ref.FinalURL)
	}
	if ref.Title != "" {
		fmt.Fprintf(&sb, "Title: %s\n", truncateReferenceText(ref.Title, 180))
	}
	if ref.Description != "" {
		fmt.Fprintf(&sb, "Description: %s\n", truncateReferenceText(ref.Description, 260))
	}

	if len(ref.Screenshots) > 0 {
		sb.WriteString("\nCaptured screenshots attached as image inputs:\n")
		for _, shot := range ref.Screenshots {
			if shot.URL == "" {
				continue
			}
			size := ""
			if shot.Width > 0 && shot.Height > 0 {
				size = fmt.Sprintf(" (%dx%d)", shot.Width, shot.Height)
			}
			fmt.Fprintf(&sb, "- %s%s: %s\n", fallbackString(shot.Viewport, "viewport"), size, shot.URL)
		}
	}

	if len(ref.Colors) > 0 {
		fmt.Fprintf(&sb, "\nExtracted colors (ranked by how often they appear — most-used first): %s\n", strings.Join(limitStrings(ref.Colors, 12), ", "))
	}
	if len(ref.Fonts) > 0 {
		fmt.Fprintf(&sb, "Extracted fonts: %s\n", strings.Join(limitStrings(ref.Fonts, 8), ", "))
	}

	if len(ref.Navigation) > 0 {
		fmt.Fprintf(&sb, "\nSite navigation links (the REAL page inventory — do not invent other pages): %s\n", strings.Join(limitStrings(ref.Navigation, 14), " · "))
	}

	if len(ref.Sections) > 0 {
		sb.WriteString("\nDetected sections in order (copy text is authoritative — reuse it verbatim):\n")
		for i, section := range ref.Sections {
			if i >= 16 {
				break
			}
			parts := []string{}
			if section.Heading != "" {
				parts = append(parts, "heading="+truncateReferenceText(section.Heading, 120))
			}
			if section.Copy != "" {
				parts = append(parts, "copy="+truncateReferenceText(section.Copy, 500))
			}
			if section.CTA != "" {
				parts = append(parts, "cta="+truncateReferenceText(section.CTA, 80))
			}
			if section.Layout != "" {
				parts = append(parts, "layout="+truncateReferenceText(section.Layout, 100))
			}
			if len(parts) > 0 {
				fmt.Fprintf(&sb, "%d. %s\n", i+1, strings.Join(parts, " | "))
			}
		}
	}

	if len(ref.Pages) > 0 {
		sb.WriteString("\nCrawled SUBPAGES — clone every page from ITS OWN evidence below (do not restyle subpages as copies of the home page):\n")
		for i, page := range ref.Pages {
			if i >= 6 {
				break
			}
			fmt.Fprintf(&sb, "\nSUBPAGE %d: %s — %s\n", i+1, fallbackString(page.Label, "page"), page.URL)
			if page.Title != "" {
				fmt.Fprintf(&sb, "Title: %s\n", truncateReferenceText(page.Title, 140))
			}
			for j, section := range page.Sections {
				if j >= 8 {
					break
				}
				parts := []string{}
				if section.Heading != "" {
					parts = append(parts, "heading="+truncateReferenceText(section.Heading, 120))
				}
				if section.Copy != "" {
					parts = append(parts, "copy="+truncateReferenceText(section.Copy, 300))
				}
				if section.CTA != "" {
					parts = append(parts, "cta="+truncateReferenceText(section.CTA, 80))
				}
				if len(parts) > 0 {
					fmt.Fprintf(&sb, "  %d.%d %s\n", i+1, j+1, strings.Join(parts, " | "))
				}
			}
		}
	}

	if len(ref.Assets) > 0 {
		sb.WriteString("\nCaptured assets to prefer over stock imagery:\n")
		for i, asset := range ref.Assets {
			if i >= 12 {
				break
			}
			if asset.URL == "" {
				continue
			}
			label := strings.TrimSpace(strings.Join([]string{asset.Type, asset.Alt}, " "))
			fmt.Fprintf(&sb, "- %s: %s\n", truncateReferenceText(label, 120), asset.URL)
		}
	}

	if len(ref.Warnings) > 0 {
		fmt.Fprintf(&sb, "\nCapture warnings: %s\n", strings.Join(limitStrings(ref.Warnings, 5), "; "))
	}
	sb.WriteString("====================================\n")
	return sb.String()
}

func appendUniqueStrings(base []string, extras ...string) []string {
	seen := make(map[string]bool, len(base)+len(extras))
	out := make([]string, 0, len(base)+len(extras))
	for _, item := range base {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	for _, item := range extras {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func truncateReferenceText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func limitStrings(values []string, max int) []string {
	limited := make([]string, 0, max)
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		limited = append(limited, truncateReferenceText(value, 120))
		if len(limited) == max {
			break
		}
	}
	return limited
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
