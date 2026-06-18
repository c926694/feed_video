package initialize

import (
	"fmt"

	"github.com/spf13/viper"
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
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	Charset  string `mapstructure:"charset"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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
}

var AppConfig *Config

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	AppConfig = &cfg
	return &cfg, nil
}
