package nocli

import (
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	chatmodel.NewChatModel,
	NewChatBiz,
)
