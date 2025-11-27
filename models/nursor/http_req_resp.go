package nursor

import (
	"encoding/base64"
	"strings"
	"time"
)

type HttpRecord struct {
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     []byte            `json:"request_body"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    []byte            `json:"response_body"`
	Url             string            `json:"url"`
	Method          string            `json:"method"`
	Host            string            `json:"host"`
	CreateAt        string            `json:"create_at"`
	HttpVersion     string            `json:"http_version"`
	UserId          int               `json:"user_id"`
	AccountId       int               `json:"account_id"`
	Status          int               `json:"status"`
}

func NewRequestRecord() *HttpRecord {
	return &HttpRecord{
		RequestHeaders:  map[string]string{},
		RequestBody:     []byte{},
		ResponseHeaders: map[string]string{},
		ResponseBody:    []byte{},
		Url:             "",
		Method:          "Post",
		Host:            "cursor.sh",
		CreateAt:        time.Now().Format("2006-01-02 15:04:05"),
		HttpVersion:     "http/1.1",
		AccountId:       0,
		UserId:          0,
		Status:          200,
	}
}

func (r *HttpRecord) AddRequestHeader(key, value string) {
	r.RequestHeaders[key] = value
}
func (r *HttpRecord) AddRequestBody(body []byte) {
	if r.RequestBody == nil {
		r.RequestBody = []byte{}
	}
	// 追加数据
	// 这里使用 append 追加数据
	r.RequestBody = append(r.RequestBody, body...)

}
func (r *HttpRecord) AddResponseHeader(key, value string) {
	r.ResponseHeaders[key] = value
}
func (r *HttpRecord) AddResponseBody(body []byte) {
	if r.ResponseBody == nil {
		r.ResponseBody = []byte{}
	}
	// 追加数据
	r.ResponseBody = append(r.ResponseBody, body...)
}

func (r *HttpRecord) Base64RequestBody() string {
	if r.RequestBody == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(r.RequestBody)
}

func (r *HttpRecord) Base64ResponseBody() string {
	if r.ResponseBody == nil {
		return ""
	}

	// 将 ResponseBody 转换为字符串
	bodyStr := string(r.ResponseBody)

	// 检查是否为有效的 Base64 编码
	// 1. 去除首尾空格
	// 2. 检查长度是否为 4 的倍数（Base64 编码长度通常是 4 的倍数）
	// 3. 尝试解码，检查是否有错误
	trimmedBody := strings.TrimSpace(bodyStr)
	if len(trimmedBody)%4 == 0 {
		// 尝试解码
		_, err := base64.StdEncoding.DecodeString(trimmedBody)
		if err == nil {
			// 如果解码成功，返回解码后的字符串
			decoded, _ := base64.StdEncoding.DecodeString(trimmedBody)
			return string(decoded)
		}
	}

	// 如果不是 Base64 编码或解码失败，原样返回
	return bodyStr
}
