//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"ai-rag-demo/internal/biz"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/data"
	"ai-rag-demo/internal/external"
	"ai-rag-demo/internal/pkg/nacos"
	"ai-rag-demo/internal/server"
	"ai-rag-demo/internal/service"

	"github.com/google/wire"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
)

// wireApp init kratos application.
func wireApp(*conf.Config) (*app, func(), error) {
	panic(wire.Build(
		_nacosNamingClient,
		nacos.NewRegistry,
		data.ProviderSet,
		cache.ProviderSet,
		external.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		newApp,
	))
}

func _nacosNamingClient() naming_client.INamingClient { return nacosNamingClient }
