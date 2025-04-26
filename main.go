package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"nursor-envoy-rpc/models/nursor"
	"nursor-envoy-rpc/provider"
	"nursor-envoy-rpc/service"
	"nursor-envoy-rpc/utils"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

type extProcServer struct {
	extprocv3.UnimplementedExternalProcessorServer
}

func (s *extProcServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	var httpRecrod = nursor.NewRequestRecord()
	var isChatRequest = false
	timeA := time.Now()
	defer func() {
		log.Printf("Stream closed after %s", time.Since(timeA))
		if httpRecrod != nil {
			go provider.PushHttpRequestToDB(httpRecrod)
		}
		if isChatRequest {
			go service.GetTokenServiceInstance().IncrementTokenUsage(stream.Context(), httpRecrod.InnerToken)
		}
	}()

	var cursorToken = ""

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by client")
			return nil
		}
		if err != nil {
			if status.Code(err) == codes.Canceled {
				log.Println("Stream closed by envoy")
				return nil
			}
			log.Printf("Error receiving from stream: %v", err)
			return err
		}

		switch r := req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			log.Println("Received request headers")
			headers := r.RequestHeaders.GetHeaders()
			isAuthHeaderExisted := false

			for _, h := range headers.Headers {
				httpRecrod.AddRequestHeader(h.Key, string(h.RawValue))
				if strings.ToLower(h.Key) == "authorization" && strings.Contains(string(h.RawValue), ".") {
					isAuthHeaderExisted = true
					log.Println("Authorization header found and replaced")
					orgAuth := string(h.RawValue)
					userService := service.GetUserServiceInstance()
					ctx := stream.Context()
					userInfo, err := userService.CheckAndGetUserFromInnerToken(ctx, orgAuth)
					if err != nil {
						log.Printf("Error parsing token: %v", err)
						resp := utils.GetResponseForErr(err)
						// 发送响应，终止流程
						if err := stream.Send(resp); err != nil {
							log.Printf("Failed to send immediate response: %v", err)
						}
					}
					if userInfo == nil {
						log.Println("Token not valid")
						resp := utils.GetResponseForExpireError()
						// 发送响应，终止流程
						if err := stream.Send(resp); err != nil {
							log.Printf("Failed to send immediate response: %v", err)
						}
					} else {
						httpRecrod.InnerToken = userInfo.InnerToken
					}

					cursorToken, err = service.GetDispatchInstance().DispatchTokenForNewUser(ctx, fmt.Sprint(userInfo.ID))
					if err != nil {
						log.Printf("Error dispatching token: %v", err)
						resp := utils.GetResponseForErr(err)
						// 发送响应，终止流程
						if err := stream.Send(resp); err != nil {
							log.Printf("Failed to send immediate response: %v", err)
						}
					}

					log.Printf("Token valid: %s\n", orgAuth)
				}
				if strings.Contains(string(h.RawValue), "chat") {
					isChatRequest = true
				}
			}

			if !isAuthHeaderExisted {
				log.Println("Authorization header not present")
				resp := &extprocv3.ProcessingResponse{}
				if err := stream.Send(resp); err != nil {
					log.Printf("Error sending response: %v", err)
					return err
				}
			} else {
				resp := &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extprocv3.HeadersResponse{
							Response: &extprocv3.CommonResponse{
								HeaderMutation: &extprocv3.HeaderMutation{
									RemoveHeaders: []string{"authorization"},
									SetHeaders: []*corev3.HeaderValueOption{
										{
											Header: &corev3.HeaderValue{
												Key:      "authorization",
												RawValue: []byte(fmt.Sprintf("Bearer %s", cursorToken)),
											},
											Append: wrapperspb.Bool(false),
										},
									},
								},
							},
						},
					},
				}
				if err := stream.Send(resp); err != nil {
					log.Printf("Error sending response: %v", err)
					return err
				}
				log.Println("Authorization header replaced")
			}
		case *extprocv3.ProcessingRequest_RequestBody:
			log.Println("Received request body")
			body := r.RequestBody.GetBody()
			httpRecrod.AddRequestBody(body)
			resp := &extprocv3.ProcessingResponse{}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		case *extprocv3.ProcessingRequest_ResponseHeaders:
			log.Println("Received response headers")
			headers := r.ResponseHeaders.GetHeaders()
			for _, h := range headers.Headers {
				httpRecrod.AddResponseHeader(h.Key, string(h.RawValue))
			}
			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseHeaders{
					ResponseHeaders: &extprocv3.HeadersResponse{
						Response: &extprocv3.CommonResponse{
							HeaderMutation: &extprocv3.HeaderMutation{
								RemoveHeaders: []string{"authorization"},
								SetHeaders:    []*corev3.HeaderValueOption{},
							},
						},
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		case *extprocv3.ProcessingRequest_ResponseBody:
			log.Println("Received response body")
			body := r.ResponseBody.GetBody()
			httpRecrod.AddResponseBody(body)
			resp := &extprocv3.ProcessingResponse{}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		default:
			// 其他阶段暂不处理
			log.Printf("Unhandled request type: %T (raw: %+v)", r, req)
			resp := &extprocv3.ProcessingResponse{}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		}
	}
}

func main() {
	listenAddr := ":8080"
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", listenAddr, err)
	}

	s := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(s, &extProcServer{})
	reflection.Register(s)

	log.Printf("Starting ext_proc gRPC server on %s...\n", listenAddr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
