package request

import "mime/multipart"

type UploadMarkdownRequest struct {
	File *multipart.FileHeader `form:"file" binding:"required"`
}
