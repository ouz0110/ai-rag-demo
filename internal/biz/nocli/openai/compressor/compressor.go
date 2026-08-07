package compressor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/log"
	"ai-rag-demo/internal/pkg/observability"

	openai "github.com/sashabaranov/go-openai"
)

const CheckpointPrefix = "💡 【上下文压缩摘要】"

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
	ToCompressTokens   int                            // 待压缩片段 Token 数
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

// EstimateStringTokens 零内存分配计算单个字符串的 Token 数
// - ASCII 字符（英文/代码/数字）: ~0.3 Token/Byte
// - 非 ASCII 字符（中日韩等 CJK 字符）: ~1.8 Token/Rune
func EstimateStringTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	nonASCIICount := 0
	nonASCIIBytes := 0
	for _, r := range s {
		if r > 127 {
			nonASCIICount++
			if r <= 0x7FF {
				nonASCIIBytes += 2
			} else if r <= 0xFFFF {
				nonASCIIBytes += 3
			} else {
				nonASCIIBytes += 4
			}
		}
	}
	asciiBytes := len(s) - nonASCIIBytes
	tokens := float64(asciiBytes)*0.3 + float64(nonASCIICount)*1.8
	return int(tokens)
}

// EstimateMessageTokens 计算单条消息的估算 Token 数（包含 OpenAI 消息结构体固定 Overhead）
func EstimateMessageTokens(m *openai.ChatCompletionMessage) int {
	tokens := 4 // 基础消息结构与 Role 标识符开销
	tokens += EstimateStringTokens(m.Role)
	tokens += EstimateStringTokens(m.Content)
	tokens += EstimateStringTokens(m.Name)

	for _, tc := range m.ToolCalls {
		tokens += 4 // ToolCall 包装开销
		tokens += EstimateStringTokens(tc.Function.Name)
		tokens += EstimateStringTokens(tc.Function.Arguments)
	}

	for _, mc := range m.MultiContent {
		tokens += EstimateStringTokens(mc.Text)
	}
	return tokens
}

// EstimateTokens 快速精准估算消息切片的 Token 总数 (支持中英双语高效混合估算，零内存分配)
func (c *ContextCompressor) EstimateTokens(msgs []openai.ChatCompletionMessage) int {
	total := 0
	for i := range msgs {
		total += EstimateMessageTokens(&msgs[i])
	}
	return total
}

// ShouldCompress 判断当前消息切片是否需要触发压缩
func (c *ContextCompressor) ShouldCompress(msgs []openai.ChatCompletionMessage) (bool, int) {
	if c.cfg == nil || !c.cfg.Enable || c.cfg.MaxContextTokens <= 0 {
		return false, 0
	}
	ratio := c.cfg.CompressRatio
	if ratio <= 0 || ratio >= 1.0 {
		ratio = 0.75
	}
	currentTokens := c.EstimateTokens(msgs)
	threshold := int(float64(c.cfg.MaxContextTokens) * ratio)
	return currentTokens > threshold, currentTokens
}

// isCheckpointMessage 判断单条消息是否为已有的 Checkpoint 摘要消息
func isCheckpointMessage(m *openai.ChatCompletionMessage) bool {
	if m.Role == openai.ChatMessageRoleSystem &&
		(strings.HasPrefix(m.Content, CheckpointPrefix) || strings.HasPrefix(m.Content, "【上下文历史摘要】")) {
		return true
	}
	return false
}

// Compress 执行工具安全裁切与上下文压缩/熔断降级
func (c *ContextCompressor) Compress(
	ctx context.Context,
	currentCompressCount int32,
	msgs []openai.ChatCompletionMessage,
	summarizerFunc SummarizerFunc,
	onStart OnCompressStartFunc,
) (compRes *CompressResult, err error) {
	totalTokens := c.EstimateTokens(msgs)
	obs := observability.GetObserver(ctx)
	compressCtx, endCompress := obs.OnCompressStart(ctx, &observability.CompressInfo{
		OriginalTokens: totalTokens,
		CompressCount:  currentCompressCount,
	})
	ctx = compressCtx
	defer func() {
		if compRes != nil {
			endCompress(compRes.CompressedTokens, compRes.IsMaxLimitReached, compRes.SummaryText, err)
		} else {
			endCompress(0, false, "", err)
		}
	}()

	should, origTokens := c.ShouldCompress(msgs)
	if !should {
		return &CompressResult{
			CompressedMessages: msgs,
			OriginalTokens:     origTokens,
			CompressedTokens:   origTokens,
			IsCompressed:       false,
		}, nil
	}

	// 1. 拆分首部初始 Base System 消息（排除已生成的历史 Checkpoint 消息）与后续对话消息
	initialSysCount := 0
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleSystem && !isCheckpointMessage(&m) {
			initialSysCount++
		} else {
			break
		}
	}

	sysMsgs := msgs[:initialSysCount]
	dialogueMsgs := msgs[initialSysCount:]

	keepCount := 6
	if c.cfg != nil && c.cfg.KeepRecentMessages > 0 {
		keepCount = c.cfg.KeepRecentMessages
	}

	minUncompressed := 6
	if c.cfg != nil && c.cfg.MinUncompressedMsgs > 0 {
		minUncompressed = c.cfg.MinUncompressedMsgs
	}

	// 未积累足够多的未压缩消息，且仍未达到死水位，暂时维持原状，避免频繁微调
	if len(dialogueMsgs) <= keepCount || len(dialogueMsgs) < (keepCount+minUncompressed) {
		return &CompressResult{CompressedMessages: msgs, IsCompressed: false}, nil
	}

	maxTokens := 16384
	if c.cfg != nil && c.cfg.MaxContextTokens > 0 {
		maxTokens = c.cfg.MaxContextTokens
	}

	ratio := 0.75
	if c.cfg != nil && c.cfg.CompressRatio > 0 && c.cfg.CompressRatio < 1.0 {
		ratio = c.cfg.CompressRatio
	}

	targetRatio := 0.35
	if c.cfg != nil && c.cfg.TargetCompressRatio > 0 && c.cfg.TargetCompressRatio < 1.0 {
		targetRatio = c.cfg.TargetCompressRatio
	}

	// 🎯 动态目标留存比率 (Target Context Ratio):
	// 当达到 compress_ratio (如 0.75) 触发水线时，将压缩后的目标上下文控制在 target_compress_ratio (如 0.35)，
	// 腾出 65% 的上下文缓冲区供后续新会话增长。
	targetContextTokens := int(float64(maxTokens) * targetRatio)

	// 🎯 三段式动态倒推预算扣除:
	// 1. 首部 Base System 提示词开销 (动态计算，如 500-1000t)
	sysTokens := c.EstimateTokens(sysMsgs)
	// 2. LLM Checkpoint 摘要开销预留 (如 500-1000t)
	summaryTokens := 500
	if c.cfg != nil && c.cfg.MaxSummaryTokens > 0 {
		summaryTokens = c.cfg.MaxSummaryTokens
	}

	// 3. 动态导出近期对话保留预算 (targetKeepTokens)
	targetKeepTokens := targetContextTokens - sysTokens - summaryTokens
	if targetKeepTokens < 500 {
		targetKeepTokens = 500
	}

	accTokens := 0
	rawCandidateIdx := len(dialogueMsgs) - keepCount
	for i := len(dialogueMsgs) - 1; i >= 0; i-- {
		msgTokens := EstimateMessageTokens(&dialogueMsgs[i])
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
	toCompressTokens := c.EstimateTokens(toCompress)

	// 🎯 动态压缩收益门禁 (Dynamic Compression ROI Guard):
	// 待压缩旧历史 toCompress 必须达到容量差额，防止在最近保留区占大头时无意义触发 LLM 摘要
	minCompressTokens := int(float64(maxTokens)*ratio) - targetKeepTokens - sysTokens
	if minCompressTokens < 500 {
		minCompressTokens = 500
	}
	if toCompressTokens < minCompressTokens {
		log.Debugw(ctx, "compress_skipped_low_roi", "to_compress_tokens", toCompressTokens, "min_required", minCompressTokens, "orig_tokens", origTokens)
		return &CompressResult{CompressedMessages: msgs, IsCompressed: false}, nil
	}

	maxCompressCount := 5
	if c.cfg != nil && c.cfg.MaxCompressCount > 0 {
		maxCompressCount = c.cfg.MaxCompressCount
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
	summaryTimeout := 30 * time.Second
	if c.cfg != nil && c.cfg.Timeout.Duration > 0 {
		summaryTimeout = c.cfg.Timeout.Duration
	}
	summaryCtx, cancel := context.WithTimeout(ctx, summaryTimeout)
	defer cancel()

	var summaryText string
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
		Content: fmt.Sprintf("%s:\n%s", CheckpointPrefix, summaryText),
	}

	// 4. 重组发送给 LLM 的切片: [首部 Base System] + [最新 Checkpoint] + [最近 N 轮]
	newLLMMsgs := make([]openai.ChatCompletionMessage, 0, len(sysMsgs)+1+len(toKeep))
	newLLMMsgs = append(newLLMMsgs, sysMsgs...)
	newLLMMsgs = append(newLLMMsgs, checkpointMsg)
	newLLMMsgs = append(newLLMMsgs, toKeep...)

	compressedTokens := c.EstimateTokens(newLLMMsgs)

	return &CompressResult{
		CompressedMessages: newLLMMsgs,
		OriginalTokens:     origTokens,
		CompressedTokens:   compressedTokens,
		ToCompressTokens:   toCompressTokens,
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
	var toolCalls []string
	var lastAssistantReply string

	for _, m := range msgs {
		switch m.Role {
		case openai.ChatMessageRoleUser:
			if m.Content != "" {
				runes := []rune(m.Content)
				if len(runes) > 100 {
					userTopics = append(userTopics, string(runes[:100])+"...")
				} else {
					userTopics = append(userTopics, m.Content)
				}
			}
		case openai.ChatMessageRoleAssistant:
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					toolCalls = append(toolCalls, tc.Function.Name)
				}
			}
			if m.Content != "" {
				runes := []rune(m.Content)
				if len(runes) > 200 {
					lastAssistantReply = string(runes[:200]) + "..."
				} else {
					lastAssistantReply = m.Content
				}
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("【上下文历史摘要】(已自动合并 %d 条早期消息):\n", len(msgs)))

	if len(userTopics) > 0 {
		sb.WriteString("📌 早期探讨核心主题:\n")
		limit := 5
		if len(userTopics) < limit {
			limit = len(userTopics)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("  - %s\n", userTopics[i]))
		}
	}

	if len(toolCalls) > 0 {
		sb.WriteString("🛠️ 期间已调用的工具:\n")
		seen := make(map[string]bool)
		var uniqueTools []string
		for _, t := range toolCalls {
			if !seen[t] {
				seen[t] = true
				uniqueTools = append(uniqueTools, t)
			}
		}
		sb.WriteString(fmt.Sprintf("  - %s\n", strings.Join(uniqueTools, ", ")))
	}

	if lastAssistantReply != "" {
		sb.WriteString(fmt.Sprintf("📌 上一阶段核心结论:\n  - %s\n", lastAssistantReply))
	}

	return sb.String()
}

// findSafeToolBoundary 向上寻找安全的裁切边界，保证绝不切断 Pair (Assistant ToolCalls <-> Tool Result)
func (c *ContextCompressor) findSafeToolBoundary(msgs []openai.ChatCompletionMessage, candidateIdx int) int {
	if candidateIdx <= 0 || candidateIdx >= len(msgs) {
		return candidateIdx
	}

	idx := candidateIdx

	// 1. 如果当前候选索引指向 Tool 消息或带 ToolCalls 的 Assistant 消息，向前回溯
	for idx > 0 && idx < len(msgs) {
		msg := msgs[idx]
		if msg.Role == openai.ChatMessageRoleTool ||
			(msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0) {
			idx--
		} else {
			break
		}
	}

	// 2. 继续向前寻找到最近的 User 消息边界，确保 toKeep 切片干净地从 User 消息开始
	for scan := idx; scan >= 0; scan-- {
		if msgs[scan].Role == openai.ChatMessageRoleUser {
			return scan
		}
	}

	// 3. 如果没有在前面找到 User 消息，退化为按 idx 安全切分
	if idx > 0 {
		return idx
	}
	return 0
}

// distillToolOutputs 预处理待压缩列表中的巨大 Tool 输出，避免庞大的工具数据冲爆摘要模型的输入窗口
func (c *ContextCompressor) distillToolOutputs(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	distilled := make([]openai.ChatCompletionMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool {
			runes := []rune(m.Content)
			if len(runes) > 600 {
				m.Content = fmt.Sprintf("%s\n...[已蒸馏长工具输出: 原始长度 %d 字符]...", string(runes[:600]), len(runes))
			}
		}
		distilled = append(distilled, m)
	}
	return distilled
}

