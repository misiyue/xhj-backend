package open

import (
	"github.com/google/wire"
	v1 "github.com/gzydong/go-chat/internal/apis/handler/open/v1"
)

var ProviderSet = wire.NewSet(
	v1.NewIndex,
	v1.NewAuth,

	wire.Struct(new(V1), "*"),
)
