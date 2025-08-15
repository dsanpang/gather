package decoder

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gather/internal/config"

	"github.com/sirupsen/logrus"
)

// InputDecoder 输入数据解码器
type InputDecoder struct {
	logger *logrus.Logger
	cache  map[string]string      // 方法签名缓存
	config *config.DecoderConfig  // 解码器配置
	client *http.Client           // HTTP客户端
}

// FourByteResponse 4byte.directory API响应
type FourByteResponse struct {
	Count    int         `json:"count"`
	Next     *string     `json:"next"`
	Previous *string     `json:"previous"`
	Results  []signature `json:"results"`
}

type signature struct {
	ID             int    `json:"id"`
	CreatedAt      string `json:"created_at"`
	TextSignature  string `json:"text_signature"`
	HexSignature   string `json:"hex_signature"`
	BytesSignature string `json:"bytes_signature"`
}

// NewInputDecoder 创建新的输入解码器
func NewInputDecoder(logger *logrus.Logger, decoderConfig *config.DecoderConfig) *InputDecoder {
	if decoderConfig == nil {
		decoderConfig = &config.DecoderConfig{
			FourByteAPIURL: "https://www.4byte.directory/api/v1/signatures/",
			APITimeout:     "5s",
			EnableCache:    true,
			CacheSize:      10000,
			EnableAPI:      true,
		}
	}

	// 解析超时时间
	timeout, err := time.ParseDuration(decoderConfig.APITimeout)
	if err != nil {
		timeout = 5 * time.Second
		logger.Warnf("解析API超时时间失败，使用默认值5s: %v", err)
	}

	// 创建缓存，限制大小
	cache := make(map[string]string)
	if decoderConfig.CacheSize <= 0 {
		decoderConfig.CacheSize = 10000
	}

	return &InputDecoder{
		logger: logger,
		cache:  cache,
		config: decoderConfig,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// DecodeInput 解码交易输入数据
func (d *InputDecoder) DecodeInput(input string) (string, string, map[string]interface{}, bool) {
	// 移除0x前缀
	if strings.HasPrefix(input, "0x") {
		input = input[2:]
	}

	// 检查是否为合约调用（有输入数据且长度 >= 8）
	if len(input) < 8 {
		return "", "", nil, false
	}

	// 提取前4字节作为方法签名
	methodSig := input[:8]
	methodSigWithPrefix := "0x" + methodSig

	// 查找方法名
	methodName := d.getMethodName(methodSigWithPrefix)

	// 简单的参数解码（这里只做基础处理）
	decodedParams := d.decodeBasicParameters(input[8:])

	return methodSigWithPrefix, methodName, decodedParams, true
}

// getMethodName 从4byte.directory获取方法名
func (d *InputDecoder) getMethodName(signature string) string {
	// 首先检查缓存
	if d.config.EnableCache {
		if name, exists := d.cache[signature]; exists {
			return name
		}
	}

	// 从4byte.directory API获取
	var name string
	if d.config.EnableAPI {
		name = d.fetchFromFourByteDirectory(signature)
		if name != "" && d.config.EnableCache {
			// 缓存结果，检查缓存大小限制
			if len(d.cache) >= d.config.CacheSize {
				d.evictCache()
			}
			d.cache[signature] = name
			return name
		}
	}

	// 如果API失败，尝试一些常见的方法签名
	commonMethods := map[string]string{
		"0xa9059cbb": "transfer(address,uint256)",
		"0x095ea7b3": "approve(address,uint256)",
		"0x23b872dd": "transferFrom(address,address,uint256)",
		"0x70a08231": "balanceOf(address)",
		"0xdd62ed3e": "allowance(address,address)",
		"0x06fdde03": "name()",
		"0x95d89b41": "symbol()",
		"0x313ce567": "decimals()",
		"0x18160ddd": "totalSupply()",
		"0x40c10f19": "mint(address,uint256)",
		"0x42966c68": "burn(uint256)",
		"0x8da5cb5b": "owner()",
		"0xf2fde38b": "transferOwnership(address)",
		"0x8f32d59b": "isOwner()",
	}

	if name, exists := commonMethods[signature]; exists {
		if d.config.EnableCache {
			if len(d.cache) >= d.config.CacheSize {
				d.evictCache()
			}
			d.cache[signature] = name
		}
		return name
	}

	return "unknown"
}

// fetchFromFourByteDirectory 从4byte.directory API获取方法签名
func (d *InputDecoder) fetchFromFourByteDirectory(signature string) string {
	// 限制API调用频率，避免被限制
	url := fmt.Sprintf("%s?hex_signature=%s", d.config.FourByteAPIURL, signature)

	resp, err := d.client.Get(url)
	if err != nil {
		d.logger.Debugf("4byte.directory API调用失败: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Debugf("4byte.directory API返回错误状态: %d", resp.StatusCode)
		return ""
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		d.logger.Debugf("读取4byte.directory响应失败: %v", err)
		return ""
	}

	var response FourByteResponse
	if err := json.Unmarshal(body, &response); err != nil {
		d.logger.Debugf("解析4byte.directory响应失败: %v", err)
		return ""
	}

	if len(response.Results) > 0 {
		// 返回第一个匹配的签名
		return response.Results[0].TextSignature
	}

	return ""
}

// decodeBasicParameters 基础参数解码
func (d *InputDecoder) decodeBasicParameters(paramData string) map[string]interface{} {
	params := make(map[string]interface{})

	if len(paramData) == 0 {
		return params
	}

	// 每个参数32字节（64个十六进制字符）
	paramLength := 64
	paramCount := len(paramData) / paramLength

	for i := 0; i < paramCount && i < 10; i++ { // 最多解析10个参数
		start := i * paramLength
		end := start + paramLength

		if end > len(paramData) {
			break
		}

		paramHex := paramData[start:end]
		paramKey := fmt.Sprintf("param_%d", i)

		// 尝试解析为地址（如果前12字节为0）
		if strings.HasPrefix(paramHex, "000000000000000000000000") && len(paramHex) == 64 {
			address := "0x" + paramHex[24:]
			params[paramKey] = address
			params[paramKey+"_type"] = "address"
		} else {
			// 作为原始十六进制数据
			params[paramKey] = "0x" + paramHex
			params[paramKey+"_type"] = "bytes32"
		}
	}

	return params
}

// ClearCache 清理缓存
func (d *InputDecoder) ClearCache() {
	d.cache = make(map[string]string)
}

// GetCacheSize 获取缓存大小
func (d *InputDecoder) GetCacheSize() int {
	return len(d.cache)
}

// evictCache 清理部分缓存（LRU简化版）
func (d *InputDecoder) evictCache() {
	// 简单策略：清理一半的缓存
	targetSize := d.config.CacheSize / 2
	if targetSize <= 0 {
		targetSize = 1000
	}
	
	// 清理旧的缓存项（这里简化处理，实际应该实现LRU）
	count := 0
	for key := range d.cache {
		if count >= len(d.cache)-targetSize {
			break
		}
		delete(d.cache, key)
		count++
	}
	
	d.logger.Debugf("缓存清理完成，剩余 %d 项", len(d.cache))
}

// GetConfig 获取解码器配置
func (d *InputDecoder) GetConfig() *config.DecoderConfig {
	return d.config
}

// UpdateConfig 更新解码器配置
func (d *InputDecoder) UpdateConfig(newConfig *config.DecoderConfig) error {
	if newConfig == nil {
		return fmt.Errorf("配置不能为空")
	}
	
	// 更新超时时间
	if newConfig.APITimeout != d.config.APITimeout {
		timeout, err := time.ParseDuration(newConfig.APITimeout)
		if err != nil {
			return fmt.Errorf("无效的API超时时间: %v", err)
		}
		d.client.Timeout = timeout
	}
	
	// 更新配置
	d.config = newConfig
	
	// 如果禁用了缓存，清空现有缓存
	if !newConfig.EnableCache {
		d.cache = make(map[string]string)
	}
	
	d.logger.Infof("解码器配置已更新")
	return nil
}
