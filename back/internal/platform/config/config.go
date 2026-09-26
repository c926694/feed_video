package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// DefaultRelativePath 配置文件相对于项目根目录的位置
const DefaultRelativePath = "config/config.yaml"

const (
	defaultMaxOpenConns    = 50
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 60
	defaultGroupID         = "feed_video"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Kafka  KafkaConfig  `mapstructure:"kafka"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	Upload UploadConfig `mapstructure:"upload"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
	// AllowOrigins 允许跨域访问的来源，支持通配符，例如 http://localhost:*
	AllowOrigins []string `mapstructure:"allow_origins"`
}

type MySQLConfig struct {
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Charset         string `mapstructure:"charset"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime_minutes"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	// AccessMinutes 是 access token 的有效期，也是"登出后旧凭证还能用多久"的上限
	AccessMinutes int64 `mapstructure:"access_token_minutes"`
	// RefreshHours 是 refresh token 的有效期，决定用户多久需要重新登录
	RefreshHours int64 `mapstructure:"refresh_token_hours"`
}

type UploadConfig struct {
	AvatarDir string    `mapstructure:"avatar_dir"`
	CoverDir  string    `mapstructure:"cover_dir"`
	VideoDir  string    `mapstructure:"video_dir"`
	OSS       OSSConfig `mapstructure:"oss"`
}

// OSSConfig 阿里云 OSS 的连接参数
type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	Bucket          string `mapstructure:"bucket"`
	// Prefix 是 bucket 内的统一前缀，留空表示放在根目录
	Prefix string `mapstructure:"prefix"`
	// CustomDomain 是绑定到 bucket 的访问域名，留空时用 bucket 与 endpoint 拼
	CustomDomain string `mapstructure:"custom_domain"`
}

// Load 读取配置文件并补齐可以省略的项
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if len(c.Server.AllowOrigins) == 0 {
		c.Server.AllowOrigins = []string{"http://localhost:*", "http://127.0.0.1:*"}
	}
	if c.MySQL.MaxOpenConns <= 0 {
		c.MySQL.MaxOpenConns = defaultMaxOpenConns
	}
	if c.MySQL.MaxIdleConns <= 0 {
		c.MySQL.MaxIdleConns = defaultMaxIdleConns
	}
	if c.MySQL.ConnMaxLifetime <= 0 {
		c.MySQL.ConnMaxLifetime = defaultConnMaxLifetime
	}
	if c.Kafka.GroupID == "" {
		c.Kafka.GroupID = defaultGroupID
	}
	if c.Upload.AvatarDir == "" {
		c.Upload.AvatarDir = "avatar"
	}
	if c.Upload.CoverDir == "" {
		c.Upload.CoverDir = "cover"
	}
	if c.Upload.VideoDir == "" {
		c.Upload.VideoDir = "video"
	}
	if c.Upload.OSS.AccessKeyID == "" {
		c.Upload.OSS.AccessKeyID = os.Getenv("OSS_ACCESS_KEY_ID")
	}
	if c.Upload.OSS.AccessKeySecret == "" {
		c.Upload.OSS.AccessKeySecret = os.Getenv("OSS_ACCESS_KEY_SECRET")
	}
}
