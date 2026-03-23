package telemetry

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type otlpCollector struct {
	traceExports  int64
	metricExports int64
}

type traceService struct {
	collectortrace.UnimplementedTraceServiceServer
	parent *otlpCollector
}

func (s *traceService) Export(
	ctx context.Context,
	req *collectortrace.ExportTraceServiceRequest,
) (*collectortrace.ExportTraceServiceResponse, error) {
	atomic.AddInt64(&s.parent.traceExports, 1)
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

type metricsService struct {
	collectormetrics.UnimplementedMetricsServiceServer
	parent *otlpCollector
}

func (s *metricsService) Export(
	ctx context.Context,
	req *collectormetrics.ExportMetricsServiceRequest,
) (*collectormetrics.ExportMetricsServiceResponse, error) {
	atomic.AddInt64(&s.parent.metricExports, 1)
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

func TestInitWithOTLPCollectorStub(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	collector := &otlpCollector{}
	grpcServer := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(grpcServer, &traceService{parent: collector})
	collectormetrics.RegisterMetricsServiceServer(grpcServer, &metricsService{parent: collector})

	go func() {
		_ = grpcServer.Serve(listener)
	}()
	defer grpcServer.Stop()

	shutdown, err := Init(context.Background(), Config{
		OTLPEndpoint:   listener.Addr().String(),
		ServiceName:    "clusterprobe-test",
		ServiceVersion: "dev",
		Environment:    "test",
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	tracer := otel.Tracer("telemetry-test")
	_, span := tracer.Start(context.Background(), "smoke")
	span.End()

	meter := otel.Meter("telemetry-test")
	counter, err := meter.Int64Counter("smoke.counter")
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	counter.Add(context.Background(), 1)

	shutdown()

	if atomic.LoadInt64(&collector.traceExports) == 0 {
		t.Fatalf("expected trace exports to be recorded")
	}
	if atomic.LoadInt64(&collector.metricExports) == 0 {
		t.Fatalf("expected metric exports to be recorded")
	}
}
