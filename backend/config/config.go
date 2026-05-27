package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	FFmpeg   FFmpegConfig   `mapstructure:"ffmpeg"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Recorder RecorderConfig `mapstructure:"recorder"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	Path   string `mapstructure:"path"`
}

type FFmpegConfig struct {
	Path string `mapstructure:"path"`
}

type StorageConfig struct {
	Default string             `mapstructure:"default"`
	Local   LocalStorageConfig `mapstructure:"local"`
	S3      S3StorageConfig    `mapstructure:"s3"`
}

type LocalStorageConfig struct {
	Path string `mapstructure:"path"`
}

type S3StorageConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
	Bucket    string `mapstructure:"bucket"`
	Region    string `mapstructure:"region"`
}

type RecorderConfig struct {
	MaxParallel          int    `mapstructure:"max_parallel"`
	RestartOnFailure     int    `mapstructure:"restart_on_failure"`
	HealthCheckInterval  int    `mapstructure:"health_check_interval"`
	StorageLocalPath     string // injected at runtime from storage.local.path
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	v.ReadInConfig()

	setDefaults(v)
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	os.MkdirAll(cfg.Storage.Local.Path, 0755)
	os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755)

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.mode", "release")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.path", "./data/living-recorder.db")
	v.SetDefault("ffmpeg.path", "ffmpeg")
	v.SetDefault("storage.default", "local")
	v.SetDefault("storage.local.path", "./recordings")
	v.SetDefault("recorder.max_parallel", 10)
	v.SetDefault("recorder.restart_on_failure", 3)
	v.SetDefault("recorder.health_check_interval", 30)
}
