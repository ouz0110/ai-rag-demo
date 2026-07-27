package nocli

import (
	chatmodel "ai-rag-demo/internal/biz/nocli/openai/chat_model"
	"ai-rag-demo/internal/biz/nocli/vector"
	vectorStore "ai-rag-demo/internal/biz/nocli/vector/store"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	chatmodel.NewChatModel,
	vectorStore.NewVectorStore,
	vector.NewVectorEngine,
	NewChatBiz,
	NewKBBiz,
)
