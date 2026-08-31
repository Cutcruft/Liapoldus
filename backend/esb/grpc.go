package esb

import (
	"context"
	"errors"

	esbpb "github.com/liapoldus/liapoldus/backend/gen/esb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcServer struct {
	esbpb.UnimplementedEsbServer
	registry *Registry
}

// RegisterGRPC attaches the extension transport to an existing gRPC server.
// ManagementService and Esb are registered independently on the same process.
func RegisterGRPC(server *grpc.Server, registry *Registry) {
	if registry == nil {
		registry = NewRegistry()
	}
	esbpb.RegisterEsbServer(server, &grpcServer{registry: registry})
}

func (s *grpcServer) Call(ctx context.Context, request *esbpb.EsbRequest) (*esbpb.EsbReply, error) {
	reply, err := s.registry.callOperation(ctx, fromProtoRequest(request))
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoReply(reply), nil
}

func (s *grpcServer) Stream(request *esbpb.EsbRequest, stream esbpb.Esb_StreamServer) error {
	err := s.registry.streamOperation(stream.Context(), fromProtoRequest(request), func(reply Reply) error {
		return stream.Send(toProtoReply(reply))
	})
	if err != nil {
		return mapError(err)
	}
	return nil
}

func fromProtoRequest(request *esbpb.EsbRequest) Request {
	if request == nil {
		return Request{}
	}
	return Request{Payload: request.GetPayload(), Metadata: cloneMetadata(request.GetMetadata())}
}

func toProtoReply(reply Reply) *esbpb.EsbReply {
	return &esbpb.EsbReply{Payload: reply.Payload, Metadata: cloneMetadata(reply.Metadata)}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrOperationNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, ErrOperationExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, ErrPayloadTooLarge):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
