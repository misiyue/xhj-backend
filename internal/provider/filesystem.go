package provider

import (
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/filesystem"
)

func NewFilesystem(conf *config.Config) filesystem.IFilesystem {
	if conf.Filesystem.Default == filesystem.MinioDriver {
		return filesystem.NewMinioFilesystem(conf.Filesystem.Minio)
	}

	if conf.Filesystem.Default == filesystem.AwsDriver {
		return filesystem.NewAwsFilesystem(conf.Filesystem.Aws)
	}

	if conf.Filesystem.Default == filesystem.CosDriver {
		return filesystem.NewCosFilesystem(conf.Filesystem.Cos)
	}

	if conf.Filesystem.Default == filesystem.OssDriver {
		return filesystem.NewOssFilesystem(conf.Filesystem.Oss)
	}

	return filesystem.NewLocalFilesystem(conf.Filesystem.Local)
}
