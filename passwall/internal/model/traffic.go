package model

import (
	"time"
)

// TrafficStatistics 流量统计
type TrafficStatistics struct {
	ID            uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	ProxyID       uint      `json:"proxy_id" gorm:"uniqueIndex;not null"`
	DownloadTotal int64     `json:"download_total"` // 下载流量(bytes)
	UploadTotal   int64     `json:"upload_total"`   // 上传流量(bytes)
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
