package esb

import (
	"context"
	"io"
	"net"
	"testing"

	esbpb "github.com/liapoldus/liapoldus/backend/gen/esb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestCallAndStream(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("test", "echo", Operation{
		Call: func(_ context.Context, request Request) (Reply, error) {
			return Reply{Payload: append([]byte("reply:"), request.Payload...), Metadata: map[string]string{"handled_by": "echo"}}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("test", "events", Operation{
		Stream: func(_ context.Context, request Request, send func(Reply) error) error {
			if err := send(Reply{Payload: append([]byte("one:"), request.Payload...)}); err != nil {
				return err
			}
			return send(Reply{Payload: append([]byte("two:"), request.Payload...)})
		},
	}); err != nil {
		t.Fatal(err)
	}

	server := grpc.NewServer()
	RegisterGRPC(server, registry)
	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := esbpb.NewEsbClient(conn)
	request := &esbpb.EsbRequest{Payload: []byte("hello"), Metadata: map[string]string{
		"service": "test", "method": "echo", "content_type": "text/plain", "correlation_id": "corr-1",
	}}

	callReply, err := client.Call(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if string(callReply.GetPayload()) != "reply:hello" || callReply.GetMetadata()["correlation_id"] != "corr-1" {
		t.Fatalf("call reply = %q %#v", callReply.GetPayload(), callReply.GetMetadata())
	}

	stream, err := client.Stream(context.Background(), &esbpb.EsbRequest{Payload: []byte("hello"), Metadata: map[string]string{
		"service": "test", "method": "events", "content_type": "text/plain", "correlation_id": "corr-2",
	}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := stream.Recv()
	if err != nil || string(first.GetPayload()) != "one:hello" || first.GetMetadata()["correlation_id"] != "corr-2" {
		t.Fatalf("first stream reply = %q %#v, error = %v", first.GetPayload(), first.GetMetadata(), err)
	}
	second, err := stream.Recv()
	if err != nil || string(second.GetPayload()) != "two:hello" {
		t.Fatalf("second stream reply = %q, error = %v", second.GetPayload(), err)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("stream final error = %v, want io.EOF", err)
	}
}

func TestUnknownOperationReturnsNotFound(t *testing.T) {
	server := grpc.NewServer()
	RegisterGRPC(server, NewRegistry())
	listener := bufconn.Listen(1024 * 1024)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = esbpb.NewEsbClient(conn).Call(context.Background(), &esbpb.EsbRequest{Metadata: map[string]string{
		"service": "missing", "method": "operation", "content_type": "application/json",
	}})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.NotFound)
	}
}
