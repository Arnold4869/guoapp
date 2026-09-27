package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyEnvironment(t *testing.T) {
	t.Setenv("PROXY_URL", "")
	t.Setenv("PROXY_MODE", "")
	if mode, address := proxyEnvironment(); mode != "" || address != "" {
		t.Fatalf("未配置时不应改动设置：%q %q", mode, address)
	}
	t.Setenv("PROXY_URL", "socks5://127.0.0.1:1080")
	if mode, address := proxyEnvironment(); mode != "manual" || address != "socks5://127.0.0.1:1080" {
		t.Fatalf("只给地址时应为 manual：%q %q", mode, address)
	}
	t.Setenv("PROXY_MODE", "direct")
	if mode, address := proxyEnvironment(); mode != "direct" || address != "socks5://127.0.0.1:1080" {
		t.Fatalf("显式模式应优先：%q %q", mode, address)
	}
}

func TestApplyProxyAndSettingsEndpoint(t *testing.T) {
	instance := newTestService(t, "", 4)
	saved, err := instance.applyProxy("manual", "http://127.0.0.1:7890")
	if err != nil {
		t.Fatalf("设置代理失败：%v", err)
	}
	if saved.ProxyMode != "manual" || saved.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("代理未生效：%+v", saved)
	}
	current, err := instance.loadSettings()
	if err != nil || current.ProxyMode != "manual" || current.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("代理设置未持久化：%+v %v", current, err)
	}
	status, body := reply(t, instance, http.MethodGet, "/api/settings", "", nil)
	if status != http.StatusOK || !strings.Contains(body, `"proxyMode":"manual"`) {
		t.Fatalf("设置接口异常：%d %s", status, body)
	}
	if mode := maskProxyAddress("socks5://user:pass@127.0.0.1:1080"); strings.Contains(mode, "pass") {
		t.Fatalf("代理地址未脱敏：%s", mode)
	}
	t.Cleanup(func() { _, _ = instance.applyProxy("auto", "") })
}

func TestInsideDirectory(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "downloads", "job", "media.mp4")
	if !insideDirectory(root, inside) {
		t.Fatal("数据目录内的文件应被接受")
	}
	if insideDirectory(root, filepath.Join(root, "..", "outside.mp4")) {
		t.Fatal("目录外的文件必须被拒绝")
	}
	if insideDirectory("", inside) {
		t.Fatal("缺少根目录时必须拒绝")
	}
}

func TestServeFileSupportsRange(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "media.mp4")
	if err := os.WriteFile(path, []byte("0123456789"), 0600); err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveFile(writer, request, path, "video/mp4", 0)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	request, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	request.Header.Set("Range", "bytes=2-4")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body := make([]byte, 8)
	count, _ := response.Body.Read(body)
	if response.StatusCode != http.StatusPartialContent || string(body[:count]) != "234" {
		t.Fatalf("Range 未生效：%d %q", response.StatusCode, body[:count])
	}
}

func TestCoverAndLocalRejectInvalidRequests(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodGet, "/api/cover", "", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("缺少剧集参数应返回 400：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/api/cover?drama=%7B%22id%22%3A%22unknown%3A1%22%2C%22source%22%3A%22unknown%22%7D", "", nil)
	if status != http.StatusBadGateway || !strings.Contains(body, "站源") {
		t.Fatalf("不可用站源应返回明确错误：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/local", "", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("缺少剧集参数应返回 400：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/local?drama=%7B%22id%22%3A%22hongguo%3A1%22%7D&index=1", "", nil)
	if status != http.StatusNotFound && status != http.StatusBadGateway {
		t.Fatalf("没有本地文件时不应返回成功：%d %s", status, body)
	}
}

func TestDownloadsEndpointListsJobs(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodGet, "/api/downloads", "", nil)
	if status != http.StatusOK {
		t.Fatalf("下载列表状态码异常：%d %s", status, body)
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Jobs []any `json:"jobs"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil || !result.OK {
		t.Fatalf("下载列表内容异常：%d %s %v", status, body, err)
	}
	if result.Data.Jobs == nil {
		t.Fatalf("下载列表应返回 jobs 数组：%s", body)
	}
	guarded := newTestService(t, "secret-token", 4)
	status, _ = reply(t, guarded, http.MethodGet, "/api/downloads", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("下载列表同样需要令牌：%d", status)
	}
}
