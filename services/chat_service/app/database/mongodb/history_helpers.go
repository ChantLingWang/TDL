package mongodb

import (
	"regexp"
	"strings"
	"time"
)

// historyCollectionNames 返回最近 days 天内需要查询的“月集合”名并去重，
// 避免同一个月被重复查询（例如当月 7 号时会把当月集合查 7 遍，导致历史消息重复）。
func historyCollectionNames(prefix string, days int, now time.Time) []string {
	seen := make(map[string]struct{}, days)
	names := make([]string, 0, days)
	for i := 0; i < days; i++ {
		name := prefix + now.AddDate(0, 0, -i).Format("200601")
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// IsPrivateParticipant 判断用户是否为私聊会话的参与者。
// 私聊会话 ID 由 GenerateSessionID 生成，格式为 "较小ID_较大ID"。
// 用户 ID 为数字或不含下划线的标识符，因此按第一个 "_" 拆分即可。
func IsPrivateParticipant(sessionID, userID string) bool {
	if userID == "" {
		return false
	}
	left, right, found := strings.Cut(sessionID, "_")
	if !found {
		return false
	}
	return left == userID || right == userID
}

// regexpQuote 转义用户输入，防止其被当作正则表达式造成注入或 ReDoS。
func regexpQuote(s string) string {
	return regexp.QuoteMeta(s)
}
