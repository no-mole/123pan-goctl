package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func Request() *requestBuilder {
	return &requestBuilder{}
}

type requestBuilder struct {
	method      string
	url         string
	query, body any
	token       string
	headers     http.Header
}

func (r *requestBuilder) Method(method string) *requestBuilder {
	r.method = method
	return r
}

func (r *requestBuilder) Url(url string) *requestBuilder {
	r.url = url
	return r
}
func (r *requestBuilder) Query(query any) *requestBuilder {
	r.query = query
	return r
}
func (r *requestBuilder) Body(body any) *requestBuilder {
	r.body = body
	return r
}
func (r *requestBuilder) Token(token string) *requestBuilder {
	r.token = token
	return r
}
func (r *requestBuilder) Headers(headers http.Header) *requestBuilder {
	r.headers = headers
	return r
}

func (r *requestBuilder) Do() ([]byte, error) {
	if r.method == "" || r.url == "" {
		return nil, errors.New("request must set method and url")
	}

	client := http.DefaultClient
	if r.query != nil {
		q, err := queryBuilder.Values(r.query)
		if err != nil {
			return nil, err
		}
		r.url += "?" + q.Encode()
	}

	var buf *bytes.Buffer

	if r.body != nil {
		jsonData, err := json.Marshal(r.body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonData)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, r.method, r.url, buf)
	if err != nil {
		return nil, err
	}

	if r.headers != nil {
		req.Header = r.headers
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Platform", "open_platform")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
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
