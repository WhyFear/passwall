package generator

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"passwall/internal/model"
)

// ShareLinkGenerator 分享链接生成器
type ShareLinkGenerator struct{}

// NewShareLinkGenerator 创建分享链接生成器
func NewShareLinkGenerator() Generator {
	return &ShareLinkGenerator{}
}

// Generate 生成分享链接
func (g *ShareLinkGenerator) Generate(proxies []*model.Proxy) ([]byte, error) {
	var links []string

	for _, proxy := range proxies {
		link, err := generateShareLink(proxy)
		if err != nil {
			continue // 跳过生成失败的代理
		}
		links = append(links, link)
	}

	if len(links) == 0 {
		return nil, fmt.Errorf("没有可生成分享链接的代理")
	}
	// 将links用base64编码
	encodedLinks := base64.StdEncoding.EncodeToString([]byte(strings.Join(links, "\n")))
	return []byte(encodedLinks), nil
}

// Format 返回生成的配置格式
func (g *ShareLinkGenerator) Format() string {
	return "share-link"
}

// generateShareLink 根据代理类型生成相应的分享链接
func generateShareLink(proxy *model.Proxy) (string, error) {
	// 实现参考：https://github1s.com/clash-verge-rev/clash-verge-rev/blob/dev/src/utils/uri-parser.ts
	switch proxy.Type {
	case model.ProxyTypeVMess:
		return generateVMessLink(proxy)
	case model.ProxyTypeVLess:
		return generateVLessLink(proxy)
	case model.ProxyTypeTrojan:
		return generateTrojanLink(proxy)
	case model.ProxyTypeHysteria2:
		return generateHysteria2Link(proxy)
	case model.ProxyTypeHysteria:
		return generateHysteriaLink(proxy)
	case model.ProxyTypeSS:
		return generateSSLink(proxy)
	case model.ProxyTypeTuic:
		return generateTuicLink(proxy)
	// 可以根据需要添加其他协议的支持
	default:
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}
}

func generateTrojanLink(proxy *model.Proxy) (string, error) {
	// https://github.com/p4gefau1t/trojan-go/issues/132
	if proxy.Type != model.ProxyTypeTrojan {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析Trojan config配置失败: %v", err)
	}

	password, ok := config["password"].(string)
	if !ok || password == "" {
		return "", fmt.Errorf("trojan配置缺少密码")
	}

	// 构建基本链接
	link := fmt.Sprintf("trojan://%s@%s:%d", password, proxy.Domain, proxy.Port)

	// 添加查询参数
	params := make(map[string]string)

	// 处理网络类型
	if network, ok := config["network"].(string); ok && network != "" && network != "tcp" {
		params["type"] = network

		switch network {
		case "ws":
			wsOpts, ok := config["ws-opts"].(map[string]any)
			if ok && wsOpts != nil {
				if path, ok := wsOpts["path"].(string); ok && path != "" {
					params["path"] = path
				}

				headers, ok := wsOpts["headers"].(map[string]any)
				if ok && headers != nil {
					if host, ok := headers["Host"].(string); ok && host != "" {
						params["host"] = host
					}
				}
			}
		case "grpc":
			grpcOpts, ok := config["grpc-opts"].(map[string]any)
			if ok && grpcOpts != nil {
				if serviceName, ok := grpcOpts["grpc-service-name"].(string); ok && serviceName != "" {
					params["path"] = serviceName
				}
			}
		}
	}

	// 处理TLS相关参数
	if sni, ok := config["sni"].(string); ok && sni != "" {
		params["sni"] = sni
	}

	if alpn, ok := config["alpn"].([]any); ok && len(alpn) > 0 {
		alpnStrs := make([]string, 0, len(alpn))
		for _, a := range alpn {
			if aStr, ok := a.(string); ok {
				alpnStrs = append(alpnStrs, aStr)
			}
		}
		if len(alpnStrs) > 0 {
			params["alpn"] = strings.Join(alpnStrs, ",")
		}
	}

	if skipCertVerify, ok := config["skip-cert-verify"].(bool); ok && skipCertVerify {
		params["skip-cert-verify"] = "1"
	}

	if fingerprint, ok := config["fingerprint"].(string); ok && fingerprint != "" {
		params["fp"] = fingerprint
	}

	if clientFingerprint, ok := config["client-fingerprint"].(string); ok && clientFingerprint != "" {
		params["client-fingerprint"] = clientFingerprint
	}

	// 处理SS加密选项
	if ssOpts, ok := config["ss-opts"].(map[string]any); ok && ssOpts != nil {
		if enabled, ok := ssOpts["enabled"].(bool); ok && enabled {
			method, _ := ssOpts["method"].(string)
			ssPassword, _ := ssOpts["password"].(string)
			if method != "" && ssPassword != "" {
				params["encryption"] = fmt.Sprintf("ss;%s;%s", method, ssPassword)
			}
		}
	}

	// 构建URL
	urlObj, _ := url.Parse(link)

	// 添加查询参数
	if len(params) > 0 {
		q := urlObj.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		urlObj.RawQuery = q.Encode()
	}

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		urlObj.Fragment = proxy.Name
	} else {
		urlObj.Fragment = fmt.Sprintf("Trojan %s:%d", proxy.Domain, proxy.Port)
	}

	return urlObj.String(), nil
}

// generateVMessLink 生成VMess分享链接
func generateVMessLink(proxy *model.Proxy) (string, error) {
	// 提案：https://github.com/2dust/v2rayN/wiki/Description-of-VMess-share-link
	if proxy.Type != model.ProxyTypeVMess {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	// 先把proxy.Config解码成map
	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析VMess config配置失败: %v", err)
	}
	obj := map[string]any{
		"v":    "2",
		"add":  proxy.Domain,
		"port": proxy.Port,
		"type": "none",
		"ps":   config["name"],
		"id":   config["uuid"],
		"aid":  config["alterId"],
	}
	keyList := []string{"scy", "tls", "net", "host", "path", "sni", "alpn", "fp"}

	for _, key := range keyList {
		if value, exists := config[key]; exists {
			obj[key] = value
		}
	}

	jsonStr, _ := json.MarshalIndent(obj, "", "  ")
	return "vmess://" + base64.StdEncoding.EncodeToString(jsonStr), nil
}

func generateVLessLink(proxy *model.Proxy) (string, error) {
	// https://github.com/XTLS/Xray-core/discussions/716
	if proxy.Type != model.ProxyTypeVLess {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}
	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析VLess config配置失败: %v", err)
	}

	uuid := config["uuid"].(string)
	port := proxy.Port
	network, _ := config["network"].(string)
	if network == "" {
		network = "tcp"
	}

	params := make(map[string]string)
	params["type"] = network

	// 处理网络配置
	switch network {
	case "ws":
		wsOpts, _ := config["ws-opts"].(map[string]any)
		if wsOpts != nil {
			if path, ok := wsOpts["path"].(string); ok && path != "" {
				params["path"] = path
			}

			headers, _ := wsOpts["headers"].(map[string]any)
			if headers != nil {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params["host"] = host
				}
			}
		}
	case "http":
		httpOpts, _ := config["http-opts"].(map[string]any)
		if httpOpts != nil {
			if path, ok := httpOpts["path"].(string); ok && len(path) > 0 {
				params["path"] = path
			} else if paths, ok := httpOpts["path"].([]any); ok && len(paths) > 0 {
				pathStrList := make([]string, 0, len(paths))
				for _, h := range paths {
					if hStr, ok := h.(string); ok && hStr != "" {
						pathStrList = append(pathStrList, hStr)
					}
				}
				if len(pathStrList) > 0 {
					params["path"] = strings.Join(pathStrList, ",")
				}
			}

			if method, ok := httpOpts["method"].(string); ok && method != "" {
				params["method"] = method
			}

			headers, _ := httpOpts["headers"].(map[string]any)
			if headers != nil {
				if host, ok := headers["Host"].(string); ok && host != "" {
					params["host"] = host
				} else if hosts, ok := headers["Host"].([]any); ok && len(hosts) > 0 {
					// 处理host是一个列表的情况
					// 尝试将整个列表转为逗号分隔字符串
					hostStrList := make([]string, 0, len(hosts))
					for _, h := range hosts {
						if hStr, ok := h.(string); ok && hStr != "" {
							hostStrList = append(hostStrList, hStr)
						}
					}
					if len(hostStrList) > 0 {
						params["host"] = strings.Join(hostStrList, ",")
					}
				}
			}
			params["headerType"] = "http"
		}
	case "grpc":
		grpcOpts, _ := config["grpc-opts"].(map[string]any)
		if grpcOpts != nil {
			if serviceName, ok := grpcOpts["serviceName"].(string); ok && serviceName != "" {
				params["serviceName"] = serviceName
			}

			if multiMode, ok := grpcOpts["multiMode"].(bool); ok && multiMode {
				params["mode"] = "multi"
			}
		}
	}

	// 处理安全设置
	if tls, ok := config["tls"].(bool); ok && tls {
		params["security"] = "tls"

		if servername, ok := config["servername"].(string); ok && servername != "" {
			params["sni"] = servername
		}

		if alpn, ok := config["alpn"].([]any); ok && len(alpn) > 0 {
			alpnStrs := make([]string, 0, len(alpn))
			for _, a := range alpn {
				if aStr, ok := a.(string); ok {
					alpnStrs = append(alpnStrs, aStr)
				}
			}
			if len(alpnStrs) > 0 {
				params["alpn"] = strings.Join(alpnStrs, ",")
			}
		}

		if fp, ok := config["client-fingerprint"].(string); ok && fp != "" {
			params["fp"] = fp
		}

		if skipCertVerify, ok := config["skip-cert-verify"].(bool); ok && skipCertVerify {
			params["allowInsecure"] = "1"
		}

		if flow, ok := config["flow"].(string); ok && flow != "" {
			params["flow"] = flow
		}
	} else if realityOpts, ok := config["reality-opts"].(map[string]any); ok {
		// Reality 设置
		params["security"] = "reality"

		if fp, ok := config["client-fingerprint"].(string); ok && fp != "" {
			params["fp"] = fp
		}

		if servername, ok := config["servername"].(string); ok && servername != "" {
			params["sni"] = servername
		}

		if publicKey, ok := realityOpts["public-key"].(string); ok && publicKey != "" {
			params["pbk"] = publicKey
		}

		if shortID, ok := realityOpts["short-id"].(string); ok && shortID != "" {
			params["sid"] = shortID
		}

		params["spx"] = "/"

		if flow, ok := config["flow"].(string); ok && flow != "" {
			params["flow"] = flow
		}
	} else {
		params["security"] = "none"
	}

	// 构建URL
	link := fmt.Sprintf("vless://%s@%s:%d", uuid, proxy.Domain, port)
	urlObj, _ := url.Parse(link)
	q := urlObj.Query()

	for k, v := range params {
		q.Add(k, v)
	}

	urlObj.RawQuery = q.Encode()

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		urlObj.Fragment = proxy.Name
	} else {
		urlObj.Fragment = fmt.Sprintf("VLESS %s:%d", proxy.Domain, port)
	}

	return urlObj.String(), nil
}

// generateSSLink 生成Shadowsocks分享链接
func generateSSLink(proxy *model.Proxy) (string, error) {
	if proxy.Type != model.ProxyTypeSS {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析SS config配置失败: %v", err)
	}

	cipher, ok := config["cipher"].(string)
	if !ok || cipher == "" {
		return "", fmt.Errorf("ss配置缺少加密方式")
	}

	password, ok := config["password"].(string)
	if !ok || password == "" {
		return "", fmt.Errorf("ss配置缺少密码")
	}

	// 构建用户信息部分
	userInfo := fmt.Sprintf("%s:%s", cipher, password)
	userInfoBase64 := base64.StdEncoding.EncodeToString([]byte(userInfo))

	// 构建基本链接
	link := fmt.Sprintf("ss://%s@%s:%d", userInfoBase64, proxy.Domain, proxy.Port)

	// 处理插件
	if plugin, ok := config["plugin"].(string); ok && plugin != "" {
		pluginOpts, _ := config["plugin-opts"].(map[string]any)
		if pluginOpts != nil {
			var pluginParams []string

			switch plugin {
			case "obfs":
				pluginParams = append(pluginParams, "plugin=obfs-local")

				if mode, ok := pluginOpts["mode"].(string); ok && mode != "" {
					pluginParams = append(pluginParams, fmt.Sprintf("obfs=%s", mode))
				}

				if host, ok := pluginOpts["host"].(string); ok && host != "" {
					pluginParams = append(pluginParams, fmt.Sprintf("obfs-host=%s", host))
				}

			case "v2ray-plugin":
				pluginParams = append(pluginParams, "plugin=v2ray-plugin")

				if mode, ok := pluginOpts["mode"].(string); ok && mode != "" {
					pluginParams = append(pluginParams, fmt.Sprintf("mode=%s", mode))
				}

				if host, ok := pluginOpts["host"].(string); ok && host != "" {
					pluginParams = append(pluginParams, fmt.Sprintf("obfs-host=%s", host))
				}

				if path, ok := pluginOpts["path"].(string); ok && path != "" {
					pluginParams = append(pluginParams, fmt.Sprintf("path=%s", path))
				}

				if tls, ok := pluginOpts["tls"].(bool); ok && tls {
					pluginParams = append(pluginParams, "tls")
				}
			}

			if len(pluginParams) > 0 {
				pluginStr := strings.Join(pluginParams, ";")
				link = fmt.Sprintf("%s?plugin=%s", link, url.QueryEscape(pluginStr))
			}
		}
	}

	// 处理额外参数
	var queryParams []string

	if udpOverTcp, ok := config["udp-over-tcp"].(bool); ok && udpOverTcp {
		queryParams = append(queryParams, "uot=1")
	}

	if tfo, ok := config["tfo"].(bool); ok && tfo {
		queryParams = append(queryParams, "tfo=1")
	}

	// 添加查询参数
	if len(queryParams) > 0 {
		if strings.Contains(link, "?") {
			link = fmt.Sprintf("%s&%s", link, strings.Join(queryParams, "&"))
		} else {
			link = fmt.Sprintf("%s?%s", link, strings.Join(queryParams, "&"))
		}
	}

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		link = fmt.Sprintf("%s#%s", link, url.QueryEscape(proxy.Name))
	}

	return link, nil
}

func generateHysteria2Link(proxy *model.Proxy) (string, error) {
	// https://v2.hysteria.network/docs/developers/URI-Scheme/
	if proxy.Type != model.ProxyTypeHysteria2 {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析Hysteria2 config配置失败: %v", err)
	}

	password, ok := config["password"].(string)
	if !ok || password == "" {
		return "", fmt.Errorf("hysteria2配置缺少密码")
	}

	// 构建基本链接
	link := fmt.Sprintf("hysteria2://%s@%s:%d", password, proxy.Domain, proxy.Port)

	// 添加查询参数
	params := make(map[string]string)

	// 处理SNI
	if sni, ok := config["sni"].(string); ok && sni != "" {
		params["sni"] = sni
	}

	// 处理混淆
	if obfs, ok := config["obfs"].(string); ok && obfs != "" && obfs != "none" {
		params["obfs"] = obfs

		if obfsPassword, ok := config["obfs-password"].(string); ok && obfsPassword != "" {
			params["obfs-password"] = obfsPassword
		}
	}

	// 处理多端口
	if ports, ok := config["ports"].(string); ok && ports != "" {
		params["mport"] = ports
	}

	// 处理证书验证
	if skipCertVerify, ok := config["skip-cert-verify"].(bool); ok && skipCertVerify {
		params["insecure"] = "1"
	}

	// 处理TFO
	if tfo, ok := config["tfo"].(bool); ok && tfo {
		params["fastopen"] = "1"
	}

	// 处理指纹
	if fingerprint, ok := config["fingerprint"].(string); ok && fingerprint != "" {
		params["pinSHA256"] = fingerprint
	}

	// 构建URL
	urlObj, _ := url.Parse(link)

	// 添加查询参数
	if len(params) > 0 {
		q := urlObj.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		urlObj.RawQuery = q.Encode()
	}

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		urlObj.Fragment = proxy.Name
	} else {
		urlObj.Fragment = fmt.Sprintf("Hysteria2 %s:%d", proxy.Domain, proxy.Port)
	}

	return urlObj.String(), nil
}

func generateHysteriaLink(proxy *model.Proxy) (string, error) {
	// https://hysteria.network/docs/developers/URI-Scheme/
	if proxy.Type != model.ProxyTypeHysteria {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析Hysteria config配置失败: %v", err)
	}

	// 构建基本链接
	link := fmt.Sprintf("hysteria://%s:%d", proxy.Domain, proxy.Port)

	// 添加查询参数
	params := make(map[string]string)

	// 处理认证字符串
	if authStr, ok := config["auth-str"].(string); ok && authStr != "" {
		params["auth"] = authStr
	}

	// 处理SNI
	if sni, ok := config["sni"].(string); ok && sni != "" {
		params["sni"] = sni
	}

	// 处理ALPN
	if alpn, ok := config["alpn"].([]any); ok && len(alpn) > 0 {
		alpnStrs := make([]string, 0, len(alpn))
		for _, a := range alpn {
			if aStr, ok := a.(string); ok {
				alpnStrs = append(alpnStrs, aStr)
			}
		}
		if len(alpnStrs) > 0 {
			params["alpn"] = strings.Join(alpnStrs, ",")
		}
	}

	// 处理混淆
	if obfs, ok := config["obfs"].(string); ok && obfs != "" {
		params["obfs"] = obfs
	}

	// 处理多端口
	if ports, ok := config["ports"].(string); ok && ports != "" {
		params["mport"] = ports
	}

	// 处理上传和下载速率
	if up, ok := config["up"].(string); ok && up != "" {
		params["upmbps"] = up
	}

	if down, ok := config["down"].(string); ok && down != "" {
		params["downmbps"] = down
	}

	// 处理证书验证
	if skipCertVerify, ok := config["skip-cert-verify"].(bool); ok && skipCertVerify {
		params["insecure"] = "1"
	}

	// 处理TFO
	if fastOpen, ok := config["fast-open"].(bool); ok && fastOpen {
		params["fast-open"] = "1"
	}

	// 处理接收窗口大小
	if recvWindowConn, ok := config["recv-window-conn"].(int); ok && recvWindowConn > 0 {
		params["recv-window-conn"] = fmt.Sprintf("%d", recvWindowConn)
	}

	if recvWindow, ok := config["recv-window"].(int); ok && recvWindow > 0 {
		params["recv-window"] = fmt.Sprintf("%d", recvWindow)
	}

	// 处理CA
	if ca, ok := config["ca"].(string); ok && ca != "" {
		params["ca"] = ca
	}

	if caStr, ok := config["ca-str"].(string); ok && caStr != "" {
		params["ca-str"] = caStr
	}

	// 处理MTU发现
	if disableMtuDiscovery, ok := config["disable-mtu-discovery"].(bool); ok && disableMtuDiscovery {
		params["disable-mtu-discovery"] = "1"
	}

	// 处理指纹
	if fingerprint, ok := config["fingerprint"].(string); ok && fingerprint != "" {
		params["fingerprint"] = fingerprint
	}

	// 处理协议
	if protocol, ok := config["protocol"].(string); ok && protocol != "" {
		params["protocol"] = protocol
	} else {
		params["protocol"] = "udp"
	}

	// 构建URL
	urlObj, _ := url.Parse(link)

	// 添加查询参数
	if len(params) > 0 {
		q := urlObj.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		urlObj.RawQuery = q.Encode()
	}

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		urlObj.Fragment = proxy.Name
	} else {
		urlObj.Fragment = fmt.Sprintf("Hysteria %s:%d", proxy.Domain, proxy.Port)
	}

	return urlObj.String(), nil
}

// generateTuicLink 生成TUIC分享链接
func generateTuicLink(proxy *model.Proxy) (string, error) {
	if proxy.Type != model.ProxyTypeTuic {
		return "", fmt.Errorf("不支持的代理类型: %s", proxy.Type)
	}

	var config map[string]any
	err := json.Unmarshal([]byte(proxy.Config), &config)
	if err != nil {
		return "", fmt.Errorf("解析TUIC config配置失败: %v", err)
	}

	uuid, ok := config["uuid"].(string)
	if !ok || uuid == "" {
		return "", fmt.Errorf("tuic配置缺少uuid")
	}

	password, ok := config["password"].(string)
	if !ok || password == "" {
		return "", fmt.Errorf("tuic配置缺少密码")
	}

	// 构建基本链接
	link := fmt.Sprintf("tuic://%s:%s@%s:%d", uuid, password, proxy.Domain, proxy.Port)

	// 添加查询参数
	params := make(map[string]string)

	// 处理token
	if token, ok := config["token"].(string); ok && token != "" {
		params["token"] = token
	}

	// 处理IP
	if ip, ok := config["ip"].(string); ok && ip != "" {
		params["ip"] = ip
	}

	// 处理心跳间隔
	if heartbeatInterval, ok := config["heartbeat-interval"].(float64); ok && heartbeatInterval > 0 {
		params["heartbeat-interval"] = fmt.Sprintf("%d", int(heartbeatInterval))
	}

	// 处理ALPN
	if alpn, ok := config["alpn"].([]any); ok && len(alpn) > 0 {
		alpnStrs := make([]string, 0, len(alpn))
		for _, a := range alpn {
			if aStr, ok := a.(string); ok {
				alpnStrs = append(alpnStrs, aStr)
			}
		}
		if len(alpnStrs) > 0 {
			params["alpn"] = strings.Join(alpnStrs, ",")
		}
	}

	// 处理禁用SNI
	if disableSni, ok := config["disable-sni"].(bool); ok && disableSni {
		params["disable-sni"] = "1"
	}

	// 处理减少RTT
	if reduceRtt, ok := config["reduce-rtt"].(bool); ok && reduceRtt {
		params["reduce-rtt"] = "1"
	}

	// 处理请求超时
	if requestTimeout, ok := config["request-timeout"].(float64); ok && requestTimeout > 0 {
		params["request-timeout"] = fmt.Sprintf("%d", int(requestTimeout))
	}

	// 处理UDP中继模式
	if udpRelayMode, ok := config["udp-relay-mode"].(string); ok && udpRelayMode != "" {
		params["udp-relay-mode"] = udpRelayMode
	}

	// 处理拥塞控制器
	if congestionController, ok := config["congestion-controller"].(string); ok && congestionController != "" {
		params["congestion-controller"] = congestionController
	}

	// 处理最大UDP中继包大小
	if maxUdpRelayPacketSize, ok := config["max-udp-relay-packet-size"].(float64); ok && maxUdpRelayPacketSize > 0 {
		params["max-udp-relay-packet-size"] = fmt.Sprintf("%d", int(maxUdpRelayPacketSize))
	}

	// 处理快速打开
	if fastOpen, ok := config["fast-open"].(bool); ok && fastOpen {
		params["fast-open"] = "1"
	}

	// 处理跳过证书验证
	if skipCertVerify, ok := config["skip-cert-verify"].(bool); ok && skipCertVerify {
		params["skip-cert-verify"] = "1"
	}

	// 处理最大打开流
	if maxOpenStreams, ok := config["max-open-streams"].(float64); ok && maxOpenStreams > 0 {
		params["max-open-streams"] = fmt.Sprintf("%d", int(maxOpenStreams))
	}

	// 处理SNI
	if sni, ok := config["sni"].(string); ok && sni != "" {
		params["sni"] = sni
	}

	// 构建URL
	urlObj, _ := url.Parse(link)

	// 添加查询参数
	if len(params) > 0 {
		q := urlObj.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		urlObj.RawQuery = q.Encode()
	}

	// 设置URL片段（Fragment）为代理名称
	if proxy.Name != "" {
		urlObj.Fragment = proxy.Name
	} else {
		urlObj.Fragment = fmt.Sprintf("TUIC %s:%d", proxy.Domain, proxy.Port)
	}

	return urlObj.String(), nil
}
