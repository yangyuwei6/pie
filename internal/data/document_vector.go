package data

import (
	"context"

	"pie/internal/data/model"
)

type DocumentVectorRepo struct {
	data *Data
}

func NewDocumentVectorRepo(data *Data) *DocumentVectorRepo {
	return &DocumentVectorRepo{data: data}
}

func (r *DocumentVectorRepo) DeleteByFileMD5(ctx context.Context, fileMD5 string) error {
	dv := r.data.q.DocumentVector
	_, err := dv.WithContext(ctx).Where(dv.FileMd5.Eq(fileMD5)).Delete()
	return err
}

func (r *DocumentVectorRepo) BatchCreate(ctx context.Context, vectors []*model.DocumentVector) error {
	if len(vectors) == 0 {
		return nil
	}
	return r.data.q.DocumentVector.WithContext(ctx).CreateInBatches(vectors, 100)
}

func (r *DocumentVectorRepo) FindByFileMD5(ctx context.Context, fileMD5 string) ([]*model.DocumentVector, error) {
	dv := r.data.q.DocumentVector
	return dv.WithContext(ctx).Where(dv.FileMd5.Eq(fileMD5)).Order(dv.ChunkID).Find()
}
