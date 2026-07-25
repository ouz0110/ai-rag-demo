package agent

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/pkg/skill"

	"github.com/sashabaranov/go-openai"
)

type RAGAgent struct {
	*base.BaseAgent
}

func NewRAGAgent(b *base.BaseAgent, model string) *RAGAgent {
	b.SetModel(model)
	return &RAGAgent{
		BaseAgent: b,
	}
}

func (a *RAGAgent) Name() string {
	return "rag_agent"
}

func (a *RAGAgent) Tools() []openai.Tool {
	return []openai.Tool{}
}

func (a *RAGAgent) Description() string {
	return "专有 RAG 知识库问答 Agent，擅长基于文档检索与知识库上下文进行精准问答与总结"
}

func (a *RAGAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 10)
}

func (a *RAGAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个专业的 RAG 知识库问答 Agent 助手，当前工作目录为：%s。

核心职责与原则：
1. 【基于知识库】你的回答必须严格依据检索到的文档或上下文知识库内容进行归纳与总结，绝不凭空编造事实。
2. 【客观真实】若知识库或上下文中信息不足以回答用户提问，应当明确告知用户“当前知识库中未检索到相关内容”。
3. 【文档探索】若需要检索或探索本地知识库文件，优先使用 read_files 与 list_files 工具阅读相关材料。
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
		}
	}

	return sb.String()
}
