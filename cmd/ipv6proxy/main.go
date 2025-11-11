package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/qza1314526-debug/v6-ee/internal/config"
	"github.com/qza1314526-debug/v6-ee/internal/proxy"
	"github.com/qza1314526-debug/v6-ee/internal/sysutils"
)

func main() {
	log.SetOutput(os.Stdout)
	cfg := config.ParseFlags()
	if cfg.CIDR == "" {
		log.Fatal("必须指定 CIDR")
	}

	if cfg.AutoForwarding {
		sysutils.SetV6Forwarding()
	}

	if cfg.AutoRoute {
		sysutils.AddV6Route(cfg.CIDR)
	}

	if cfg.AutoIpNoLocalBind {
		sysutils.SetIpNonLocalBind()
	}

	var wg sync.WaitGroup

	randomIPv6Proxy := proxy.NewProxyServer(cfg, true)
	startHTTPServer(&wg, fmt.Sprintf("%s:%d", cfg.Bind, cfg.RandomIPv6Port), randomIPv6Proxy, "随机 IPv6 代理服务", true)

	if len(cfg.MultiIPv4Config) > 0 {
		log.Printf("启用多 IPv4 出口模式，共 %d 个地址", len(cfg.MultiIPv4Config))
		for _, ipConfig := range cfg.MultiIPv4Config {
			ipv4Cfg := *cfg
			ipv4Cfg.RealIPv4 = ipConfig.IPv4
			ipv4Cfg.RealIPv4Port = ipConfig.Port

			handler := proxy.NewProxyServerWithSpecificIP(&ipv4Cfg, false, ipConfig.IPv4)
			addr := fmt.Sprintf("%s:%d", ipConfig.IPv4, ipConfig.Port)
			desc := fmt.Sprintf("IPv4 代理服务（出口 %s）", ipConfig.IPv4)
			startHTTPServer(&wg, addr, handler, desc, false)
		}
	} else if cfg.RealIPv4 != "" {
		realIPv4Proxy := proxy.NewProxyServer(cfg, false)
		startHTTPServer(&wg, fmt.Sprintf("%s:%d", cfg.Bind, cfg.RealIPv4Port), realIPv4Proxy, "IPv4 出口代理服务", true)
	} else {
		log.Fatal("必须通过 --real-ipv4 或 --multi-ipv4 指定至少一个 IPv4 出口")
	}

	wg.Wait()
}

func startHTTPServer(wg *sync.WaitGroup, addr string, handler http.Handler, description string, fatalOnError bool) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("正在启动 %s，监听地址 %s", description, addr)
		if err := http.ListenAndServe(addr, handler); err != nil {
			if fatalOnError {
				log.Fatalf("%s 启动失败：%v", description, err)
			}
			log.Printf("%s 已停止：%v", description, err)
		}
	}()
}
