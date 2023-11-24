run:
	go run main.go

run-instrumenting:
	ENABLE_TRACER=true OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 OTEL_EXPORTER_OTLP_INSECURE=true go run main.go