package kafka

import (
	"os"
	"strings"
	"zengo/platform/sdk/config"

	kafkacfg "zengo/platform/api/config/queue/kafka"
)

const defaultBroker = "localhost:9092"

// Config is the typed Kafka client configuration used by runtime helpers.
type Config = kafkacfg.Config

// BrokersFromLoader returns broker addresses from typed configuration and fallbacks.
func BrokersFromLoader(loader *config.Loader, kind string, manifestBrokers []string) []string {
	cfg := ConfigFromLoader(loader, kind, manifestBrokers)
	return Brokers(cfg)
}

// ConfigFromLoader loads typed Kafka configuration and applies broker fallbacks.
func ConfigFromLoader(loader *config.Loader, kind string, manifestBrokers []string) *kafkacfg.Config {
	if kind == "" {
		kind = "kafka"
	}
	if loader != nil {
		cfg, err := loader.Kafka(kind)
		if err == nil {
			spec := cfg.GetSpec()
			if spec == nil {
				spec = &kafkacfg.Spec{}
				cfg.Spec = spec
			}
			if len(specBrokers(cfg)) == 0 {
				spec.Brokers = fallbackBrokers(manifestBrokers)
			}
			return cfg
		}
	}
	return &kafkacfg.Config{
		Kind:       kind,
		ApiVersion: "v1",
		Spec: &kafkacfg.Spec{
			Brokers: fallbackBrokers(manifestBrokers),
		},
	}
}

// Brokers returns the normalized broker list or the local default.
func Brokers(cfg *kafkacfg.Config) []string {
	brokers := specBrokers(cfg)
	if len(brokers) > 0 {
		return brokers
	}
	return []string{defaultBroker}
}

func specBrokers(cfg *kafkacfg.Config) []string {
	if cfg == nil || cfg.GetSpec() == nil {
		return nil
	}
	return normalizeBrokers(cfg.GetSpec().GetBrokers())
}

func normalizeBrokers(brokers []string) []string {
	out := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			out = append(out, broker)
		}
	}
	return out
}

func splitBrokers(raw string) []string {
	return normalizeBrokers(strings.Split(raw, ","))
}

func fallbackBrokers(manifestBrokers []string) []string {
	brokers := normalizeBrokers(manifestBrokers)
	if len(brokers) > 0 {
		return brokers
	}
	raw := os.Getenv("KAFKA_BROKERS")
	if raw != "" {
		return splitBrokers(raw)
	}
	return []string{defaultBroker}
}
