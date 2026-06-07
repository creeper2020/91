package googledrive

import "time"

type tokenResp struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type googleErrorResp struct {
	Error googleAPIError `json:"error"`
}

type googleAPIError struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Status  string              `json:"status"`
	Errors  []googleErrorDetail `json:"errors"`
}

type googleErrorDetail struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type fileResp struct {
	ID              string              `json:"id"`
	Name            string              `json:"name"`
	MimeType        string              `json:"mimeType"`
	Size            string              `json:"size"`
	ModifiedTime    time.Time           `json:"modifiedTime"`
	Parents         []string            `json:"parents"`
	MD5Checksum     string              `json:"md5Checksum"`
	SHA1Checksum    string              `json:"sha1Checksum"`
	SHA256Checksum  string              `json:"sha256Checksum"`
	WebContentLink  string              `json:"webContentLink"`
	ThumbnailLink   string              `json:"thumbnailLink"`
	ShortcutDetails shortcutDetailsResp `json:"shortcutDetails"`
}

type shortcutDetailsResp struct {
	TargetID       string `json:"targetId"`
	TargetMimeType string `json:"targetMimeType"`
}

type listResp struct {
	Files         []fileResp `json:"files"`
	NextPageToken string     `json:"nextPageToken"`
}
