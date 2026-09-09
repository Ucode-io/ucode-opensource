package v1

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xuri/excelize/v2"
)

const (
	// referenceDocReadTimeout bounds a single document download from MinIO.
	referenceDocReadTimeout = 30 * time.Second
	// referenceDocMaxBytes caps how much we read of one attachment, so a giant
	// upload can't exhaust memory while we parse it.
	referenceDocMaxBytes = 25 << 20
	// referenceDocMaxTextPerDoc caps the extracted text of a single document.
	referenceDocMaxTextPerDoc = 20000
	// referenceDocsMaxTotalText caps the combined extracted text across all docs,
	// so we never blow the model's context with raw template content.
	referenceDocsMaxTotalText = 60000
)

// ooxmlTextRe matches the run-text elements that hold visible text in OOXML:
// <a:t>…</a:t> in PowerPoint slides and <w:t>…</w:t> in Word documents.
var ooxmlTextRe = regexp.MustCompile(`<(?:a|w):t(?:\s[^>]*)?>([^<]*)</(?:a|w):t>`)

// extractReferenceDocsText turns the builder's attached example/template documents
// (xlsx/pptx/docx — same URL channel as images) into plain text the model can study
// when designing the agent. Non-document attachments (real images) and anything that
// fails to parse are skipped silently; the result is best-effort context, never fatal.
func (p *ChatProcessor) extractReferenceDocsText(ctx context.Context, urls []string) string {
	if len(urls) == 0 {
		return ""
	}

	var (
		sb    strings.Builder
		total int
		n     int
	)
	for _, rawURL := range urls {
		ext := referenceDocExt(rawURL)
		if !isReferenceDocExt(ext) {
			continue
		}

		data, name, err := p.readMinioObject(ctx, rawURL)
		if err != nil {
			log.Printf("[AGENT REFDOCS] skip %s: %v", rawURL, err)
			continue
		}

		text, err := parseDocumentText(ext, data)
		if err != nil {
			log.Printf("[AGENT REFDOCS] parse %s failed: %v", name, err)
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		text = clampText(text, referenceDocMaxTextPerDoc)

		n++
		fmt.Fprintf(&sb, "----- Document %d: %s -----\n%s\n\n", n, name, text)
		total += len(text)
		if total >= referenceDocsMaxTotalText {
			break
		}
	}

	return clampText(strings.TrimSpace(sb.String()), referenceDocsMaxTotalText)
}

// isReferenceDocExt reports whether an attachment extension is a document format we
// can extract text from. Image extensions return false so real images are skipped.
func isReferenceDocExt(ext string) bool {
	switch ext {
	case "xlsx", "pptx", "docx":
		return true
	default:
		return false
	}
}

// referenceDocExt returns the lowercased file extension (without dot) of a URL's path.
func referenceDocExt(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(u.Path), "."))
	return ext
}

// readMinioObject downloads an attachment from the project's MinIO storage. It only
// fetches URLs whose host is our own MinIO endpoint — this is the SSRF guard: the
// builder-controlled URL can never make us reach into private networks. Returns the
// object bytes (capped) and its file name.
func (p *ChatProcessor) readMinioObject(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("parse url: %w", err)
	}
	if u.Host != p.baseConf.MinioEndpoint {
		return nil, "", fmt.Errorf("host %q is not the MinIO endpoint", u.Host)
	}

	trimmed := strings.TrimPrefix(u.Path, "/")
	bucket, object, ok := strings.Cut(trimmed, "/")
	if !ok || bucket == "" || object == "" {
		return nil, "", fmt.Errorf("url %q has no bucket/object path", rawURL)
	}

	minioClient, err := minio.New(p.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(p.baseConf.MinioAccessKeyID, p.baseConf.MinioSecretAccessKey, ""),
		Secure: p.baseConf.MinioProtocol,
	})
	if err != nil {
		return nil, "", fmt.Errorf("minio client: %w", err)
	}

	readCtx, cancel := context.WithTimeout(ctx, referenceDocReadTimeout)
	defer cancel()

	obj, err := minioClient.GetObject(readCtx, bucket, object, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(io.LimitReader(obj, referenceDocMaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("read object: %w", err)
	}

	return data, path.Base(object), nil
}

// parseDocumentText extracts plain text from a supported document by its extension.
func parseDocumentText(ext string, data []byte) (string, error) {
	switch ext {
	case "xlsx":
		return parseXLSXText(data)
	case "pptx", "docx":
		return parseOOXMLText(ext, data)
	default:
		return "", fmt.Errorf("unsupported document type %q", ext)
	}
}

// parseXLSXText flattens an xlsx workbook to text: every sheet's rows, cells joined by
// tabs and rows by newlines, so the model sees the table layout of the sample.
func parseXLSXText(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		fmt.Fprintf(&sb, "# Sheet: %s\n", sheet)
		for _, row := range rows {
			line := strings.TrimRight(strings.Join(row, "\t"), "\t")
			if line == "" {
				continue
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// parseOOXMLText extracts visible text from a pptx or docx (both are zip archives of
// XML). For pptx it walks the slides in order; for docx it reads the main document
// part. Text lives in <a:t>/<w:t> run elements, which we pull with a regex — adequate
// for handing the model the wording and structure of the sample.
func parseOOXMLText(ext string, data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open %s archive: %w", ext, err)
	}

	parts := make([]*zip.File, 0, len(zr.File))
	for _, file := range zr.File {
		if isOOXMLTextPart(ext, file.Name) {
			parts = append(parts, file)
		}
	}
	// Slides sort lexically as slide1, slide10, slide2 — order numerically so the
	// sample reads top to bottom.
	sort.Slice(parts, func(i, j int) bool {
		return ooxmlPartOrder(parts[i].Name) < ooxmlPartOrder(parts[j].Name)
	})

	var sb strings.Builder
	for _, file := range parts {
		rc, err := file.Open()
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(rc, referenceDocMaxBytes))
		rc.Close()
		if err != nil {
			continue
		}
		for _, m := range ooxmlTextRe.FindAllSubmatch(raw, -1) {
			frag := strings.TrimSpace(unescapeXML(string(m[1])))
			if frag == "" {
				continue
			}
			sb.WriteString(frag)
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// isOOXMLTextPart reports whether a zip entry holds visible text for the given format.
func isOOXMLTextPart(ext, name string) bool {
	switch ext {
	case "pptx":
		return strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml")
	case "docx":
		return name == "word/document.xml"
	default:
		return false
	}
}

// ooxmlPartOrder extracts the trailing slide number from a part name for ordering.
func ooxmlPartOrder(name string) int {
	base := strings.TrimSuffix(path.Base(name), ".xml")
	digits := strings.TrimLeft(base, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	n := 0
	for _, r := range digits {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// unescapeXML decodes the handful of XML entities that appear in OOXML run text.
func unescapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
	)
	return replacer.Replace(s)
}

// clampText truncates s to at most max bytes, appending an ellipsis marker so the
// model knows the sample was cut.
func clampText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 0 {
		return ""
	}
	return s[:max] + "\n…[truncated]"
}
