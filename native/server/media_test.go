package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testStreamToken = "0123456789abcdef0123456789abcdef0123456789abcdef"
	testAsset       = "abcdef012345abcdef012345"
)

func mediaService(t *testing.T, upstream, token string) *service {
	t.Helper()
	instance := newTestService(t, token, 4)
	instance.streamBase = upstream
	return instance
}

func TestMediaProxyStreamsSegments(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/"+testStreamToken+"/"+testAsset+".ts" {
			t.Errorf("意外的上游路径：%s", request.URL.Path)
		}
		if request.Header.Get("Range") != "bytes=0-3" {
			t.Errorf("Range 未透传：%q", request.Header.Get("Range"))
		}
		writer.Header().Set("Content-Type", "video/mp2t")
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(writer, "data")
	}))
	defer upstream.Close()
	instance := mediaService(t, upstream.URL, "")
	status, body := reply(t, instance, http.MethodGet, "/media/"+testStreamToken+"/"+testAsset+".ts", "", map[string]string{"Range": "bytes=0-3"})
	if status != http.StatusPartialContent || body != "data" {
		t.Fatalf("媒体透传失败：%d %q", status, body)
	}
}

func TestMediaProxyRewritesPlaylists(t *testing.T) {
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(writer, "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2,\n"+upstream.URL+"/"+testStreamToken+"/"+testAsset+".ts\n")
	}))
	defer upstream.Close()
	instance := mediaService(t, upstream.URL, "")
	status, body := reply(t, instance, http.MethodGet, "/media/"+testStreamToken+"/"+testAsset+".m3u8", "", nil)
	if status != http.StatusOK {
		t.Fatalf("播放列表状态码异常：%d %s", status, body)
	}
	if strings.Contains(body, upstream.URL) || !strings.Contains(body, "/media/"+testStreamToken+"/"+testAsset+".ts") {
		t.Fatalf("播放列表未重写到 /media：%s", body)
	}
	guarded := mediaService(t, upstream.URL, "secret-token")
	status, body = reply(t, guarded, http.MethodGet, "/media/"+testStreamToken+"/"+testAsset+".m3u8?token=secret-token", "", nil)
	if status != http.StatusOK || !strings.Contains(body, "/media/"+testStreamToken+"/"+testAsset+".ts?token=secret-token") {
		t.Fatalf("启用令牌后子链接未带令牌：%d %s", status, body)
	}
}

func TestMediaRequiresTokenAndValidPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "video/mp2t")
		_, _ = io.WriteString(writer, "data")
	}))
	defer upstream.Close()
	instance := mediaService(t, upstream.URL, "secret-token")
	path := "/media/" + testStreamToken + "/" + testAsset + ".ts"
	status, body := reply(t, instance, http.MethodGet, path, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("缺少令牌时应拒绝：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, path+"?token=secret-token", "", nil)
	if status != http.StatusOK || body != "data" {
		t.Fatalf("带令牌时应放行：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/media/not-a-token/nothing.ts?token=secret-token", "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("非法媒体路径应拒绝：%d %s", status, body)
	}
	open := mediaService(t, "", "")
	status, body = reply(t, open, http.MethodGet, path, "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("没有播放会话时应提示重新打开：%d %s", status, body)
	}
}

func TestPlanURLRewritesToMediaPath(t *testing.T) {
	instance := newTestService(t, "", 4)
	instance.streamBase = "http://127.0.0.1:41234"
	plan := `{"ok":true,"data":{"url":"http://127.0.0.1:41234/` + testStreamToken + `/` + testAsset + `.m3u8","session":"abc","local":false,"routeIndex":0,"routeCount":1,"expiresAt":1799999999999}}`
	rewritten := instance.rewritePlan(plan)
	var payload struct {
		OK   bool `json:"ok"`
		Data struct {
			URL       string `json:"url"`
			Session   string `json:"session"`
			ExpiresAt int64  `json:"expiresAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(rewritten), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.URL != "/media/"+testStreamToken+"/"+testAsset+".m3u8" || payload.Data.Session != "abc" || payload.Data.ExpiresAt != 1799999999999 {
		t.Fatalf("播放地址未正确改写：%s", rewritten)
	}
	guarded := newTestService(t, "secret-token", 4)
	guarded.streamBase = "http://127.0.0.1:41234"
	rewritten = guarded.rewritePlan(plan)
	if !strings.Contains(rewritten, "?token=secret-token") {
		t.Fatalf("启用令牌后播放地址未带令牌：%s", rewritten)
	}
	untouched := instance.rewritePlan(`{"ok":true,"data":{"url":"https://cdn.example.test/a.m3u8"}}`)
	if untouched != `{"ok":true,"data":{"url":"https://cdn.example.test/a.m3u8"}}` {
		t.Fatalf("非本地播放地址不应改写：%s", untouched)
	}
}
