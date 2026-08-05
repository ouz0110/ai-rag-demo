package ragtool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-rag-demo/internal/biz/nocli/vector"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

const (
	ToolName               = "rag_search"
	parentKBTenantIDKey    = "parent_kb_tenant_id"
	parentKBIDKey          = "parent_kb_id"
	parentEnableRAGKey     = "parent_enable_rag"
	parentEnableRerankKey  = "parent_enable_rerank"
	parentKBNameKey        = "parent_kb_name"
	parentKBDescriptionKey = "parent_kb_description"
)

type Args struct {
	Query        string `json:"query"`                  // 用户输入的原始或模糊查询/文档提问
	TenantID     string `json:"tenant_id"`              // 租户 ID，若为空则自动读取会话上下文中的租户，默认 default_tenant
	KBID         string `json:"kb_id"`                  // 知识库 ID，若为空则自动读取会话上下文中的知识库，默认 kb_default_system
	TopK         int    `json:"top_k"`                  // 召回结果数量，默认 5
	OnlyActive   bool   `json:"only_active"`            // 是否仅检索当前生效文档切片，默认 true
	EnableRerank *bool  `json:"enable_rerank,omitempty"` // 是否启用 Rerank 二次精排
}

type Tool struct {
	cfg          *conf.Config
	vectorEngine *vector.VectorEngine
}

func NewTool(cfg *conf.Config, engine *vector.VectorEngine) *Tool {
	return &Tool{
		cfg:          cfg,
		vectorEngine: engine,
	}
}

func (t *Tool) RequiresApproval(ctx context.Context, argsJSON string) bool {
	return false
}

func (t *Tool) Definition() openai.Tool {
	parameters := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "优化重写后的标准 RAG 检索 Query。调用前请务必结合上下文，将用户口语、模糊或带代词的提问改写为实体明确、主干清晰的标准检索语句（例如：将模糊的'那个价格'重写为'手机A 最新售价与费用标准'）。",
			},
			"tenant_id": map[string]interface{}{
				"type":        "string",
				"description": "租户ID。若不传，系统将自动使用当前会话指定的知识库租户",
			},
			"kb_id": map[string]interface{}{
				"type":        "string",
				"description": "知识库ID标识。若不传，系统将自动使用当前会话指定的知识库ID",
			},
			"top_k": map[string]interface{}{
				"type":        "integer",
				"description": "召回最相关切片数量，默认 5 条",
			},
			"only_active": map[string]interface{}{
				"type":        "boolean",
				"description": "是否仅检索当前生效中的文档，默认 true",
			},
		},
		"required": []string{"query"},
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        ToolName,
			Description: "专有 RAG 知识库检索工具。注意：调用本工具前，AI 上层必须先结合对话上下文将用户模糊、口语化或带代词的提问重写优化为规范、实体明确的标准 RAG 检索 Query。工具接收后会再次进行规则增强并召回最匹配的知识库上下文。",
			Parameters:  parameters,
		},
	}
}

func (t *Tool) Run(ctx context.Context, argsJSON string) (string, error) {
	// 【安全阀控制】：判断当前 Context 是否启用了 RAG 检索功能
	if enableRAG, ok := ctx.Value(parentEnableRAGKey).(bool); ok && !enableRAG {
		return "[RAG 提示] 当前对话会话未启用 RAG 知识库检索功能。请勿继续检索向量库，直接回答用户的问题或提示用户开启 RAG 检索开关。", nil
	}

	var args Args
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("RAG Tool 参数解析失败: %v", err)
	}

	rawQuery := strings.TrimSpace(args.Query)
	if rawQuery == "" {
		return "", fmt.Errorf("query 参数不能为空")
	}

	tenantID := args.TenantID
	if tenantID == "" {
		if ctxVal, ok := ctx.Value(parentKBTenantIDKey).(string); ok && ctxVal != "" {
			tenantID = ctxVal
		} else {
			tenantID = vector.DefaultTenantID
		}
	}

	kbID := args.KBID
	if kbID == "" {
		if ctxVal, ok := ctx.Value(parentKBIDKey).(string); ok && ctxVal != "" {
			kbID = ctxVal
		} else {
			kbID = vector.DefaultKBID
		}
	}

	topK := args.TopK
	if topK <= 0 {
		topK = 5
	}

	// 1. 【核心转换逻辑】：将用户输入的模糊文档/提问转换重写为标准的 RAG 精准检索问题
	standardQueries := RewriteToStandardQueries(rawQuery)
	log.Infof(ctx, "[RAG Tool] Raw Query: '%s' -> Rewritten Standard Queries: %v", rawQuery, standardQueries)

	if t.vectorEngine == nil {
		return fmt.Sprintf("RAG 搜索引擎未初始化。原始查询: '%s'，已转换为标准问题: %v", rawQuery, standardQueries), nil
	}

	enableRerank := false
	if ctxVal, ok := ctx.Value(parentEnableRerankKey).(bool); ok {
		enableRerank = ctxVal
	}
	if args.EnableRerank != nil {
		enableRerank = *args.EnableRerank
	}

	// 2. 依次用主标准问题及相关扩展问题检索向量库
	primaryQuery := standardQueries[0]
	ragContexts, err := t.vectorEngine.RetrieveContext(ctx, tenantID, primaryQuery, topK, enableRerank)
	if err != nil {
		// 备用检索：若主标准问题检索异常，使用原始 Query 尝试降级检索
		var fallbackErr error
		ragContexts, fallbackErr = t.vectorEngine.RetrieveContext(ctx, tenantID, rawQuery, topK, enableRerank)
		if fallbackErr != nil {
			return "", fmt.Errorf("RAG 检索执行失败: %v", err)
		}
	}

	// 3. 结果格式化输出给 LLM
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== RAG 知识库检索结果 ===\n"))
	if kbName, ok := ctx.Value(parentKBNameKey).(string); ok && kbName != "" {
		kbDesc, _ := ctx.Value(parentKBDescriptionKey).(string)
		sb.WriteString(fmt.Sprintf("【目标知识库】: %s (%s)\n", kbName, kbDesc))
	}
	sb.WriteString(fmt.Sprintf("【原始输入查询】: %s\n", rawQuery))
	sb.WriteString(fmt.Sprintf("【转换后的标准 RAG 问题】: %s\n", strings.Join(standardQueries, " | ")))
	sb.WriteString(fmt.Sprintf("【召回切片数量】: %d 条\n\n", len(ragContexts)))

	if len(ragContexts) == 0 {
		sb.WriteString("未找到与该标准问题匹配的相关知识库文档切片。请提示用户补充更具体的信息或确认知识库中是否有相关材料。\n")
		return sb.String(), nil
	}

	for i, c := range ragContexts {
		docTitle := "未知文档"
		if title, ok := c.Metadata["doc_id"].(string); ok {
			docTitle = title
		}

		sb.WriteString(fmt.Sprintf("--- 切片 #%d [相似度得分: %.4f] [文档ID: %s] ---\n", i+1, c.Score, docTitle))
		sb.WriteString(fmt.Sprintf("%s\n", c.FullContext))
		sb.WriteString("--- END 切片 ---\n\n")
	}

	return sb.String(), nil
}

// RewriteToStandardQueries 将模糊口语化、短小或表达不完整的问题转换为符合向量与语义检索特征的标准问题列表
func RewriteToStandardQueries(raw string) []string {
	cleaned := strings.TrimSpace(raw)

	// 去除无意义的前缀修饰短语
	prefixesToStrip := []string{
		"请问", "帮我查一下", "帮我查下", "帮我查", "我想问下", "我想知道", "查一下", "查下", "查查", "告诉我", "你知道", "有没有", "关于", "那个", "怎么看",
	}
	changed := true
	for changed {
		changed = false
		for _, p := range prefixesToStrip {
			if strings.HasPrefix(cleaned, p) {
				cleaned = strings.TrimPrefix(cleaned, p)
				cleaned = strings.TrimSpace(cleaned)
				changed = true
			}
		}
	}
	if cleaned == "" {
		cleaned = raw
	}

	queries := make([]string, 0, 3)

	// 规则 1：价格/费用类模糊提问标准化转换
	if strings.Contains(cleaned, "价格") || strings.Contains(cleaned, "多少钱") || strings.Contains(cleaned, "费用") || strings.Contains(cleaned, "售价") {
		// 提取核心主语
		entity := strings.NewReplacer("价格", "", "多少钱", "", "费用", "", "售价", "", "最新的", "", "那个", "").Replace(cleaned)
		entity = strings.TrimSpace(entity)
		if entity != "" {
			queries = append(queries, fmt.Sprintf("%s 最新价格与历史售价变动说明", entity))
			queries = append(queries, fmt.Sprintf("%s 价格 费用 计费标准", entity))
		} else {
			queries = append(queries, "产品最新价格与费用标准说明")
		}
	}

	// 规则 2：功能/描述变动与配置类标准化转换
	if strings.Contains(cleaned, "配置") || strings.Contains(cleaned, "参数") || strings.Contains(cleaned, "规格") || strings.Contains(cleaned, "限制") {
		entity := strings.NewReplacer("配置", "", "参数", "", "规格", "", "限制", "", "说明", "", "那个", "").Replace(cleaned)
		entity = strings.TrimSpace(entity)
		if entity != "" {
			queries = append(queries, fmt.Sprintf("%s 详细配置参数与功能规格限制说明", entity))
		}
	}

	// 规则 3：安装/部署/操作指导标准化转换
	if strings.Contains(cleaned, "怎么安装") || strings.Contains(cleaned, "部署") || strings.Contains(cleaned, "教程") || strings.Contains(cleaned, "步骤") {
		entity := strings.NewReplacer("怎么安装", "", "部署", "", "教程", "", "步骤", "", "怎么", "").Replace(cleaned)
		entity = strings.TrimSpace(entity)
		if entity != "" {
			queries = append(queries, fmt.Sprintf("%s 安装部署步骤与操作指南教程", entity))
		}
	}

	// 默认兜底：规范化基础问句（主实体 + 规范描述词）
	primaryStandard := fmt.Sprintf("%s 详细说明与标准定义", cleaned)
	if len(queries) == 0 {
		queries = append(queries, primaryStandard)
		queries = append(queries, cleaned)
	} else {
		queries = append(queries, primaryStandard)
	}

	return queries
}
