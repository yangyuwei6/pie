package message

type FileProcessing struct {
	FileMD5   string `json:"file_md5"`
	ObjectURL string `json:"object_url"`
	FileName  string `json:"file_name"`
	UserID    int64  `json:"user_id"`
	OrgTag    string `json:"org_tag"`
	IsPublic  bool   `json:"is_public"`
}
