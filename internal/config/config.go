package config

import (
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Addr            string        `mapstructure:"addr"`
	DatabaseURL     string        `mapstructure:"db_url"`
	DefaultTimeout  int           `mapstructure:"default_timeout"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	BotToken        string        `mapstructure:"bot_token"`
	Email           EmailConfig   `mapstructure:"email"`
}

type EmailConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

func Load() (Config, error) {
	_ = godotenv.Load(".env")

	v := viper.New()

	v.AddConfigPath(".")
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		if !errors.As(err, &viper.ConfigFileNotFoundError{}) {
			return Config{}, errors.WithStack(err)
		}
	}

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	_ = v.BindEnv("db_url")
	_ = v.BindEnv("bot_token")
	_ = v.BindEnv("email.host")
	_ = v.BindEnv("email.port")
	_ = v.BindEnv("email.user")
	_ = v.BindEnv("email.password")
	_ = v.BindEnv("email.from")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, errors.WithDetail(err, "unable to decode into struct")
	}

	return cfg, nil
}
