package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type userContextKey struct{}

type User struct {
	Openid string
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

func UserFromContext(ctx context.Context) (bool, User) {
	u, ok := ctx.Value(userContextKey{}).(User)
	return ok, u
}

// GetStrictUserKBWorkspaceDir 规划并构建用户在特定知识库下的专属子存储目录路径
// 路径结构：{baseDir}/{openid}/{kbID}
func GetStrictUserKBWorkspaceDir(baseDir, openid, kbID string) (string, error) {
	if baseDir == "" {
		baseDir = "./workspace/uploads"
	}
	if openid == "" {
		return "", fmt.Errorf("user openid is required for kb workspace, access denied")
	}
	if kbID == "" {
		return "", fmt.Errorf("kb_id is required for kb workspace, access denied")
	}
	userWorkDir := filepath.Join(baseDir, openid, kbID)
	if err := os.MkdirAll(userWorkDir, 0755); err != nil {
		return "", fmt.Errorf("create user kb workspace dir failed [%s]: %w", userWorkDir, err)
	}

	cleanWorkDir, err := filepath.Abs(userWorkDir)
	if err != nil {
		return "", fmt.Errorf("resolve user kb workspace abs path failed: %w", err)
	}

	return cleanWorkDir, nil
}

// GetStrictUserAgentWorkDir 基础函数：严格获取当前用户专属的 Agent 工作目录 (绝对路径)
// 强制约束：必须由 Context 提取有效的 openid；若无 openid 或未鉴权则直接报错，绝不使用默认工作目录！
func GetStrictUserAgentWorkDir(ctx context.Context, baseAgentDir string) (string, error) {
	ok, u := UserFromContext(ctx)
	if !ok || u.Openid == "" {
		return "", fmt.Errorf("user openid is required for agent workspace, access denied")
	}

	if baseAgentDir == "" {
		baseAgentDir = "./workspace/agent"
	}

	userWorkDir := filepath.Join(baseAgentDir, u.Openid)
	if err := os.MkdirAll(userWorkDir, 0755); err != nil {
		return "", fmt.Errorf("create user agent workspace dir failed [%s]: %w", userWorkDir, err)
	}

	cleanWorkDir, err := filepath.Abs(userWorkDir)
	if err != nil {
		return "", fmt.Errorf("resolve user agent workspace abs path failed: %w", err)
	}

	return cleanWorkDir, nil
}
