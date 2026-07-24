package readfiles

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	phoneRegex     = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	emailRegex     = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	ipRegex        = regexp.MustCompile(`\b(\d{1,3}\.){3}\d{1,3}\b`)
	idCardRegex    = regexp.MustCompile(`\b\d{17}[\dXx]\b`)
	tokenRegex     = regexp.MustCompile(`(?i)(sk|token|api[_-]?key|access[_-]?key|secret|password|passwd|pwd)[^\S\r\n]{0,5}[:=][^\S\r\n]{0,5}[^\s"']{8,}`)
	secretKeyRegex = regexp.MustCompile(`(?i)(aws|ak|sk|secret)[^\S\r\n]{0,5}[:=][^\S\r\n]{0,5}[^\s"']{8,}`)
)

func Desensitize(content string) string {
	content = phoneRegex.ReplaceAllStringFunc(content, maskPhone)
	content = emailRegex.ReplaceAllStringFunc(content, maskEmail)
	content = ipRegex.ReplaceAllString(content, "*.*.*.*")
	content = idCardRegex.ReplaceAllStringFunc(content, maskIDCard)
	content = tokenRegex.ReplaceAllStringFunc(content, maskSensitiveKeyValue)
	content = secretKeyRegex.ReplaceAllStringFunc(content, maskSensitiveKeyValue)
	return content
}

func maskPhone(phone string) string {
	if len(phone) < 7 {
		return strings.Repeat("*", len(phone))
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***.***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) > 2 {
		local = local[:1] + strings.Repeat("*", len(local)-1)
	} else if len(local) > 0 {
		local = strings.Repeat("*", len(local))
	}
	return local + "@" + domain
}

func maskIDCard(id string) string {
	if len(id) < 8 {
		return strings.Repeat("*", len(id))
	}
	return id[:6] + strings.Repeat("*", len(id)-10) + id[len(id)-4:]
}

func maskSensitiveKeyValue(s string) string {
	lower := strings.ToLower(s)
	for _, prefix := range []string{"password", "passwd", "pwd", "token", "secret", "apikey", "api_key", "access_key", "secret_key", "sk"} {
		idx := strings.Index(lower, prefix)
		if idx >= 0 {
			sepIdx := strings.IndexAny(lower[idx:], "=:")
			if sepIdx > 0 {
				start := idx + sepIdx + 1
				valueStart := start
				for valueStart < len(s) && unicode.IsSpace(rune(s[valueStart])) {
					valueStart++
				}
				if valueStart < len(s) {
					return s[:valueStart] + "****"
				}
			}
		}
	}
	return s[:strings.IndexFunc(s, unicode.IsSpace)] + " ****"
}

func DesensitizeBytes(data []byte) []byte {
	return []byte(Desensitize(string(data)))
}

func RuneLen(s string) int {
	return utf8.RuneCountInString(s)
}

func RuneLenBytes(b []byte) int {
	return utf8.RuneCount(b)
}

func ContainsSensitive(content string) bool {
	return phoneRegex.MatchString(content) ||
		emailRegex.MatchString(content) ||
		idCardRegex.MatchString(content) ||
		tokenRegex.MatchString(content) ||
		secretKeyRegex.MatchString(content)
}
