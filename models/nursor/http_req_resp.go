package nursor

import (
	"net/url"
	"time"
)

// type HttpRecordToInsert struct {
// 	RequestHeaders  map[string]string `json:"request_headers"`
// 	RequestBody     string            `json:"request_body"`
// 	ResponseHeaders map[string]string `json:"response_headers"`
// 	ResponseBody    string            `json:"response_body"`
// 	Url             string            `json:"url"`
// 	Method          string            `json:"method"`
// 	Host            string            `json:"host"`
// 	Datetime        string            `json:"datetime"`
// 	HttpVersion     string            `json:"http_version"`
// }

type HttpRecord struct {
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     []byte            `json:"request_body"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    []byte            `json:"response_body"`
	Url             *url.URL          `json:"url"`
	Method          string            `json:"method"`
	Host            string            `json:"host"`
	Datetime        string            `json:"datetime"`
	HttpVersion     string            `json:"http_version"`
	InnerToken      string            `json:"inner_token"`
}

func NewRequestRecord() *HttpRecord {
	return &HttpRecord{
		RequestHeaders:  map[string]string{},
		RequestBody:     []byte{},
		ResponseHeaders: map[string]string{},
		ResponseBody:    []byte{},
		Url:             &url.URL{},
		Method:          "Post",
		Host:            "cursor.sh",
		Datetime:        time.Now().Format("2006-01-02 15:04:05"),
		HttpVersion:     "http/1.1",
		InnerToken:      "",
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

// func (r *HttpRecord) ToInsertModel() *HttpRecordToInsert {
// 	return &HttpRecordToInsert{
// 		RequestHeaders:  r.RequestHeaders,
// 		RequestBody:     string(r.RequestBody),
// 		ResponseHeaders: r.ResponseHeaders,
// 		ResponseBody:    string(r.ResponseBody),
// 		Url:             r.Url.String(),
// 		Method:          r.Method,
// 		Host:            r.Host,
// 		Datetime:        r.Datetime,
// 		HttpVersion:     r.HttpVersion,
// 	}
// }
