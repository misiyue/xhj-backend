package v1

import (
	"bytes"
	"context"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/repository/cache"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"

	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/service"
)

var _ web.IEmoticonHandler = (*Emoticon)(nil)

type Emoticon struct {
	RedisLock       *cache.RedisLock
	EmoticonRepo    *repo.Emoticon
	EmoticonService service.IEmoticonService
	Filesystem      filesystem.IFilesystem
}

// List 收藏列表
//
//	@Summary		表情包列表
//	@Description	获取用户自定义表情包列表
//	@Tags			表情包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.EmoticonListRequest	true	"列表请求"
//	@Success		200		{object}	web.EmoticonListResponse
//	@Router			/api/v1/emoticon/customize/list [post]
//	@Security		Bearer
func (c *Emoticon) List(ctx context.Context, req *web.EmoticonListRequest) (*web.EmoticonListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	resp := &web.EmoticonListResponse{
		Items: make([]*web.EmoticonItem, 0),
	}

	items, err := c.EmoticonRepo.GetCustomizeList(session.GetAuthID())
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		resp.Items = append(resp.Items, &web.EmoticonItem{
			EmoticonId: int32(item.Id),
			Url:        item.Url,
		})
	}

	return resp, nil
}

// Delete 删除收藏表情包
//
//	@Summary		删除表情包
//	@Description	移除自定义表情包
//	@Tags			表情包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.EmoticonDeleteRequest	true	"删除请求"
//	@Success		200		{object}	web.EmoticonDeleteResponse
//	@Router			/api/v1/emoticon/customize/delete [post]
//	@Security		Bearer
func (c *Emoticon) Delete(ctx context.Context, in *web.EmoticonDeleteRequest) (*web.EmoticonDeleteResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	return nil, c.EmoticonService.DeleteCollect(session.GetAuthID(), []int{int(in.GetEmoticonId())})
}

// Create 创建自定义表情包
//
//	@Summary		创建表情包
//	@Description	添加新的自定义表情包
//	@Tags			表情包
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.EmoticonCreateRequest	true	"创建请求"
//	@Success		200		{object}	web.EmoticonCreateResponse
//	@Router			/api/v1/emoticon/customize/create [post]
//	@Security		Bearer
func (c *Emoticon) Create(ctx context.Context, in *web.EmoticonCreateRequest) (*web.EmoticonCreateResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	m := &model.EmoticonItem{
		UserId:   session.GetAuthID(),
		Describe: "自定义表情包",
		Url:      in.Url,
	}

	if err := c.EmoticonRepo.Db.Create(m).Error; err != nil {
		return nil, err
	}

	return &web.EmoticonCreateResponse{
		EmoticonId: int32(m.Id),
		Url:        m.Url,
	}, nil
}

// Upload 上传自定义表情包
//
//	@Summary		上传表情包
//	@Description	上传文件以创建新的自定义表情包
//	@Tags			表情包
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"表情包文件"
//	@Success		200		{object}	web.EmoticonUploadResponse
//	@Router			/api/v1/emoticon/customize/upload [post]
//	@Security		Bearer
func (c *Emoticon) Upload(ginContext *gin.Context, _ *web.EmoticonUploadRequest) (*web.EmoticonUploadResponse, error) {
	ctx := ginContext.Request.Context()
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	file, err := ginContext.FormFile("file")
	if err != nil {
		return nil, errorx.New(400, "file 字段必传")
	}

	if !slices.Contains([]string{"png", "jpg", "jpeg", "gif"}, strutil.FileSuffix(file.Filename)) {
		return nil, errorx.New(400, "上传文件格式不正确,仅支持 png、jpg、jpeg 和 gif")
	}

	// 判断上传文件大小（5M）
	if file.Size > 5<<20 {
		return nil, errorx.New(400, "上传文件大小不能超过5M")
	}

	stream, err := filesystem.ReadMultipartStream(file)
	if err != nil {
		return nil, err
	}

	meta := utils.ReadImageMeta(bytes.NewReader(stream))

	src := strutil.GenMediaObjectName(file.Filename, meta.Width, meta.Height)
	if err = c.Filesystem.Write(c.Filesystem.BucketPublicName(), src, stream); err != nil {
		return nil, err
	}

	m := &model.EmoticonItem{
		UserId:   session.GetAuthID(),
		Describe: "自定义表情包",
		Url:      c.Filesystem.PublicUrl(c.Filesystem.BucketPublicName(), src),
	}

	if err := c.EmoticonRepo.Db.Create(m).Error; err != nil {
		return nil, err
	}

	return &web.EmoticonUploadResponse{
		EmoticonId: int32(m.Id),
		Url:        m.Url,
	}, nil
}
