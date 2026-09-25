package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

const (
	defaultMaxOpenConns    = 50
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime = 60
	defaultGroupID         = "feed_video"
	defaultURLPrefix       = "/static/"
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
	Secret      string `mapstructure:"secret"`
	ExpireHours int64  `mapstructure:"expire_hours"`
}

type UploadConfig struct {
	BasePath  string `mapstructure:"base_path"`
	AvatarDir string `mapstructure:"avatar_dir"`
	CoverDir  string `mapstructure:"cover_dir"`
	VideoDir  string `mapstructure:"video_dir"`
	URLPrefix string `mapstructure:"url_prefix"`
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
	if c.Upload.BasePath == "" {
		c.Upload.BasePath = envOr("STORAGE_PATH", "storage")
	}
	if c.Upload.URLPrefix == "" {
		c.Upload.URLPrefix = envOr("HTTP_PATH", defaultURLPrefix)
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
}

func envOr(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
