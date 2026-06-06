package biz

type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func NewError(code int, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

var (
	ErrUserAlreadyExists   = NewError(10001, "user already exists")
	ErrInvalidCredentials  = NewError(10002, "invalid credentials")
	ErrUserNotFound        = NewError(10003, "user not found")
	ErrInvalidRefreshToken = NewError(10004, "invalid refresh token")
)
