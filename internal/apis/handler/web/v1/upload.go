package v1

import (
	"bytes"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/service"
)

type Upload struct {
	Config             *config.Config
	Filesystem         filesystem.IFilesystem
	SplitUploadService service.ISplitUploadService
}

// Image 图片上传
// Image 图片上传
//
//	@Summary		上传图片
//	@Description	上传媒体文件（图片）
//	@Tags			上传
//	@Accept			mpfd
//	@Produce		json
//	@Param			file	formData	file	true	"媒体文件"
//	@Param			width	formData	int		false	"宽度"
//	@Param			height	formData	int		false	"高度"
//	@Success		200		{object}	web.UploadImageResponse
//	@Router			/api/v1/upload/media-file [post]
//	@Security		Bearer
func (u *Upload) Image(ctx *gin.Context) (*web.UploadImageResponse, error) {
	file, err := ctx.FormFile("file")
	if err != nil {
		return nil, err
	}

	var (
		width, _  = strconv.Atoi(ctx.DefaultPostForm("width", "0"))
		height, _ = strconv.Atoi(ctx.DefaultPostForm("height", "0"))
	)

	stream, _ := filesystem.ReadMultipartStream(file)
	if width == 0 || height == 0 {
		meta := utils.ReadImageMeta(bytes.NewReader(stream))
		width = meta.Width
		height = meta.Height
	}

	object := strutil.GenMediaObjectName(file.Filename, width, height)
	if err := u.Filesystem.Write(u.Filesystem.BucketPublicName(), object, stream); err != nil {
		return nil, err
	}

	return &web.UploadImageResponse{
		Src: u.Filesystem.PublicUrl(u.Filesystem.BucketPublicName(), object),
	}, nil
}

// InitiateMultipart 批量上传初始化
// InitiateMultipart 批量上传初始化
//
//	@Summary		初始化分片上传
//	@Description	启动分片文件上传流程
//	@Tags			上传
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.UploadInitiateMultipartRequest	true	"初始化请求"
//	@Success		200		{object}	web.UploadInitiateMultipartResponse
//	@Router			/api/v1/upload/init-multipart [post]
//	@Security		Bearer
func (u *Upload) InitiateMultipart(ctx *gin.Context) (*web.UploadInitiateMultipartResponse, error) {
	in := &web.UploadInitiateMultipartRequest{}
	if err := ctx.ShouldBindJSON(in); err != nil {
		return nil, errorx.New(400, err.Error())
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	info, err := u.SplitUploadService.InitiateMultipartUpload(ctx, &service.MultipartInitiateOpt{
		Name:   in.FileName,
		Size:   in.FileSize,
		UserId: uid,
	})

	if err != nil {
		return nil, err
	}

	return &web.UploadInitiateMultipartResponse{
		UploadId:  info.UploadId,
		ShardSize: 5 << 20,
		ShardNum:  int32(math.Ceil(float64(in.FileSize) / float64(5<<20))),
	}, nil
}

// MultipartUpload 批量分片上传
// MultipartUpload 批量分片上传
//
//	@Summary		分片上传
//	@Description	上传分片文件的单个分片
//	@Tags			上传
//	@Accept			mpfd
//	@Produce		json
//	@Param			upload_id	formData	string	true	"上传 ID"
//	@Param			split_index	formData	int		true	"分片索引"
//	@Param			split_num	formData	int		true	"总分片数"
//	@Param			file		formData	file	true	"分片文件"
//	@Success		200			{object}	web.UploadMultipartResponse
//	@Router			/api/v1/upload/multipart [post]
//	@Security		Bearer
func (u *Upload) MultipartUpload(ctx *gin.Context) (*web.UploadMultipartResponse, error) {
	in := &web.UploadMultipartRequest{
		UploadId: ctx.PostForm("upload_id"),
	}

	splitIndex, err := strconv.Atoi(ctx.PostForm("split_index"))
	if err != nil {
		return nil, errorx.New(400, "split_index 不合法")
	}

	splitNum, err := strconv.Atoi(ctx.PostForm("split_num"))
	if err != nil {
		return nil, errorx.New(400, "split_num 不合法")
	}

	in.SplitIndex = int32(splitIndex)
	in.SplitNum = int32(splitNum)

	file, err := ctx.FormFile("file")
	if err != nil {
		return nil, errorx.New(400, "文件上传失败")
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	err = u.SplitUploadService.MultipartUpload(ctx.Request.Context(), &service.MultipartUploadOpt{
		UserId:     uid,
		UploadId:   in.UploadId,
		SplitIndex: int(in.SplitIndex),
		SplitNum:   int(in.SplitNum),
		File:       file,
	})
	if err != nil {
		return nil, err
	}

	if in.SplitIndex != in.SplitNum {
		return &web.UploadMultipartResponse{
			IsMerge: false,
		}, nil
	}

	return &web.UploadMultipartResponse{
		UploadId: in.UploadId,
		IsMerge:  true,
	}, nil
}
