package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"duanjuapp/native/core"
)

type resourceSettings struct {
	ProxyMode           string `json:"proxyMode"`
	ProxyURL            string `json:"proxyUrl"`
	CatalogConcurrency  int    `json:"catalogConcurrency"`
	CatalogIntervalMS   int    `json:"catalogIntervalMs"`
	DownloadConcurrency int    `json:"downloadConcurrency"`
	DownloadBySource    bool   `json:"downloadBySource"`
	Warning             string `json:"warning,omitempty"`
	SystemProxyStatus   string `json:"systemProxyStatus,omitempty"`
}

func callCore(payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.New("请求内容无效")
	}
	var result envelope
	if err := json.Unmarshal([]byte(core.NativeRequest(string(body))), &result); err != nil {
		return nil, errors.New("原生核心返回无效数据")
	}
	if !result.OK {
		return nil, errors.New(result.Error)
	}
	return result.Data, nil
}

func (s *service) loadSettings() (resourceSettings, error) {
	data, err := callCore(map[string]any{"action": "resourceSettings"})
	if err != nil {
		return resourceSettings{}, err
	}
	var settings resourceSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return resourceSettings{}, errors.New("原生核心返回的设置无效")
	}
	return settings, nil
}

func (s *service) saveSettings(settings resourceSettings) (resourceSettings, error) {
	data, err := callCore(map[string]any{
		"action": "saveResourceSettings",
		"settings": map[string]any{
			"proxyMode":           settings.ProxyMode,
			"proxyUrl":            settings.ProxyURL,
			"catalogConcurrency":  settings.CatalogConcurrency,
			"catalogIntervalMs":   settings.CatalogIntervalMS,
			"downloadConcurrency": settings.DownloadConcurrency,
			"downloadBySource":    settings.DownloadBySource,
		},
	})
	if err != nil {
		return settings, err
	}
	var saved resourceSettings
	if err := json.Unmarshal(data, &saved); err != nil {
		return settings, errors.New("原生核心返回的设置无效")
	}
	return saved, nil
}

func (s *service) applyProxy(mode, address string) (resourceSettings, error) {
	settings, err := s.loadSettings()
	if err != nil {
		return settings, err
	}
	if mode == "direct" {
		address = ""
	}
	settings.ProxyMode, settings.ProxyURL = mode, address
	return s.saveSettings(settings)
}

func maskProxyAddress(address string) string {
	if address == "" {
		return ""
	}
	parsed, err := url.Parse(address)
	if err != nil || parsed.User == nil {
		return address
	}
	parsed.User = url.User("***")
	return parsed.String()
}

func (s *service) proxyPayload() map[string]any {
	if s.settings == nil {
		return nil
	}
	return map[string]any{
		"mode": s.settings.ProxyMode,
		"url":  maskProxyAddress(s.settings.ProxyURL),
	}
}

func (s *service) settingsView(writer http.ResponseWriter, request *http.Request) {
	if !s.authorize(request) {
		writeFailure(writer, http.StatusUnauthorized, "缺少或无效的访问令牌")
		return
	}
	settings, err := s.loadSettings()
	if err != nil {
		writeFailure(writer, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok": true,
		"settings": map[string]any{
			"proxyMode":           settings.ProxyMode,
			"proxyUrl":            maskProxyAddress(settings.ProxyURL),
			"catalogConcurrency":  settings.CatalogConcurrency,
			"catalogIntervalMs":   settings.CatalogIntervalMS,
			"downloadConcurrency": settings.DownloadConcurrency,
			"downloadBySource":    settings.DownloadBySource,
			"systemProxyStatus":   settings.SystemProxyStatus,
		},
		"env": []string{"PROXY_MODE", "PROXY_URL"},
	})
}

func proxyEnvironment() (string, string) {
	address := strings.TrimSpace(env("PROXY_URL", ""))
	mode := strings.ToLower(strings.TrimSpace(env("PROXY_MODE", "")))
	if mode == "" {
		if address == "" {
			return "", ""
		}
		mode = "manual"
	}
	return mode, address
}
