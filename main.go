package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"nursor-envoy-rpc/models"
	"nursor-envoy-rpc/models/nursor"

	"nursor-envoy-rpc/service"
	"nursor-envoy-rpc/utils"
	"strconv"
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
	var isChatHasException = false
	timeA := time.Now()
	defer func() {
		// 异步处理
		go func() {
			log.Printf("Stream closed after %s", time.Since(timeA))
			if httpRecrod != nil {
				// Push HTTP record to external service
				httpRecordService := service.GetHttpRecordInstance()
				if err := httpRecordService.PushHttpRecord(context.Background(), httpRecrod); err != nil {
					log.Printf("Failed to push HTTP record: %v", err)
				}
			}
			if isChatRequest {
				if !isChatHasException {
					dispatcherService := service.GetDispatchInstance()
					dispatcherService.IncrTokenUsage(context.Background(), httpRecrod.AccountId)
				} else {
					dispatcherService := service.GetDispatchInstance()
					dispatcherService.HandleTokenExpired(context.Background(), httpRecrod.AccountId)
				}

			}
		}()
	}()

	var account *models.AccountInfo

	for {
		req, err := stream.Recv()
		ctx := stream.Context()
		// ctx := context.Background()
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
		var user *models.User
		var innerToken string

		switch r := req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			log.Println("Received request headers")
			headers := r.RequestHeaders.GetHeaders()
			isAuthHeaderExisted := false

			// 从headers中提取nursor-token

			for _, h := range headers.Headers {
				if strings.ToLower(h.Key) == "nursor-token" {
					innerToken = string(h.RawValue)
					userService := service.GetUserServiceInstance()
					user, err = userService.GetUserByInnerToken(ctx, innerToken)
					if err != nil {
						log.Printf("Error getting user by inner token: %v", err)
						return err
					}
					httpRecrod.UserId = user.ID
					log.Printf("Found and set nursor-token: %s", innerToken)
					break
				}
			}
			if user == nil {
				log.Println("User not found")
				resp := &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_ImmediateResponse{
						ImmediateResponse: &extprocv3.ImmediateResponse{},
					},
				}
				return stream.Send(resp)
			}

			for _, h := range headers.Headers {
				httpRecrod.AddRequestHeader(h.Key, string(h.RawValue))
				if strings.Contains(h.Key, ":authority") {
					httpRecrod.HttpVersion = "http/2.0"
					if strings.Contains(string(h.RawValue), "metrics.cursor.sh") {
						resp := &extprocv3.ProcessingResponse{
							Response: &extprocv3.ProcessingResponse_ImmediateResponse{
								ImmediateResponse: &extprocv3.ImmediateResponse{},
							},
						}

						if err := stream.Send(resp); err != nil {
							log.Printf("Error sending response: %v", err)
							return err
						}
						return nil
					} else if !strings.Contains(string(h.RawValue), "cursor.sh") && !strings.Contains(string(h.RawValue), "cursor.com") {
						// log.Println("not cursor.sh or cursor.com", string(h.RawValue))
						// 只处理cursor.sh和cursor.com的请求
						resp := &extprocv3.ProcessingResponse{
							Response: &extprocv3.ProcessingResponse_RequestHeaders{
								RequestHeaders: &extprocv3.HeadersResponse{
									Response: &extprocv3.CommonResponse{
										HeaderMutation: &extprocv3.HeaderMutation{},
									},
								},
							},
						}

						if err := stream.Send(resp); err != nil {
							log.Printf("Error sending response: %v", err)
							return err
						}
						return nil
					}

				} else if strings.Contains(h.Key, ":path") && strings.Contains(string(h.RawValue), "AuthService/GetEmail") {
					resp := &extprocv3.ProcessingResponse{
						Response: &extprocv3.ProcessingResponse_ImmediateResponse{
							ImmediateResponse: &extprocv3.ImmediateResponse{
								Body: string([]byte{
									0x0a, 0x10, // 前两个字节
									0x6a, 0x69, 0x6d, 0x6d, 0x79, 0x6c, 0x65, 0x65, // jimmylee
									0x40,                                     // @
									0x6d, 0x69, 0x74, 0x2e, 0x65, 0x64, 0x75, // mit.edu
									0x10, 0x01, // 后两个字节
								}),
							},
						},
					}

					if err := stream.Send(resp); err != nil {
						log.Printf("Error sending response: %v", err)
						return err
					}
					return nil
				} else if strings.Contains(h.Key, ":path") && strings.Contains(string(h.RawValue), "GetTeam") {
					fmt.Print("in get Eamil")
				} else if strings.Contains(h.Key, ":path") && strings.Contains(string(h.RawValue), "ReportBug") {
					resp := &extprocv3.ProcessingResponse{
						Response: &extprocv3.ProcessingResponse_ImmediateResponse{
							ImmediateResponse: &extprocv3.ImmediateResponse{},
						},
					}

					if err := stream.Send(resp); err != nil {
						log.Printf("Error sending response: %v", err)
						return err
					}
					return nil
				}

				if strings.ToLower(h.Key) == "authorization" && strings.Contains(string(h.RawValue), ".") {
					isAuthHeaderExisted = true
					log.Println("Authorization header found and replaced")

					userService := service.GetUserServiceInstance()
					var userInfo *models.User
					// userInfo, err := userService.CheckAndGetUserFromInnerToken(ctx, orgAuth)
					// 新版本，使用用户数据库的绑定token
					if innerToken != "" {
						userInfo, err = userService.GetUserByInnerToken(ctx, innerToken)
						if err != nil {
							log.Printf("Error getting user by inner token: %v", err)
							resp := utils.GetResponseForErr(err)
							// 发送响应，终止流程
							if err := stream.Send(resp); err != nil {
								log.Printf("Failed to send immediate response: %v", err)
							}
							return err
						}
					}

					dispatcherService := service.GetDispatchInstance()
					account, err = dispatcherService.GetAccountByUserId(ctx, userInfo.ID)
					if err != nil || account == nil {
						log.Printf("Error dispatching token: %v", err)
						resp := utils.GetResponseForErr(err)
						// 发送响应，终止流程
						if err := stream.Send(resp); err != nil {
							log.Printf("Failed to send immediate response: %v", err)
						}
						return err
					}
					// Set account ID in HTTP record
					httpRecrod.AccountId = account.ID
				}
				// 聊天请求单独处理
				if strings.Contains(string(h.RawValue), "StreamUnifiedChatWithTools") {
					isChatRequest = true
				}

				switch h.Key {
				case ":method":
					httpRecrod.Method = string(h.RawValue) // e.g., "POST"
				case ":authority":
					httpRecrod.Host = string(h.RawValue) // e.g., "cursor.sh"
				case ":path":
					// :path 包含路径和查询参数，需拼接 scheme 和 host 构成完整 URL
					scheme := httpRecrod.RequestHeaders[":scheme"] // e.g., "http" or "https"
					if scheme == "" {
						scheme = "http" // 默认值
					}
					httpRecrod.Url = scheme + "://" + httpRecrod.Host + string(h.RawValue) // e.g., "http://cursor.sh/path?query"
				case ":scheme":
					// 用于 URL 拼接，单独处理
					httpRecrod.AddRequestHeader(h.Key, string(h.RawValue))
				default:
					httpRecrod.RequestHeaders[h.Key] = string(h.RawValue)
				}
			}

			if !isAuthHeaderExisted {
				log.Println("Authorization header not present")
				resp := &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_RequestHeaders{
						RequestHeaders: &extprocv3.HeadersResponse{
							Response: &extprocv3.CommonResponse{
								HeaderMutation: &extprocv3.HeaderMutation{
									RemoveHeaders: []string{"nursor-token"},
								},
							},
						},
					},
				}

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
									RemoveHeaders: []string{"authorization", "nursor-token"},
									SetHeaders: []*corev3.HeaderValueOption{
										{
											Header: &corev3.HeaderValue{
												Key:      "authorization",
												RawValue: []byte(fmt.Sprintf("Bearer %s", account.AccessToken)),
											},
											// TODO： 是不是还需要修改x-cleint-id字段？
											Append: wrapperspb.Bool(false),
										},
										{
											Header: &corev3.HeaderValue{
												Key:      "x-client-key",
												RawValue: []byte(account.ClientKey),
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
			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_RequestBody{
					RequestBody: &extprocv3.BodyResponse{
						Response: &extprocv3.CommonResponse{},
					},
				},
			}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		case *extprocv3.ProcessingRequest_ResponseHeaders:
			log.Println("Received response headers")
			headers := r.ResponseHeaders.GetHeaders()
			for _, h := range headers.Headers {
				httpRecrod.AddResponseHeader(h.Key, string(h.RawValue))
				if strings.ToLower(h.Key) == ":status" {
					respStatus := string(h.RawValue)
					respStatusInt, err := strconv.Atoi(respStatus)
					if err != nil {
						log.Printf("Error converting response status to int: %v", err)
					}
					if respStatusInt >= 400 {
						isChatHasException = true
					}
				}
			}
			var resp *extprocv3.ProcessingResponse
			if !isChatHasException {
				resp = &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_ResponseHeaders{
						ResponseHeaders: &extprocv3.HeadersResponse{
							Response: &extprocv3.CommonResponse{
								// HeaderMutation: &extprocv3.HeaderMutation{
								// 	RemoveHeaders: []string{"authorization"},
								// 	SetHeaders:    []*corev3.HeaderValueOption{},
								// },
							},
						},
					},
				}

			} else {
				resp = &extprocv3.ProcessingResponse{
					Response: &extprocv3.ProcessingResponse_ImmediateResponse{
						ImmediateResponse: &extprocv3.ImmediateResponse{
							// Status: &tv3.HttpStatus{
							// 	Code: 567,
							// },
						},
					},
				}
			}
			if err := stream.Send(resp); err != nil {
				log.Printf("Error sending response: %v", err)
				return err
			}
		case *extprocv3.ProcessingRequest_ResponseBody:
			log.Println("Received response body")
			body := r.ResponseBody.GetBody()
			httpRecrod.AddResponseBody(body)
			var bodyMutation *extprocv3.BodyMutation
			// TODO: 需要优化
			if strings.Contains(string(body), "resource_exhausted") || isChatHasException {
				fmt.Println("resource_exhausted")
				bodyMutation = &extprocv3.BodyMutation{
					Mutation: &extprocv3.BodyMutation_Body{
						Body: []byte(`1`),
					},
				}
			}
			resp := &extprocv3.ProcessingResponse{
				Response: &extprocv3.ProcessingResponse_ResponseBody{
					ResponseBody: &extprocv3.BodyResponse{
						Response: &extprocv3.CommonResponse{
							BodyMutation: bodyMutation,
						},
					},
				},
			}
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
