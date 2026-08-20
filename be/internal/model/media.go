package model

// MediaUpload is what an upload hands back to the client.
//
// URL is what callers embed (a Markdown `![](url)` in a 투자 리포트 body today);
// Key is what a future cleanup or delete path needs. Width/Height are included
// so a client can reserve layout space before the image loads — they are 0 for
// formats whose header we do not parse (webp).
type MediaUpload struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}
