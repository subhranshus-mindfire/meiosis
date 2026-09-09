package identity

import "errors"

var (
	ErrInvalidParams = errors.New("identity: invalid issue parameters")
	ErrTokenAltered  = errors.New("identity: capability token signature invalid")
	ErrTokenExpired  = errors.New("identity: capability token expired")
	ErrTokenRevoked  = errors.New("identity: capability token revoked")
	ErrScopeDenied   = errors.New("identity: capability token does not permit this path")
)
