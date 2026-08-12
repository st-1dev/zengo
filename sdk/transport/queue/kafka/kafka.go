package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/policy"
	"zengo/platform/sdk/router"
	"zengo/platform/sdk/tlsconfig"

	kafkacfg "zengo/platform/api/config/queue/kafka"

	"github.com/IBM/sarama"
)

var (
	newSyncProducer  = sarama.NewSyncProducer
	newConsumerGroup = sarama.NewConsumerGroup
)

// Producer lazily manages Sarama sync producers keyed by broker set.
type Producer struct {
	producers      map[string]sarama.SyncProducer
	defaultBrokers []string
	saramaConfig   *sarama.Config
	mu             sync.Mutex
}

// NewProducer creates a Producer that falls back to brokers when Publish is called with none.
func NewProducer(brokers []string) *Producer {
	return newProducerWithConfig(brokers, producerConfig("", nil))
}

// NewProducerFromConfig creates a Producer from typed Kafka config, including TLS when configured.
func NewProducerFromConfig(cfg *kafkacfg.Config) (*Producer, error) {
	spec := cfg.GetSpec()
	tlsCfg, err := kafkaTLSConfig(spec)
	if err != nil {
		return nil, err
	}
	clientID := ""
	if spec != nil {
		clientID = spec.GetClientId()
	}
	return newProducerWithConfig(Brokers(cfg), producerConfig(clientID, tlsCfg)), nil
}

func newProducerWithConfig(brokers []string, cfg *sarama.Config) *Producer {
	return &Producer{
		producers:      map[string]sarama.SyncProducer{},
		defaultBrokers: normalizeBrokers(brokers),
		saramaConfig:   cfg,
	}
}

func (p *Producer) producer(brokers []string) (sarama.SyncProducer, error) {
	key := strings.Join(brokers, ",")
	p.mu.Lock()
	defer p.mu.Unlock()
	producer, ok := p.producers[key]
	if ok {
		return producer, nil
	}
	var err error

	producer, err = newSyncProducer(brokers, p.saramaConfig)
	if err != nil {
		return nil, err
	}
	p.producers[key] = producer
	return producer, nil
}

// Publish marshals an EventEnvelope and sends it to topic.
func (p *Producer) Publish(
	ctx context.Context,
	brokers []string,
	topic, apiVersion, kind string,
	payload []byte,
) error {
	brokers = p.effectiveBrokers(brokers)
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("messaging.system", "kafka"),
		observability.StringAttribute("messaging.operation", "publish"),
		observability.StringAttribute("messaging.destination.name", topic),
	)
	ctx = spanCtx
	defer endSpan()

	env := router.NewEventEnvelope(apiVersion, kind, payload)
	err := env.Validate()
	if err != nil {
		observability.RecordException(ctx, err, "validate event envelope")
		return err
	}
	var body []byte

	body, err = json.Marshal(env)
	if err != nil {
		observability.RecordException(ctx, err, "marshal event envelope")
		return fmt.Errorf("marshal event envelope: %w", err)
	}
	var producer sarama.SyncProducer
	producer, err = p.producer(brokers)
	if err != nil {
		observability.RecordException(ctx, err, "create kafka producer")
		return fmt.Errorf("create kafka producer: %w", err)
	}
	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(kind),
		Value:     sarama.ByteEncoder(body),
		Timestamp: time.Now().UTC(),
		Headers:   producerHeaders(observability.InjectKafkaHeaders(ctx, nil)),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		observability.RecordException(ctx, err, "send kafka message")
	}
	return err
}

func (p *Producer) effectiveBrokers(brokers []string) []string {
	brokers = normalizeBrokers(brokers)
	if len(brokers) > 0 {
		return brokers
	}
	if len(p.defaultBrokers) > 0 {
		return append([]string(nil), p.defaultBrokers...)
	}
	return []string{defaultBroker}
}

// Close closes all lazily created underlying Sarama producers.
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var first error
	for key, producer := range p.producers {
		err := producer.Close()
		if err != nil && first == nil {
			first = fmt.Errorf("close producer %s: %w", key, err)
		}
		delete(p.producers, key)
	}
	return first
}

// HandlerFunc processes a decoded EventEnvelope from Kafka.
type HandlerFunc func(ctx context.Context, env router.EventEnvelope) error

// Consumer manages Sarama consumer groups for registered topics.
type Consumer struct {
	groups       []sarama.ConsumerGroup
	defaultGroup string
	baseConfig   *sarama.Config
	ctx          context.Context
	stop         context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex
}

// NewConsumer creates a Consumer with the provided default consumer-group name.
func NewConsumer(group string) *Consumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Consumer{defaultGroup: group, baseConfig: consumerConfig(group, "", nil), ctx: ctx, stop: cancel}
}

// NewConsumerFromConfig creates a Consumer from typed Kafka config, including TLS when configured.
func NewConsumerFromConfig(group string, cfg *kafkacfg.Config) (*Consumer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	spec := cfg.GetSpec()
	tlsCfg, err := kafkaTLSConfig(spec)
	if err != nil {
		cancel()
		return nil, err
	}
	clientID := ""
	defaultGroup := group
	if spec != nil {
		clientID = spec.GetClientId()
		if defaultGroup == "" {
			defaultGroup = spec.GetGroupId()
		}
	}
	return &Consumer{
		defaultGroup: defaultGroup,
		baseConfig:   consumerConfig(defaultGroup, clientID, tlsCfg),
		ctx:          ctx,
		stop:         cancel,
	}, nil
}

// ConsumeSpec describes one topic subscription handled by Consumer.
type ConsumeSpec struct {
	// Topic is the Kafka topic to subscribe to.
	Topic string
	// Group overrides the Consumer default group for this subscription.
	Group string
	// Brokers is the broker list used to create the consumer group.
	Brokers []string
	// Handler processes decoded envelopes delivered from Topic.
	Handler HandlerFunc
	// Policy wraps handler execution for retries, timeouts, and admission control.
	Policy policy.Options
}

// Register starts background consumption for the provided topic subscription.
func (c *Consumer) Register(spec ConsumeSpec) error {
	if c == nil {
		return fmt.Errorf("kafka consumer is nil")
	}
	if spec.Topic == "" {
		return fmt.Errorf("kafka consumer topic is required")
	}
	if spec.Handler == nil {
		return fmt.Errorf("kafka consumer handler is required for topic %q", spec.Topic)
	}
	groupID := spec.Group
	if groupID == "" {
		groupID = c.defaultGroup
	}
	if groupID == "" {
		return fmt.Errorf("kafka consumer group is required for topic %q", spec.Topic)
	}
	brokers := normalizeBrokers(spec.Brokers)
	if len(brokers) == 0 {
		brokers = []string{defaultBroker}
	}
	group, err := newConsumerGroup(brokers, groupID, c.consumerConfig(groupID))
	if err != nil {
		return fmt.Errorf("create kafka consumer group %q for topic %q: %w", groupID, spec.Topic, err)
	}
	c.mu.Lock()
	c.groups = append(c.groups, group)
	c.mu.Unlock()
	c.wg.Go(func() {
		c.consume(group, spec, groupID)
	})
	c.wg.Go(func() {
		c.consumeErrors(group, spec.Topic, groupID)
	})
	return nil
}

func (c *Consumer) consume(group sarama.ConsumerGroup, spec ConsumeSpec, groupID string) {
	handler := consumerGroupHandler{
		rootCtx: c.ctx,
		handler: spec.Handler,
		exec:    policy.NewExecutor(spec.Policy),
	}
	for {
		err := c.ctx.Err()
		if err != nil {
			return
		}
		err = group.Consume(c.ctx, []string{spec.Topic}, handler)
		if err != nil {
			if errors.Is(err, context.Canceled) || c.ctx.Err() != nil {
				return
			}
			slog.Error("kafka consume failed", "group", groupID, "topic", spec.Topic, "err", err)
			select {
			case <-c.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}
}

func (c *Consumer) consumeErrors(group sarama.ConsumerGroup, topic, groupID string) {
	for err := range group.Errors() {
		if err == nil || c.ctx.Err() != nil {
			continue
		}
		slog.Error("kafka consumer group error", "group", groupID, "topic", topic, "err", err)
	}
}

// Close stops background consumers and closes all consumer groups.
func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}
	c.stop()
	c.mu.Lock()
	groups := append([]sarama.ConsumerGroup(nil), c.groups...)
	c.mu.Unlock()
	var closeErr error
	for _, group := range groups {
		err := group.Close()
		if err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	c.wg.Wait()
	return closeErr
}

func (c *Consumer) consumerConfig(group string) *sarama.Config {
	base := c.baseConfig
	if base == nil {
		base = consumerConfig(group, "", nil)
	}
	cfg := *base
	if group != "" {
		cfg.ClientID = "zengo-platform-consumer-" + group
	}
	return &cfg
}

type consumerGroupHandler struct {
	rootCtx context.Context
	handler HandlerFunc
	exec    *policy.Executor
}

func (consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error { return nil }

func (consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-h.rootCtx.Done():
			return nil
		case <-sess.Context().Done():
			return nil
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			h.consumeMessage(sess, msg)
		}
	}
}

func (h consumerGroupHandler) consumeMessage(sess sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	ctx := observability.ExtractKafkaContext(h.rootCtx, msg.Headers)
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("messaging.system", "kafka"),
		observability.StringAttribute("messaging.operation", "receive"),
		observability.StringAttribute("messaging.destination.name", msg.Topic),
	)
	ctx = spanCtx
	defer endSpan()

	var env router.EventEnvelope
	err := json.Unmarshal(msg.Value, &env)
	if err != nil {
		observability.RecordException(ctx, err, "decode kafka envelope")
		slog.Error(
			"kafka decode envelope failed",
			"topic",
			msg.Topic,
			"partition",
			msg.Partition,
			"offset",
			msg.Offset,
			"err",
			err,
		)
		sess.MarkMessage(msg, "")
		return
	}
	exec := h.exec
	if exec == nil {
		exec = policy.NewExecutor(policy.Options{})
	}
	err = exec.Do(ctx, func(callCtx context.Context) error {
		return h.handler(callCtx, env)
	})
	if err != nil {
		observability.RecordException(ctx, err, "handle kafka message")
		slog.Error(
			"kafka handler failed",
			"topic",
			msg.Topic,
			"partition",
			msg.Partition,
			"offset",
			msg.Offset,
			"err",
			err,
		)
	}
	sess.MarkMessage(msg, "")
}

func producerHeaders(headers []*sarama.RecordHeader) []sarama.RecordHeader {
	if len(headers) == 0 {
		return nil
	}
	out := make([]sarama.RecordHeader, 0, len(headers))
	for _, header := range headers {
		if header == nil {
			continue
		}
		out = append(
			out,
			sarama.RecordHeader{Key: append([]byte(nil), header.Key...), Value: append([]byte(nil), header.Value...)},
		)
	}
	return out
}

func producerConfig(clientID string, clientTLS *tls.Config) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.ClientID = "zengo-platform-producer"
	if clientID != "" {
		cfg.ClientID = clientID
	}
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Return.Successes = true
	cfg.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	if clientTLS != nil {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = clientTLS
	}
	return cfg
}

func consumerConfig(group, clientID string, clientTLS *tls.Config) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.ClientID = "zengo-platform-consumer"
	if clientID != "" {
		cfg.ClientID = clientID
	}
	if group != "" {
		cfg.ClientID = "zengo-platform-consumer-" + group
	}
	cfg.Consumer.Return.Errors = true
	if clientTLS != nil {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = clientTLS
	}
	return cfg
}

func kafkaTLSConfig(spec *kafkacfg.Spec) (*tls.Config, error) {
	if spec == nil {
		return nil, nil
	}
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		return nil, fmt.Errorf("kafka tls: %w", err)
	}
	return clientTLS, nil
}
