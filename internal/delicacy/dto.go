package delicacy

type CreateDelicacyRequest struct {
	Name        string `json:"name" binding:"required,min=3"`
	Description string `json:"description" binding:"required,min=3"`
	ThumbnaiUrls []string `json:"thumbnail_urls"`
}
