package mongodb

import "time"

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
