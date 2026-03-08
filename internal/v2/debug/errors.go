package debug

import "errors"

var (
	ErrNilFixContext      = errors.New("fix context is nil")
	ErrNilLearnContext    = errors.New("learn context is nil")
	ErrNilValidateContext = errors.New("validate context is nil")
)
