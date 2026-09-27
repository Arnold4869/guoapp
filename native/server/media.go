package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"duanjuapp/native/core"
)

const (
	mediaPrefix      = "/media/"
	maxPlaylistBytes = 4 << 20
)

var (
	mediaAssetPattern = regexp.MustCompile(`^/[0-9a-f]{48}/[0-9a-f]{24}\.(m3u8|ts|m4s|mp4|aac|m4a|mp3|vtt|webvtt|key)$`)
	mediaLinkPattern  = regexp.MustCompile(`/media/[0-9a-f]{48}/[0-9a-f]{24}\.[a-z0-9]+`)
)

var mediaClient = &http.Client{Timeout: 0}

func (s *service) streamAddress() string {
	if s.streamBase != "" {
		return strings.TrimRight(s.streamBase, "/")
	}
	return strings.TrimRight(core.NativeStreamBase(), "/")
}

func (s *service) mediaToken(request *http.Request) string {
	if value := strings.TrimSpace(request.URL.Query().Get("token")); value != "" {
		return value
	}
	return bearer(request)
}

func (s *service) authorizeMedia(request *http.Request) bool {
	if s.token == "" {
		return true
	}
	return constantTimeEqual(s.mediaToken(request), s.token)
}

func (s *service) media(request *http.Request) string {
	target := s.streamAddress()
	if target == "" {
		return ""
	}
	return target + strings.TrimPrefix(request.URL.Path, "/media")
}

func (s *service) mediaQuery(request *http.Request) string {
	values := request.URL.Query()
	values.Del("token")
	return values.Encode()
}

func (s *service) rewritePlaylist(body, base string, request *http.Request) string {
	rewritten := strings.ReplaceAll(body, base+"/", mediaPrefix)
	if s.token == "" {
		return rewritten
	}
	if s.mediaToken(request) == "" {
		return rewritten
	}
	escape := url.QueryEscape(s.token)
	return mediaLinkPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		return match + "?token=" + escape
	})
}

func (s *service) rewritePlan(body string) string {
	base := s.streamAddress()
	if base == "" || !strings.Contains(body, base+"/") {
		return body
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal([]byte(body), &envelope) != nil {
		return body
	}
	raw, found := envelope["data"]
	if !found {
		return body
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(raw, &data) != nil {
		return body
	}
	value, found := data["url"]
	if !found {
		return body
	}
	var address string
	if json.Unmarshal(value, &address) != nil || !strings.HasPrefix(address, base+"/") {
		return body
	}
	link := mediaPrefix + strings.TrimPrefix(address, base+"/")
	if s.token != "" {
		link += "?token=" + url.QueryEscape(s.token)
	}
	encoded, err := json.Marshal(link)
	if err != nil {
		return body
	}
	data["url"] = encoded
	updated, err := json.Marshal(data)
	if err != nil {
		return body
	}
	envelope["data"] = updated
	result, err := json.Marshal(envelope)
	if err != nil {
		return body
	}
	return string(result)
}

func (s *service) mediaHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writeFailure(writer, http.StatusMethodNotAllowed, "濯掍綋璇锋眰鍙敮鎸?GET 涓?HEAD")
		return
	}
	if !s.authorizeMedia(request) {
		writeFailure(writer, http.StatusUnauthorized, "缂哄皯鎴栨棤鏁堢殑璁块棶浠ょ墝")
		return
	}
	asset := strings.TrimPrefix(request.URL.Path, "/media")
	if !mediaAssetPattern.MatchString(asset) {
		writeFailure(writer, http.StatusNotFound, "濯掍綋鍦板潃鏃犳晥锛岃閲嶆柊鎵撳紑璇ラ泦")
		return
	}
	base := s.streamAddress()
	if base == "" {
		writeFailure(writer, http.StatusNotFound, "鎾斁浼氳瘽涓嶅瓨鍦紝璇烽噸鏂版墦寮€璇ラ泦")
		return
	}
	target := base + asset
	if query := s.mediaQuery(request); query != "" {
		target += "?" + query
	}
	upstream, err := http.NewRequestWithContext(request.Context(), request.Method, target, nil)
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, "濯掍綋鍦板潃鏃犳晥")
		return
	}
	for _, name := range []string{"Range", "If-Range", "If-None-Match"} {
		if value := request.Header.Get(name); value != "" {
			upstream.Header.Set(name, value)
		}
	}
	response, err := mediaClient.Do(upstream)
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, "璇诲彇濯掍綋澶辫触锛岃绋嶅悗閲嶈瘯")
		return
	}
	defer response.Body.Close()
	isPlaylist := strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "mpegurl") ||
		strings.HasSuffix(strings.ToLower(asset), ".m3u8")
	if !isPlaylist {
		for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "ETag", "Last-Modified", "Cache-Control"} {
			if value := response.Header.Get(name); value != "" {
				writer.Header().Set(name, value)
			}
		}
		writer.WriteHeader(response.StatusCode)
		if request.Method == http.MethodGet {
			_, _ = io.Copy(writer, response.Body)
		}
		return
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxPlaylistBytes+1))
	if err != nil || len(body) > maxPlaylistBytes {
		writeFailure(writer, http.StatusBadGateway, "鎾斁鍒楄〃璇诲彇澶辫触锛岃绋嶅悗閲嶈瘯")
		return
	}
	writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(response.StatusCode)
	if request.Method == http.MethodGet {
		_, _ = io.WriteString(writer, s.rewritePlaylist(string(body), base, request))
	}
}
