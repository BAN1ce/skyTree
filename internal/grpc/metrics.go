package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/pkg/metric"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func metricsUnaryServerInterceptor() grpcpkg.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpcpkg.UnaryServerInfo,
		handler grpcpkg.UnaryHandler,
	) (resp interface{}, err error) {
		service, method := splitFullMethod(info.FullMethod)
		startTime := time.Now()
		finishInflight := metric.BeginGRPCServerRequest(service, method)
		requestBytes := protoMessageSize(req)
		defer func() {
			finishInflight()
			metric.RecordGRPCServerRequest(
				service,
				method,
				status.Code(err),
				err,
				time.Since(startTime),
				requestBytes,
				protoMessageSize(resp),
			)
		}()
		return handler(ctx, req)
	}
}

func splitFullMethod(fullMethod string) (string, string) {
	trimmed := strings.TrimPrefix(fullMethod, "/")
	if trimmed == "" {
		return "unknown", "unknown"
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return trimmed, "unknown"
	}
	serviceParts := strings.Split(parts[0], ".")
	service := serviceParts[len(serviceParts)-1]
	if service == "" {
		service = "unknown"
	}
	method := parts[1]
	if method == "" {
		method = "unknown"
	}
	return service, method
}

func protoMessageSize(v interface{}) int {
	msg, ok := v.(proto.Message)
	if !ok || msg == nil {
		return 0
	}
	return proto.Size(msg)
}
