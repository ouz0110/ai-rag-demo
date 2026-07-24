package conf

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

var c *Config

const (
	EnvLocal = "local" // 本地环境
	EnvDev   = "dev"   // 开发环境
	EnvTest  = "qa"    // 测试环境
	EnvProd  = "prod"  // 生产环境
)

type durationWrapper struct {
	time.Duration
}

func (d *durationWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *durationWrapper) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid duration: %v", v)
	}
}

type DBConfig struct {
	Driver       string          `json:"driver"`
	Source       string          `json:"source"`
	AutoMigrate  string          `json:"auto_migrate"`
	MaxIdleConns int             `json:"max_idle_conns"`
	MaxOpenConns int             `json:"max_open_conns"`
	MaxLifetime  durationWrapper `json:"max_lifetime"`
	Ca           string          `json:"ca"`
}

type RedisConfig struct {
	Password     string          `json:"password"`
	Addrs        string          `json:"addrs"`
	ReadTimeout  durationWrapper `json:"read_timeout"`
	WriteTimeout durationWrapper `json:"write_timeout"`
	DB           int             `json:"db"`
}

type OTel struct {
	Enable     bool    `json:"enable"`
	Endpoint   string  `yaml:"endpoint"`
	SampleRate float32 `yaml:"sample_rate"`
	StdOut     bool    `yaml:"std_out"`
}

type Config struct {
	Secret      string `json:"secret"`
	Name        string `json:"name"`
	Env         string `json:"env"`
	Version     string `json:"version"`
	Host        string `json:"host"`
	Config      string `json:"config"`
	ConfigGroup string `json:"config_group"`
	I18N        struct {
		Default     string   `json:"default"`
		ConfigGroup string   `json:"config_group"`
		Config      []string `json:"config"`
	} `json:"i18n"`
	Server struct {
		Domain string `json:"domain"`
		HTTP   struct {
			Addr    string          `json:"addr"`
			Timeout durationWrapper `json:"timeout"`
		} `json:"http"`
		Grpc struct {
			Addr            string          `json:"addr"`
			Timeout         durationWrapper `json:"timeout"`
			OutboundTimeout durationWrapper `json:"outbound_timeout"`
		} `json:"grpc"`
		Srv map[string]string `json:"srv"`
	} `json:"server"`
	Source struct {
		Log struct {
			Level        string `json:"level"`
			SdkEnableLog int32  `json:"sdk_enable_log"`
		} `json:"log"`
		AesEcb struct {
			Code string `json:"code"`
		} `json:"aes_ecb"`
		Redis          map[string]*RedisConfig `json:"redis"`
		MysqlDefaultCa string                  `json:"mysql_default_ca"`
		Database       map[string]*DBConfig    `json:"database"`
		OTel           *OTel                   `json:"otel"`
		Nacos          struct {
			Addr      string `json:"addr"`
			Port      string `json:"port"`
			Username  string `json:"username"`
			Password  string `json:"password"`
			Namespace string `json:"namespace"`
			Log       struct {
				Level string `json:"level"`
			} `json:"log"`
		} `json:"nacos"`
	} `json:"source"`
	External struct {
	} `json:"external"`
	Service struct {
	} `json:"service"`
}

func IsLocalEnv() bool {
	return c.Env == EnvLocal
}

func IsTestEnv() bool {
	return slices.Contains([]string{EnvLocal, EnvDev, EnvTest}, c.Env)
}

func IsProdEnv() bool {
	return c.Env == EnvProd
}

func IsDevEnv() bool {
	return c.Env == EnvDev
}

func Init(cfg *Config) {
	c = cfg
}

func GetRPCServiceNameByMethod(method string) string {
	// method 示例：/global.v1.ShumeiService/SVerifyCaptcha
	method = strings.TrimPrefix(method, "/")
	ps := strings.Split(method, "/")
	ps = strings.Split(ps[0], ".")
	firstPart := ps[0]

	// 优先从配置中获取
	sn, ok := c.Server.Srv[firstPart]
	if ok {
		return sn
	}
	// 没有则按 srv.grpc 的格式
	return fmt.Sprintf("%s.grpc", firstPart)
}
