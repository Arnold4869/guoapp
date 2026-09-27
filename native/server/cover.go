package main

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxDramaQueryBytes = 8192
	maxCoverBytes      = 16 << 20
)

var imageTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
	".avif": "image/avif",
	".heic": "image/heic",
}

var videoTypes = map[string]string{
	".mp4":  "video/mp4",
	".m4v":  "video/mp4",
	".ts":   "video/mp2t",
	".m4s":  "video/iso.segment",
	".webm": "video/webm",
	".mov":  "video/quicktime",
	".mkv":  "video/x-matroska",
}

func dramaPayload(value string) (map[string]any, bool) {
	if value == "" || len(value) > maxDramaQueryBytes {
		return nil, false
	}
	var drama map[string]any
	if json.Unmarshal([]byte(value), &drama) != nil || drama["id"] == nil {
		return nil, false
	}
	return drama, true
}

func contentTypeFor(path string, table map[string]string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if value, found := table[extension]; found {
		return value
	}
	if value := mime.TypeByExtension(extension); value != "" {
		return value
	}
	return "application/octet-stream"
}

func insideDirectory(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(base, absolute)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func serveFile(writer http.ResponseWriter, request *http.Request, path, contentType string, limit int64) {
	file, err := os.Open(path)
	if err != nil {
		writeFailure(writer, http.StatusNotFound, "文件不存在或已被清理")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeFailure(writer, http.StatusNotFound, "文件不存在或已被清理")
		return
	}
	if limit > 0 && info.Size() > limit {
		writeFailure(writer, http.StatusRequestEntityTooLarge, "文件过大，无法在此接口返回")
		return
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
}

func (s *service) cover(writer http.ResponseWriter, request *http.Request) {
	if !s.authorizeMedia(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	drama, ok := dramaPayload(request.URL.Query().Get("drama"))
	if !ok {
		writeFailure(writer, http.StatusBadRequest, "缺少有效的剧集参数")
		return
	}
	data, err := callCore(map[string]any{
		"action": "cover",
		"drama":  drama,
		"force":  request.URL.Query().Get("force") == "1",
	})
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, err.Error())
		return
	}
	var result struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(data, &result) != nil || result.Path == "" {
		writeFailure(writer, http.StatusBadGateway, "封面缓存未返回文件")
		return
	}
	if !insideDirectory(s.dataDir, result.Path) {
		writeFailure(writer, http.StatusBadGateway, "封面文件路径异常")
		return
	}
	serveFile(writer, request, result.Path, contentTypeFor(result.Path, imageTypes), maxCoverBytes)
}

func (s *service) downloadRoot() string {
	data, err := callCore(map[string]any{"action": "downloadDirectory"})
	if err != nil {
		return ""
	}
	var result struct {
		Directory string `json:"directory"`
	}
	if json.Unmarshal(data, &result) != nil {
		return ""
	}
	return result.Directory
}

func (s *service) local(writer http.ResponseWriter, request *http.Request) {
	if !s.authorizeMedia(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	drama, ok := dramaPayload(request.URL.Query().Get("drama"))
	if !ok {
		writeFailure(writer, http.StatusBadRequest, "缺少有效的剧集参数")
		return
	}
	index := 0
	if value := request.URL.Query().Get("index"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 {
			writeFailure(writer, http.StatusBadRequest, "分集序号无效")
			return
		}
		index = parsed
	}
	data, err := callCore(map[string]any{"action": "localPlayback", "drama": drama, "index": index})
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, err.Error())
		return
	}
	var plan struct {
		URL   string `json:"url"`
		Local bool   `json:"local"`
	}
	if json.Unmarshal(data, &plan) != nil || plan.URL == "" || !plan.Local {
		writeFailure(writer, http.StatusNotFound, "该集没有已下载的本地文件")
		return
	}
	if !insideDirectory(s.dataDir, plan.URL) && !insideDirectory(s.downloadRoot(), plan.URL) {
		writeFailure(writer, http.StatusForbidden, "本地文件不在数据目录内")
		return
	}
	serveFile(writer, request, plan.URL, contentTypeFor(plan.URL, videoTypes), 0)
}

func (s *service) downloads(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	data, err := callCore(map[string]any{"action": "downloads"})
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "data": data})
}
