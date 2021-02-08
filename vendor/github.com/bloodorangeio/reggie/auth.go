package reggie

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
)

var (
	authHeaderMatcher = regexp.MustCompile("(?i).*(bearer|basic).*")
)

type (
	authHeader struct {
		Realm   string
		Service string
		Scope   string
	}

	authInfo struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
)

func (client *Client) retryRequestWithAuth(originalRequest *Request, originalResponse *Response) (*Response, error) {
	authHeaderRaw := originalResponse.Header().Get("Www-Authenticate")
	if authHeaderRaw == "" {
		return originalResponse, nil
	}

	for k, _ := range originalRequest.QueryParam {
		originalRequest.QueryParam.Del(k)
	}

	if originalRequest.retryCallback != nil {
		err := originalRequest.retryCallback(originalRequest)
		if err != nil {
			return nil, fmt.Errorf("retry callback returned error: %s", err)
		}
	}

	authenticationType := authHeaderMatcher.ReplaceAllString(authHeaderRaw, "$1")
	if strings.EqualFold(authenticationType, "bearer") {
		h := parseAuthHeader(authHeaderRaw)
		req := client.Client.NewRequest().
			SetQueryParam("service", h.Service).
			SetHeader("Accept", "application/json").
			SetHeader("User-Agent", client.Config.UserAgent).
			SetBasicAuth(client.Config.Username, client.Config.Password)
		if s := client.Config.AuthScope; s != "" {
			req.SetQueryParam("scope", s)
		} else if h.Scope != "" {
			req.SetQueryParam("scope", h.Scope)
		}

		authResp, err := req.Execute(GET, h.Realm)
		if err != nil {
			return nil, err
		}

		var info authInfo
		bodyBytes := authResp.Body()
		err = json.Unmarshal(bodyBytes, &info)
		if err != nil {
			return nil, err
		}

		token := info.Token
		if token == "" {
			token = info.AccessToken
		}
		originalRequest.SetAuthToken(token)
		return originalRequest.Execute(originalRequest.Method, originalRequest.URL)
	} else if strings.EqualFold(authenticationType, "basic") {
		originalRequest.SetBasicAuth(client.Config.Username, client.Config.Password)
		return originalRequest.Execute(originalRequest.Method, originalRequest.URL)
	}

	return nil, errors.New("something went wrong with authorization")
}

func parseAuthHeader(authHeaderRaw string) *authHeader {
	re := regexp.MustCompile(`([a-zA-z]+)="(.+?)"`)
	matches := re.FindAllStringSubmatch(authHeaderRaw, -1)
	m := make(map[string]string)
	for i := 0; i < len(matches); i++ {
		m[matches[i][1]] = matches[i][2]
	}
	var h authHeader
	mapstructure.Decode(m, &h)
	return &h
}
