package reggie

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
)

type (
	// Response is an HTTP response returned from an OCI registry.
	Response struct {
		*resty.Response
	}
)

// GetRelativeLocation returns the path component of the URL contained
// in the `Location` header of the response.
func (resp *Response) GetRelativeLocation() string {
	loc := resp.Header().Get("Location")
	u, err := url.Parse(loc)
	if err != nil {
		return ""
	}

	path := u.Path
	if q := u.RawQuery; q != "" {
		path += "?" + q
	}

	return path
}

// GetAbsoluteLocation returns the full URL, including protocol and host,
// of the location contained in the `Location` header of the response.
func (resp *Response) GetAbsoluteLocation() string {
	return resp.Header().Get("Location")
}

// IsUnauthorized returns whether or not the response is a 401
func (resp *Response) IsUnauthorized() bool {
	return resp.StatusCode() == http.StatusUnauthorized
}

// Errors attempts to parse a response as OCI-compliant errors array
func (resp *Response) Errors() ([]ErrorInfo, error) {
	errorResponse := &ErrorResponse{}
	bodyBytes := []byte(resp.String())
	err := json.Unmarshal(bodyBytes, errorResponse)
	if err != nil {
		return nil, err
	} else if len(errorResponse.Errors) == 0 {
		return nil, errors.New("body was valid json but could not be parsed")
	}
	errorList := []ErrorInfo{}
	for _, errorInfo := range errorResponse.Errors {
		errorList = append(errorList, errorInfo)
	}
	return errorList, nil
}
