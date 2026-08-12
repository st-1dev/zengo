package config

import (
	cassandracfg "zengo/platform/api/config/db/cassandra"
	oraclecfg "zengo/platform/api/config/db/oracle"
	rediscfg "zengo/platform/api/config/db/redis"
	kafkacfg "zengo/platform/api/config/queue/kafka"
	natscfg "zengo/platform/api/config/queue/nats"
	rabbitmqcfg "zengo/platform/api/config/queue/rabbitmq"
	s3cfg "zengo/platform/api/config/storage/s3"
	tracingcfg "zengo/platform/api/config/tracing"
)

// Tracing loads tracing config by kind.
func (l *Loader) Tracing(kind string) (*tracingcfg.Config, error) {
	cfg := &tracingcfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Kafka loads Kafka config by kind.
func (l *Loader) Kafka(kind string) (*kafkacfg.Config, error) {
	cfg := &kafkacfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Cassandra loads Cassandra config by kind.
func (l *Loader) Cassandra(kind string) (*cassandracfg.Config, error) {
	cfg := &cassandracfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Redis loads Redis config by kind.
func (l *Loader) Redis(kind string) (*rediscfg.Config, error) {
	cfg := &rediscfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Nats loads NATS config by kind.
func (l *Loader) Nats(kind string) (*natscfg.Config, error) {
	cfg := &natscfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// S3 loads S3 config by kind.
func (l *Loader) S3(kind string) (*s3cfg.Config, error) {
	cfg := &s3cfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// Oracle loads Oracle config by kind.
func (l *Loader) Oracle(kind string) (*oraclecfg.Config, error) {
	cfg := &oraclecfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// RabbitMQ loads RabbitMQ config by kind.
func (l *Loader) RabbitMQ(kind string) (*rabbitmqcfg.Config, error) {
	cfg := &rabbitmqcfg.Config{}
	err := l.Get(kind, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
