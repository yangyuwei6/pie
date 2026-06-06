package repo

import (
	"context"

	"pie/internal/data/model"
)

type OrgTagRepo interface {
	Create(ctx context.Context, orgTag *model.OrganizationTag) error
	FindByID(ctx context.Context, tagID string) (*model.OrganizationTag, error)
}
