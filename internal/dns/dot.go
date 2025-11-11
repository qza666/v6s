package dns

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

func ResolveDNSOverTLS(domain string) (string, error) {
	c := new(dns.Client)
	c.Net = "tcp-tls"
	c.Timeout = 5 * time.Second
	c.TLSConfig = &tls.Config{
		ServerName: "dns.cloudflare.com",
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), dns.TypeAAAA)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, "dns.cloudflare.com:853")
	if err != nil {
		return "", fmt.Errorf("DNS over TLS 查询失败：%w", err)
	}

	if r.Rcode != dns.RcodeSuccess {
		return "", fmt.Errorf("DNS 查询失败：%v", dns.RcodeToString[r.Rcode])
	}

	for _, answer := range r.Answer {
		if aaaa, ok := answer.(*dns.AAAA); ok {
			return aaaa.AAAA.String(), nil
		}
	}

	return "", fmt.Errorf("未找到 %s 的 AAAA 记录", domain)
}
