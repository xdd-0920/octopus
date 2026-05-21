package op

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
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

// testUsage 用于从原始响应中提取 usage 信息
type testUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type testRawResponse struct {
	Usage *testUsage `json:"usage"`
}

// streamChunk 流式响应的单个数据块
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *testUsage `json:"usage"`
}

// parseSSEChunk 解析单个 SSE 数据块并更新结果
func parseSSEChunk(data string, contentBuilder *strings.Builder, gotFirstToken *bool, startTime time.Time, firstTokenMs *int64, inputTokens, outputTokens *int) {
	if data == "[DONE]" {
		return
	}

	var chunk streamChunk
	if json.Unmarshal([]byte(data), &chunk) != nil {
		return
	}

	// 记录首字时间
	if !*gotFirstToken && len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
		*firstTokenMs = time.Since(startTime).Milliseconds()
		*gotFirstToken = true
	}

	// 累积响应内容
	for _, choice := range chunk.Choices {
		contentBuilder.WriteString(choice.Delta.Content)
	}

	// 提取 usage（部分 provider 在最后一个 chunk 中返回）
	if chunk.Usage != nil {
		*inputTokens = chunk.Usage.PromptTokens
		*outputTokens = chunk.Usage.CompletionTokens
	}
}

// parseStreamResponse 解析 SSE 流式响应，返回首字时间(ms)、聚合响应内容、输入/输出 tokens
// 按照 SSE 规范，多个 data: 行通过空行分隔为事件，同一事件的多行 data: 用换行符拼接后解析
func parseStreamResponse(body io.Reader, startTime time.Time) (firstTokenMs int64, response string, inputTokens, outputTokens int) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024)
	var contentBuilder strings.Builder
	var gotFirstToken bool
	var dataBuffer strings.Builder // 累积同一事件的 data: 行
	var done bool

	for scanner.Scan() {
		line := scanner.Text()

		// 空行 = SSE 事件边界，尝试解析已累积的数据
		if strings.TrimSpace(line) == "" {
			if dataBuffer.Len() == 0 {
				continue
			}
			data := dataBuffer.String()
			dataBuffer.Reset()

			if data == "[DONE]" {
				done = true
				break
			}

			parseSSEChunk(data, &contentBuilder, &gotFirstToken, startTime, &firstTokenMs, &inputTokens, &outputTokens)
			continue
		}

		// 累积 data: 行（多个 data: 行拼接为完整事件数据）
		if strings.HasPrefix(line, "data: ") {
			if dataBuffer.Len() > 0 {
				dataBuffer.WriteByte('\n')
			}
			dataBuffer.WriteString(strings.TrimPrefix(line, "data: "))
		}
		// 忽略其他 SSE 字段（event:, id:, retry:, 注释行）
	}

	// 处理流末尾没有空行结尾的最后一个事件
	if !done && dataBuffer.Len() > 0 {
		data := dataBuffer.String()
		if data != "[DONE]" {
			parseSSEChunk(data, &contentBuilder, &gotFirstToken, startTime, &firstTokenMs, &inputTokens, &outputTokens)
		}
	}

	return firstTokenMs, contentBuilder.String(), inputTokens, outputTokens
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

	// 确定是否使用流式模式（默认 true）
	useStream := true
	if req.Stream != nil {
		useStream = *req.Stream
	}

	// 构建测试消息（必须包含 messages 字段）
	testContent := "Hello"
	testReq := &transformerModel.InternalLLMRequest{
		Stream: &useStream,
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

	// 序列化请求内容用于日志
	reqJSON, _ := json.Marshal(testReq)

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

	var inputTokens, outputTokens int
	var firstTokenTime int64
	var responseBody string

	if useStream {
		// 流式模式：逐行读取 SSE 事件
		firstTokenTime, responseBody, inputTokens, outputTokens = parseStreamResponse(resp.Body, startTime)
	} else {
		// 非流式模式：一次性读取响应
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		if readErr != nil {
			result.Error = fmt.Sprintf("读取响应失败: %v", readErr)
			return result
		}
		responseBody = string(body)
		var rawResp testRawResponse
		if json.Unmarshal(body, &rawResp) == nil && rawResp.Usage != nil {
			inputTokens = rawResp.Usage.PromptTokens
			outputTokens = rawResp.Usage.CompletionTokens
		}
	}

	// 格式化响应内容
	var formatted bytes.Buffer
	if json.Indent(&formatted, []byte(responseBody), "", "  ") == nil {
		result.Response = formatted.String()
	} else {
		result.Response = responseBody
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true
		log.Debugf("channel test success: %d, stream=%v, response: %s", req.ChannelID, useStream, result.Response)
	} else {
		result.Error = responseBody
		if len(result.Error) > 500 {
			result.Error = result.Error[:500] + "..."
		}
		log.Warnf("channel test failed: %d, status: %d, error: %s", req.ChannelID, resp.StatusCode, result.Error)
	}

	// 写入测试日志，包含完整的请求/响应上下文
	useTimeMs := int(result.ResponseTimeMs)
	ftut := useTimeMs
	if firstTokenTime > 0 {
		ftut = int(firstTokenTime)
	}
	attemptStatus := model.AttemptSuccess
	attemptMsg := "测试通过"
	if !result.Success {
		attemptStatus = model.AttemptFailed
		attemptMsg = result.Error
	}
	testLog := model.RelayLog{
		Time:              startTime.Unix(),
		RequestModelName:  testModel,
		RequestAPIKeyName: "渠道测试",
		ChannelId:         channel.ID,
		ChannelName:       channel.Name,
		ActualModelName:   testModel,
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		Ftut:              ftut,
		UseTime:           useTimeMs,
		RequestContent:    string(reqJSON),
		ResponseContent:   result.Response,
		Error:             result.Error,
		Attempts: []model.ChannelAttempt{
			{
				ChannelID:   channel.ID,
				ChannelName: channel.Name,
				ModelName:   testModel,
				AttemptNum:  1,
				Status:      attemptStatus,
				Duration:    useTimeMs,
				Msg:         attemptMsg,
			},
		},
		TotalAttempts: 1,
	}
	if logErr := RelayLogAdd(ctx, testLog); logErr != nil {
		log.Warnf("channel test log add failed: %d, error: %v", req.ChannelID, logErr)
	}

	return result
}
