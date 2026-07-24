package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/metrics"
	"ai-rag-demo/internal/pkg/i18n"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/nacos"
	"ai-rag-demo/internal/pkg/utils"

	kratosconfig "github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/encoding/json"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/encoding/protojson"
)

var nacosNamingClient naming_client.INamingClient

func prepare() *conf.Config {
	// local config
	localConfig, err := scanConfig(file.NewSource(flagconf))
	if err != nil {
		panic(errors.Wrap(err, "initLocalConfig error"))
	}
	fmt.Println("local config loaded")
	// nacos
	nacosCfg := localConfig.Source.Nacos
	nacosPort, err := strconv.Atoi(nacosCfg.Port)
	if err != nil {
		panic(errors.Wrap(err, "init nacos port error"))
	}
	clientParam := vo.NacosClientParam{
		ServerConfigs: []constant.ServerConfig{*constant.NewServerConfig(nacosCfg.Addr, uint64(nacosPort))},
		ClientConfig: &constant.ClientConfig{
			NamespaceId: nacosCfg.Namespace,
			Username:    nacosCfg.Username,
			Password:    nacosCfg.Password,
		},
	}
	clientParam.ClientConfig.CustomLogger = log.NewNacosLogger("nacos", nacosCfg.Log.Level)
	var nacosConfigClient config_client.IConfigClient
	nacosNamingClient, nacosConfigClient, err = nacos.NewClient(clientParam)
	if err != nil {
		panic(errors.Wrap(err, "init nacos client error"))
	}
	fmt.Println("nacos init success")
	// remote config
	remoteConfig, err := initRemoteConfig(localConfig, nacosConfigClient)
	if err != nil {
		panic(errors.Wrap(err, "initRemoteConfig error"))
	}
	config := mergeConfig(localConfig, remoteConfig)
	conf.Init(config)
	log.Init(config.Source.Log.Level,
		zap.Hooks(func(entry zapcore.Entry) error {
			metrics.LogCount.WithLabelValues(entry.LoggerName, entry.Level.String()).Inc()
			return nil
		}),
		zap.Fields(zap.String("App", config.Name)),
	)
	if err = initRemoteServiceConfig(config, nacosConfigClient); err != nil {
		panic(errors.Wrap(err, "initRemoteServiceConfig error"))
	}
	fmt.Println("remote config loaded")
	// json
	json.MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true, // 默认值不忽略
	}
	// snowflake
	if err = utils.InitSnowflakeNode(); err != nil {
		panic(errors.Wrap(err, "InitSnowflakeNode error"))
	}

	utils.InitTSID(0, 0)
	utils.AESECBEncodeKeyInit(config.Source.AesEcb.Code)

	if Name == "" {
		Name = config.Name
	}
	if Version == "" {
		Version = config.Version
	}

	return config
}

func scanConfig(source kratosconfig.Source) (*conf.Config, error) {
	c := kratosconfig.New(kratosconfig.WithSource(env.NewSource(""), source))
	defer c.Close()
	err := c.Load()
	if err != nil {
		return nil, err
	}
	var cfg conf.Config
	// Scan 最终使用 json/protojson.Unmarshal 反序列化，因此 conf.TracerConfig 中需包含 json tag。
	err = c.Scan(&cfg)
	return &cfg, err
}

func initRemoteConfig(localConfig *conf.Config, nacosConfigClient config_client.IConfigClient) (*conf.Config, error) {
	if localConfig.Env == "local" {
		return localConfig, nil
	}
	return scanConfig(&nacosConfigSource{
		dataID: localConfig.Config,
		group:  localConfig.ConfigGroup,
		client: nacosConfigClient,
	})
}

func mergeConfig(local, remote *conf.Config) *conf.Config {
	remote.Config = local.Config
	remote.ConfigGroup = local.ConfigGroup
	remote.Source.Nacos = local.Source.Nacos

	return remote
}

func initRemoteServiceConfig(config *conf.Config, nacosConfigClient config_client.IConfigClient) error {
	ctx := context.TODO()
	var remoteConfigs []*nacos.RemoteConfig

	// i18n
	err := i18n.Init(config.I18N.Default)
	if err != nil {
		return err
	}
	if config.Env == "local" {
		return nil
	}
	loadI18N := func(dataID, data string) error {
		log.Debugf(ctx, "remote %s loaded: %s", dataID, data)
		return i18n.ParseMessageFileBytes([]byte(data), dataID)
	}
	for _, v := range config.I18N.Config {
		remoteConfigs = append(remoteConfigs, &nacos.RemoteConfig{
			DataID:   v,
			Group:    config.I18N.ConfigGroup,
			Init:     loadI18N,
			OnChange: loadI18N,
		})
	}
	return nacos.InitRemoteConfig(nacosConfigClient, remoteConfigs...)
}

type nacosConfigSource struct {
	dataID string
	group  string
	client config_client.IConfigClient
}

func (c *nacosConfigSource) Load() ([]*kratosconfig.KeyValue, error) {
	content, err := c.client.GetConfig(vo.ConfigParam{
		DataId: c.dataID,
		Group:  c.group,
	})
	if err != nil {
		return nil, err
	}
	k := c.dataID
	return []*kratosconfig.KeyValue{
		{
			Key:    k,
			Value:  []byte(content),
			Format: strings.TrimPrefix(filepath.Ext(k), "."),
		},
	}, nil
}

type dummyWatcher struct{}

func (d *dummyWatcher) Next() ([]*kratosconfig.KeyValue, error) {
	// 返回 context.Canceled 以使 github.com/go-kratos/kratos/v2@v2.7.0/config/kratosconfig.go:66 处直接退出
	// 否则 kratosconfig.go#watch 方法死循环会导致高 CPU
	return nil, context.Canceled
}

func (d *dummyWatcher) Stop() error {
	return nil
}

func (c *nacosConfigSource) Watch() (kratosconfig.Watcher, error) {
	// TODO
	return &dummyWatcher{}, nil
}
