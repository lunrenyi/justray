package domain

import "time"

type Traffic struct {
	UploadBytes   int64     `yaml:"upload_bytes,omitempty"`
	DownloadBytes int64     `yaml:"download_bytes,omitempty"`
	TotalBytes    int64     `yaml:"total_bytes,omitempty"`
	ExpiresAt     time.Time `yaml:"expires_at,omitempty"`
}
