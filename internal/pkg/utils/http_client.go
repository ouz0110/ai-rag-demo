package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// HttpClient 是一个封装了 HTTP 请求的客户端
type HttpClient struct {
	client  *http.Client
	baseURL string
	headers map[string]string
}

// NewHttpClient 创建一个新的 HttpClient 实例
func NewHttpClient(baseURL string, timeout time.Duration, headers map[string]string) *HttpClient {
	return &HttpClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		headers: headers,
	}
}

// SetHeaders 设置请求头
func (c *HttpClient) SetHeaders(headers map[string]string) {
	for key, val := range headers {
		c.headers[key] = val
	}
}

// SetTimeout 设置超时时间
func (c *HttpClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}

// SetBaseURL 设置基础 URL
func (c *HttpClient) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

// Get 发送 GET 请求
func (c *HttpClient) Get(result interface{}, endpoint string, queryParams map[string]string) error {
	// 构造 URL
	reqURL, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return err
	}

	// 添加查询参数
	q := reqURL.Query()
	for key, value := range queryParams {
		q.Add(key, value)
	}
	reqURL.RawQuery = q.Encode()

	// 创建请求
	req, err := http.NewRequest("GET", reqURL.String(), nil)
	if err != nil {
		return err
	}

	// 设置请求头
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error: %s", resp.Status)
	}

	if len(body) == 0 {
		return nil
	}

	// 解析响应
	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	return nil
}

// Post 发送 POST 请求
func (c *HttpClient) Post(result interface{}, endpoint string, body interface{}) error {
	// 序列化请求体
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// 创建请求
	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// 设置请求头
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		if len(respBody) != 0 {
			return fmt.Errorf("error: %s ret: %s", resp.Status, respBody)
		}
		return fmt.Errorf("error: %s", resp.Status)
	}

	if len(respBody) == 0 {
		return nil
	}

	// 解析响应
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return err
	}

	return nil
}
func (c *HttpClient) Delete(result interface{}, endpoint string, body interface{}) error {
	// 序列化请求体
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	// 创建请求
	req, err := http.NewRequest("DELETE", c.baseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// 设置请求头
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 读取响应
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		if len(respBody) != 0 {
			return fmt.Errorf("error: %s ret: %s", resp.Status, respBody)
		}
		return fmt.Errorf("error: %s", resp.Status)
	}
	if len(respBody) == 0 {
		return nil
	}
	// 解析响应
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return err
	}
	return nil
}
