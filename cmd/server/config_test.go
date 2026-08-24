package main

import "testing"

func TestParseConfigUsesLoopbackPORTAndAddrOverride(t *testing.T) {
	cfg, err := parseConfig(nil, func(key string) string {
		if key == "PORT" {
			return "19444"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:19444" {
		t.Fatalf("PORT 地址 = %s", cfg.Addr)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19999"}, func(string) string { return "19444" })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:19999" {
		t.Fatalf("-addr 未覆盖 PORT: %s", cfg.Addr)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19998"}, func(string) string { return "invalid" })
	if err != nil || cfg.Addr != "127.0.0.1:19998" {
		t.Fatalf("显式 -addr 应忽略无效 PORT: %#v %v", cfg, err)
	}
}

func TestParseConfigRejectsInvalidPORTAndEmptyAddr(t *testing.T) {
	if _, err := parseConfig(nil, func(string) string { return "8080x" }); err == nil {
		t.Fatal("无效 PORT 应返回错误")
	}
	if _, err := parseConfig([]string{"-addr="}, func(string) string { return "" }); err == nil {
		t.Fatal("空监听地址应返回错误")
	}
}
