package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"duanjuapp/native/core"
)

const (
	serviceName     = "zhenguojian-core"
	maxRequestBytes = 1 << 20
)

type envelope struct {
	OK    bool            `json:"ok"`
	Error string          `json:"error"`
	Code  string          `json:"code"`
	Data  json.RawMessage `json:"data"`
}

type service struct {
	token   string
	dataDir string
	slots   chan struct{}
	started time.Time
	edition map[string]any
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func bearer(request *http.Request) string {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	if len(value) > 7 && strings.EqualFold(value[:7], "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	if value != "" {
		return value
	}
	return strings.TrimSpace(request.Header.Get("X-Auth-Token"))
}

func (s *service) authorize(request *http.Request) bool {
	if s.token == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(bearer(request)), []byte(s.token)) == 1
}

func (s *service) enter() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *service) leave() { <-s.slots }

func writeRaw(writer http.ResponseWriter, status int, body string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = io.WriteString(writer, body)
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		writeRaw(writer, http.StatusInternalServerError, `{"ok":false,"error":"服务内部错误"}`)
		return
	}
	writeRaw(writer, status, string(body))
}

func writeFailure(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{"ok": false, "error": message})
}

func (s *service) health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":            true,
		"service":       serviceName,
		"coreVersion":   s.edition["version"],
		"allSources":    s.edition["allSources"],
		"uptimeSeconds": int(time.Since(s.started).Seconds()),
	})
}

func (s *service) index(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":          true,
		"service":     serviceName,
		"coreVersion": s.edition["version"],
		"edition":     map[string]any{"allSources": s.edition["allSources"]},
		"endpoints": []string{
			"GET /healthz",
			"GET /api/actions",
			"GET /api/sources",
			"POST /api/request",
		},
		"request": "POST /api/request 使用与客户端原生核心一致的 JSON：{\"action\":\"catalog\",\"source\":\"hongguo\",\"page\":1}",
		"auth":    "设置 TOKEN 后，除 /healthz 外的请求需要 Authorization: Bearer <token>",
	})
}

func (s *service) actions(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "items": actionCatalog})
}

func (s *service) sources(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	s.dispatch(writer, request, `{"action":"sources"}`)
}

func (s *service) dispatch(writer http.ResponseWriter, request *http.Request, payload string) {
	if len(payload) > maxRequestBytes {
		writeFailure(writer, http.StatusRequestEntityTooLarge, "请求过大")
		return
	}
	if !s.enter() {
		writeFailure(writer, http.StatusTooManyRequests, "服务正忙，请稍后重试")
		return
	}
	defer s.leave()
	writeRaw(writer, http.StatusOK, core.NativeRequest(payload))
}

func (s *service) call(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	if request.Method != http.MethodPost {
		writeFailure(writer, http.StatusMethodNotAllowed, "请使用 POST /api/request")
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBytes+1))
	if err != nil {
		writeFailure(writer, http.StatusBadRequest, "无法读取请求内容")
		return
	}
	if len(body) > maxRequestBytes {
		writeFailure(writer, http.StatusRequestEntityTooLarge, "请求过大")
		return
	}
	if !json.Valid(body) {
		writeFailure(writer, http.StatusBadRequest, "请求不是有效的 JSON")
		return
	}
	s.dispatch(writer, request, string(body))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.ResponseWriter.Write(body)
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /api/actions", s.actions)
	mux.HandleFunc("GET /api/sources", s.sources)
	mux.HandleFunc("POST /api/request", s.call)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: writer}
		mux.ServeHTTP(recorder, request)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		log.Printf("%s %s %d %s", request.Method, request.URL.Path, status, time.Since(started).Round(time.Millisecond))
	})
}

func initialize(directory string) (map[string]any, error) {
	payload, err := json.Marshal(map[string]any{"action": "initialize", "directory": directory})
	if err != nil {
		return nil, err
	}
	var result envelope
	if err := json.Unmarshal([]byte(core.NativeRequest(string(payload))), &result); err != nil {
		return nil, errors.New("原生核心返回无效数据")
	}
	if !result.OK {
		return nil, errors.New(result.Error)
	}
	edition := map[string]any{}
	_ = json.Unmarshal(result.Data, &edition)
	return edition, nil
}

func concurrency() int {
	value, err := strconv.Atoi(env("MAX_CONCURRENCY", "8"))
	if err != nil || value < 1 {
		return 8
	}
	if value > 64 {
		return 64
	}
	return value
}

func main() {
	flagAddress := flag.String("addr", "", "监听地址，默认 :$PORT")
	flagData := flag.String("data", "", "数据目录，默认 $DATA_DIR 或 /data")
	flag.Parse()

	address := strings.TrimSpace(*flagAddress)
	if address == "" {
		address = ":" + env("PORT", "8080")
	}
	directory := strings.TrimSpace(*flagData)
	if directory == "" {
		directory = env("DATA_DIR", "/data")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		log.Fatalf("数据目录无效：%v", err)
	}
	if err := os.MkdirAll(absolute, 0700); err != nil {
		log.Fatalf("无法创建数据目录：%v", err)
	}
	edition, err := initialize(absolute)
	if err != nil {
		log.Fatalf("原生核心初始化失败：%v", err)
	}
	instance := &service{
		token:   env("TOKEN", ""),
		dataDir: absolute,
		slots:   make(chan struct{}, concurrency()),
		started: time.Now(),
		edition: edition,
	}
	server := &http.Server{
		Addr:              address,
		Handler:           instance.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
	}
	go func() {
		log.Printf("%s 已启动：%s，数据目录 %s，全站源 %v，访问令牌 %v，并发上限 %d",
			serviceName, address, absolute, edition["allSources"], instance.token != "", cap(instance.slots))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败：%v", err)
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务停止超时：%v", err)
	}
	log.Printf("%s 已停止", serviceName)
}
