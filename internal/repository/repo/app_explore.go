package repo

import (
	"context"
	"strings"

	"github.com/gzydong/go-chat/internal/repository/model"
	"gorm.io/gorm"
)

type AppExplore struct {
	db *gorm.DB
}

func NewAppExplore(db *gorm.DB) *AppExplore {
	return &AppExplore{db: db}
}

// ListOpen 返回已开放（is_open = 1）的探索位列表；positions 为逗号分隔的 position 筛选，空则不限
func (r *AppExplore) ListOpen(ctx context.Context, positions string) ([]*model.AppExplore, error) {
	q := r.db.WithContext(ctx).Where("is_open = ?", model.AppExploreOpenYes)

	if parts := splitPositions(positions); len(parts) > 0 {
		q = q.Where("position IN ?", parts)
	}

	var list []*model.AppExplore
	err := q.Order("`sort` DESC, id DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

func splitPositions(positions string) []string {
	if positions == "" {
		return nil
	}
	raw := strings.Split(positions, ",")
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}
