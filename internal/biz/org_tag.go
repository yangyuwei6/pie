package biz

import (
	"context"
	"errors"

	"pie/internal/data/model"

	"gorm.io/gorm"
)

type OrgTagRepo interface {
	Create(ctx context.Context, orgTag *model.OrganizationTag) error
	FindByID(ctx context.Context, tagID string) (*model.OrganizationTag, error)
}

type OrgTagUsecase struct {
	orgTagRepo OrgTagRepo
}

func NewOrgTagUsecase(orgTagRepo OrgTagRepo) *OrgTagUsecase {
	return &OrgTagUsecase{orgTagRepo: orgTagRepo}
}

func (u *OrgTagUsecase) CreatePrivateOrgTag(ctx context.Context, userID int64, username string) (string, error) {
	privateOrgTag := "PRIVATE_" + username
	_, err := u.orgTagRepo.FindByID(ctx, privateOrgTag)
	if err == nil {
		return privateOrgTag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	description := "Private organization tag for this user."
	orgTag := &model.OrganizationTag{
		TagID:       privateOrgTag,
		Name:        username + " private space",
		Description: &description,
		CreatedBy:   userID,
	}
	if err := u.orgTagRepo.Create(ctx, orgTag); err != nil {
		return "", err
	}

	return privateOrgTag, nil
}
