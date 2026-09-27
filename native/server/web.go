package main

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"
)

//go:embed web
var webAssets embed.FS

var (
	pageOnce    sync.Once
	pageBody    []byte
	pageFailure error
	scriptOnce  sync.Once
	scriptBody  []byte
	scriptErr   error
)

func readAsset(name string) ([]byte, error) {
	once, body, failure := &pageOnce, &pageBody, &pageFailure
	if name == "web/hls.min.js" {
		once, body, failure = &scriptOnce, &scriptBody, &scriptErr
	}
	once.Do(func() {
		*body, *failure = fs.ReadFile(webAssets, name)
	})
	return *body, *failure
}

func (s *service) serveAsset(writer http.ResponseWriter, request *http.Request, name, contentType, cache string) {
	body, err := readAsset(name)
	if err != nil {
		writeFailure(writer, http.StatusInternalServerError, "页面资源缺失")
		return
	}
	writer.Header().Set("Content-Type", contentType)
	if cache == "" {
		writer.Header().Set("Cache-Control", "no-store")
	} else {
		writer.Header().Set("Cache-Control", cache)
	}
	writer.WriteHeader(http.StatusOK)
	if request.Method == http.MethodGet {
		_, _ = writer.Write(body)
	}
}

func (s *service) page(writer http.ResponseWriter, request *http.Request) {
	s.serveAsset(writer, request, "web/index.html", "text/html; charset=utf-8", "")
}

func (s *service) script(writer http.ResponseWriter, request *http.Request) {
	s.serveAsset(writer, request, "web/hls.min.js", "text/javascript; charset=utf-8", "public, max-age=86400")
}
