package reggie

import (
	"github.com/go-resty/resty/v2"
)

const (
	// GET represents the HTTP GET method.
	GET = resty.MethodGet

	// PUT represents the HTTP PUT method.
	PUT = resty.MethodPut

	// PATCH represents the HTTP PATCH method.
	PATCH = resty.MethodPatch

	// DELETE represents the HTTP DELETE method.
	DELETE = resty.MethodDelete

	// POST represents the HTTP POST method.
	POST = resty.MethodPost

	// HEAD represents the HTTP HEAD method.
	HEAD = resty.MethodHead

	// OPTIONS represents the HTTP OPTIONS method.
	OPTIONS = resty.MethodOptions
)
