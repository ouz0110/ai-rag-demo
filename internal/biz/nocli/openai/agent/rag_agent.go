package agent

import (
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/openai/agent/base"
	"ai-rag-demo/internal/biz/nocli/openai/tool"
	ragtool "ai-rag-demo/internal/biz/nocli/openai/tool/rag"
	read_files "ai-rag-demo/internal/biz/nocli/openai/tool/read_files"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/skill"
)

const RAGAgentName = "rag_agent"

type RAGAgent struct {
	*base.BaseAgent
}

func NewRAGAgent(cfg *conf.Config, baseTools *tool.Registry) *RAGAgent {
	// 📌 在此显式声明该 Agent 所需绑定的物理工具集 (包含 read_files 读取文档与 rag_search 专有知识库向量检索)
	tools := baseTools.Filter(
		read_files.ToolName,
		ragtool.ToolName,
	)
	b := base.NewBaseAgent(RAGAgentName, cfg, tools)
	return &RAGAgent{
		BaseAgent: b,
	}
}

func (a *RAGAgent) Name() string {
	return RAGAgentName
}

func (a *RAGAgent) Description() string {
	return "专有 RAG 知识库与文档问答专家。擅长基于业务文档、产品规范及知识库材料进行归纳与总结。当用户询问知识库、文档或业务规范相关问题时，必须委派给此工具。"
}

func (a *RAGAgent) MaxIterations() int {
	return a.GetMaxIterationsForAgent(a.Name(), 10)
}

func (a *RAGAgent) SystemPrompt(workDir string, skillMgr *skill.Manager) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个专业的 RAG 知识库问答 Agent 助手，当前工作目录为：%s。

核心职责与原则：
1. 【基于知识库】你的回答必须严格依据检索到的文档或上下文知识库内容进行归纳与总结，绝不凭空编造事实。
2. 【Query 优化重写 (CRITICAL)】在调用 rag_search 工具检索前，你必须先结合当前对话上下文，将用户模糊、口语化、含代词（如“那个”、“价格是多少”、“它有什么限制”）或过短的提问，优化重写为一个概念完整、主干明确、实体清晰的标准 RAG 问题，再传入 rag_search。
3. 【客观真实】若知识库或上下文中信息不足以回答用户提问，应当明确告知用户“当前知识库中未检索到相关内容”。
4. 【文档探索】若需要检索或探索本地知识库文件，优先使用 rag_search 检索向量库，禁止直接读取文件或执行终端命令。
`, workDir))

	if skillMgr != nil {
		skillPrompt := skillMgr.BuildLevel1PromptForAgent(a.Name())
		if skillPrompt != "" {
			sb.WriteString("\n" + skillPrompt)
		}
	}

	return sb.String()
}
