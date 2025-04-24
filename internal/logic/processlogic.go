package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"

	"nursor-envoy-rpc/internal/svc"
	"nursor-envoy-rpc/protobuf/extproc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProcessLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	flowRecords []FlowRecord // 存储 HTTP 流记录
}

// FlowRecord 保存完整的 HTTP 流
type FlowRecord struct {
	RequestHeaders  map[string]string `json:"request_headers"`
	RequestBody     string            `json:"request_body"`
	ResponseHeaders map[string]string `json:"response_headers"`
	ResponseBody    string            `json:"response_body"`
}

func NewProcessLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProcessLogic {
	return &ProcessLogic{
		ctx:         ctx,
		svcCtx:      svcCtx,
		Logger:      logx.WithContext(ctx),
		flowRecords: []FlowRecord{}, // 初始化流记录
	}
}

// 双向流式 RPC，处理 HTTP 请求和响应
func (l *ProcessLogic) Process(stream extproc.ExternalProcessor_ProcessServer) error {
	var currentRecord FlowRecord
	currentRecord.RequestHeaders = make(map[string]string)
	currentRecord.ResponseHeaders = make(map[string]string)
	var requestBodyBuffer, responseBodyBuffer bytes.Buffer

	for {
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				// 流结束，保存记录
				currentRecord.RequestBody = requestBodyBuffer.String()
				currentRecord.ResponseBody = responseBodyBuffer.String()
				l.flowRecords = append(l.flowRecords, currentRecord)
				if err := l.saveRecords(); err != nil {
					log.Printf("Failed to save records: %v", err)
				}
				return nil
			}
			return err
		}

		resp := &extproc.ProcessingResponse{}

		switch req.Request.(type) {
		case *extproc.ProcessingRequest_RequestHeaders:
			// 处理请求 header
			headers := req.GetRequestHeaders().GetHeaders()
			for _, header := range headers {
				currentRecord.RequestHeaders[header.Key] = header.Value
			}

			// 检查并替换 Authorization 头部
			if auth, exists := currentRecord.RequestHeaders["authorization"]; exists {
				logx.Info("Authorization header found:", auth)
				resp = &extproc.ProcessingResponse{
					Response: &extproc.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extproc.HeadersResponse{
							Response: &extproc.CommonResponse{
								HeaderMutation: &extproc.HeaderMutation{
									SetHeaders: []*extproc.HeaderValueOption{
										{
											Header: &extproc.HeaderValue{
												Key:   "authorization",
												Value: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhdXRoMHx1c2VyXzAxSlJCWUhXOTMyQTBUOEYyM0ZQUFpFMDhaIiwidGltZSI6IjE3NDU0NzMzNzEiLCJyYW5kb21uZXNzIjoiNjJkYzFmMjQtYmI2MS00YWUxIiwiZXhwIjoxNzUwNjU3MzcxLCJpc3MiOiJodHRwczovL2F1dGhlbnRpY2F0aW9uLmN1cnNvci5zaCIsInNjb3BlIjoib3BlbmlkIHByb2ZpbGUgZW1haWwgb2ZmbGluZV9hY2Nlc3MiLCJhdWQiOiJodHRwczovL2N1cnNvci5jb20ifQ.MLmGo_4kPsGOEqwl0VE3hi2RGSnSZwbE3hsMBkGDIes",
											},
										},
									},
								},
							},
						},
					},
				}
			}

		case *extproc.ProcessingRequest_RequestBody:
			// 处理请求 body（分块）
			body := req.GetRequestBody()
			if body.Body != nil {
				requestBodyBuffer.Write(body.Body)
			}
			if body.EndOfStream {
				currentRecord.RequestBody = requestBodyBuffer.String()
			}

		case *extproc.ProcessingRequest_ResponseHeaders:
			// 处理响应 header
			headers := req.GetResponseHeaders().GetHeaders()
			for _, header := range headers {
				currentRecord.ResponseHeaders[header.Key] = header.Value
			}

		case *extproc.ProcessingRequest_ResponseBody:
			// 处理响应 body（分块）
			body := req.GetResponseBody()
			if body.Body != nil {
				responseBodyBuffer.Write(body.Body)
			}
			if body.EndOfStream {
				currentRecord.ResponseBody = responseBodyBuffer.String()
			}
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}

// saveRecords 将 HTTP 流记录保存到文件（可替换为 Kafka）
func (l *ProcessLogic) saveRecords() error {
	file, err := os.OpenFile("http_flows.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, record := range l.flowRecords {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	l.flowRecords = nil // 清空记录
	return nil
}
