package ipbaseinfo

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"passwall/internal/util"
	"strings"
	"sync"

	"github.com/metacubex/mihomo/log"
)

type IPBaseInfo struct {
	IPV4 string
	IPV6 string
}

// IPService 定义IP获取服务的配置
type IPService struct {
	Name string
	URL  string
}

// 定义支持的IP获取服务
var ipServices = []IPService{
	{"MyIPCheckPlace", "https://myip.check.place"},
	{"IPsb", "https://api.ip.sb/ip"},
	{"Ping0", "https://ping0.cc/ip"},
	{"ICanHazIP", "https://icanhazip.com"},
	{"IPify", "https://api64.ipify.org"},
	{"IfConfig", "https://ifconfig.co/ip"},
	{"IdentMeV4", "https://4.ident.me"},
	{"IdentMeV6", "https://6.ident.me"},
}

// GetProxyIP 从多个服务获取代理IP地址
func GetProxyIP(proxyClient *http.Client) (*IPBaseInfo, error) {
	if proxyClient == nil {
		log.Errorln("GetProxyIP error: proxyClient is nil")
		return nil, errors.New("proxyClient is nil")
	}
	var wg sync.WaitGroup
	results := make(chan string, len(ipServices))

	for _, service := range ipServices {
		wg.Add(1)
		go func(svc IPService) {
			defer wg.Done()
			ip, err := util.GetUrl(proxyClient, svc.URL)
			if err != nil {
				log.Infoln("IP服务 %s 获取IP失败: %v", svc.Name, err)
				return
			}
			ipStr := strings.TrimSpace(string(ip))
			ipStr = strings.Replace(ipStr, "\n", "", -1)
			log.Infoln("IP服务 %s 获取IP成功: %s", svc.Name, ipStr)
			results <- ipStr
		}(service)
	}

	wg.Wait()
	close(results)

	ipInfo := &IPBaseInfo{}
	ipv4Map := make(map[string]int)
	ipv6Map := make(map[string]int)

	for ip := range results {
		if ipInfo.IPV4 == "" && checkIPV4(ip) {
			ipv4Map[ip]++
		} else if ipInfo.IPV6 == "" && checkIPV6(ip) {
			ipv6Map[ip]++
		}
	}
	var ipv4Count int
	var ipv6Count int
	for ip, count := range ipv4Map {
		if count > ipv4Count {
			ipv4Count = count
			ipInfo.IPV4 = ip
		}
	}
	for ip, count := range ipv6Map {
		if count > ipv6Count {
			ipv6Count = count
			ipInfo.IPV6 = ip
		}
	}

	if ipInfo.IPV4 == "" && ipInfo.IPV6 == "" {
		return nil, fmt.Errorf("no IP services available")
	}

	return ipInfo, nil
}

func checkIPV4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return parsedIP.To4() != nil
}

func checkIPV6(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return parsedIP.To4() == nil && parsedIP.To16() != nil
}
