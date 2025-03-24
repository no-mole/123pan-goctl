/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package file

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/no-mole/123pan-goctl/cmd/terrors"
	"github.com/no-mole/123pan-goctl/cmd/token"
	"github.com/no-mole/123pan-goctl/cmd/utils"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var (
	workers       uint8
	maxRetryTimes uint8
	errLocker     = &sync.Mutex{}
	errorUploads  []string
	totalFiles    int64
)

var wg = &sync.WaitGroup{}
var ch = make(chan string, 256)

// fileUploadCmd represents the fileUpload command
var fileUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "file upload file... target",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if maxRetryTimes < 1 || workers < 1 {
			return errors.New("args error")
		}
		if len(args) < 2 {
			return errors.New("args not enough")
		}
		filepaths := args[:len(args)-1]
		target := args[len(args)-1]

		for i := 0; i < int(workers); i++ {
			go func() {
				for uploadFilepath := range ch {
					atomic.AddInt64(&totalFiles, 1)
					var err error
					for i := 0; i < int(maxRetryTimes); i++ {
						err = uploadFile(0, uploadFilepath, target)
						if err != nil {
							utils.Logger.Error("upload file error,retrying", zap.String("filepath", uploadFilepath), zap.Error(err))
							continue
						}
						break
					}
					wg.Done()
					if err != nil {
						errLocker.Lock()
						errorUploads = append(errorUploads, uploadFilepath)
						errLocker.Unlock()
					}
				}
			}()
		}
		for _, fp := range filepaths {
			info, err := os.Stat(fp)
			if err != nil {
				utils.Logger.Error("open file stat error", zap.Error(err))
				return err
			}
			if !info.IsDir() {
				wg.Add(1)
				ch <- fp
				continue
			}
			err = filepath.Walk(fp, func(path string, info fs.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				wg.Add(1)
				ch <- path
				return nil
			})
			if err != nil {
				return err
			}
		}
		wg.Wait()
		utils.Logger.Info("all files uploaded",
			zap.Int64("totalFiles", totalFiles),
			zap.Int("totalFailed", len(errorUploads)),
		)
		if len(errorUploads) > 0 {
			utils.Logger.Warn("upload failed file list", zap.Strings("failedFiles", errorUploads))
		}
		return nil
	},
}

func init() {
	FileCommand.AddCommand(fileUploadCmd)
	fileUploadCmd.Flags().Uint8VarP(&workers, "workers", "w", 2, "concurrent upload thread number,default 2")
	fileUploadCmd.Flags().Uint8VarP(&maxRetryTimes, "maxRetryTimes", "r", 3, "file upload max retry times,default 3")
}

type UploadInitReq struct {
	ParentFileID int64  `json:"parentFileID"`
	Filename     string `json:"filename"`
	Etag         string `json:"etag"`
	Size         int64  `json:"size"`
	//重复文件处理策略 1保留两者，新文件名将自动添加后缀，2覆盖原文件 ，默认2
	Duplicate int64 `json:"duplicate"`
	// 上传文件是否包含路径，默认fasle
	ContainDir bool `json:"containDir"`
}
type UploadInitResp struct {
	Reuse       bool   `json:"reuse"`
	PreuploadID string `json:"preuploadID"`
	SliceSize   int64  `json:"sliceSize"`
	FileID      int64  `json:"fileID"`
}

type UploadPart struct {
	PartNumber int64  `json:"partNumber"`
	Size       int64  `json:"size"`
	ETag       string `json:"etag"`
}

type GetUploadUrlReq struct {
	PreuploadID string `json:"preuploadID"`
	SliceNo     int64  `json:"sliceNo"`
}

type GetUploadUrlResp struct {
	PresignedURL string `json:"presignedURL"`
}

type CompleteReq struct {
	PreuploadID string `json:"preuploadID"`
}

type CompleteResp struct {
	Async     bool `json:"async"`
	Completed bool `json:"completed"`
	FileID    int  `json:"fileID"`
}

func uploadFile(parent int64, filepath string, target string) error {
	targetFile := path.Join(target, filepath)
	logger := utils.Logger.Sugar()
	logger = logger.With(
		zap.String("filepath", filepath),
		zap.String("target", target),
		zap.String("targetFilepath", targetFile),
	)
	logger.Info("start upload")
	file, err := os.Open(filepath)
	if err != nil {
		return terrors.New(terrors.FileOpenError, err)
	}
	defer file.Close()

	r := bufio.NewReader(file)

	hash := md5.New()
	_, err = io.Copy(hash, r)
	if err != nil {
		return terrors.New(terrors.GenFileMD5Error, err)
	}
	etag := hex.EncodeToString(hash.Sum(nil))

	stat, _ := file.Stat()

	tkn, err := token.GetAccessToken()
	if err != nil {
		return terrors.New(terrors.GetAccessTokenError, err)
	}

	uploadInitReq := &UploadInitReq{
		ParentFileID: parent,
		Filename:     targetFile,
		Etag:         etag,
		Size:         stat.Size(),
		Duplicate:    2,
		ContainDir:   true,
	}

	logger.Infow("file upload init", "info", uploadInitReq)
	body, err := utils.Request().Method(http.MethodPost).Url(utils.CreateFileApi).Body(uploadInitReq).Token(tkn).Do()
	if err != nil {
		return terrors.New(terrors.FileUploadTaskError, err)
	}
	var uploadInitResp *UploadInitResp
	err = json.Unmarshal(body, &uploadInitResp)
	if err != nil {
		logger.Error("unmarshal resp body err", zap.ByteString("body", body))
		return terrors.New(terrors.FileUploadTaskError, err)
	}
	if uploadInitResp.Reuse {
		logger.Infow("reuse upload success", "fileID", uploadInitResp.FileID)
		return nil
	}

	totalSlices := int64(math.Ceil(float64(stat.Size() / uploadInitResp.SliceSize)))
	logger = logger.With(
		zap.String("totalSize", humanize.Bytes(uint64(stat.Size()))),
		zap.String("sliceSize", humanize.Bytes(uint64(uploadInitResp.SliceSize))),
		zap.Int64("totalSlices", totalSlices),
	)
	logger.Info("start upload")

	//前边计算hash已经read all，重置偏移
	_, err = file.Seek(0, 0)
	if err != nil {
		return terrors.New(terrors.FileSeekError, err)
	}

	//文件可能小于分片大小，所以两者取min
	buffer := make([]byte, min(stat.Size(), uploadInitResp.SliceSize))
	var sliceNo int64
	for {
		n, err := file.Read(buffer)
		if err == io.EOF || n == 0 {
			break
		}
		sliceNo++
		logger.Infow("upload file slice", "curSliceNo", sliceNo)
		getUploadUrlReq := GetUploadUrlReq{
			PreuploadID: uploadInitResp.PreuploadID,
			SliceNo:     sliceNo,
		}
		body, err := utils.Request().Method(http.MethodPost).Url(utils.GetUploadFileUrlApi).Body(getUploadUrlReq).Token(tkn).Do()
		if err != nil {
			return terrors.New(terrors.FileGetUploadUrlError, err)
		}
		var uploadURL GetUploadUrlResp
		err = json.Unmarshal(body, &uploadURL)
		if err != nil {
			logger.Error("unmarshal body err", zap.ByteString("body", body))
			return err
		}
		err = putPart(uploadURL.PresignedURL, bytes.NewReader(buffer[:n]), int64(n))
		if err != nil {
			return terrors.New(terrors.FileSliceUploadError, err)
		}
	}

	logger.Info("all slices uploaded,try to complete file upload")

	body, err = utils.Request().Method(http.MethodPost).Url(utils.UploadFileCompleteApi).Body(&CompleteReq{PreuploadID: uploadInitResp.PreuploadID}).Token(tkn).Do()
	if err != nil {
		logger.Error("fail to complete upload file", zap.Error(err))
		return err
	}
	var completedResp *CompleteResp
	err = json.Unmarshal(body, &completedResp)
	if err != nil {
		logger.Error("fail to get upload file complete status", zap.Error(err))
		return err
	}
	if completedResp == nil {
		err = errors.New("fail to complete upload file")
		logger.Error("fail to complete upload file", zap.ByteString("resp", body))
		return err
	}

	if !completedResp.Async && (completedResp.FileID == 0 || completedResp.Completed == false) {
		err = errors.New("fail to complete file")
		logger.Error(err.Error())
		return err
	}
	if completedResp.Completed || !completedResp.Async {
		logger.Infow("file upload complete", "fileID", completedResp.FileID)
		return nil
	}

	logger.Info("file upload completed,waiting for file ready")

	for i := 0; i < 100; i++ {
		time.Sleep(3 * time.Second)
		body, err = utils.Request().Method(http.MethodPost).Url(utils.UploadFileSyncResultApi).Body(&CompleteReq{PreuploadID: uploadInitResp.PreuploadID}).Token(tkn).Do()
		if err != nil {
			logger.Warn("fail to fetch upload result,retrying")
			continue
		}
		var resp struct {
			Completed bool `json:"completed"`
			FileID    int  `json:"fileID"`
		}
		err = json.Unmarshal(body, &resp)
		if err != nil {
			logger.Warn("fail to fetch upload result,retrying", zap.ByteString("raw", body))
			continue
		}
		if resp.Completed {
			logger.Infow("file upload success", "fileID", resp.FileID)
			return nil
		}
		logger.Info("file not ready")
	}
	err = terrors.New(terrors.FetchFileUploadSatusError, nil)
	return err
}

func putPart(url string, reader io.Reader, size int64) error {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = size
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("slice upload err,code is：%d", resp.StatusCode)
	}
	return nil
}
