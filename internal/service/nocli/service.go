package nocli

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewChatService,
)
