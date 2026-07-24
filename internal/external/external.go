package external

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewRealTimeDiscoveryGrpcClientConn,
	NewRPCProxy,
)
