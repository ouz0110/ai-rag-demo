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

type OpenAIContextCompressConfig struct {
	Enable              bool    `json:"enable" yaml:"enable"`                                 // 是否开启上下文压缩
	MaxContextTokens    int     `json:"max_context_tokens" yaml:"max_context_tokens"`         // 模型最大上下文 Token 数
	CompressRatio       float64 `json:"compress_ratio" yaml:"compress_ratio"`                 // 触发压缩的比例 (如 0.75)
	MaxCompressCount    int     `json:"max_compress_count" yaml:"max_compress_count"`         // 单会话最大压缩次数限制
	MinUncompressedMsgs int     `json:"min_uncompressed_msgs" yaml:"min_uncompressed_msgs"`   // 两次压缩之间必须积累的最小未压缩消息条数
	KeepRecentMessages  int     `json:"keep_recent_messages" yaml:"keep_recent_messages"`     // 压缩时固定保留的最新消息数
	MaxSummaryTokens    int     `json:"max_summary_tokens" yaml:"max_summary_tokens"`         // 摘要最大 Token 限制
	Timeout             int     `json:"timeout" yaml:"timeout"`                               // 摘要生成超时时间 (单位：秒，默认 30)
}

type OpenAI struct {
	APIKey          string                       `json:"api_key" yaml:"api_key"`
	BaseURL         string                       `json:"base_url" yaml:"base_url"`
	Model           string                       `json:"model" yaml:"model"`
	Billing         *BillingConfig               `json:"billing" yaml:"billing"`
	ContextCompress *OpenAIContextCompressConfig `json:"context_compress" yaml:"context_compress"`
}

type AgentConfig struct {
	MaxIterations int `json:"max_iterations"`
}

type NocliConfig struct {
	WorkDir            string                  `json:"work_dir"`
	AllowedPaths       []string                `json:"allowed_paths" yaml:"allowed_paths"`
	IgnoredPaths       []string                `json:"ignored_paths"`
	AllowedSuffixes    []string                `json:"allowed_suffixes"`
	MaxReadFiles       int                     `json:"max_read_files"`
	MaxTotalBytes      int                     `json:"max_total_bytes"`
	ChunkLines         int                     `json:"chunk_lines"`
	MaxAgentIterations int                     `json:"max_agent_iterations"`
	Agents             map[string]*AgentConfig `json:"agents"`
}

type SkillConfig struct {
	Enable bool   `json:"enable" yaml:"enable"`
	Path   string `json:"path" yaml:"path"`
}

type MCPClientInfo struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

type MCPServerConfig struct {
	Name          string            `json:"name" yaml:"name"`
	Description   string            `json:"description" yaml:"description"`
	URL           string            `json:"url" yaml:"url"`
	Enabled       bool              `json:"enabled" yaml:"enabled"`
	Headers       map[string]string `json:"headers" yaml:"headers"`
	Timeout       durationWrapper   `json:"timeout" yaml:"timeout"`
	MaxRetries    int               `json:"max_retries" yaml:"max_retries"`
	RetryInterval durationWrapper   `json:"retry_interval" yaml:"retry_interval"`
}

type MCPConfig struct {
	Enable         bool              `json:"enable" yaml:"enable"`
	ClientInfo     MCPClientInfo     `json:"client_info" yaml:"client_info"`
	DefaultTimeout durationWrapper   `json:"default_timeout" yaml:"default_timeout"`
	Servers        []MCPServerConfig `json:"servers" yaml:"servers"`
}

type MilvusConfig struct {
	Address        string `json:"address"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	DBName         string `json:"db_name"`
	CollectionName string `json:"collection_name"`
	Dimension      int    `json:"dimension"`
}

type VectorDBConfig struct {
	Driver string        `json:"driver"` // milvus | qdrant | pgvector
	Milvus *MilvusConfig `json:"milvus"`
}

type EmbeddingConfig struct {
	APIKey    string `json:"api_key"`
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	Dimension int    `json:"dimension"` // 动态向量维度 (如 1024, 1536, 3072)
}

type ChunkerConfig struct {
	ParentSize     int     `json:"parent_size"`     // 父块字符数
	ChildSize      int     `json:"child_size"`      // 子块字符数
	Overlap        int     `json:"overlap"`         // 重叠字符数
	MergeThreshold float32 `json:"merge_threshold"` // 高相似度语义合并阈值 (默认 0.75)
	SplitThreshold float32 `json:"split_threshold"` // 话题断层切割阈值 (默认 0.45)
}

type RerankConfig struct {
	Enable  bool   `json:"enable"`   // 是否开启重排
	Driver  string `json:"driver"`   // llm | bge | cohere
	APIKey  string `json:"api_key"`  // 独立 Rerank APIKey
	BaseURL string `json:"base_url"` // 独立 Rerank BaseURL
	Model   string `json:"model"`    // 重排模型名称 (如 gpt-4o-mini 或 bge-reranker-v2-m3)
	Timeout int    `json:"timeout"`  // 硬超时时间 (毫秒，默认 1000ms)
}

type ModelPricingConfig struct {
	InputUnitPrice  float64 `json:"input_unit_price" yaml:"input_unit_price"`   // 每千 units 单价
	OutputUnitPrice float64 `json:"output_unit_price" yaml:"output_unit_price"` // 每千 units 输出单价 (适用于 openai)
	UnitSize        int     `json:"unit_size" yaml:"unit_size"`                 // 计费单位 (默认 1000)
}

type BillingConfig struct {
	Enable        bool                           `json:"enable" yaml:"enable"`
	DefaultPrices map[string]*ModelPricingConfig `json:"default_prices" yaml:"default_prices"`
}

type RAGConfig struct {
	Enable         bool             `json:"enable" yaml:"enable"`           // 是否开启 RAG 检索全局总开关
	KnowledgeDir   string           `json:"knowledge_dir" yaml:"knowledge_dir"`   // 默认公共知识库文件目录
	UploadDir      string           `json:"upload_dir" yaml:"upload_dir"`      // 用户自主上传文件的存储目录
	AutoReload     bool             `json:"auto_reload" yaml:"auto_reload"`     // 启动时是否自动增量重载知识库
	TopK           int              `json:"top_k" yaml:"top_k"`           // 默认召回 TopK
	ScoreThreshold float32          `json:"score_threshold" yaml:"score_threshold"` // 默认相似度得分阈值
	Embedding      *EmbeddingConfig `json:"embedding" yaml:"embedding"`       // 独立 Embedding AI 配置
	Chunker        *ChunkerConfig   `json:"chunker" yaml:"chunker"`         // 细粒度切片配置
	Rerank         *RerankConfig    `json:"rerank" yaml:"rerank"`          // 独立 Rerank 重排配置
	Billing        *BillingConfig   `json:"billing" yaml:"billing"`         // 计费配置
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
		OpenAI         *OpenAI                 `json:"openai"`
		Nocli          *NocliConfig            `json:"nocli"`
		Skill          *SkillConfig            `json:"skill"`
		MCP            *MCPConfig              `json:"mcp"`
		VectorDB       *VectorDBConfig         `json:"vector_db"`
		RAG      *RAGConfig      `json:"rag"`
		Nacos    struct {
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
