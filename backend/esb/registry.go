package esb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	ErrInvalidRequest    = errors.New("invalid ESB request")
	ErrOperationNotFound = errors.New("ESB operation not found")
	ErrOperationExists   = errors.New("ESB operation already registered")
	ErrPayloadTooLarge   = errors.New("ESB payload is too large")
)

const DefaultMaxPayloadSize = 4 << 20

type Request struct {
	Payload  []byte
	Metadata map[string]string
}

type Reply struct {
	Payload  []byte
	Metadata map[string]string
}

type CallHandler func(context.Context, Request) (Reply, error)
type StreamHandler func(context.Context, Request, func(Reply) error) error

type Operation struct {
	Call   CallHandler
	Stream StreamHandler
}

type Registry struct {
	maxPayloadSize int
	operations     map[string]Operation
	mu             sync.RWMutex
}

func NewRegistry(maxPayloadSize ...int) *Registry {
	limit := DefaultMaxPayloadSize
	if len(maxPayloadSize) > 0 && maxPayloadSize[0] > 0 {
		limit = maxPayloadSize[0]
	}
	return &Registry{maxPayloadSize: limit, operations: make(map[string]Operation)}
}

func (r *Registry) Register(service, method string, operation Operation) error {
	service, method = strings.TrimSpace(service), strings.TrimSpace(method)
	if service == "" || method == "" {
		return fmt.Errorf("%w: service and method are required", ErrInvalidRequest)
	}
	if operation.Call == nil && operation.Stream == nil {
		return fmt.Errorf("%w: call or stream handler is required", ErrInvalidRequest)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	key := operationKey(service, method)
	if _, exists := r.operations[key]; exists {
		return fmt.Errorf("%w: %s/%s", ErrOperationExists, service, method)
	}
	r.operations[key] = operation
	return nil
}

func (r *Registry) call(service, method string) (Operation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	operation, exists := r.operations[operationKey(service, method)]
	if !exists {
		return Operation{}, fmt.Errorf("%w: %s/%s", ErrOperationNotFound, service, method)
	}
	return operation, nil
}

func (r *Registry) validate(request Request) error {
	if strings.TrimSpace(request.Metadata["service"]) == "" || strings.TrimSpace(request.Metadata["method"]) == "" {
		return fmt.Errorf("%w: metadata service and method are required", ErrInvalidRequest)
	}
	if strings.TrimSpace(request.Metadata["content_type"]) == "" {
		return fmt.Errorf("%w: metadata content_type is required", ErrInvalidRequest)
	}
	if len(request.Payload) > r.maxPayloadSize {
		return fmt.Errorf("%w: limit is %d bytes", ErrPayloadTooLarge, r.maxPayloadSize)
	}
	return nil
}

func (r *Registry) callOperation(ctx context.Context, request Request) (Reply, error) {
	if err := r.validate(request); err != nil {
		return Reply{}, err
	}
	operation, err := r.call(request.Metadata["service"], request.Metadata["method"])
	if err != nil {
		return Reply{}, err
	}
	if operation.Call == nil {
		return Reply{}, fmt.Errorf("%w: operation only supports Stream", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return Reply{}, err
	}
	reply, err := operation.Call(ctx, request)
	if err != nil {
		return Reply{}, err
	}
	return withCorrelation(request, reply), nil
}

func (r *Registry) streamOperation(ctx context.Context, request Request, send func(Reply) error) error {
	if err := r.validate(request); err != nil {
		return err
	}
	operation, err := r.call(request.Metadata["service"], request.Metadata["method"])
	if err != nil {
		return err
	}
	if operation.Stream == nil {
		return fmt.Errorf("%w: operation only supports Call", ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return operation.Stream(ctx, request, func(reply Reply) error {
		return send(withCorrelation(request, reply))
	})
}

func operationKey(service, method string) string { return service + "\x00" + method }

func withCorrelation(request Request, reply Reply) Reply {
	reply.Metadata = cloneMetadata(reply.Metadata)
	if correlationID := request.Metadata["correlation_id"]; correlationID != "" && reply.Metadata["correlation_id"] == "" {
		reply.Metadata["correlation_id"] = correlationID
	}
	return reply
}

func cloneMetadata(metadata map[string]string) map[string]string {
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[key] = value
	}
	return result
}
