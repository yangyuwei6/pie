package search

type HybridSearchRequest struct {
	Query string `form:"query" binding:"required"`
	TopK  int    `form:"topK"`
}
