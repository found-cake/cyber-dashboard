package summary

import "strings"

func stripJSONCodeFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") {
		return trimmed
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "```"), "```"))
	if len(inner) >= len("json") && strings.EqualFold(inner[:len("json")], "json") {
		remainder := inner[len("json"):]
		if remainder != "" {
			switch remainder[0] {
			case '{', '[', ' ', '\t', '\r', '\n':
				return strings.TrimSpace(remainder)
			}
		}
	}
	firstLine, remainder, hasLineBreak := strings.Cut(inner, "\n")
	if hasLineBreak {
		firstLine = strings.TrimSpace(firstLine)
		if firstLine != "" && !strings.HasPrefix(firstLine, "{") && !strings.HasPrefix(firstLine, "[") {
			return strings.TrimSpace(remainder)
		}
	}
	return inner
}

func stripJSONLineComments(content string) string {
	var normalized strings.Builder
	normalized.Grow(len(content))
	inString := false
	escaped := false
	for index := 0; index < len(content); index++ {
		current := content[index]
		if inString {
			normalized.WriteByte(current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
				continue
			}
			if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			normalized.WriteByte(current)
			continue
		}
		if current == '/' && index+1 < len(content) && content[index+1] == '/' {
			index += 2
			for index < len(content) && content[index] != '\n' {
				index++
			}
			if index < len(content) {
				normalized.WriteByte('\n')
			}
			continue
		}
		normalized.WriteByte(current)
	}
	return strings.TrimSpace(normalized.String())
}
