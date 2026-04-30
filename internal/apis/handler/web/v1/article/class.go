package article

import (
	"context"

	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/core/errorx"
	"github.com/gzydong/go-chat/internal/pkg/core/middleware"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/service"
	"github.com/samber/lo"
)

var _ web.IArticleClassHandler = (*Class)(nil)

type Class struct {
	ArticleClassService service.IArticleClassService
}

// List 获取文章分类列表接口
//
//	@Summary		笔记分类列表
//	@Description	获取当前用户的笔记分类列表
//	@Tags			笔记分类
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleClassListRequest	true	"分类列表请求"
//	@Success		200		{object}	web.ArticleClassListResponse
//	@Router			/api/v1/article/classify/list [post]
//	@Security		Bearer
func (c Class) List(ctx context.Context, req *web.ArticleClassListRequest) (*web.ArticleClassListResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.GetAuthID()

	list, err := c.ArticleClassService.List(ctx, uid)
	if err != nil {
		return nil, err
	}

	items := make([]*web.ArticleClassListResponse_Item, 0, len(list))
	for _, item := range list {
		items = append(items, &web.ArticleClassListResponse_Item{
			Id:        int32(item.Id),
			ClassName: item.ClassName,
			IsDefault: int32(item.IsDefault),
			Count:     int32(item.Count),
		})
	}

	_, ok := lo.Find(list, func(item *model.ArticleClassItem) bool {
		return item.IsDefault == 1
	})

	if !ok {
		id, err := c.ArticleClassService.Create(ctx, uid, "默认分类", model.Yes)
		if err != nil {
			return nil, err
		}

		items = append(items, &web.ArticleClassListResponse_Item{
			Id:        int32(id),
			ClassName: "默认分类",
			IsDefault: model.Yes,
			Count:     0,
		})
	}

	return &web.ArticleClassListResponse{
		Items: items,
	}, nil
}

// Edit 文章分类编辑接口
//
//	@Summary		编辑笔记分类
//	@Description	创建或更新笔记分类
//	@Tags			笔记分类
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleClassEditRequest	true	"编辑分类请求"
//	@Success		200		{object}	web.ArticleClassEditResponse
//	@Router			/api/v1/article/classify/edit [post]
//	@Security		Bearer
func (c Class) Edit(ctx context.Context, in *web.ArticleClassEditRequest) (*web.ArticleClassEditResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.GetAuthID()

	if in.Name == "默认分类" {
		return nil, errorx.New(40001, "该分类名称禁止被创建/编辑")
	}

	if in.ClassifyId == 0 {
		id, err := c.ArticleClassService.Create(ctx, uid, in.Name, model.No)
		if err == nil {
			in.ClassifyId = int32(id)
		}
	} else {
		class, err := c.ArticleClassService.Find(ctx, int(in.ClassifyId))
		if err != nil {
			if utils.IsSqlNoRows(err) {
				return nil, entity.ErrNoteClassNotExist
			}

			return nil, err
		}

		if class.IsDefault == model.Yes {
			return nil, entity.ErrNoteClassDefaultNotAllow
		}

		err = c.ArticleClassService.Update(ctx, uid, int(in.ClassifyId), in.Name)
		if err != nil {
			return nil, err
		}
	}

	return &web.ArticleClassEditResponse{
		ClassifyId: in.ClassifyId,
	}, nil
}

// Delete 文章分类删除接口
//
//	@Summary		删除笔记分类
//	@Description	移除笔记分类
//	@Tags			笔记分类
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleClassDeleteRequest	true	"删除分类请求"
//	@Success		200		{object}	web.ArticleClassDeleteResponse
//	@Router			/api/v1/article/classify/delete [post]
//	@Security		Bearer
func (c Class) Delete(ctx context.Context, in *web.ArticleClassDeleteRequest) (*web.ArticleClassDeleteResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.GetAuthID()

	class, err := c.ArticleClassService.Find(ctx, int(in.ClassifyId))
	if err != nil {
		if utils.IsSqlNoRows(err) {
			return nil, entity.ErrNoteClassNotExist
		}

		return nil, err
	}

	if class.IsDefault == model.Yes {
		return nil, entity.ErrNoteClassDefaultNotDelete
	}

	err = c.ArticleClassService.Delete(ctx, uid, int(in.ClassifyId))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Sort 文章分类排序接口
//
//	@Summary		笔记分类排序
//	@Description	更新笔记分类的排序顺序
//	@Tags			笔记分类
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleClassSortRequest	true	"排序分类请求"
//	@Success		200		{object}	web.ArticleClassSortResponse
//	@Router			/api/v1/article/classify/sort [post]
//	@Security		Bearer
func (c Class) Sort(ctx context.Context, in *web.ArticleClassSortRequest) (*web.ArticleClassSortResponse, error) {
	session, _ := middleware.FormContext[entity.WebClaims](ctx)
	uid := session.UserId

	err := c.ArticleClassService.Sort(ctx, uid, in.ClassifyIds)
	if err != nil {
		return nil, err
	}

	return &web.ArticleClassSortResponse{}, nil
}
