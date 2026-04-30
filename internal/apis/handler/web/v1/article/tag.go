package article

import (
	"github.com/gzydong/go-chat/api/pb/web/v1"
	"github.com/gzydong/go-chat/internal/pkg/core"
	"github.com/gzydong/go-chat/internal/service"
)

type Tag struct {
	ArticleTagService service.IArticleTagService
}

// List 标签列表
//
//	@Summary		笔记标签列表
//	@Description	获取当前用户的笔记标签列表
//	@Tags			笔记标签
//	@Accept			json
//	@Produce		json
//	@Success		200		{object}	web.ArticleTagListResponse
//	@Router			/api/v1/article/tag/list [get]
//	@Security		Bearer
func (c *Tag) List(ctx *core.Context) error {

	list, err := c.ArticleTagService.List(ctx.GetContext(), ctx.AuthId())
	if err != nil {
		return ctx.Error(err)
	}

	items := make([]*web.ArticleTagListResponse_Item, 0, len(list))
	for _, item := range list {
		items = append(items, &web.ArticleTagListResponse_Item{
			Id:      int32(item.Id),
			TagName: item.TagName,
			Count:   int32(item.Count),
		})
	}

	return ctx.Success(&web.ArticleTagListResponse{Tags: items})
}

// Edit 添加或修改标签
//
//	@Summary		编辑笔记标签
//	@Description	创建或更新笔记标签
//	@Tags			笔记标签
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleTagEditRequest	true	"编辑标签请求"
//	@Success		200		{object}	web.ArticleTagEditResponse
//	@Router			/api/v1/article/tag/edit [post]
//	@Security		Bearer
func (c *Tag) Edit(ctx *core.Context) error {

	var (
		err error
		in  = &web.ArticleTagEditRequest{}
		uid = ctx.AuthId()
	)

	if err = ctx.Context.ShouldBindJSON(in); err != nil {
		return ctx.InvalidParams(err)
	}

	if in.TagId == 0 {
		id, err := c.ArticleTagService.Create(ctx.GetContext(), uid, in.TagName)
		if err == nil {
			in.TagId = int32(id)
		}
	} else {
		err = c.ArticleTagService.Update(ctx.GetContext(), uid, int(in.TagId), in.TagName)
	}

	if err != nil {
		return ctx.Error(err)
	}

	return ctx.Success(&web.ArticleTagEditResponse{TagId: in.TagId})
}

// Delete 删除标签
//
//	@Summary		删除笔记标签
//	@Description	移除笔记标签
//	@Tags			笔记标签
//	@Accept			json
//	@Produce		json
//	@Param			request	body		web.ArticleTagDeleteRequest	true	"删除标签请求"
//	@Success		200		{object}	web.ArticleTagDeleteResponse
//	@Router			/api/v1/article/tag/delete [post]
//	@Security		Bearer
func (c *Tag) Delete(ctx *core.Context) error {

	in := &web.ArticleTagDeleteRequest{}
	if err := ctx.Context.ShouldBindJSON(in); err != nil {
		return ctx.InvalidParams(err)
	}

	err := c.ArticleTagService.Delete(ctx.GetContext(), ctx.AuthId(), int(in.TagId))
	if err != nil {
		return ctx.Error(err)
	}

	return ctx.Success(&web.ArticleTagDeleteResponse{})
}
