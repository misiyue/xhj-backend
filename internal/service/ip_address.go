package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/ipaddress"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/redis/go-redis/v9"
)

var _ IIpAddressService = (*IpAddressService)(nil)

const (
	ipInfoLegacyHash = "hash:ip_info"
	ipInfoKeyFmt     = "ip:info:%s"
	ipInfoTTL        = 7 * 24 * time.Hour
)

type IIpAddressService interface {
	FindAddress(ip string) (string, error)
}

type IpAddressService struct {
	*repo.Source
	Config          *config.Config
	IpAddressClient *ipaddress.Client
}

func (i *IpAddressService) FindAddress(ip string) (string, error) {
	if val, err := i.getCache(ip); err == nil {
		return val, nil
	}

	address, err := i.IpAddressClient.GetIpInfo(context.Background(), ip)
	if err != nil {
		return "", err
	}

	_ = i.setCache(ip, address)

	return address, nil
}

func (i *IpAddressService) getCache(ip string) (string, error) {
	ctx := context.TODO()
	val, err := i.Source.Redis().Get(ctx, fmt.Sprintf(ipInfoKeyFmt, ip)).Result()
	if err == nil {
		return val, nil
	}
	if err != redis.Nil {
		// 继续尝试旧 Hash
	}

	val, err = i.Source.Redis().HGet(ctx, ipInfoLegacyHash, ip).Result()
	if err != nil {
		return "", err
	}
	// 读时迁移到带 TTL 的独立 key，并从旧 Hash 摘除
	_ = i.setCache(ip, val)
	return val, nil
}

func (i *IpAddressService) setCache(ip string, value string) error {
	ctx := context.TODO()
	pipe := i.Source.Redis().Pipeline()
	pipe.Set(ctx, fmt.Sprintf(ipInfoKeyFmt, ip), value, ipInfoTTL)
	pipe.HDel(ctx, ipInfoLegacyHash, ip)
	_, err := pipe.Exec(ctx)
	return err
}
