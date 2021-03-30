package urlpath

import (
	urlpath "path"
	"strings"
)

func Base(path string) string {
	return urlpath.Base(path)
}

func Join(elem ...string) string {
	return urlpath.Join(elem...)
}

func Split(path string) (string, string) {
	return urlpath.Split(path)
}

func PopLeft(path string) (string, string) {
	idx := strings.Index(path, "/")
	if idx < 0 {
		return path, ""
	}
	if idx > 0 {
		endIdx := idx
		for strings.Index(path[idx+1:], "/") == 0 {
			idx++
		}
		return path[:endIdx], path[idx+1:]
	}
	startIdx := 0
	for idx == 0 {
		startIdx++
		idx = strings.Index(path[startIdx:], "/")
	}
	if idx < 0 {
		return path[startIdx:], ""
	}
	return path[startIdx : startIdx+idx], path[startIdx+idx:]
}
