// Package events предоставляет адаптеры для публикации доменных событий.
package events

// splitAggregateID разбивает aggregate ID на части
func splitAggregateID(id string) []string {
	for _, sep := range []string{"-", "_"} {
		parts := splitString(id, sep)
		if len(parts) > 1 {
			return parts
		}
	}
	return []string{id}
}

// splitString разбивает строку по разделителю
func splitString(s, sep string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			parts = append(parts, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

