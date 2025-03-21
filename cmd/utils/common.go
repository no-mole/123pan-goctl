package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	queryBuilder "github.com/google/go-querystring/query"
	"go.uber.org/zap"
	"io"
	"net/http"
	"time"
)

var (
	Logger       *zap.Logger
	ClientSecret string
	ClientId     string
)

const (
	AccessTokenApi          = "https://open-api.123pan.com/api/v1/access_token"
	CreateFileApi           = "https://open-api.123pan.com/upload/v1/file/create"
	GetUploadFileUrlApi     = "https://open-api.123pan.com/upload/v1/file/get_upload_url"
	UploadFileCompleteApi   = "https://open-api.123pan.com/upload/v1/file/upload_complete"
	UploadFileSyncResultApi = "https://open-api.123pan.com/upload/v1/file/upload_async_result"
	GetFIleListApi          = "https://open-api.123pan.com/upload/api/v2/file/list"
)

type Config struct {
	ClientId     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
}
type APIResponse struct {
	Code     int             `json:"code"`
	Message  string          `json:"message"`
	Data     json.RawMessage `json:"data"`
	XtraceID string          `json:"x-traceID"`
}

func DoRequest(method string, api string, query, data any, token string, headers http.Header) ([]byte, error) {
	client := http.DefaultClient

	if query != nil {
		q, err := queryBuilder.Values(query)
		if err != nil {
			return nil, err
		}
		api += "?" + q.Encode()
	}

	var buf *bytes.Buffer

	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, api, buf)
	if err != nil {
		return nil, err
	}

	if headers != nil {
		req.Header = headers
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Platform", "open_platform")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var apiResp APIResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, err
	}

	if apiResp.Code != 0 {
		return nil, fmt.Errorf("api err,code:%d,msg:%s,xtrace:%s", apiResp.Code, apiResp.Message, apiResp.XtraceID)
	}

	return apiResp.Data, nil
}
