package utils

import (
	"context"
	"crypto/rand"
	"strings"
	"time"

	cmpb "ai-rag-demo/api/common/v1"

	tr "github.com/go-kratos/kratos/v2/transport"
	"github.com/golang-jwt/jwt/v4"
)

var secretKey = "M1hcFGdzHqN6efgWUE9jklmY4a3A"

const JWTExpireDays = 7

func JWTInit(secret string) {
	secretKey = secret
}

// JWTWrap 表达当前用户的账号
type JWTWrap struct {
	APP          string `json:"app,omitempty"`          // 应用
	Zone         string `json:"zone,omitempty"`         // 分区（CN, MO, HK）
	Openid       string `json:"openid,omitempty"`       // 用户开放标识，祼标识，不带属地和前缀
	Feather      string `json:"feather,omitempty"`      // 短令牌，通过请求续期
	Platform     string `json:"platform,omitempty"`     // 平台类型(粗)
	PlatformType string `json:"platformType,omitempty"` // 平台类型(细)
	jwt.RegisteredClaims
}

// RedisKey redis 缓存键
func (w *JWTWrap) RedisKey() string {
	return w.Zone + ":" + w.Openid + ":" + w.Platform + ":" + w.APP + ":" + w.Feather
}

const authKey = "auth"

func GetJWTWrapCTX(ctx context.Context) *JWTWrap {
	v := ctx.Value(authKey)
	if v == nil {
		return &JWTWrap{}
	}

	wrap, ok := v.(*JWTWrap)
	if !ok {
		panic("not user jwt detail type")
	}

	return wrap
}

func GetJWTWrapByToken(ctx context.Context) *JWTWrap {
	token := ctxHeader(ctx, "Authorization")
	if token != "" {
		strs := strings.Split(token, " ")
		if len(strs) == 2 {
			wrap, err := ParseJWT(strs[1])
			if err == nil {
				return wrap
			}
		}
	}

	return &JWTWrap{}
}

// GenerateJWT 创建访问凭证（7天有效期）
func GenerateJWT(openid string) string {
	c := &JWTWrap{
		Openid: openid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(JWTExpireDays * 24 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	jwtToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(secretKey))

	return jwtToken
}

// JWTTokenCacheKey 返回用于单机登录校验的 Redis key。
func JWTTokenCacheKey(openid string) string {
	return "jwt:token:" + openid
}

func ParseJWT(token string) (*JWTWrap, error) {
	obj, err := jwt.ParseWithClaims(token, &JWTWrap{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, cmpb.ErrorSignatureIsInvalid("")
		}
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, cmpb.ErrorSignatureIsInvalid("")
	}

	wrap, ok := obj.Claims.(*JWTWrap)
	if !ok {
		return nil, cmpb.ErrorSignatureIsInvalid("")
	}

	return wrap, nil
}

func RandomString(length byte) string {
	buf := make([]byte, length)
	for i := 0; i < 10; i++ {
		if i > 9 {
			panic("random number exception")
		}
		n, err := rand.Read(buf)
		if err == nil && n == len(buf) {
			break
		}
	}
	for i, v := range buf {
		buf[i] = v%94 + 33
	}
	return string(buf)
}

func JWTAuthParse(ctx context.Context) (*JWTWrap, error) {
	if trans, ok := tr.FromServerContext(ctx); ok {

		token := trans.RequestHeader().Get("Authorization")
		if token == "" {
			return nil, cmpb.ErrorHeaderMissing("")
		}

		strs := strings.Split(token, " ")
		if len(strs) != 2 {
			return nil, cmpb.ErrorSignatureIsInvalid("")
		}

		wrap, err := ParseJWT(strs[1])
		if err != nil {
			return nil, err
		}

		return wrap, err
	}
	return nil, cmpb.ErrorWrongParamsError("")
}

// IsRpcServiceIn 检查目标服务是否在列表中，支持两种匹配模式。
// 完整匹配，示例：/square.v1.SquareSrv/ListSquare
// service 层级匹配：/square.v1.SquareSrv
func IsRpcServiceIn(s string, list map[string]struct{}) bool {
	if _, ok := list[s]; ok {
		return true
	}
	for k := range list {
		if strings.Count(k, "/") == 1 {
			if strings.HasPrefix(s, k) {
				return true
			}
		}
	}

	return false
}
