package open

import (
	v1 "github.com/gzydong/go-chat/internal/apis/handler/open/v1"
)

type V1 struct {
	Index *v1.Index
	Auth  *v1.Auth
}

type Handler struct {
	V1 *V1
}
