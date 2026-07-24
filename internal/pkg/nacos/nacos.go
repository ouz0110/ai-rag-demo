package nacos

import (
	"log"

	"github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

func NewClient(clientParam vo.NacosClientParam) (naming_client.INamingClient, config_client.IConfigClient, error) {
	nacosCli, err := clients.NewNamingClient(clientParam)
	if err != nil {
		return nil, nil, err
	}
	nacosConfigCli, err := clients.NewConfigClient(clientParam)

	return nacosCli, nacosConfigCli, err
}

func NewRegistry(c naming_client.INamingClient) *nacos.Registry {
	return nacos.New(c)
}

type RemoteConfig struct {
	DataID   string
	Group    string
	Init     func(dataID, data string) error
	OnChange func(dataID, data string) error
}

func InitRemoteConfig(nacosConfigCli config_client.IConfigClient, ss ...*RemoteConfig) error {
	for _, v := range ss {
		c, err := nacosConfigCli.GetConfig(vo.ConfigParam{
			DataId: v.DataID,
			Group:  v.Group,
		})
		if err != nil {
			return err
		}
		err = v.Init(v.DataID, c)
		if err != nil {
			return err
		}
		log.Printf("remote config %s loaded\n", v.DataID)
		if v.OnChange != nil {
			f := v.OnChange
			err = nacosConfigCli.ListenConfig(vo.ConfigParam{
				DataId: v.DataID,
				Group:  v.Group,
				OnChange: func(_, _, dataId, data string) { // TODO 同一个请求可能获取到不同版本的配置
					log.Printf("remote config %s changed\n", v.DataID)
					err = f(dataId, data)
					if err != nil {
						log.Printf("ERROR: apply remote config error, dataID: %s, %v\n", v.DataID, err)
					}
				},
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
