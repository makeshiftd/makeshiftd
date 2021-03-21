package urlpath

import "strings"

func PopPath(p string) (string, string) {
	idx := strings.Index(p, "/")
	if idx < 0 {
		return p, ""
	}
	if idx > 0 {
		return p[:idx-1], p[idx+1:]
	}
	startIdx := 0
	for idx == 0 {
		startIdx++
		idx = strings.Index(p[startIdx:], "/")
	}
	if idx < 0 {
		return p[startIdx:], ""
	}
	return p[startIdx : startIdx+idx], p[startIdx+idx:]
}
