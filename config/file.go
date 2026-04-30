package config

import "github.com/gzydong/go-chat/internal/pkg/filesystem"

type Filesystem struct {
	Default string                       `json:"default" yaml:"default"`
	Local   filesystem.LocalSystemConfig `json:"local" yaml:"local"`
	Minio   filesystem.MinioSystemConfig `json:"minio" yaml:"minio"`
	Aws     filesystem.AwsSystemConfig   `json:"aws" yaml:"aws"`
	Cos     filesystem.CosSystemConfig   `json:"cos" yaml:"cos"`
	Oss     filesystem.OssSystemConfig   `json:"oss" yaml:"oss"`
}
