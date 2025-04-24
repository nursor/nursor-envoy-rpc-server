package main

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/wrapperspb"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

type extProcServer struct {
	extprocv3.UnimplementedExternalProcessorServer
}

func (s *extProcServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	log.Println("New stream from Envoy")
	timeA := time.Now()
	defer func() {
		log.Printf("Stream closed after %s", time.Since(timeA))
	}()
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			log.Println("Stream closed by client")
			return nil
		}
		if err != nil {
			log.Printf("Error receiving from stream: %v", err)
			return err
		}

		switch r := req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			log.Println("Received headers")

			headers := r.RequestHeaders.GetHeaders()
			isAuthHeaderExisted := false

			for _, h := range headers.Headers {
				if strings.ToLower(h.Key) == "authorization" {
					isAuthHeaderExisted = true
					log.Println("Authorization header found and replaced")
					// orgAuth := string(h.RawValue)
					// userService := service.GetUserServiceInstance()
					// ctx := stream.Context()
					// isValid, err := userService.ParseRequestToken(ctx, orgAuth)
					// if err != nil {
					// 	log.Printf("Error parsing token: %v", err)
					// 	return err
					// }
					// if isValid {
					// 	log.Printf("Token is valid: %s", orgAuth)
					// }
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
												RawValue: []byte("Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhdXRoMHx1c2VyXzAxSlJCWUhXOTMyQTBUOEYyM0ZQUFpFMDhaIiwidGltZSI6IjE3NDU0NzMzNzEiLCJyYW5kb21uZXNzIjoiNjJkYzFmMjQtYmI2MS00YWUxIiwiZXhwIjoxNzUwNjU3MzcxLCJpc3MiOiJodHRwczovL2F1dGhlbnRpY2F0aW9uLmN1cnNvci5zaCIsInNjb3BlIjoib3BlbmlkIHByb2ZpbGUgZW1haWwgb2ZmbGluZV9hY2Nlc3MiLCJhdWQiOiJodHRwczovL2N1cnNvci5jb20ifQ.MLmGo_4kPsGOEqwl0VE3hi2RGSnSZwbE3hsMBkGDIes"),
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
			log.Println("Received body")
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
