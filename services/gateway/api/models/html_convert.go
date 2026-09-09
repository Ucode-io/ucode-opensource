package models

type HtmlConvertRequest struct {
	HtmlContent  string `json:"html_content"`
	OutputFormat string `json:"output_format"` // e.g., "pdf", "docx"
}

type HtmlConvertResponse struct {
	FileUrl string `json:"file_url"`
}
