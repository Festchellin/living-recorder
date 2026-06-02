package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig    `mapstructure:"server"`
	Database DatabaseConfig  `mapstructure:"database"`
	FFmpeg   FFmpegConfig    `mapstructure:"ffmpeg"`
	Storage  StorageConfig   `mapstructure:"storage"`
	Recorder RecorderConfig  `mapstructure:"recorder"`

	v       *viper.Viper
	cfgFile string
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
	MaxParallel           int    `mapstructure:"max_parallel"`
	RestartOnFailure      int    `mapstructure:"restart_on_failure"`
	HealthCheckInterval   int    `mapstructure:"health_check_interval"`
	DefaultVideoCodec     string `mapstructure:"default_video_codec"`
	DefaultAudioCodec     string `mapstructure:"default_audio_codec"`
	DefaultOutputTemplate string `mapstructure:"default_output_template"`
	SegmentDuration       int    `mapstructure:"segment_duration"`         // >0 启用分段录制（秒）
	RetryWithReEncode     bool   `mapstructure:"retry_with_re_encode"`     // 重试时降级为重新编码
	StorageLocalPath      string // injected at runtime from storage.local.path
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	cfgFile := ""
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		log.Printf("config file not found, using defaults")
	} else {
		cfgFile = v.ConfigFileUsed()
	}

	setDefaults(v)
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	cfg.v = v
	cfg.cfgFile = cfgFile

	if err := os.MkdirAll(cfg.Storage.Local.Path, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755); err != nil {
		return nil, err
	}

	cfg.Recorder.StorageLocalPath = cfg.Storage.Local.Path

	return &cfg, nil
}

func (c *Config) Save() error {
	if c.cfgFile == "" {
		c.cfgFile = "config.yaml"
	}

	c.v.Set("server.port", c.Server.Port)
	c.v.Set("server.mode", c.Server.Mode)
	c.v.Set("ffmpeg.path", c.FFmpeg.Path)
	c.v.Set("storage.default", c.Storage.Default)
	c.v.Set("storage.local.path", c.Storage.Local.Path)
	c.v.Set("storage.s3.endpoint", c.Storage.S3.Endpoint)
	c.v.Set("storage.s3.access_key", c.Storage.S3.AccessKey)
	c.v.Set("storage.s3.secret_key", c.Storage.S3.SecretKey)
	c.v.Set("storage.s3.bucket", c.Storage.S3.Bucket)
	c.v.Set("storage.s3.region", c.Storage.S3.Region)
	c.v.Set("recorder.max_parallel", c.Recorder.MaxParallel)
	c.v.Set("recorder.restart_on_failure", c.Recorder.RestartOnFailure)
	c.v.Set("recorder.health_check_interval", c.Recorder.HealthCheckInterval)

	return c.v.WriteConfigAs(c.cfgFile)
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
	v.SetDefault("recorder.default_video_codec", "copy")
	v.SetDefault("recorder.default_audio_codec", "copy")
	v.SetDefault("recorder.default_output_template", "{name}/{date}_{time}.mp4")
	v.SetDefault("recorder.segment_duration", 0)
	v.SetDefault("recorder.retry_with_re_encode", true)
}
