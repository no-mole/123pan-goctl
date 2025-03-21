package token

import (
	"encoding/json"
	"errors"
	"github.com/no-mole/123pan-goctl/cmd/utils"
	"net/http"
	"sync"
)

type TokenRequest struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
}
type TokenResponse struct {
	AccessToken string `json:"accessToken"`
}

var token string
var mux = &sync.Mutex{}

func GetAccessToken() (string, error) {
	mux.Lock()
	defer mux.Unlock()
	if token != "" {
		return token, nil
	}
	data, err := utils.DoRequest(http.MethodPost, utils.AccessTokenApi, nil, &TokenRequest{
		ClientID:     utils.ClientId,
		ClientSecret: utils.ClientSecret}, "", nil)
	if err != nil {
		return "", err
	}
	var tokenResp TokenResponse
	err = json.Unmarshal(data, &tokenResp)
	if err != nil {
		return "", err
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("access token empty")
	}
	token = tokenResp.AccessToken
	return token, nil
}
