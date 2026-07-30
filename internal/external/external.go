package external

import (
	"ai-rag-demo/internal/external/mcp"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewRealTimeDiscoveryGrpcClientConn,
	NewRPCProxy,
	mcp.NewManager,
)
