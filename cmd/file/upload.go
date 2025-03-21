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
	"time"
)

var workers uint8

var wg = &sync.WaitGroup{}
var ch = make(chan string, 256)

// fileUploadCmd represents the fileUpload command
var fileUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "file upload file... target",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return errors.New("args not enough")
		}
		filepaths := args[:len(args)-1]
		target := args[len(args)-1]

		for i := 0; i < int(workers); i++ {
			go func() {
				for path := range ch {
					utils.Logger.Info("file upload", zap.String("filepath", path), zap.String("target", target))
					err := uploadFile(0, path, target)
					if err != nil {
						utils.Logger.Error("upload file error", zap.Error(err))
					}
					wg.Done()
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
		return nil
	},
}

func init() {
	FileCommand.AddCommand(fileUploadCmd)

	fileUploadCmd.Flags().Uint8VarP(&workers, "workers", "w", 1, "concurrent upload thread number")
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

type CompleteResponse struct {
	Async     bool `json:"async"`
	Completed bool `json:"completed"`
	FileID    int  `json:"fileID"`
}

func uploadFile(parent int64, filepath string, target string) error {
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

	var containDir bool
	if filepath != stat.Name() && path.Dir(filepath) != "" {
		containDir = true
	}

	uploadInitReq := &UploadInitReq{
		ParentFileID: parent,
		Filename:     path.Join(target, filepath),
		Etag:         etag,
		Size:         stat.Size(),
		Duplicate:    2,
		ContainDir:   containDir,
	}
	utils.Logger.Info("file upload init", zap.Any("info", uploadInitReq))
	body, err := utils.DoRequest(utils.CreateFileApi, uploadInitReq, tkn)
	if err != nil {
		return terrors.New(terrors.FileUploadTaskError, err)
	}
	var uploadInitResp *UploadInitResp
	err = json.Unmarshal(body, &uploadInitResp)
	if err != nil {
		utils.Logger.Error("unmarshal resp body err", zap.ByteString("body", body))
		return terrors.New(terrors.FileUploadTaskError, err)
	}
	if uploadInitResp.Reuse {
		utils.Logger.Info("reuse upload success", zap.Int64("fileID", uploadInitResp.FileID))
		return nil
	}

	utils.Logger.Info("start upload", zap.Int64("totalSlices", int64(math.Ceil(float64(stat.Size()/uploadInitResp.SliceSize)))), zap.String("totalSize", humanize.Bytes(uint64(stat.Size()))), zap.String("sliceSize", humanize.Bytes(uint64(uploadInitResp.SliceSize))))

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
		utils.Logger.Info("upload file slice", zap.Any("sliceNo", sliceNo))
		urlReq := GetUploadUrlReq{
			PreuploadID: uploadInitResp.PreuploadID,
			SliceNo:     sliceNo,
		}
		body, err := utils.DoRequest(utils.GetUploadFileUrlApi, urlReq, tkn)
		if err != nil {
			return terrors.New(terrors.FileGetUploadUrlError, err)
		}
		var uploadURL GetUploadUrlResp
		err = json.Unmarshal(body, &uploadURL)
		if err != nil {
			utils.Logger.Error("unmarshal body err", zap.ByteString("body", body))
			return err
		}
		err = putPart(uploadURL.PresignedURL, bytes.NewReader(buffer[:n]), int64(n))
		if err != nil {
			return terrors.New(terrors.FileSliceUploadError, err)
		}
	}

	utils.Logger.Info("all slices uploaded,try to complete file upload")

	body, err = utils.DoRequest(utils.UploadFileCompleteApi, map[string]interface{}{
		"preuploadID": uploadInitResp.PreuploadID,
	}, tkn)
	if err != nil {
		utils.Logger.Error("fail to complete upload file", zap.Error(err))
		return err
	}
	var completedResp *CompleteResponse
	err = json.Unmarshal(body, &completedResp)
	if err != nil {
		utils.Logger.Error("fail to get upload file complete status", zap.String("filepath", filepath), zap.Error(err))
		return err
	}
	if completedResp == nil {
		err = errors.New("fail to complete upload file")
		utils.Logger.Error("fail to complete upload file", zap.ByteString("resp", body))
		return err
	}

	if !completedResp.Async && (completedResp.FileID == 0 || completedResp.Completed == false) {
		err = errors.New("fail to complete file")
		utils.Logger.Error(err.Error())
		return err
	}
	if completedResp.Completed || !completedResp.Async {
		utils.Logger.Info("file upload complete", zap.Int("fileID", completedResp.FileID))
		return nil
	}

	utils.Logger.Info("file upload completed,waiting for file ready")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for i := 0; i < 60; i++ {
		<-ticker.C
		body, err := utils.DoRequest(utils.UploadFileSyncResultApi, map[string]interface{}{
			"preuploadID": uploadInitResp.PreuploadID,
		}, tkn)
		if err != nil {
			utils.Logger.Warn("fail to fetch upload result,retrying")
			continue
		}
		var resp struct {
			Completed bool `json:"completed"`
			FileID    int  `json:"fileID"`
		}
		err = json.Unmarshal(body, &resp)
		if err != nil {
			utils.Logger.Warn("fail to fetch upload result,retrying", zap.ByteString("raw", body))
			continue
		}
		if resp.Completed {
			utils.Logger.Info("file upload success", zap.Int("fileID", resp.FileID))
			return nil
		}
		utils.Logger.Info("file not ready")
	}
	utils.Logger.Warn("fetch upload result timeout")
	return nil
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
