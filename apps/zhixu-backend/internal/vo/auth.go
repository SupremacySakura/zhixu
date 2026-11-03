package vo

import "time"

type LoginResp struct {
	Token string `json:"token"`
}

// ProfileResp 当前登录用户信息（不含密码哈希）
type ProfileResp struct {
	ID           uint      `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	AvatarURL    string    `json:"avatar_url"`
	StorageLimit uint64    `json:"storage_limit"`
	StorageUsed  uint64    `json:"storage_used"`
	CreatedAt    time.Time `json:"created_at"`
}
