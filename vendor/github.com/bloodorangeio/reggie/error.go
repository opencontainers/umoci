package reggie

type (
	ErrorResponse struct {
		Errors []ErrorInfo `json:"errors"`
	}

	// ErrorInfo describes a server error returned from a registry.
	ErrorInfo struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Detail  interface{} `json:"detail"`
	}
)
