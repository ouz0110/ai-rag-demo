package middleware

import (
	"context"
	"strings"

	cmpb "ai-rag-demo/api/common/v1"
	"ai-rag-demo/internal/cache"
	"ai-rag-demo/internal/common"
	"ai-rag-demo/internal/conf"
	"ai-rag-demo/internal/pkg/utils"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv4 "github.com/golang-jwt/jwt/v4"
)

type JWTClaims struct {
	jwtv4.RegisteredClaims
	Openid string `json:"openid,omitempty"` // 用户开放标识，祼标识，不带属地和前缀
}

func getJWTClaims(ctx context.Context) *JWTClaims {
	cls, _ := JwtFromContext(ctx)
	return cls.(*JWTClaims)
}

// 无需鉴权的接口。
var whitelist = map[string]struct{}{
	"/ai_rag_demo_api.base.v1.Accounts/Register": {},
	"/ai_rag_demo_api.base.v1.Accounts/Login":    {},
}

func exclude(acl map[string]struct{}) selector.MatchFunc {
	return func(ctx context.Context, operation string) bool {
		return !utils.IsRpcServiceIn(operation, acl)
	}
}

func HTTPAuth(cfg *conf.Config, cache *cache.Cache) middleware.Middleware {
	return middleware.Chain(
		// 创建 selector.JwtCheck 并匹配排除规则
		selector.Server(
			jwtAuth(cfg),
			jwtVerified(cache),
		).Match(exclude(whitelist)).Build(),
	)
}

func getRequestHeader(ctx context.Context) transport.Header {
	hd, _ := transport.FromServerContext(ctx)
	return hd.RequestHeader()
}

func jwtAuth(cfg *conf.Config) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			ctx1, err := JwtCheck(ctx, func(token *jwtv4.Token) (interface{}, error) { return []byte(cfg.Secret), nil },
				JwtWithSigningMethod(jwtv4.SigningMethodHS256),
				JwtWithClaims(func() jwtv4.Claims { return &JWTClaims{} }))
			if err != nil {
				return nil, cmpb.ErrorUnauthorized("登录失效")
			}
			return handler(ctx1, req)
		}
	}
}

func jwtVerified(cache1 *cache.Cache) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			claims := getJWTClaims(ctx)
			authHeader := getRequestHeader(ctx).Get("Authorization")
			requestToken := ""
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 {
					requestToken = parts[1]
				}
			}

			if !conf.IsLocalEnv() {
				if !cache1.Base.CheckToken(ctx, claims.Openid, requestToken) {
					return nil, cmpb.ErrorTokenExpired("token expired")
				}
			}

			ctx = common.WithUser(ctx, common.User{
				Openid: claims.Openid,
			})

			return handler(ctx, req)
		}
	}
}

func getIP(ctx context.Context) (ip string) {
	ip = getRequestHeader(ctx).Get("X-Original-Forwarded-For")
	if ip == "" {
		ip = getRequestHeader(ctx).Get("X-Forwarded-For")
	}

	return
}
