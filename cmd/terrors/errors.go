package terrors

import (
	"errors"
	"fmt"
)

type ErrFormat string

var (
	//FileUploadError       ErrFormat = "fail to upload file"
	FileOpenError         ErrFormat = "fail to open file"
	FileUploadTaskError   ErrFormat = "fail to create upload task"
	FileSeekError         ErrFormat = "fail to seek file"
	FileGetUploadUrlError ErrFormat = "fail to get upload file url"
	FileSliceUploadError  ErrFormat = "fail to upload file slice"

	GetAccessTokenError ErrFormat = "fail to gen access token"

	GenFileMD5Error ErrFormat = "fail to gen file MD5 SUM"
)

func New(format ErrFormat, err error, args ...any) error {
	return errors.Join(fmt.Errorf((string)(format), args...), err)
}
