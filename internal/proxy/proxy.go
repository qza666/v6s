package proxy

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/qza1314526-debug/v6-ee/internal/config"
)

func NewProxyServer(cfg *config.Config, useRandomIPv6 bool) *goproxy.ProxyHttpServer {
	return NewProxyServerWithSpecificIP(cfg, useRandomIPv6, cfg.RealIPv4)
}

func NewProxyServerWithSpecificIP(cfg *config.Config, useRandomIPv6 bool, specificIPv4 string) *goproxy.ProxyHttpServer {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = cfg.Verbose

	var fixedIPv4 net.IP
	if !useRandomIPv6 {
		fixedIPv4 = net.ParseIP(specificIPv4)
		if fixedIPv4 == nil {
			log.Fatalf("提供的 IPv4 地址无效：%s", specificIPv4)
		}
	}

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if !checkAuth(cfg.AuthConfig.Username, cfg.AuthConfig.Password, req) {
			return req, unauthorizedResponse(req)
		}
		return req, nil
	})

	proxy.OnRequest().HijackConnect(func(req *http.Request, client net.Conn, ctx *goproxy.ProxyCtx) {
		if !checkAuth(cfg.AuthConfig.Username, cfg.AuthConfig.Password, req) {
			writeProxyError(client, req.Proto, http.StatusProxyAuthRequired, "需要代理身份验证")
			client.Close()
			return
		}

		host := req.URL.Hostname()
		outgoingIP, targetIP, err := prepareOutgoingAddress(cfg, host, useRandomIPv6, fixedIPv4)
		if err != nil {
			log.Printf("建立到 %s 的 CONNECT 隧道失败：%v", req.URL.Host, err)
			writeProxyError(client, req.Proto, http.StatusInternalServerError, "代理服务器内部错误")
			client.Close()
			return
		}

		if useRandomIPv6 {
			log.Printf("CONNECT: %s [%s] 使用地址 %s（CIDR: %s）", req.URL.Host, targetIP, outgoingIP, cfg.CIDR)
		} else {
			log.Printf("CONNECT: %s 使用 IPv4 地址 %s", req.URL.Host, outgoingIP)
		}

		dialer := &net.Dialer{
			LocalAddr: &net.TCPAddr{IP: outgoingIP, Port: 0},
			Timeout:   30 * time.Second,
		}

		server, err := dialer.Dial("tcp", req.URL.Host)
		if err != nil {
			log.Printf("连接 %s 失败（出口 %s）：%v", req.URL.Host, outgoingIP, err)
			writeProxyError(client, req.Proto, http.StatusBadGateway, "无法连接目标服务器")
			client.Close()
			return
		}

		_, _ = client.Write([]byte(fmt.Sprintf("%s 200 Connection established\r\n\r\n", req.Proto)))

		go copyData(client, server)
		go copyData(server, client)
	})

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		host := req.URL.Hostname()
		outgoingIP, targetIP, err := prepareOutgoingAddress(cfg, host, useRandomIPv6, fixedIPv4)
		if err != nil {
			log.Printf("处理 %s 请求时准备出口地址失败：%v", req.URL.Host, err)
			return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusBadGateway, "目标主机解析失败")
		}

		if useRandomIPv6 {
			log.Printf("HTTP: %s [%s] 使用地址 %s（CIDR: %s）", req.URL.Host, targetIP, outgoingIP, cfg.CIDR)
		} else {
			log.Printf("HTTP: %s 使用 IPv4 地址 %s", req.URL.Host, outgoingIP)
		}

		dialer := &net.Dialer{
			LocalAddr: &net.TCPAddr{IP: outgoingIP, Port: 0},
			Timeout:   30 * time.Second,
		}

		transport := &http.Transport{
			Dial:        dialer.Dial,
			DialContext: dialer.DialContext,
		}

		ctx.RoundTripper = goproxy.RoundTripperFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Response, error) {
			return transport.RoundTrip(r)
		})

		return req, nil
	})

	return proxy
}

func prepareOutgoingAddress(cfg *config.Config, host string, useRandomIPv6 bool, fixedIPv4 net.IP) (net.IP, string, error) {
	if useRandomIPv6 {
		targetIP, err := getIPv6Address(host)
		if err != nil {
			return nil, "", fmt.Errorf("解析 IPv6 失败：%w", err)
		}

		outgoingIP, err := generateRandomIPv6(cfg.CIDR)
		if err != nil {
			return nil, "", fmt.Errorf("生成随机 IPv6 失败：%w", err)
		}

		return outgoingIP, targetIP, nil
	}

	if fixedIPv4 == nil {
		return nil, "", fmt.Errorf("未提供有效的 IPv4 地址")
	}

	return fixedIPv4, "", nil
}

func unauthorizedResponse(req *http.Request) *http.Response {
	return goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusProxyAuthRequired, "需要代理身份验证")
}

func writeProxyError(conn net.Conn, proto string, status int, message string) {
	if conn == nil {
		return
	}

	body := []byte(message)
	response := fmt.Sprintf("%s %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\n\r\n%s",
		proto, status, http.StatusText(status), len(body), message)
	_, _ = conn.Write([]byte(response))
}

func generateRandomIPv6(cidr string) (net.IP, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("CIDR 解析失败：%w", err)
	}

	ip := make(net.IP, net.IPv6len)
	copy(ip, ipNet.IP)

	prefixLen, totalBits := ipNet.Mask.Size()
	if totalBits != net.IPv6len*8 {
		return nil, fmt.Errorf("CIDR 掩码长度无效：%d", totalBits)
	}

	hostBits := totalBits - prefixLen
	if hostBits <= 0 {
		return nil, fmt.Errorf("CIDR 前缀过长，主机位已耗尽")
	}

	startByte := prefixLen / 8
	startBit := prefixLen % 8

	if startBit != 0 {
		mask := byte(0xFF >> startBit)
		var randomByte [1]byte
		if _, err := rand.Read(randomByte[:]); err != nil {
			return nil, fmt.Errorf("生成随机 IPv6 失败：%w", err)
		}
		ip[startByte] = (ip[startByte] & ^mask) | (randomByte[0] & mask)
		startByte++
	}

	if err := fillRandomBytes(ip[startByte:]); err != nil {
		return nil, err
	}

	if !ipNet.Contains(ip) {
		return nil, fmt.Errorf("生成的 IPv6 地址不在指定网段中")
	}

	return ip, nil
}

func fillRandomBytes(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("生成随机 IPv6 失败：%w", err)
	}
	return nil
}

func getIPv6Address(domain string) (string, error) {
	if ip := net.ParseIP(domain); ip != nil {
		if ip.To4() == nil {
			return ip.String(), nil
		}
		return "", fmt.Errorf("提供的地址 %s 不是 IPv6 地址", domain)
	}

	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", fmt.Errorf("解析域名 %s 失败：%w", domain, err)
	}

	for _, ip := range ips {
		if ip.To4() == nil {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("未找到 %s 的 IPv6 地址", domain)
}

func checkAuth(username string, password string, req *http.Request) bool {
	if username == "" || password == "" {
		return true
	}

	auth := req.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return false
	}

	credentials := strings.SplitN(string(decoded), ":", 2)
	if len(credentials) != 2 {
		return false
	}

	return credentials[0] == username && credentials[1] == password
}

func copyData(dst, src net.Conn) {
	defer dst.Close()
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil && !errors.Is(err, net.ErrClosed) {
		log.Printf("转发数据时发生错误：%v", err)
	}
}
