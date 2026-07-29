package compressor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"

	openai "github.com/sashabaranov/go-openai"
)

type SummarizerFunc func(ctx context.Context, toSummarize []openai.ChatCompletionMessage) (string, error)
type OnCompressStartFunc func(origTokens int, compressedCount int)

type ICompressor interface {
	EstimateTokens(msgs []openai.ChatCompletionMessage) int
	ShouldCompress(msgs []openai.ChatCompletionMessage) (bool, int)
	Compress(
		ctx context.Context,
		currentCompressCount int32,
		msgs []openai.ChatCompletionMessage,
		summarizerFunc SummarizerFunc,
		onStart OnCompressStartFunc,
	) (*CompressResult, error)
}

type CompressResult struct {
	CompressedMessages []openai.ChatCompletionMessage // 准备发送给 LLM 的切片
	OriginalTokens     int                            // 压缩前 Token 数
	CompressedTokens   int                            // 压缩后 Token 数
	CompressedCount    int                            // 被裁切消息条数
	SummaryText        string                         // 生成的精炼摘要
	IsCompressed       bool                           // 是否发生了压缩/裁切
	IsMaxLimitReached  bool                           // 是否达到了最大压缩次数限制 (触发熔断)
	NewCheckpointMsg   *openai.ChatCompletionMessage  // 需要持久化落盘的 Checkpoint 描述消息
}

type ContextCompressor struct {
	cfg *conf.OpenAIContextCompressConfig
}

var _ ICompressor = (*ContextCompressor)(nil)

func NewContextCompressor(cfg *conf.OpenAIContextCompressConfig) *ContextCompressor {
	return &ContextCompressor{cfg: cfg}
}

// EstimateTokens 快速加权估算 Token 数量 (1 中文字符 ≈ 0.75 Token)
func (c *ContextCompressor) EstimateTokens(msgs []openai.ChatCompletionMessage) int {
	total := 0
	for _, m := range msgs {
		total += len([]rune(m.Content)) * 3 / 4
		for _, tc := range m.ToolCalls {
			total += len([]rune(tc.Function.Arguments)) * 3 / 4
		}
	}
	return total
}

// ShouldCompress 判断当前消息切片是否需要触发压缩
func (c *ContextCompressor) ShouldCompress(msgs []openai.ChatCompletionMessage) (bool, int) {
	if c.cfg == nil || !c.cfg.Enable || c.cfg.MaxContextTokens <= 0 {
		return false, 0
	}
	currentTokens := c.EstimateTokens(msgs)
	threshold := int(float64(c.cfg.MaxContextTokens) * c.cfg.CompressRatio)
	return currentTokens > threshold, currentTokens
}

// Compress 执行工具安全裁切与上下文压缩/熔断降级
func (c *ContextCompressor) Compress(
	ctx context.Context,
	currentCompressCount int32,
	msgs []openai.ChatCompletionMessage,
	summarizerFunc SummarizerFunc,
	onStart OnCompressStartFunc,
) (*CompressResult, error) {
	should, origTokens := c.ShouldCompress(msgs)
	if !should {
		return &CompressResult{
			CompressedMessages: msgs,
			OriginalTokens:     origTokens,
			CompressedTokens:   origTokens,
			IsCompressed:       false,
		}, nil
	}

	// 1. 拆分首部初始 System 消息与后续对话消息
	initialSysCount := 0
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleSystem {
			initialSysCount++
		} else {
			break
		}
	}

	sysMsgs := msgs[:initialSysCount]
	dialogueMsgs := msgs[initialSysCount:]

	keepCount := c.cfg.KeepRecentMessages
	if keepCount <= 0 {
		keepCount = 6
	}

	minUncompressed := c.cfg.MinUncompressedMsgs
	if minUncompressed <= 0 {
		minUncompressed = 6
	}

	// 未积累足够多的未压缩消息，且仍未达到死水位，暂时维持原状，避免频繁微调
	if len(dialogueMsgs) <= keepCount || len(dialogueMsgs) < (keepCount+minUncompressed) {
		return &CompressResult{CompressedMessages: msgs, IsCompressed: false}, nil
	}

	// 🎯 动态 Token 预算留存算法 (Token-Budget Dynamic Retention):
	// 对于 128k 等大上下文，按比例逆向保留近期 ~30% 的 Token 预算 (或至少 4k~38k Tokens)，
	// 避免在 128k 窗口下将上下文“机械断崖式”压缩到仅剩几条消息。
	targetKeepTokens := int(float64(c.cfg.MaxContextTokens) * 0.30)
	if targetKeepTokens < 4000 {
		targetKeepTokens = 4000
	}

	accTokens := 0
	rawCandidateIdx := len(dialogueMsgs) - keepCount
	for i := len(dialogueMsgs) - 1; i >= 0; i-- {
		msgTokens := c.EstimateTokens([]openai.ChatCompletionMessage{dialogueMsgs[i]})
		accTokens += msgTokens
		if accTokens >= targetKeepTokens && (len(dialogueMsgs)-i) >= keepCount {
			rawCandidateIdx = i
			break
		}
	}
	if rawCandidateIdx < 0 {
		rawCandidateIdx = 0
	}

	splitIdx := c.findSafeToolBoundary(dialogueMsgs, rawCandidateIdx)
	if splitIdx <= 0 {
		return &CompressResult{CompressedMessages: msgs, IsCompressed: false}, nil
	}

	toCompress := dialogueMsgs[:splitIdx]
	toKeep := dialogueMsgs[splitIdx:]

	maxCompressCount := c.cfg.MaxCompressCount
	if maxCompressCount <= 0 {
		maxCompressCount = 5
	}

	// 2. 检查是否达到了最大压缩次数限制 (触发熔断)
	if int(currentCompressCount) >= maxCompressCount {
		log.Warnw(ctx, "compress_max_limit_reached_fallback_fifo",
			"current_compress_count", currentCompressCount,
			"max_limit", maxCompressCount,
		)

		// 降级为安全 FIFO 滑动窗口：只保留 首部 System + 最近安全窗口
		newLLMMsgs := make([]openai.ChatCompletionMessage, 0, len(sysMsgs)+len(toKeep))
		newLLMMsgs = append(newLLMMsgs, sysMsgs...)
		newLLMMsgs = append(newLLMMsgs, toKeep...)
		compressedTokens := c.EstimateTokens(newLLMMsgs)

		return &CompressResult{
			CompressedMessages: newLLMMsgs,
			OriginalTokens:     origTokens,
			CompressedTokens:   compressedTokens,
			CompressedCount:    len(toCompress),
			SummaryText:        "已达到单会话最大压缩次数限制，系统已启动安全 FIFO 滚动窗口。",
			IsCompressed:       true,
			IsMaxLimitReached:  true,
			NewCheckpointMsg:   nil, // 熔断后不再持久化新的 LLM 摘要 Checkpoint
		}, nil
	}

	// 🎯 确定进行 LLM 摘要生成，在此处第一时间向客户端触发 onStart 回调
	if onStart != nil {
		onStart(origTokens, len(toCompress))
	}

	// 3. 正常调用 LLM 生成摘要 (读取配置的动态超时时间，默认 30 秒)
	timeoutSec := 30
	if c.cfg != nil && c.cfg.Timeout > 0 {
		timeoutSec = c.cfg.Timeout
	}
	summaryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	var summaryText string
	var err error
	if summarizerFunc != nil {
		summaryText, err = summarizerFunc(summaryCtx, c.distillToolOutputs(toCompress))
	} else {
		err = fmt.Errorf("summarizerFunc is nil")
	}

	if err != nil {
		log.Warnw(ctx, "llm_summarize_failed_fallback_text_extraction", "error", err)
		// 🎯 智能降级：从被裁切的消息中提取用户核心提问与最新回答摘要，避免丢失前文语义
		summaryText = c.buildFallbackExtractionSummary(toCompress)
	}

	checkpointMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: fmt.Sprintf("💡 【上下文压缩摘要】:\n%s", summaryText),
	}

	// 4. 重组发送给 LLM 的切片: [首部 System] + [最新 Checkpoint] + [最近 N 轮]
	newLLMMsgs := make([]openai.ChatCompletionMessage, 0, len(sysMsgs)+1+len(toKeep))
	newLLMMsgs = append(newLLMMsgs, sysMsgs...)
	newLLMMsgs = append(newLLMMsgs, checkpointMsg)
	newLLMMsgs = append(newLLMMsgs, toKeep...)

	compressedTokens := c.EstimateTokens(newLLMMsgs)

	return &CompressResult{
		CompressedMessages: newLLMMsgs,
		OriginalTokens:     origTokens,
		CompressedTokens:   compressedTokens,
		CompressedCount:    len(toCompress),
		SummaryText:        summaryText,
		IsCompressed:       true,
		IsMaxLimitReached:  false,
		NewCheckpointMsg:   &checkpointMsg,
	}, nil
}

// buildFallbackExtractionSummary 文本抽取式降级摘要逻辑 (当 LLM 摘要超时/报错时使用)
func (c *ContextCompressor) buildFallbackExtractionSummary(msgs []openai.ChatCompletionMessage) string {
	var userTopics []string
	var lastAssistantReply string

	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleUser && m.Content != "" {
			runes := []rune(m.Content)
			if len(runes) > 80 {
				userTopics = append(userTopics, string(runes[:80])+"...")
			} else {
				userTopics = append(userTopics, m.Content)
			}
		} else if m.Role == openai.ChatMessageRoleAssistant && m.Content != "" {
			runes := []rune(m.Content)
			if len(runes) > 150 {
				lastAssistantReply = string(runes[:150]) + "..."
			} else {
				lastAssistantReply = m.Content
			}
		}
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("【上下文历史摘要】(已自动合并 %d 条早期消息):\n", len(msgs)))
	
	if len(userTopics) > 0 {
		sb.WriteString("📌 早期探讨核心主题:\n")
		// 最多列出前 3 个核心提问主题
		limit := 3
		if len(userTopics) < limit {
			limit = len(userTopics)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("  - %s\n", userTopics[i]))
		}
	}

	if lastAssistantReply != "" {
		sb.WriteString(fmt.Sprintf("📌 上一阶段核心结论:\n  - %s\n", lastAssistantReply))
	}

	return sb.String()
}

// findSafeToolBoundary 向上寻找安全的 Tool-Call 闭包裁切边界，避免裁切掉成对的 ToolCall 与 ToolResult
func (c *ContextCompressor) findSafeToolBoundary(msgs []openai.ChatCompletionMessage, candidateIdx int) int {
	idx := candidateIdx
	for idx > 0 && idx < len(msgs) {
		msg := msgs[idx]
		if msg.Role == openai.ChatMessageRoleTool ||
			(msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0) {
			idx--
		} else {
			break
		}
	}
	return idx
}

// distillToolOutputs 预处理待压缩列表中的巨大 Tool 输出，避免庞大的工具数据冲爆摘要模型的输入窗口
func (c *ContextCompressor) distillToolOutputs(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	distilled := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool && len(m.Content) > 1200 {
			runes := []rune(m.Content)
			m.Content = fmt.Sprintf("%s\n...[已蒸馏长工具输出: 原始长度 %d 字符]...", string(runes[:600]), len(runes))
		}
		distilled = append(distilled, m)
	}
	return distilled
}
