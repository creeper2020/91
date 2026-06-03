package scanner

import (
	"path"
	"regexp"
	"strings"

	"github.com/video-site/backend/internal/fixedtags"
)

// ParsedName 从文件名里解析出的视频元数据
type ParsedName struct {
	Title  string
	Author string
	Tags   []string
}

var (
	reTags   = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*`) // [前缀]
	reAuthor = regexp.MustCompile(`\s*-\s*([^-]+?)\s*$`) // - author
)

// Parse 按约定解析：[前缀] 标题 - 作者.ext
// 任何字段缺失都能降级
func Parse(filename string) ParsedName {
	name := strings.TrimSuffix(filename, path.Ext(filename))

	var out ParsedName

	if m := reTags.FindStringSubmatch(name); m != nil {
		name = strings.TrimSpace(name[len(m[0]):])
	}

	if m := reAuthor.FindStringSubmatch(name); m != nil {
		out.Author = strings.TrimSpace(m[1])
		name = strings.TrimSpace(name[:len(name)-len(m[0])])
	}

	out.Title = strings.TrimSpace(name)
	out.Tags = fixedtags.MatchFilename(filename)
	return out
}
