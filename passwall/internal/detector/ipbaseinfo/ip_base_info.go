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
	"github.com/tidwall/gjson"
)

type IPBaseInfo struct {
	IPV4 string
	IPV6 string
}

// IPService 定义IP获取服务的配置
type IPService struct {
	Name   string
	URL    string
	Format *IPFormat
}

// IPFormat 直接返回string，json、html等格式的IP地址
type IPFormat struct {
	Format string // string、json、html
	IPPath string // 提取IP地址的路径，如"ip.address"
}

// 定义支持的IP获取服务
var ipServices = []IPService{
	{"MyIPCheckPlace", "https://myip.check.place", nil},
	{"IPsb", "https://api.ip.sb/ip", nil},
	//{"Ping0", "https://ping0.cc/ip", nil},
	{"ICanHazIP", "https://icanhazip.com", nil},
	{"IPify", "https://api64.ipify.org", nil},
	{"IfConfig", "https://ifconfig.co/ip", nil},
	{"IdentMeV4", "https://4.ident.me", nil},
	{"IdentMeV6", "https://6.ident.me", nil},
	{"ITDogV6", "https://ipv6.itdog.cn/", &IPFormat{Format: "json", IPPath: "ip"}},
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
			resp, err := util.GetUrl(proxyClient, svc.URL)
			if err != nil {
				log.Infoln("IP服务 %s 获取IP失败: %v", svc.Name, err)
				return
			}
			ipStr := getIPAddress(resp, svc.Format)
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
		return nil, fmt.Errorf("no IP address found")
	}

	return ipInfo, nil
}

func getIPAddress(resp []byte, format *IPFormat) string {
	if resp == nil || len(resp) == 0 {
		return ""
	}
	if format == nil {
		format = &IPFormat{Format: "string"}
	}
	switch format.Format {
	case "string":
		ipStr := strings.TrimSpace(string(resp))
		ipStr = strings.Replace(ipStr, "\n", "", -1)
		return ipStr
	case "json":
		return gjson.ParseBytes(resp).Get(format.IPPath).String()
	case "html":
		// fixme 后续遇到了再加上
		return strings.TrimSpace(string(resp))
	default:
		return string(resp)
	}
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
