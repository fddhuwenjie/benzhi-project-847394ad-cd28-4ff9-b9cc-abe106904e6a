package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Addr      string
	DataPath  string
	SelfCheck bool
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	addr := defaultAddress
	fs := flag.NewFlagSet("confined-permit-server", flag.ContinueOnError)
	fs.StringVar(&addr, "addr", addr, "HTTP 监听地址")
	data := fs.String("data", "data/permits.json", "JSON 数据快照路径")
	selfCheck := fs.Bool("self-check", false, "运行完整 HTTP 闭环自检后退出")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("存在无法识别的位置参数")
	}
	addrSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			addrSet = true
		}
	})
	if !addrSet {
		if raw := strings.TrimSpace(getenv("PORT")); raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil || port < 1 || port > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1 至 65535 的端口号")
			}
			addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	if strings.TrimSpace(addr) == "" {
		return config{}, fmt.Errorf("监听地址不能为空")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return config{}, fmt.Errorf("监听地址无效: %w", err)
	}
	if strings.TrimSpace(*data) == "" {
		return config{}, fmt.Errorf("数据快照路径不能为空")
	}
	return config{Addr: addr, DataPath: *data, SelfCheck: *selfCheck}, nil
}
