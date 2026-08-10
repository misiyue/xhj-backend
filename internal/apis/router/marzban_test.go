package router

import (
	"testing"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/apis/handler/web"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPatchMarzbanDeps(t *testing.T) {
	conf := &config.Config{Marzban: &config.Marzban{BaseURL: "https://example.com"}}
	handler := &web.Handler{V1: &web.V1{}}

	patchMarzbanDeps(conf, handler)

	require.NotNil(t, handler.V1.Marzban)
	marzbanService, ok := handler.V1.Marzban.MarzbanService.(*service.MarzbanService)
	require.True(t, ok)
	require.Same(t, conf, marzbanService.Config)
	require.NotNil(t, marzbanService.HTTPClient)
}
