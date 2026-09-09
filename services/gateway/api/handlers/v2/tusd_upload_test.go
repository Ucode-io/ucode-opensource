package v2

import "testing"

func TestUploadContentType(t *testing.T) {
	tests := map[string]struct {
		fileType string
		fileName string
		want     string
	}{
		"missing metadata":      {fileType: "", fileName: "", want: "application/octet-stream"},
		"pdf metadata":          {fileType: "application/pdf", fileName: "book.pdf", want: "application/pdf"},
		"mp4 metadata":          {fileType: "video/mp4", fileName: "clip.mp4", want: "video/mp4"},
		"webm metadata":         {fileType: "video/webm", fileName: "clip.webm", want: "video/webm"},
		"metadata params":       {fileType: "video/mp4; codecs=avc1.4D401F", fileName: "clip.mp4", want: "video/mp4"},
		"invalid metadata":      {fileType: "not-a-mime-type", fileName: "book.pdf", want: "application/pdf"},
		"no metadata by ext":    {fileType: "", fileName: "book.PDF", want: "application/pdf"},
		"unknown ext":           {fileType: "", fileName: "archive.unknownext", want: "application/octet-stream"},
		"no extension":          {fileType: "", fileName: "archive", want: "application/octet-stream"},
		"invalid type and name": {fileType: "not-a-mime-type", fileName: "archive", want: "application/octet-stream"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := uploadContentType(test.fileType, test.fileName); got != test.want {
				t.Fatalf("uploadContentType(%q, %q) = %q, want %q", test.fileType, test.fileName, got, test.want)
			}
		})
	}
}

func TestUploadContentDisposition(t *testing.T) {
	tests := map[string]struct {
		fileName string
		want     string
	}{
		"empty name": {fileName: "", want: "inline"},
		"plain name": {fileName: "OfertaBetradeREn.pdf", want: "inline; filename=OfertaBetradeREn.pdf"},
		"path name":  {fileName: "/tmp/dir/OfertaBetradeREn.pdf", want: "inline; filename=OfertaBetradeREn.pdf"},
		"spaced":     {fileName: "my book.pdf", want: `inline; filename="my book.pdf"`},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := uploadContentDisposition(test.fileName); got != test.want {
				t.Fatalf("uploadContentDisposition(%q) = %q, want %q", test.fileName, got, test.want)
			}
		})
	}
}
