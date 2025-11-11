package config

import (
	"flag"
	"strconv"
	"strings"
)

type Config struct {
	RandomIPv6Port    int
	RealIPv4Port      int
	CIDR              string
	Bind              string
	AutoRoute         bool
	AutoForwarding    bool
	AutoIpNoLocalBind bool
	UseDOH            bool
	Verbose           bool
	AuthConfig        AuthConfig
	RealIPv4          string
	MultiIPv4Config   []MultiIPConfig
}

type MultiIPConfig struct {
	IPv4 string
	Port int
}

type AuthConfig struct {
	Username string
	Password string
}

func ParseFlags() *Config {
	cfg := &Config{}
	var multiIPv4Str string

	flag.IntVar(&cfg.RandomIPv6Port, "random-ipv6-port", 100, "随机 IPv6 代理监听端口")
	flag.IntVar(&cfg.RealIPv4Port, "real-ipv4-port", 101, "真实 IPv4 代理监听端口")
	flag.StringVar(&cfg.CIDR, "cidr", "", "必须指定 IPv6 CIDR")
	flag.StringVar(&cfg.AuthConfig.Username, "username", "", "基础认证用户名")
	flag.StringVar(&cfg.AuthConfig.Password, "password", "", "基础认证密码")
	flag.StringVar(&cfg.Bind, "bind", "0.0.0.0", "服务器监听地址")
	flag.BoolVar(&cfg.AutoRoute, "auto-route", true, "自动为本机添加路由")
	flag.BoolVar(&cfg.AutoForwarding, "auto-forwarding", true, "自动开启 IPv6 转发")
	flag.BoolVar(&cfg.AutoIpNoLocalBind, "auto-ip-nonlocal-bind", true, "自动开启 IPv6 非本地绑定")
	flag.BoolVar(&cfg.UseDOH, "use-doh", true, "使用 DoH（否则使用 DoT）")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "输出详细日志")
	flag.StringVar(&cfg.RealIPv4, "real-ipv4", "", "服务器出口 IPv4 地址")
	flag.StringVar(&multiIPv4Str, "multi-ipv4", "", "指定多个 IPv4 出口（格式：ip1:port1,ip2:port2）")

	flag.Parse()

	// 解析多 IP 配置
	if multiIPv4Str != "" {
		pairs := strings.Split(multiIPv4Str, ",")
		for _, pair := range pairs {
			parts := strings.Split(strings.TrimSpace(pair), ":")
			if len(parts) != 2 || parts[0] == "" {
				continue
			}

			port := cfg.RealIPv4Port
			if trimmedPort := strings.TrimSpace(parts[1]); trimmedPort != "" {
				if parsedPort, err := strconv.Atoi(trimmedPort); err == nil && parsedPort > 0 && parsedPort <= 65535 {
					port = parsedPort
				} else {
					continue
				}
			}

			cfg.MultiIPv4Config = append(cfg.MultiIPv4Config, MultiIPConfig{
				IPv4: strings.TrimSpace(parts[0]),
				Port: port,
			})
		}
	}

	return cfg
}
