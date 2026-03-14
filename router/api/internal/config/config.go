package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Proxy    ProxyConfig    `mapstructure:"proxy"`
	Queen    QueenConfig    `mapstructure:"queen"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Alipay   AlipayConfig   `mapstructure:"alipay"`
	Wechat   WechatConfig   `mapstructure:"wechat"`
}

// QueenConfig holds connection info for Queen's internal API (star energy billing)
type QueenConfig struct {
	URL   string `mapstructure:"url"`   // http://queen-api:8080
	Token string `mapstructure:"token"` // internal service auth token
}

type AlipayConfig struct {
	AppID          string `mapstructure:"app_id"`
	IsProduction   bool   `mapstructure:"is_production"`
	PrivateKeyPath string `mapstructure:"private_key_path"` // path to private_key.pem
	AppCertPath    string `mapstructure:"app_cert_path"`
	AliCertPath    string `mapstructure:"ali_cert_path"`
	RootCertPath   string `mapstructure:"root_cert_path"`
	ReturnURL      string `mapstructure:"return_url"`
	NotifyURL      string `mapstructure:"notify_url"`
}

type WechatConfig struct {
	AppID          string `mapstructure:"app_id"`
	MchID          string `mapstructure:"mch_id"`
	MchSerialNo    string `mapstructure:"mch_serial_no"`
	APIKey         string `mapstructure:"api_key"`
	APIv3Key       string `mapstructure:"apiv3_key"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
	CertPath       string `mapstructure:"cert_path"`
	PublicKeyPath  string `mapstructure:"public_key_path"`
	PublicKeyID    string `mapstructure:"public_key_id"`
	NotifyURL      string `mapstructure:"notify_url"`
	ReturnURL      string `mapstructure:"return_url"`
	H5WapURL       string `mapstructure:"h5_wap_url"`
	H5WapName      string `mapstructure:"h5_wap_name"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// ProxyConfig holds the connection info for the Node.js overseas relay proxy
type ProxyConfig struct {
	URL       string `mapstructure:"url"`        // http://proxy:8000
	SecretKey string `mapstructure:"secret_key"` // internal auth between API → Proxy
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	// Environment variable overrides: STAR_AI_SERVER_PORT, STAR_AI_DATABASE_HOST, etc.
	viper.SetEnvPrefix("STAR_AI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("server.port", 8096)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.password", "starclaw123")
	viper.SetDefault("database.dbname", "star_ai")
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("proxy.url", "http://localhost:8000")
	viper.SetDefault("proxy.secret_key", "star-ai-internal-secret")
	viper.SetDefault("queen.url", "http://localhost:8080")
	viper.SetDefault("queen.token", "")
	viper.SetDefault("jwt.secret", "star-ai-jwt-change-me")
	viper.SetDefault("jwt.expire_hours", 72)

	// Bind payment env vars (different naming convention from STAR_AI_ prefix)
	viper.BindEnv("alipay.app_id", "ALIPAY_APPID")
	viper.BindEnv("alipay.is_production", "ALIPAY_IS_PRODUCTION")
	viper.BindEnv("alipay.private_key_path", "ALIPAY_PRIVATE_KEY_PATH")
	viper.BindEnv("alipay.app_cert_path", "ALIPAY_APP_CERT_PATH")
	viper.BindEnv("alipay.ali_cert_path", "ALIPAY_ALIPAY_CERT_PATH")
	viper.BindEnv("alipay.root_cert_path", "ALIPAY_ROOT_CERT_PATH")
	viper.BindEnv("alipay.return_url", "ALIPAY_RETURN_URL")
	viper.BindEnv("alipay.notify_url", "ALIPAY_NOTIFY_URL")
	viper.BindEnv("wechat.app_id", "WECHATPAY_APPID")
	viper.BindEnv("wechat.mch_id", "WECHATPAY_MCHID")
	viper.BindEnv("wechat.mch_serial_no", "WECHATPAY_MERCHANT_SERIAL_NUMBER")
	viper.BindEnv("wechat.api_key", "WECHATPAY_API_KEY")
	viper.BindEnv("wechat.apiv3_key", "WECHATPAY_APIv3_KEY")
	viper.BindEnv("wechat.private_key_path", "WECHATPAY_PRIVATE_KEY_PATH")
	viper.BindEnv("wechat.cert_path", "WECHATPAY_CERT_PATH")
	viper.BindEnv("wechat.public_key_path", "WECHATPAY_PUBLIC_KEY_PATH")
	viper.BindEnv("wechat.public_key_id", "WECHATPAY_PUBLIC_KEY_ID")
	viper.BindEnv("wechat.notify_url", "WECHATPAY_NOTIFY_URL")
	viper.BindEnv("wechat.return_url", "WECHATPAY_RETURN_URL")
	viper.BindEnv("wechat.h5_wap_url", "WECHATPAY_H5_WAP_URL")
	viper.BindEnv("wechat.h5_wap_name", "WECHATPAY_H5_WAP_NAME")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
