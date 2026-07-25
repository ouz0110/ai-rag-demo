package session

import (
	"context"
)

// LoadSessionApprovedTools 载入当前 Session 历史上已获得 AS_SESSION_TOOL 授权的工具白名单
func (m *SessionManager) LoadSessionApprovedTools(ctx context.Context, sessionID string) map[string]bool {
	approvedMap := make(map[string]bool)
	if sessionID == "" {
		return approvedMap
	}

	toolNames, err := m.allDb.Base.NocliInterruptRepo.GetApprovedSessionTools(ctx, sessionID)
	if err == nil {
		for _, name := range toolNames {
			approvedMap[name] = true
		}
	}
	return approvedMap
}
