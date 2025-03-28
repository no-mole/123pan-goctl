package terrors

import (
	"errors"
	"fmt"
)

type ErrFormat string

var (
	FileOpenError             ErrFormat = "fail to open file"
	FileUploadTaskError       ErrFormat = "fail to create upload task"
	FileSeekError             ErrFormat = "fail to seek file"
	FileGetUploadUrlError     ErrFormat = "fail to get upload file url"
	FileSliceUploadError      ErrFormat = "fail to upload file slice"
	FetchFileUploadSatusError ErrFormat = "fail to fetch upload result"
	GetAccessTokenError       ErrFormat = "fail to gen access token"
	GenFileMD5Error           ErrFormat = "fail to gen file MD5 SUM"
	CanNotFormatConfigFIle    ErrFormat = "can not format config file"
	MustSpecifyFolderError    ErrFormat = "Must specify folder"

	NotADir ErrFormat = "not a dir name"
)

func New(format ErrFormat, err error, args ...any) error {
	return errors.Join(fmt.Errorf((string)(format), args...), err)
}
