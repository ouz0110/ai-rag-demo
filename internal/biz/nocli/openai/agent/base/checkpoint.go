package base

import (
	"ai-rag-demo/internal/biz/nocli/checkpoint"
)

const (
	SubAgentCheckpointSaverKey  = "sub_agent_checkpoint_saver"
	SubAgentCheckpointGetterKey = "sub_agent_checkpoint_getter"
)

// SubAgentCheckpoint 子 Agent 专属中断恢复 Checkpoint 快照
type SubAgentCheckpoint = checkpoint.SubAgentCheckpoint
