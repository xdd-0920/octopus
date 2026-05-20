package op

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/bestruirui/octopus/internal/model"
	transformerModel "github.com/bestruirui/octopus/internal/transformer/model"
	"github.com/bestruirui/octopus/internal/transformer/outbound"
	"github.com/bestruirui/octopus/internal/utils/log"
)

// testHTTPClient 创建一个用于渠道测试的 HTTP 客户端，避免引入 helper 包的循环依赖
func testHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			MaxIdleConnsPerHost:   2,
		},
	}
}

// TestChannel 测试渠道连通性和实时性
// 发送一条简单的 "Hello" 消息并检查返回结果
func TestChannel(ctx context.Context, req model.ChannelTestRequest) model.ChannelTestResult {
	startTime := time.Now()
	result := model.ChannelTestResult{}

	// 获取渠道配置
	channel, err := ChannelGet(req.ChannelID, ctx)
	if err != nil {
		result.Error = fmt.Sprintf("获取渠道失败: %v", err)
		return result
	}

	if !channel.Enabled {
		result.Error = "渠道未启用"
		return result
	}

	// 获取出站适配器
	outAdapter := outbound.Get(channel.Type)
	if outAdapter == nil {
		result.Error = fmt.Sprintf("不支持的渠道类型: %d", channel.Type)
		return result
	}

	// 获取可用 key
	usedKey := channel.GetChannelKey()
	if usedKey.ChannelKey == "" {
		result.Error = "没有可用的 channel key"
		return result
	}

	// 选择测试模型
	testModel := req.Model
	if testModel == "" {
		testModel = channel.Model
		if testModel == "" {
			testModel = "gpt-3.5-turbo"
		}
	}

	// 构建测试消息（必须包含 messages 字段）
	stream := false
	testContent := "Hello"
	testReq := &transformerModel.InternalLLMRequest{
		Stream: &stream,
		Model:  testModel,
		Messages: []transformerModel.Message{
			{
				Role: "user",
				Content: transformerModel.MessageContent{
					Content: &testContent,
				},
			},
		},
	}

	// 构建 HTTP 请求
	outReq, err := outAdapter.TransformRequest(
		ctx,
		testReq,
		channel.GetBaseUrl(),
		usedKey.ChannelKey,
	)
	if err != nil {
		result.Error = fmt.Sprintf("构建请求失败: %v", err)
		return result
	}

	// 发送请求（使用内联 HTTP 客户端避免循环依赖）
	httpClient := testHTTPClient()

	resp, err := httpClient.Do(outReq)
	if err != nil {
		result.Error = fmt.Sprintf("发送请求失败: %v", err)
		result.ResponseTimeMs = time.Since(startTime).Milliseconds()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ResponseTimeMs = time.Since(startTime).Milliseconds()

	// 读取响应体
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		result.Error = fmt.Sprintf("读取响应失败: %v", err)
		return result
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
		// 尝试格式化 JSON 响应
		var formatted bytes.Buffer
		if err := json.Indent(&formatted, body, "", "  "); err == nil {
			result.Response = formatted.String()
		} else {
			result.Response = string(body)
		}
		log.Debugf("channel test success: %d, response: %s", req.ChannelID, result.Response)
	} else {
		result.Error = string(body)
		if len(result.Error) > 500 {
			result.Error = result.Error[:500] + "..."
		}
		log.Warnf("channel test failed: %d, status: %d, error: %s", req.ChannelID, resp.StatusCode, result.Error)
	}

	return result
}
