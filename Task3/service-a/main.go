
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "time"

    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/propagation"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/resource"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

const (
    defaultAddr = ":8080"
)

func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
    // Endpoint example: simplest-collector.observability.svc.cluster.local:4317
    endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
    if endpoint == "" {
        endpoint = "simplest-collector.observability.svc.cluster.local:4317"
    }

    exp, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
    )
    if err != nil {
        return nil, err
    }

    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(serviceName),
        ),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))), // always sample in MVP
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.TraceContext{})

    return tp.Shutdown, nil
}

func main() {
    ctx := context.Background()

    shutdown, err := initTracer(ctx, "service-a")
    if err != nil {
        log.Fatalf("init tracer: %v", err)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := shutdown(ctx); err != nil {
            log.Printf("shutdown tracer: %v", err)
        }
    }()

    mux := http.NewServeMux()

    mux.Handle("/healthz", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    }), "healthz"))

// Root handler — will call service-b so that the whole flow is visible in one trace.
mux.Handle("/", otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Propagation happens automatically for outgoing request when we use otelhttp.Transport.
    url := os.Getenv("PEER_URL")
    if url == "" {
        url = "http://service-b:8080/"
    }

    client := http.Client{
        Transport: otelhttp.NewTransport(http.DefaultTransport),
        Timeout: 5 * time.Second,
    }

    req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
    resp, err := client.Do(req)
    if err != nil {
        http.Error(w, fmt.Sprintf("call peer failed: %v", err), http.StatusBadGateway)
        return
    }
    defer resp.Body.Close()

    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte("service-a: ok -> called service-b\n"))
}), "GET /"))addr := os.Getenv("ADDR")
    if addr == "" {
        addr = defaultAddr
    }

    srv := &http.Server{
        Addr:              addr,
        Handler:           mux,
        ReadHeaderTimeout: 5 * time.Second,
    }

    log.Printf("listening on %s", addr)
    if err := srv.ListenAndServe(); err != nil {
        log.Fatalf("server: %v", err)
    }
}
