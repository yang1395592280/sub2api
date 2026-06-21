package domain

type WorkbenchImageOutput struct {
	URL      string `json:"url,omitempty"`
	B64JSON  string `json:"b64_json,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}
