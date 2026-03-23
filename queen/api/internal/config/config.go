package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	SMS      SMSConfig      `mapstructure:"sms"`
	CORS     CORSConfig     `mapstructure:"cors"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
	Pay      PayConfig
	StarAI   StarAIConfig  `mapstructure:"starai"`
	Gateway  GatewayConfig `mapstructure:"gateway"`
}

// GatewayConfig holds upstream provider API keys for the star-ai.net gateway
type GatewayConfig struct {
	Providers map[string]GatewayProviderConfig `mapstructure:"providers"`
}

type GatewayProviderConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

type OAuthConfig struct {
	Google OAuthProviderConfig `mapstructure:"google"`
	GitHub OAuthProviderConfig `mapstructure:"github"`
}

type OAuthProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type SMSConfig struct {
	Provider     string `mapstructure:"provider"`
	AccessKey    string `mapstructure:"access_key"`
	AccessSecret string `mapstructure:"access_secret"`
	SignName     string `mapstructure:"sign_name"`
	TemplateCode string `mapstructure:"template_code"`
}

type CORSConfig struct {
	Origins []string `mapstructure:"origins"`
}

// PayConfig holds Alipay + WeChat Pay settings, loaded from .env
type PayConfig struct {
	Alipay    AlipayConfig
	WechatPay WechatPayConfig
}

type AlipayConfig struct {
	AppID          string `mapstructure:"appid"`
	IsProduction   bool   `mapstructure:"is_production"`
	PrivateKeyPath string `mapstructure:"private_key_path"`
	AppCertPath    string `mapstructure:"app_cert_path"`
	AlipayCertPath string `mapstructure:"alipay_cert_path"`
	RootCertPath   string `mapstructure:"root_cert_path"`
	ReturnURL      string `mapstructure:"return_url"`
	NotifyURL      string `mapstructure:"notify_url"`
}

type WechatPayConfig struct {
	AppID                string `mapstructure:"appid"`
	MchID                string `mapstructure:"mchid"`
	APIKey               string `mapstructure:"api_key"`
	APIv3Key             string `mapstructure:"apiv3_key"`
	PrivateKeyPath       string `mapstructure:"private_key_path"`
	CertPath             string `mapstructure:"cert_path"`
	PublicKeyPath        string `mapstructure:"public_key_path"`
	PublicKeyID          string `mapstructure:"public_key_id"`
	MerchantSerialNumber string `mapstructure:"merchant_serial_number"`
	ReturnURL            string `mapstructure:"return_url"`
	NotifyURL            string `mapstructure:"notify_url"`
	H5WapURL             string `mapstructure:"h5_wap_url"`
	H5WapName            string `mapstructure:"h5_wap_name"`
}

// StarAIConfig holds connection info for StarAI Router (payment gateway)
type StarAIConfig struct {
	URL          string `mapstructure:"url"`           // Router URL, e.g. http://host.docker.internal:8096
	Token        string `mapstructure:"token"`         // X-Internal-Token for service auth
	CallbackBase string `mapstructure:"callback_base"` // Queen's URL as seen by Router, e.g. http://host.docker.internal:8085
}

var C Config

// GetConfig returns the viper instance for flexible key lookups
func GetConfig() *viper.Viper {
	return viper.GetViper()
}

func Load() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// Env overrides
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic("Failed to read config: " + err.Error())
	}
	if err := viper.Unmarshal(&C); err != nil {
		panic("Failed to unmarshal config: " + err.Error())
	}

	// Env override for DSN
	if dsn := viper.GetString("QUEEN_DSN"); dsn != "" {
		C.Database.DSN = dsn
	}
	if port := viper.GetString("QUEEN_PORT"); port != "" {
		C.Server.Port = port
	}
	if secret := viper.GetString("JWT_SECRET"); secret != "" {
		C.JWT.Secret = secret
	}

	// Env override for gateway provider API keys
	loadGatewayEnv()

	// StarAI Router config from env
	if url := viper.GetString("STARAI_URL"); url != "" {
		C.StarAI.URL = url
	}
	if token := viper.GetString("STARAI_TOKEN"); token != "" {
		C.StarAI.Token = token
	}
	if cb := viper.GetString("STARAI_CALLBACK_BASE"); cb != "" {
		C.StarAI.CallbackBase = cb
	}

	// Load payment config from .env
	loadPayConfig()
}

func loadGatewayEnv() {
	if C.Gateway.Providers == nil {
		C.Gateway.Providers = make(map[string]GatewayProviderConfig)
	}
	// 国内直连 Provider — 用各自的 API Key
	directEnv := map[string]string{
		"deepseek":   "DEEPSEEK_API_KEY",
		"qwen":       "DASHSCOPE_API_KEY",
		"minimax":    "MINIMAX_API_KEY",
		"volcengine": "VOLCENGINE_API_KEY",
	}
	for provider, envKey := range directEnv {
		if key := viper.GetString(envKey); key != "" {
			p := C.Gateway.Providers[provider]
			p.APIKey = key
			C.Gateway.Providers[provider] = p
		}
	}
	// 海外 Provider — 统一用 PROXY_INTERNAL_SECRET 认证 proxy
	if secret := viper.GetString("PROXY_INTERNAL_SECRET"); secret != "" {
		for _, provider := range []string{"openai", "anthropic", "gemini", "grok"} {
			p := C.Gateway.Providers[provider]
			p.APIKey = secret
			C.Gateway.Providers[provider] = p
		}
	}
}

func loadPayConfig() {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	if err := v.ReadInConfig(); err != nil {
		// .env is optional
		return
	}

	// Alipay
	C.Pay.Alipay.AppID = v.GetString("ALIPAY_APPID")
	C.Pay.Alipay.IsProduction = v.GetBool("ALIPAY_IS_PRODUCTION")
	C.Pay.Alipay.PrivateKeyPath = v.GetString("ALIPAY_PRIVATE_KEY_PATH")
	C.Pay.Alipay.AppCertPath = v.GetString("ALIPAY_APP_CERT_PATH")
	C.Pay.Alipay.AlipayCertPath = v.GetString("ALIPAY_ALIPAY_CERT_PATH")
	C.Pay.Alipay.RootCertPath = v.GetString("ALIPAY_ROOT_CERT_PATH")
	C.Pay.Alipay.ReturnURL = v.GetString("ALIPAY_RETURN_URL")
	C.Pay.Alipay.NotifyURL = v.GetString("ALIPAY_NOTIFY_URL")

	// WeChat Pay
	C.Pay.WechatPay.AppID = v.GetString("WECHATPAY_APPID")
	C.Pay.WechatPay.MchID = v.GetString("WECHATPAY_MCHID")
	C.Pay.WechatPay.APIKey = v.GetString("WECHATPAY_API_KEY")
	C.Pay.WechatPay.APIv3Key = v.GetString("WECHATPAY_APIv3_KEY")
	C.Pay.WechatPay.PrivateKeyPath = v.GetString("WECHATPAY_PRIVATE_KEY_PATH")
	C.Pay.WechatPay.CertPath = v.GetString("WECHATPAY_CERT_PATH")
	C.Pay.WechatPay.PublicKeyPath = v.GetString("WECHATPAY_PUBLIC_KEY_PATH")
	C.Pay.WechatPay.PublicKeyID = v.GetString("WECHATPAY_PUBLIC_KEY_ID")
	C.Pay.WechatPay.MerchantSerialNumber = v.GetString("WECHATPAY_MERCHANT_SERIAL_NUMBER")
	C.Pay.WechatPay.ReturnURL = v.GetString("WECHATPAY_RETURN_URL")
	C.Pay.WechatPay.NotifyURL = v.GetString("WECHATPAY_NOTIFY_URL")
	C.Pay.WechatPay.H5WapURL = v.GetString("WECHATPAY_H5_WAP_URL")
	C.Pay.WechatPay.H5WapName = v.GetString("WECHATPAY_H5_WAP_NAME")
}
