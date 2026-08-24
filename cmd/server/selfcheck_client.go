package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type probeClient struct {
	base   string
	client *http.Client
}

type permitProbe struct {
	Data struct {
		Permit struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			Revision int64  `json:"revision"`
		} `json:"permit"`
		Replay bool `json:"replayed"`
	} `json:"data"`
}

type timelineProbe struct {
	Data struct {
		Status      string `json:"status"`
		Revision    int64  `json:"revision"`
		Transitions []any  `json:"transitions"`
		Reviews     []any  `json:"reviews"`
	} `json:"data"`
}

func (p *probeClient) write(ctx context.Context, path string, body any, status int) (permitProbe, error) {
	var out permitProbe
	b, err := json.Marshal(body)
	if err != nil {
		return out, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+path, bytes.NewReader(b))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return out, fmt.Errorf("POST %s 返回 %d，期望 %d: %s", path, resp.StatusCode, status, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, fmt.Errorf("解析 %s 响应: %w", path, err)
	}
	return out, nil
}

func (p *probeClient) getTimeline(ctx context.Context, path string) (timelineProbe, error) {
	var out timelineProbe
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.base+path, nil)
	if err != nil {
		return out, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return out, fmt.Errorf("GET %s 返回 %d: %s", path, resp.StatusCode, strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func newProbeClient(addr string) *probeClient {
	return &probeClient{base: "http://" + addr, client: &http.Client{Timeout: 4 * time.Second}}
}
