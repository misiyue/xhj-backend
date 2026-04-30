package article

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/strutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
)

var _ web.IArticleAnnexHandler = (*Annex)(nil)

type Annex struct {
	ArticleAnnexRepo    *repo.ArticleAnnex
	ArticleAnnexService service.IArticleAnnexService
	Filesystem          filesystem.IFilesystem
}

// Upload 文章附件上传接口
//
//	@Summary		上传笔记附件
//	@Description	为笔记上传文件附件
//	@Tags			笔记附件
//	@Accept			mpfd
//	@Produce		json
//	@Param			article_id	formData	int		true	"笔记 ID"
//	@Param			annex		formData	file	true	"附件文件"
//	@Success		200			{object}	web.ArticleAnnexUploadResponse
//	@Router			/api/v1/article-annex/upload [post]
//	@Security		Bearer
func (a *Annex) Upload(ctx *gin.Context, _ *web.ArticleAnnexUploadRequest) (*web.ArticleAnnexUploadResponse, error) {
	in := &web.ArticleAnnexUploadRequest{}

	value := ctx.PostForm("article_id")
	if value == "" {
		return nil, errorx.New(400, "请选择文章")
	}

	id, _ := strconv.Atoi(value)
	if id <= 0 {
		return nil, errorx.New(400, "请选择文章")
	}

	in.ArticleId = int32(id)

	file, err := ctx.FormFile("annex")
	if err != nil {
		return nil, errorx.New(400, "annex 字段必传")
	}

	// 判断上传文件大小（10M）
	if file.Size > 10<<20 {
		return nil, errorx.New(400, "附件大小不能超过10M")
	}

	stream, err := filesystem.ReadMultipartStream(file)
	if err != nil {
		return nil, err
	}

	ext := strutil.FileSuffix(file.Filename)

	filePath := fmt.Sprintf("article-files/%s/%s", time.Now().Format("200601"), strutil.GenFileName(ext))
	if err := a.Filesystem.Write(a.Filesystem.BucketPrivateName(), filePath, stream); err != nil {
		return nil, err
	}

	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())
	data := &model.ArticleAnnex{
		UserId:       uid,
		ArticleId:    int(in.ArticleId),
		Drive:        entity.FileDriveMode(a.Filesystem.Driver()),
		Suffix:       ext,
		Size:         int(file.Size),
		Path:         filePath,
		OriginalName: file.Filename,
		Status:       1,
		DeletedAt: sql.NullTime{
			Valid: false,
		},
	}

	if err := a.ArticleAnnexService.Create(ctx, data); err != nil {
		return nil, err
	}

	return &web.ArticleAnnexUploadResponse{
		AnnexId:   int32(data.Id),
		AnnexSize: int32(data.Size),
		AnnexName: data.OriginalName,
		CreatedAt: data.CreatedAt.Format(time.DateTime),
	}, nil
}

// Delete 文章附件删除接口
//
//	@Summary		删除笔记附件
//	@Description	将笔记附件移至回收站
//	@Tags			笔记附件
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleAnnexDeleteRequest	true	"删除附件请求"
//	@Success		200		{object}	web.ArticleAnnexDeleteResponse
//	@Router			/api/v1/article-annex/delete [post]
//	@Security		Bearer
func (a *Annex) Delete(ctx context.Context, in *web.ArticleAnnexDeleteRequest) (*web.ArticleAnnexDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	err := a.ArticleAnnexService.UpdateStatus(ctx, uid, int(in.AnnexId), 2)
	if err != nil {
		return nil, err
	}

	return &web.ArticleAnnexDeleteResponse{}, nil
}

// Recover 文章附件恢复删除接口
//
//	@Summary		恢复笔记附件
//	@Description	从回收站恢复笔记附件
//	@Tags			笔记附件
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleAnnexRecoverRequest	true	"恢复附件请求"
//	@Success		200		{object}	web.ArticleAnnexRecoverResponse
//	@Router			/api/v1/article-annex/recover [post]
//	@Security		Bearer
func (a *Annex) Recover(ctx context.Context, req *web.ArticleAnnexRecoverRequest) (*web.ArticleAnnexRecoverResponse, error) {
	err := a.ArticleAnnexService.UpdateStatus(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx), int(req.AnnexId), 1)
	if err != nil {
		return nil, err
	}

	return &web.ArticleAnnexRecoverResponse{}, nil
}

// ForeverDelete 文章附件永久删除接口
//
//	@Summary		永久删除笔记附件
//	@Description	永久删除一个笔记附件
//	@Tags			笔记附件
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleAnnexForeverDeleteRequest	true	"永久删除请求"
//	@Success		200		{object}	web.ArticleAnnexForeverDeleteResponse
//	@Router			/api/v1/article-annex/forever-delete [post]
//	@Security		Bearer
func (a *Annex) ForeverDelete(ctx context.Context, req *web.ArticleAnnexForeverDeleteRequest) (*web.ArticleAnnexForeverDeleteResponse, error) {
	if err := a.ArticleAnnexService.ForeverDelete(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx), int(req.AnnexId)); err != nil {
		return nil, err
	}

	return &web.ArticleAnnexForeverDeleteResponse{}, nil
}

// Download 文章附件下载接口
//
//	@Summary		下载笔记附件
//	@Description	下载笔记附件
//	@Tags			笔记附件
//	@Accept			json
//	@Produce		octet-stream
//	@Param			annex_id	query		int	true	"附件 ID"
//	@Param			request		body		web.ArticleAnnexDownloadRequest	false	"下载请求"
//	@Success		200			{file}		binary
//	@Router			/api/v1/article-annex/download [get]
//	@Security		Bearer
func (a *Annex) Download(ctx *gin.Context, _ *web.ArticleAnnexDownloadRequest) (*web.ArticleAnnexDownloadResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx.Request.Context())

	in := &web.ArticleAnnexDownloadRequest{}
	if err := ctx.ShouldBind(in); err != nil {
		return nil, err
	}

	annexId, err := strconv.Atoi(ctx.DefaultQuery("annex_id", "0"))
	if err != nil {
		return nil, err
	}

	info, err := a.ArticleAnnexRepo.FindById(ctx, annexId)
	if err != nil {
		return nil, err
	}

	if info.UserId != uid {
		return nil, errorx.New(403, "无权限下载")
	}

	switch info.Drive {
	case entity.FileDriveLocal:
		if a.Filesystem.Driver() != filesystem.LocalDriver {
			return nil, errorx.New(400, "未知文件驱动类型")
		}

		filePath := a.Filesystem.(*filesystem.LocalFilesystem).Path(a.Filesystem.BucketPrivateName(), info.Path)
		ctx.FileAttachment(filePath, info.OriginalName)
	case entity.FileDriveMinio:
		ctx.Redirect(http.StatusFound, a.Filesystem.PrivateUrl(a.Filesystem.BucketPrivateName(), info.Path, info.OriginalName, 60*time.Second))
	default:
		return nil, errorx.New(400, "未知文件驱动类型")
	}

	return &web.ArticleAnnexDownloadResponse{}, nil
}

// RecoverList 文章附件回收站列表接口
//
//	@Summary		笔记附件回收站列表
//	@Description	获取回收站中已删除笔记附件的列表
//	@Tags			笔记附件
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleAnnexRecoverListRequest	true	"回收站列表请求"
//	@Success		200		{object}	web.ArticleAnnexRecoverListResponse
//	@Router			/api/v1/article-annex/recover-list [post]
//	@Security		Bearer
func (a *Annex) RecoverList(ctx context.Context, req *web.ArticleAnnexRecoverListRequest) (*web.ArticleAnnexRecoverListResponse, error) {
	items, err := a.ArticleAnnexRepo.RecoverList(ctx, middleware.FormContextAuthId[entity.WebClaims](ctx))
	if err != nil {
		return nil, err
	}

	data := make([]*web.ArticleAnnexRecoverListResponse_Item, 0)

	for _, item := range items {
		at := time.Until(item.DeletedAt.Add(time.Hour * 24 * 30))

		data = append(data, &web.ArticleAnnexRecoverListResponse_Item{
			AnnexId:      int32(item.Id),
			AnnexName:    item.OriginalName,
			ArticleId:    int32(item.ArticleId),
			ArticleTitle: item.Title,
			CreatedAt:    item.CreatedAt.Format(time.DateTime),
			DeletedAt:    item.DeletedAt.Format(time.DateTime),
			Day:          int32(math.Ceil(at.Seconds() / 86400)),
		})
	}

	return &web.ArticleAnnexRecoverListResponse{
		Items: data,
		Paginate: &web.Paginate{
			Page:  1,
			Size:  10000,
			Total: int32(len(data)),
		},
	}, nil
}
