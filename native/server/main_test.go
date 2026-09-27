package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

var testDataDir string

func TestMain(m *testing.M) {
	directory, err := os.MkdirTemp("", "zhenguojian-server-")
	if err != nil {
		panic(err)
	}
	testDataDir = directory
	code := m.Run()
	_ = os.RemoveAll(directory)
	os.Exit(code)
}

func newTestService(t *testing.T, token string, slots int) *service {
	t.Helper()
	edition, err := initialize(testDataDir)
	if err != nil {
		t.Fatalf("初始化原生核心失败：%v", err)
	}
	if slots < 1 {
		slots = 4
	}
	return &service{
		token:   token,
		dataDir: testDataDir,
		slots:   make(chan struct{}, slots),
		started: time.Now(),
		edition: edition,
	}
}

func reply(t *testing.T, instance *service, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	server := httptest.NewServer(instance.routes())
	defer server.Close()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(payload)
}

func envelopeOf(t *testing.T, body string) envelope {
	t.Helper()
	var result envelope
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("返回不是有效 JSON：%v %s", err, body)
	}
	return result
}

func TestHealthReportsEdition(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodGet, "/healthz", "", nil)
	if status != http.StatusOK {
		t.Fatalf("健康检查状态码异常：%d %s", status, body)
	}
	result := envelopeOf(t, body)
	if !result.OK || !strings.Contains(body, serviceName) {
		t.Fatalf("健康检查内容异常：%s", body)
	}
}

func TestSourcesMatchCompiledEdition(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodGet, "/api/sources", "", nil)
	if status != http.StatusOK {
		t.Fatalf("站源接口状态码异常：%d %s", status, body)
	}
	result := envelopeOf(t, body)
	if !result.OK {
		t.Fatalf("站源接口返回失败：%s", body)
	}
	var payload struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		AllSources bool `json:"allSources"`
	}
	if err := json.Unmarshal(result.Data, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Items) == 0 || payload.Items[0].ID != "hongguo" || payload.Items[0].Name == "" {
		t.Fatalf("站源列表缺少红果：%s", body)
	}
	if payload.AllSources != (len(payload.Items) > 1) {
		t.Fatalf("站源数量与编译版本不一致：%s", body)
	}
}

func TestActionsListIsAvailable(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodGet, "/api/actions", "", nil)
	if status != http.StatusOK || !strings.Contains(body, "catalog") {
		t.Fatalf("操作列表异常：%d %s", status, body)
	}
}

func TestRequestEnvelopeKeepsCoreProtocol(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodPost, "/api/request", `{"action":"sources"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("请求接口状态码异常：%d %s", status, body)
	}
	if !envelopeOf(t, body).OK {
		t.Fatalf("sources 请求失败：%s", body)
	}
	status, body = reply(t, instance, http.MethodPost, "/api/request", `{"action":"不存在的操作"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("未知操作应返回 200 与 ok=false：%d %s", status, body)
	}
	result := envelopeOf(t, body)
	if result.OK || result.Error == "" {
		t.Fatalf("未知操作未被拒绝：%s", body)
	}
}

func TestMalformedRequestIsRejected(t *testing.T) {
	instance := newTestService(t, "", 4)
	status, body := reply(t, instance, http.MethodPost, "/api/request", "not-json", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("非法 JSON 未被拒绝：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodPost, "/api/request", strings.Repeat("a", maxRequestBytes+16), nil)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("超大请求未被拒绝：%d %s", status, body)
	}
}

func TestTokenGuardsApiButNotHealth(t *testing.T) {
	instance := newTestService(t, "secret-token", 4)
	status, body := reply(t, instance, http.MethodGet, "/api/sources", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("缺少令牌时应拒绝：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/api/sources", "", map[string]string{"Authorization": "Bearer wrong"})
	if status != http.StatusUnauthorized {
		t.Fatalf("错误令牌时应拒绝：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/api/sources", "", map[string]string{"Authorization": "Bearer secret-token"})
	if status != http.StatusOK || !envelopeOf(t, body).OK {
		t.Fatalf("正确令牌应放行：%d %s", status, body)
	}
	status, body = reply(t, instance, http.MethodGet, "/healthz", "", nil)
	if status != http.StatusOK {
		t.Fatalf("健康检查不应要求令牌：%d %s", status, body)
	}
}

func TestBusyServiceRejectsExtraWork(t *testing.T) {
	instance := newTestService(t, "", 1)
	if !instance.enter() {
		t.Fatal("首个请求应占用并发额度")
	}
	defer instance.leave()
	status, body := reply(t, instance, http.MethodPost, "/api/request", `{"action":"sources"}`, nil)
	if status != http.StatusTooManyRequests {
		t.Fatalf("并发已满时应返回 429：%d %s", status, body)
	}
}
