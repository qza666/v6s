package dns

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type DoHResponse struct {
	Status   int  `json:"Status"`
	TC       bool `json:"TC"`
	RD       bool `json:"RD"`
	RA       bool `json:"RA"`
	AD       bool `json:"AD"`
	CD       bool `json:"CD"`
	Question []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
	} `json:"Question"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func ResolveDNSOverHTTPS(domain string) (string, error) {
	const dohURL = "https://cloudflare-dns.com/dns-query"

	query := url.Values{}
	query.Add("name", domain)
	query.Add("type", "AAAA")

	req, err := http.NewRequest(http.MethodGet, dohURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建 DoH 请求失败：%w", err)
	}

	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/dns-json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DoH 请求发送失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DoH 请求返回状态码 %d", resp.StatusCode)
	}

	var dohResp DoHResponse
	if err := json.NewDecoder(resp.Body).Decode(&dohResp); err != nil {
		return "", fmt.Errorf("解析 DoH 响应失败：%w", err)
	}

	for _, answer := range dohResp.Answer {
		if answer.Type == 28 { // AAAA 记录
			return answer.Data, nil
		}
	}

	return "", fmt.Errorf("未找到 %s 的 AAAA 记录", domain)
}
