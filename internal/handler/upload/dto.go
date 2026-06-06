package upload

type CheckFileRequest struct {
	MD5 string `json:"md5" binding:"required"`
}

type UploadChunkRequest struct {
	FileMD5    string `form:"fileMd5" binding:"required"`
	FileName   string `form:"fileName" binding:"required"`
	TotalSize  int64  `form:"totalSize" binding:"required"`
	ChunkIndex int    `form:"chunkIndex" binding:"required"`
	OrgTag     string `form:"orgTag"`
	IsPublic   bool   `form:"isPublic"`
}
