package article

import (
	"context"
	"html"
	"math"
	"time"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
	"github.com/gzydong/go-chat/internal/pkg/sliceutil"
	"github.com/gzydong/go-chat/internal/pkg/timeutil"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

var _ web.IArticleHandler = (*Article)(nil)

type Article struct {
	Source              *repo.Source
	ArticleAnnexRepo    *repo.ArticleAnnex
	ArticleClassRepo    *repo.ArticleClass
	ArticleRepo         *repo.Article
	ArticleService      service.IArticleService
	ArticleAnnexService service.IArticleAnnexService
	Filesystem          filesystem.IFilesystem
}

// Edit 文章编辑接口
//
//	@Summary		编辑笔记
//	@Description	创建或更新笔记
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleEditRequest	true	"编辑笔记请求"
//	@Success		200		{object}	web.ArticleEditResponse
//	@Router			/api/v1/article/editor [post]
//	@Security		Bearer
func (a *Article) Edit(ctx context.Context, in *web.ArticleEditRequest) (*web.ArticleEditResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	uid := session.GetAuthID()

	opt := &service.ArticleEditOpt{
		UserId:    uid,
		ArticleId: int(in.ArticleId),
		ClassId:   int(in.ClassifyId),
		Title:     in.Title,
		MdContent: in.MdContent,
	}

	if in.ArticleId == 0 {
		id, err := a.ArticleService.Create(ctx, opt)
		if err == nil {
			in.ArticleId = int32(id)
		}
	} else {
		err := a.ArticleService.Update(ctx, opt)
		if err != nil {
			return nil, err
		}
	}

	var info *model.Article
	if err := a.Source.Db().First(&info, in.ArticleId).Error; err != nil {
		return nil, err
	}

	return &web.ArticleEditResponse{
		ArticleId: int32(info.Id),
		Title:     info.Title,
		Abstract:  info.Abstract,
		Image:     info.Image,
	}, nil
}

// Detail 获取文章详情接口
//
//	@Summary		笔记详情
//	@Description	获取笔记的详细信息
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleDetailRequest	true	"笔记详情请求"
//	@Success		200		{object}	web.ArticleDetailResponse
//	@Router			/api/v1/article/detail [post]
//	@Security		Bearer
func (a *Article) Detail(ctx context.Context, in *web.ArticleDetailRequest) (*web.ArticleDetailResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	uid := session.GetAuthID()

	detail, err := a.ArticleService.Detail(ctx, uid, int(in.ArticleId))
	if err != nil {
		return nil, err
	}

	tags := make([]*web.ArticleDetailResponse_Tag, 0)
	for _, id := range sliceutil.ParseIds(detail.TagsId) {
		tags = append(tags, &web.ArticleDetailResponse_Tag{Id: int32(id)})
	}

	files := make([]*web.ArticleDetailResponse_AnnexFile, 0)
	items, err := a.ArticleAnnexRepo.AnnexList(ctx, uid, int(in.ArticleId))
	if err == nil {
		for _, item := range items {
			files = append(files, &web.ArticleDetailResponse_AnnexFile{
				AnnexId:   int32(item.Id),
				AnnexName: item.OriginalName,
				AnnexSize: int32(item.Size),
				CreatedAt: timeutil.FormatDatetime(item.CreatedAt),
			})
		}
	}

	return &web.ArticleDetailResponse{
		ArticleId:  int32(detail.Id),
		ClassifyId: int32(detail.ClassId),
		Title:      detail.Title,
		MdContent:  html.UnescapeString(detail.MdContent),
		IsAsterisk: int32(detail.IsAsterisk),
		CreatedAt:  timeutil.FormatDatetime(detail.CreatedAt),
		UpdatedAt:  timeutil.FormatDatetime(detail.UpdatedAt),
		TagIds:     tags,
		AnnexList:  files,
	}, nil
}

// List 获取文章列表接口
//
//	@Summary		笔记列表
//	@Description	获取带有分页和过滤条件的笔记列表
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleListRequest	true	"笔记列表请求"
//	@Success		200		{object}	web.ArticleListResponse
//	@Router			/api/v1/article/list [post]
//	@Security		Bearer
func (a *Article) List(ctx context.Context, in *web.ArticleListRequest) (*web.ArticleListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)

	uid := session.GetAuthID()

	items, err := a.ArticleService.List(ctx, &service.ArticleListOpt{
		UserId:     uid,
		FindType:   int(in.FindType),
		Keyword:    in.Keyword,
		ClassifyId: int(in.ClassifyId),
		TagId:      int(in.TagId),
	})
	if err != nil {
		return nil, err
	}

	list := make([]*web.ArticleListResponse_Item, 0)
	for _, item := range items {
		list = append(list, &web.ArticleListResponse_Item{
			ArticleId:  int32(item.Id),
			ClassifyId: int32(item.ClassId),
			TagsId:     item.TagsId,
			Title:      item.Title,
			ClassName:  item.ClassName,
			Image:      item.Image,
			IsAsterisk: int32(item.IsAsterisk),
			Status:     int32(item.Status),
			CreatedAt:  timeutil.FormatDatetime(item.CreatedAt),
			UpdatedAt:  timeutil.FormatDatetime(item.UpdatedAt),
			Abstract:   item.Abstract,
		})
	}

	return &web.ArticleListResponse{
		Items: list,
	}, nil
}

// Delete 删除文章接口
//
//	@Summary		删除笔记
//	@Description	将笔记移至回收站
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleDeleteRequest	true	"删除笔记请求"
//	@Success		200		{object}	web.ArticleDeleteResponse
//	@Router			/api/v1/article/delete [post]
//	@Security		Bearer
func (a *Article) Delete(ctx context.Context, in *web.ArticleDeleteRequest) (*web.ArticleDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	err := a.ArticleService.UpdateStatus(ctx, uid, int(in.ArticleId), 2)
	if err != nil {
		return nil, err
	}

	return &web.ArticleDeleteResponse{}, nil
}

// Recover 恢复文章接口
//
//	@Summary		恢复笔记
//	@Description	从回收站恢复笔记
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleRecoverRequest	true	"恢复笔记请求"
//	@Success		200		{object}	web.ArticleRecoverResponse
//	@Router			/api/v1/article/recover [post]
//	@Security		Bearer
func (a *Article) Recover(ctx context.Context, in *web.ArticleRecoverRequest) (*web.ArticleRecoverResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	err := a.ArticleService.UpdateStatus(ctx, uid, int(in.ArticleId), 1)
	if err != nil {
		return nil, err
	}

	return &web.ArticleRecoverResponse{}, nil
}

// ForeverDelete 永久删除文章接口
//
//	@Summary		永久删除笔记
//	@Description	永久删除一篇笔记
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleForeverDeleteRequest	true	"永久删除请求"
//	@Success		200		{object}	web.ArticleForeverDeleteResponse
//	@Router			/api/v1/article/forever-delete [post]
//	@Security		Bearer
func (a *Article) ForeverDelete(ctx context.Context, in *web.ArticleForeverDeleteRequest) (*web.ArticleForeverDeleteResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := a.ArticleService.ForeverDelete(ctx, uid, int(in.ArticleId)); err != nil {
		return nil, err
	}

	return &web.ArticleForeverDeleteResponse{}, nil
}

// Move 移动文章分类接口
//
//	@Summary		移动笔记
//	@Description	修改笔记的分类
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleMoveRequest	true	"移动笔记请求"
//	@Success		200		{object}	web.ArticleMoveResponse
//	@Router			/api/v1/article/move [post]
//	@Security		Bearer
func (a *Article) Move(ctx context.Context, in *web.ArticleMoveRequest) (*web.ArticleMoveResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if err := a.ArticleService.Move(ctx, uid, int(in.ArticleId), int(in.ClassifyId)); err != nil {
		return nil, err
	}

	return &web.ArticleMoveResponse{}, nil
}

// Asterisk 收藏/取消收藏文章接口
//
//	@Summary		设置星标
//	@Description	将笔记添加至或从收藏夹中移除
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleAsteriskRequest	true	"星标请求"
//	@Success		200		{object}	web.ArticleAsteriskResponse
//	@Router			/api/v1/article/asterisk [post]
//	@Security		Bearer
func (a *Article) Asterisk(ctx context.Context, in *web.ArticleAsteriskRequest) (*web.ArticleAsteriskResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)
	if err := a.ArticleService.Asterisk(ctx, uid, int(in.ArticleId), int(in.Action)); err != nil {
		return nil, err
	}

	return &web.ArticleAsteriskResponse{}, nil
}

// SetTags 设置文章标签接口
//
//	@Summary		设置笔记标签
//	@Description	为笔记分配标签
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleTagsRequest	true	"设置标签请求"
//	@Success		200		{object}	web.ArticleTagsResponse
//	@Router			/api/v1/article/tags [post]
//	@Security		Bearer
func (a *Article) SetTags(ctx context.Context, in *web.ArticleTagsRequest) (*web.ArticleTagsResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	if err := a.ArticleService.Tag(ctx, uid, int(in.ArticleId), in.GetTagIds()); err != nil {
		return nil, err
	}

	return &web.ArticleTagsResponse{}, nil
}

// RecoverList 获取回收站文章列表接口
//
//	@Summary		回收站列表
//	@Description	获取回收站中已删除笔记的列表
//	@Tags			笔记
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleRecoverListRequest	true	"回收站列表请求"
//	@Success		200		{object}	web.ArticleRecoverListResponse
//	@Router			/api/v1/article/recover-list [post]
//	@Security		Bearer
func (a *Article) RecoverList(ctx context.Context, _ *web.ArticleRecoverListRequest) (*web.ArticleRecoverListResponse, error) {
	uid := middleware.FormContextAuthId[entity.WebClaims](ctx)

	items := make([]*web.ArticleRecoverListResponse_Item, 0)

	list, err := a.ArticleRepo.FindAll(ctx, func(db *gorm.DB) {
		db.Where("user_id = ? and status = ?", uid, 2)
		db.Where("deleted_at > ?", time.Now().Add(-time.Hour*24*30))
		db.Order("deleted_at desc,id desc")
	})

	if err != nil {
		return nil, err
	}

	classList, err := a.ArticleClassRepo.FindByIds(ctx, lo.Map(list, func(item *model.Article, index int) any {
		return item.ClassId
	}))

	if err != nil {
		return nil, err
	}

	classListMap := lo.KeyBy(classList, func(item *model.ArticleClass) int {
		return item.Id
	})

	for _, item := range list {
		className := ""

		if class, ok := classListMap[item.ClassId]; ok {
			className = class.ClassName
		}

		at := time.Until(item.DeletedAt.Time.Add(time.Hour * 24 * 30))

		items = append(items, &web.ArticleRecoverListResponse_Item{
			ArticleId:    int32(item.Id),
			ClassifyId:   int32(item.ClassId),
			ClassifyName: className,
			Title:        item.Title,
			Abstract:     item.Abstract,
			Image:        item.Image,
			CreatedAt:    item.CreatedAt.Format(time.DateTime),
			DeletedAt:    item.DeletedAt.Time.Format(time.DateTime),
			Day:          int32(math.Ceil(at.Seconds() / 86400)),
		})
	}

	return &web.ArticleRecoverListResponse{
		Items: items,
	}, nil
}
