package external

type RPCProxy struct {
}

func NewRPCProxy(c *RealTimeDiscoveryGrpcClientConn) *RPCProxy {
	return &RPCProxy{}
}
