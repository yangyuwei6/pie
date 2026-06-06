package data

import (
	"context"

	"pie/internal/data/model"
)

type OrgTagRepo struct {
	data *Data
}

func NewOrgTagRepo(data *Data) *OrgTagRepo {
	return &OrgTagRepo{data: data}
}

func (r *OrgTagRepo) Create(ctx context.Context, orgTag *model.OrganizationTag) error {
	return r.data.q.OrganizationTag.WithContext(ctx).Create(orgTag)
}

func (r *OrgTagRepo) FindByID(ctx context.Context, tagID string) (*model.OrganizationTag, error) {
	o := r.data.q.OrganizationTag

	return o.WithContext(ctx).Where(o.TagID.Eq(tagID)).First()
}
