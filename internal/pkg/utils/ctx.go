package utils

import (
	"context"
	"net"
	"strings"

	"github.com/go-kratos/kratos/v2/transport/http"

	"github.com/go-kratos/kratos/v2/transport"
)

const HeaderOfCtxKey = "headerOfCtx"

var (
	ClientIPHeaders = []string{
		"X-Original-Forwarded-For",
		"X-Forwarded-For", // 更通用
		"X-Real-IP",
		"X-Forwarded",
		"HTTP_CLIENT_IP",
		"HTTP_X_FORWARDED_FOR",
		"Proxy-Client-IP",
		"WL-Proxy-Client-IP",
	}
)

func ctxHeader(ctx context.Context, header string) string {
	cc, ok := transport.FromServerContext(ctx)
	if !ok {
		return ""
	}
	return cc.RequestHeader().Get(header)
}

func WrapCTXHeaderZone(ctx context.Context) string {
	return ctxHeader(ctx, "Zone")
}

func WrapCTXHeaderLanguage(ctx context.Context) string {
	return ctxHeader(ctx, "Accept-Language")
}

func WrapCTXHeaderSrcIP(ctx context.Context) string {
	remoteAddr := ctxHeader(ctx, "X-Original-Forwarded-For")
	if len(remoteAddr) == 0 {
		remoteAddr = ctxHeader(ctx, "X-Forwarded-For")
	}
	return remoteAddr
}

func IsValidPublicIP(ip net.IP) bool {
	return len(ip) > 0 &&
		ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsInterfaceLocalMulticast() &&
		!ip.IsLinkLocalMulticast()
}
func WrapCTXClientIP(ctx context.Context) (ip string, found bool) {
	if req, ok := http.RequestFromServerContext(ctx); ok && req != nil && req.Header != nil {
		for _, header := range ClientIPHeaders {
			if vs := req.Header.Values(header); len(vs) > 0 {
				for _, v := range vs {
					if v == "" {
						continue
					}
					v = strings.ReplaceAll(v, " ", "")
					v = strings.ReplaceAll(v, ";", ",")
					if strings.ContainsRune(v, ',') {
						vv := strings.Split(v, ",")
						v = vv[0] // get first
					}
					if x := net.ParseIP(v); IsValidPublicIP(x) {
						return v, true
					}
				}
			}
		}
	}
	return
}

func GetRequestHeader(ctx context.Context, names ...string) ([]string, bool) {
	if req, ok := http.RequestFromServerContext(ctx); ok && req != nil && req.Header != nil {
		for _, name := range names {
			if v := req.Header.Values(name); len(v) > 0 {
				return v, true
			}
		}
	}
	return nil, false
}
