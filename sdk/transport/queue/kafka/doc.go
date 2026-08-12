// Package kafka provides producers and consumers with OpenTelemetry messaging spans.
// Broker addresses are loaded via BrokersFromLoader from configs/kafka.yaml (kind: kafka).
// Call observability.Init before use so publish/consume spans and header propagation work.
package kafka
