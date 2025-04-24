package service

import (
	"errors"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	v32 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetResponseForErr(err error) *extprocv3.ProcessingResponse {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &extprocv3.ProcessingResponse{
			Response: &extprocv3.ProcessingResponse_ImmediateResponse{
				ImmediateResponse: &extprocv3.ImmediateResponse{
					Status: &v32.HttpStatus{
						Code: v32.StatusCode_Unauthorized,
					},
					Body: "Invalid token: access denied",
					Headers: &extprocv3.HeaderMutation{
						SetHeaders: []*corev3.HeaderValueOption{
							{
								Header: &corev3.HeaderValue{
									Key:      "Content-Type",
									RawValue: []byte("text/plain"),
								},
							},
						},
					},
				},
			},
		}

	}
	logrus.Error(err)
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &v32.HttpStatus{
					Code: v32.StatusCode_Unauthorized,
				},
				Body: "Invalid token: access denied",
				Headers: &extprocv3.HeaderMutation{
					SetHeaders: []*corev3.HeaderValueOption{
						{
							Header: &corev3.HeaderValue{
								Key:      "Content-Type",
								RawValue: []byte("text/plain"),
							},
						},
					},
				},
			},
		},
	}

}

func GetExpireError() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_ImmediateResponse{
			ImmediateResponse: &extprocv3.ImmediateResponse{
				Status: &v32.HttpStatus{
					Code: v32.StatusCode_Unauthorized,
				},
				Body: "Token Expired",
				Headers: &extprocv3.HeaderMutation{
					SetHeaders: []*corev3.HeaderValueOption{
						{
							Header: &corev3.HeaderValue{
								Key:      "Content-Type",
								RawValue: []byte("text/plain"),
							},
						},
					},
				},
			},
		},
	}

}
