package apperr

import "fmt"

const (
	CodeInvalidArgs = "invalid_args"
	CodeConfig      = "config_error"
	CodeAuth        = "auth_error"
	CodeNotFound    = "not_found"
	CodeKaitenAPI   = "kaiten_api_error"
)

const (
	ExitOK          = 0
	ExitInvalidArgs = 2
	ExitConfig      = 3
	ExitAuth        = 4
	ExitNotFound    = 5
	ExitKaitenAPI   = 7
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
	Err     error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func New(code, message string, details any) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

func Wrap(code, message string, err error, details any) *Error {
	return &Error{Code: code, Message: message, Err: err, Details: details}
}

func ExitCode(err error) int {
	app, ok := err.(*Error)
	if !ok {
		return ExitKaitenAPI
	}
	switch app.Code {
	case CodeInvalidArgs:
		return ExitInvalidArgs
	case CodeConfig:
		return ExitConfig
	case CodeAuth:
		return ExitAuth
	case CodeNotFound:
		return ExitNotFound
	case CodeKaitenAPI:
		return ExitKaitenAPI
	default:
		return ExitKaitenAPI
	}
}
