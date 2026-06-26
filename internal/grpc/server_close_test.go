package grpc

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type blockingServiceServer interface {
	Slow(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
}

type blockingService struct {
	enterOnce sync.Once
	entered   chan struct{}
	release   chan struct{}
}

func (s *blockingService) Slow(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	s.enterOnce.Do(func() {
		close(s.entered)
	})
	select {
	case <-s.release:
		return &emptypb.Empty{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var blockingServiceDesc = grpc.ServiceDesc{
	ServiceName: "test.BlockingService",
	HandlerType: (*blockingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Slow",
			Handler: func(
				srv interface{},
				ctx context.Context,
				dec func(interface{}) error,
				interceptor grpc.UnaryServerInterceptor,
			) (interface{}, error) {
				in := new(emptypb.Empty)
				if err := dec(in); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return srv.(blockingServiceServer).Slow(ctx, in)
				}
				info := &grpc.UnaryServerInfo{
					Server:     srv,
					FullMethod: "/test.BlockingService/Slow",
				}
				handler := func(ctx context.Context, req interface{}) (interface{}, error) {
					return srv.(blockingServiceServer).Slow(ctx, req.(*emptypb.Empty))
				}
				return interceptor(ctx, in, info, handler)
			},
		},
	},
}

func startBlockingServer(t *testing.T, svc *blockingService) (*Server, *grpc.ClientConn) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	gs := grpc.NewServer()
	gs.RegisterService(&blockingServiceDesc, svc)
	go func() {
		_ = gs.Serve(listener)
	}()

	dialCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(
		dialCtx,
		listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return &Server{server: gs, listener: listener}, conn
}

func TestServerCloseGracefulStopWaitsInFlightRPC(t *testing.T) {
	svc := &blockingService{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	s, conn := startBlockingServer(t, svc)

	callErrCh := make(chan error, 1)
	go func() {
		callErrCh <- conn.Invoke(context.Background(), "/test.BlockingService/Slow", &emptypb.Empty{}, &emptypb.Empty{})
	}()

	select {
	case <-svc.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("rpc did not start in time")
	}

	closeErrCh := make(chan error, 1)
	go func() {
		closeErrCh <- s.Close()
	}()

	select {
	case err := <-closeErrCh:
		t.Fatalf("Close returned early while request is in-flight: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(svc.release)

	select {
	case err := <-callErrCh:
		if err != nil {
			t.Fatalf("rpc returned error under graceful shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting rpc result")
	}

	select {
	case err := <-closeErrCh:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting Close")
	}
}

func TestServerCloseFallsBackToStopAfterTimeout(t *testing.T) {
	old := gracefulStopTimeout
	gracefulStopTimeout = 120 * time.Millisecond
	t.Cleanup(func() {
		gracefulStopTimeout = old
	})

	svc := &blockingService{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	s, conn := startBlockingServer(t, svc)

	callErrCh := make(chan error, 1)
	go func() {
		callErrCh <- conn.Invoke(context.Background(), "/test.BlockingService/Slow", &emptypb.Empty{}, &emptypb.Empty{})
	}()

	select {
	case <-svc.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("rpc did not start in time")
	}

	closeErrCh := make(chan error, 1)
	go func() {
		closeErrCh <- s.Close()
	}()

	select {
	case err := <-closeErrCh:
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after graceful timeout fallback")
	}

	select {
	case err := <-callErrCh:
		if err == nil {
			t.Fatal("expected rpc to be interrupted by forced Stop")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting rpc interruption")
	}
}
