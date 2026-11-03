package sts

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	stsclient "github.com/aliyun/alibaba-cloud-sdk-go/services/sts"

	"simple_tiktok/internal/platform/config"
)

// Credential STS 临时凭证
type Credential struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
	Expiration      time.Time
}

// Service 签发直传用的 STS 临时凭证
type Service struct {
	cfg config.RAMConfig
}

// New 创建 STS 服务，RAM 配置缺失时返回 nil 表示直传未启用
func New(cfg config.RAMConfig) *Service {
	if cfg.RegionId == "" || cfg.RoleArn == "" || cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil
	}
	return &Service{cfg: cfg}
}

// Assume 签发临时凭证，权限范围由 RAM 角色策略控制
func (s *Service) Assume(ctx context.Context) (*Credential, error) {
	client, err := stsclient.NewClientWithAccessKey(s.cfg.RegionId, s.cfg.AccessKeyID, s.cfg.AccessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("创建 STS 客户端失败: %w", err)
	}
	request := stsclient.CreateAssumeRoleRequest()
	request.Scheme = "https"
	request.RoleArn = s.cfg.RoleArn
	request.RoleSessionName = s.cfg.RoleSessionName
	request.DurationSeconds = requests.NewInteger(900)
	response, err := client.AssumeRole(request)
	if err != nil {
		return nil, fmt.Errorf("签发临时凭证失败: %w", err)
	}
	expiration, err := time.Parse(time.RFC3339, response.Credentials.Expiration)
	if err != nil {
		expiration = time.Now().Add(15 * time.Minute)
	}
	return &Credential{
		AccessKeyID:     response.Credentials.AccessKeyId,
		AccessKeySecret: response.Credentials.AccessKeySecret,
		SecurityToken:   response.Credentials.SecurityToken,
		Expiration:      expiration,
	}, nil
}
