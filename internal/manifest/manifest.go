package manifest

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/internal/naming"
	"zengo/platform/pkg/sdk/config/configfmt"
	"zengo/platform/sdk/tlsconfig"

	manifestpb "zengo/platform/api/zengo/manifest"
)

const configDir = "configs"

var manifestCandidates = []string{
	"zengo.yaml",
	"zengo.yml",
	"zengo.textproto",
	"zengo.pbtxt",
}

// Manifest is the service infrastructure manifest loaded from zengo.{yaml,textproto}.
type Manifest struct {
	// Service contains the logical service identity and module path.
	Service Service
	// Transports declares enabled gRPC and REST transports.
	Transports Transports
	// Observability configures metrics, tracing, and health wiring.
	Observability Observability
	// Auth enables optional API-key authentication.
	Auth *Auth
	// DB declares database integrations.
	DB *DB
	// Queue declares messaging integrations.
	Queue *Queue
	// Cache declares cache integrations.
	Cache *Cache
	// Storage declares object storage integrations.
	Storage *Storage
	// Compatibility controls legacy version compatibility generation.
	Compatibility *Compatibility
}

// Service identifies one service repository.
type Service struct {
	// Name is the logical service name used by runtime and generators.
	Name string
	// Module is the Go module path for the service repository.
	Module string
}

// Transports declares enabled network transports.
type Transports struct {
	// GRPC configures the gRPC transport when present.
	GRPC *GRPC
	// REST configures the grpc-gateway REST transport when present.
	REST *REST
}

// GRPC configures the gRPC transport.
type GRPC struct {
	// Port is the service listen port.
	Port int
	// TLS configures server-side TLS for the gRPC listener.
	TLS *tlsconfig.ServerOptions
}

// REST configures the REST transport.
type REST struct {
	// Port is the service listen port.
	Port int
	// TLS configures server-side TLS for the REST listener.
	TLS *tlsconfig.ServerOptions
	// GRPCClientTLS configures grpc-gateway TLS when dialing the local gRPC listener.
	GRPCClientTLS *tlsconfig.ClientOptions
}

// DB declares database integrations used by the service.
type DB struct {
	// Postgres configures PostgreSQL integration.
	Postgres *PostgresDB
	// Cassandra configures Cassandra integration.
	Cassandra *CassandraDB
	// Oracle configures Oracle integration.
	Oracle *OracleDB
}

// PostgresDB configures PostgreSQL integration.
type PostgresDB struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
	// Queries points to the sqlc query directory.
	Queries string
}

// CassandraDB configures Cassandra integration.
type CassandraDB struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
	// Keyspace is the logical Cassandra keyspace used by the service.
	Keyspace string
	// CQL points to the cql query directory.
	CQL string
}

// OracleDB configures Oracle integration.
type OracleDB struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
	// Queries points to the sqlc query directory.
	Queries string
}

// Queue declares messaging integrations used by the service.
type Queue struct {
	// Kafka configures Kafka integration.
	Kafka *Kafka
	// Nats configures NATS integration.
	Nats *Nats
	// RabbitMQ configures RabbitMQ integration.
	RabbitMQ *RabbitMQ
}

// Cache declares cache integrations used by the service.
type Cache struct {
	// Redis configures Redis integration.
	Redis *Redis
}

// Storage declares object storage integrations used by the service.
type Storage struct {
	// S3 configures S3-compatible storage integration.
	S3 *S3
}

// Kafka configures Kafka integration.
type Kafka struct {
	// BrokersFromConfig overrides the typed config kind to load.
	BrokersFromConfig string
	// Brokers supplies manifest-level fallback broker addresses.
	Brokers []string
}

// RabbitMQ configures RabbitMQ integration.
type RabbitMQ struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
	// URLs supplies manifest-level fallback connection URLs.
	URLs []string
}

// Redis configures Redis integration.
type Redis struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
}

// Nats configures NATS integration.
type Nats struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
	// URLs supplies manifest-level fallback server URLs.
	URLs []string
}

// S3 configures S3-compatible object storage integration.
type S3 struct {
	// ConfigFrom overrides the typed config kind to load.
	ConfigFrom string
}

// Observability configures runtime observability features.
type Observability struct {
	// Metrics enables metrics collection and /metrics.
	Metrics bool
	// Tracing enables distributed tracing.
	Tracing bool
	// Health enables readiness checks in generated runtime wiring.
	Health bool
	// TracingConfigFrom overrides the tracing config kind to load.
	TracingConfigFrom string
	// LoggingConfigFrom overrides the logging config kind to load.
	LoggingConfigFrom string
}

// InitRequired reports whether observability initialization should be wired into main.
func (o Observability) InitRequired() bool {
	return o.Metrics || o.Tracing
}

// TracingConfigKey returns the tracing config kind, falling back to the default.
func (o Observability) TracingConfigKey() string {
	if o.TracingConfigFrom != "" {
		return o.TracingConfigFrom
	}
	return "tracing"
}

// LoggingConfigKey returns the logging config kind, falling back to the default.
func (o Observability) LoggingConfigKey() string {
	if o.LoggingConfigFrom != "" {
		return o.LoggingConfigFrom
	}
	return "logging"
}

// ConfigKey returns the config kind for kafka settings.
func (k *Kafka) ConfigKey() string {
	if k == nil {
		return "kafka"
	}
	if k.BrokersFromConfig != "" {
		return k.BrokersFromConfig
	}
	return "kafka"
}

// ConfigKey returns the config kind for nats settings.
func (n *Nats) ConfigKey() string {
	if n == nil || n.ConfigFrom == "" {
		return "nats"
	}
	return n.ConfigFrom
}

// ConfigKey returns the config kind for rabbitmq settings.
func (r *RabbitMQ) ConfigKey() string {
	if r == nil || r.ConfigFrom == "" {
		return "rabbitmq"
	}
	return r.ConfigFrom
}

// ConfigKey returns the config kind for postgres settings.
func (p *PostgresDB) ConfigKey() string {
	if p == nil || p.ConfigFrom == "" {
		return "postgres"
	}
	return p.ConfigFrom
}

// ConfigKey returns the config kind for cassandra settings.
func (c *CassandraDB) ConfigKey() string {
	if c == nil || c.ConfigFrom == "" {
		return "cassandra"
	}
	return c.ConfigFrom
}

// ConfigKey returns the config kind for oracle settings.
func (o *OracleDB) ConfigKey() string {
	if o == nil || o.ConfigFrom == "" {
		return "oracle"
	}
	return o.ConfigFrom
}

// ConfigKey returns the config kind for redis settings.
func (r *Redis) ConfigKey() string {
	if r == nil || r.ConfigFrom == "" {
		return "redis"
	}
	return r.ConfigFrom
}

// ConfigKey returns the config kind for S3 settings.
func (s *S3) ConfigKey() string {
	if s == nil || s.ConfigFrom == "" {
		return "s3"
	}
	return s.ConfigFrom
}

// Auth configures API-key authentication.
type Auth struct {
	// Enabled toggles the auth layer on or off.
	Enabled bool
	// APIKeys maps logical names to API key values.
	APIKeys map[string]string
}

// Compatibility configures legacy version compatibility generation.
type Compatibility struct {
	// LegacyVersions controls whether discovered legacy versions are generated.
	LegacyVersions CompatibilityMode
}

// CompatibilityMode controls whether discovered legacy versions are generated.
type CompatibilityMode string

const (
	// CompatibilityUnspecified uses auto-detection behavior.
	CompatibilityUnspecified CompatibilityMode = ""
	// CompatibilityDisabled disables legacy compatibility generation.
	CompatibilityDisabled CompatibilityMode = "disabled"
	// CompatibilityEnabled enables compatibility generation for discovered legacy versions.
	CompatibilityEnabled CompatibilityMode = "enabled"
)

// Load reads a manifest from path. When path is empty, auto-discovers a single zengo.* file in the current directory.
func Load(path string) (*Manifest, error) {
	_, pb, err := loadProto(path)
	if err != nil {
		return nil, err
	}
	m := fromProto(pb)
	err = validate(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ResolvePath returns the manifest path selected by explicit input or auto-discovery.
func ResolvePath(path string) (string, error) {
	return resolveManifestPath(path)
}

func resolveManifestPath(path string) (string, error) {
	if path != "" {
		if !configfmt.IsSupported(path) {
			return "", fmt.Errorf("unsupported manifest format %q", filepath.Ext(path))
		}
		return path, nil
	}
	var found []string
	for _, name := range manifestCandidates {
		_, err := os.Stat(name)
		if err == nil {
			found = append(found, name)
		}
	}
	switch len(found) {
	case 0:
		return "", fmt.Errorf(
			"manifest not found: expected one of %s in current directory",
			strings.Join(manifestCandidates, ", "),
		)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf(
			"multiple manifest files found (%s); pass --manifest explicitly",
			strings.Join(found, ", "),
		)
	}
}

func validate(m *Manifest) error {
	if m.Service.Name == "" {
		return fmt.Errorf("manifest.service.name is required")
	}
	if m.Service.Module == "" {
		return fmt.Errorf("manifest.service.module is required")
	}
	if m.Transports.REST != nil && m.Transports.GRPC == nil {
		return fmt.Errorf("manifest.transports.grpc is required when manifest.transports.rest is enabled")
	}
	if m.Transports.REST != nil && m.Transports.GRPC != nil && m.Transports.GRPC.TLS != nil &&
		m.Transports.REST.GRPCClientTLS == nil {
		return fmt.Errorf(
			"manifest.transports.rest.grpc_client_tls is required when manifest.transports.grpc.tls is enabled",
		)
	}
	if m.Transports.GRPC != nil {
		err := validatePort("manifest.transports.grpc.port", m.Transports.GRPC.Port)
		if err != nil {
			return err
		}
	}
	if m.Transports.REST != nil {
		err := validatePort("manifest.transports.rest.port", m.Transports.REST.Port)
		if err != nil {
			return err
		}
	}
	return nil
}

func validatePort(name string, port int) error {
	if port == 0 {
		return nil
	}
	if port < 0 || port > math.MaxUint16 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func loadProto(path string) (string, *manifestpb.Manifest, error) {
	resolved, err := resolveManifestPath(path)
	if err != nil {
		return "", nil, err
	}
	var data []byte

	data, err = os.ReadFile(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("read manifest: %w", err)
	}
	pb := &manifestpb.Manifest{}
	err = configfmt.Unmarshal(resolved, data, pb)
	if err != nil {
		return "", nil, fmt.Errorf("parse manifest: %w", err)
	}
	return resolved, pb, nil
}

func fromProto(pb *manifestpb.Manifest) *Manifest {
	m := &Manifest{}
	svc := pb.GetService()
	if svc != nil {
		m.Service = Service{Name: svc.GetName(), Module: svc.GetModule()}
	}
	t := pb.GetTransports()
	if t != nil {
		m.Transports = Transports{}
		g := t.GetGrpc()
		if g != nil {
			m.Transports.GRPC = &GRPC{
				Port: int(g.GetPort()),
				TLS:  tlsconfig.ServerOptionsFromProto(g.GetTls()),
			}
		}
		r := t.GetRest()
		if r != nil {
			m.Transports.REST = &REST{
				Port:          int(r.GetPort()),
				TLS:           tlsconfig.ServerOptionsFromProto(r.GetTls()),
				GRPCClientTLS: tlsconfig.ClientOptionsFromProto(r.GetGrpcClientTls()),
			}
		}
	}
	db := pb.GetDb()
	if db != nil {
		m.DB = dbFromProto(db)
	}
	q := pb.GetQueue()
	if q != nil {
		m.Queue = queueFromProto(q)
	}
	cache := pb.GetCache()
	if cache != nil {
		m.Cache = cacheFromProto(cache)
	}
	storage := pb.GetStorage()
	if storage != nil {
		m.Storage = storageFromProto(storage)
	}
	o := pb.GetObservability()
	if o != nil {
		m.Observability = Observability{
			Metrics:           o.GetMetrics(),
			Tracing:           o.GetTracing(),
			Health:            o.GetHealth(),
			TracingConfigFrom: o.GetTracingConfigFrom(),
			LoggingConfigFrom: o.GetLoggingConfigFrom(),
		}
	}
	a := pb.GetAuth()
	if a != nil {
		m.Auth = &Auth{Enabled: a.GetEnabled(), APIKeys: cloneMap(a.GetApiKeys())}
	}
	compatibility := pb.GetCompatibility()
	if compatibility != nil {
		m.Compatibility = &Compatibility{LegacyVersions: compatibilityModeFromProto(compatibility.GetLegacyVersions())}
	}
	return m
}

func dbFromProto(db *manifestpb.DB) *DB {
	if db == nil {
		return nil
	}
	out := &DB{}
	p := db.GetPostgres()
	if p != nil {
		out.Postgres = &PostgresDB{ConfigFrom: p.GetConfigFrom(), Queries: p.GetQueries()}
	}
	c := db.GetCassandra()
	if c != nil {
		out.Cassandra = &CassandraDB{
			ConfigFrom: c.GetConfigFrom(),
			Keyspace:   c.GetKeyspace(),
			CQL:        c.GetCql(),
		}
	}
	o := db.GetOracle()
	if o != nil {
		out.Oracle = &OracleDB{ConfigFrom: o.GetConfigFrom(), Queries: o.GetQueries()}
	}
	return out
}

func queueFromProto(q *manifestpb.Queue) *Queue {
	if q == nil {
		return nil
	}
	out := &Queue{}
	k := q.GetKafka()
	if k != nil {
		out.Kafka = kafkaFromProto(k)
	}
	n := q.GetNats()
	if n != nil {
		out.Nats = natsFromProto(n)
	}
	r := q.GetRabbitmq()
	if r != nil {
		out.RabbitMQ = rabbitMQFromProto(r)
	}
	return out
}

func cacheFromProto(c *manifestpb.Cache) *Cache {
	if c == nil {
		return nil
	}
	out := &Cache{}
	r := c.GetRedis()
	if r != nil {
		out.Redis = redisFromProto(r)
	}
	return out
}

func storageFromProto(s *manifestpb.Storage) *Storage {
	if s == nil {
		return nil
	}
	out := &Storage{}
	cfg := s.GetS3()
	if cfg != nil {
		out.S3 = s3FromProto(cfg)
	}
	return out
}

func kafkaFromProto(k *manifestpb.Kafka) *Kafka {
	return &Kafka{
		BrokersFromConfig: k.GetBrokersFromConfig(),
		Brokers:           append([]string(nil), k.GetBrokers()...),
	}
}

func rabbitMQFromProto(r *manifestpb.RabbitMQ) *RabbitMQ {
	return &RabbitMQ{
		ConfigFrom: r.GetConfigFrom(),
		URLs:       append([]string(nil), r.GetUrls()...),
	}
}

func redisFromProto(r *manifestpb.Redis) *Redis {
	return &Redis{ConfigFrom: r.GetConfigFrom()}
}

func natsFromProto(n *manifestpb.Nats) *Nats {
	return &Nats{
		ConfigFrom: n.GetConfigFrom(),
		URLs:       append([]string(nil), n.GetUrls()...),
	}
}

func s3FromProto(s *manifestpb.S3) *S3 {
	return &S3{ConfigFrom: s.GetConfigFrom()}
}

// GRPCPort returns the configured gRPC port, or the default.
func (m *Manifest) GRPCPort() int {
	if m.Transports.GRPC != nil && m.Transports.GRPC.Port > 0 {
		return m.Transports.GRPC.Port
	}
	return 9090
}

// RESTPort returns the configured REST port, or the default.
func (m *Manifest) RESTPort() int {
	if m.Transports.REST != nil && m.Transports.REST.Port > 0 {
		return m.Transports.REST.Port
	}
	return 8080
}

// AuthEnabled reports whether API-key auth should be wired into runtime generation.
func (m *Manifest) AuthEnabled() bool {
	return m.Auth != nil && m.Auth.Enabled
}

// EffectiveRabbitMQ returns the canonical rabbitmq settings.
func (m *Manifest) EffectiveRabbitMQ() *RabbitMQ {
	if m.Queue != nil {
		return m.Queue.RabbitMQ
	}
	return nil
}

// ConfigPath returns the default config directory name used by generated services.
func (m *Manifest) ConfigPath() string {
	return configDir
}

// NeedsConfigLoader reports whether generated bootstrap needs typed config access.
func (m *Manifest) NeedsConfigLoader() bool {
	if m == nil {
		return false
	}
	if m.DB != nil && (m.DB.Postgres != nil || m.DB.Cassandra != nil || m.DB.Oracle != nil) {
		return true
	}
	if m.Queue != nil && m.Queue.Kafka != nil {
		return true
	}
	if m.Cache != nil && m.Cache.Redis != nil {
		return true
	}
	if m.Storage != nil && m.Storage.S3 != nil {
		return true
	}
	return m.Observability.InitRequired()
}

// HandlerPackage derives the canonical handler package from the service name.
func (m *Manifest) HandlerPackage() string {
	return naming.HandlerPackage(m.Service.Name)
}

// HandlerImport returns the import path for the primary service handler package.
func (m *Manifest) HandlerImport() string {
	return m.Service.Module + "/internal/" + m.HandlerPackage()
}

// LegacyCompatibilityMode returns the configured legacy adapter mode.
func (m *Manifest) LegacyCompatibilityMode() CompatibilityMode {
	if m == nil || m.Compatibility == nil {
		return CompatibilityUnspecified
	}
	return m.Compatibility.LegacyVersions
}

// LegacyCompatibilityConfigured reports whether the manifest explicitly sets compatibility mode.
func (m *Manifest) LegacyCompatibilityConfigured() bool {
	return m.LegacyCompatibilityMode() != CompatibilityUnspecified
}

// LegacyCompatibilityEnabled reports whether legacy api/vN adapters are explicitly enabled.
func (m *Manifest) LegacyCompatibilityEnabled() bool {
	return m.LegacyCompatibilityMode() == CompatibilityEnabled
}

// PostgresDBName returns the default postgres database name used by local fallbacks.
func (m *Manifest) PostgresDBName() string {
	return m.HandlerPackage() + "s"
}

func (m *Manifest) toProto() *manifestpb.Manifest {
	pb := &manifestpb.Manifest{
		Service: &manifestpb.Service{
			Name:   m.Service.Name,
			Module: m.Service.Module,
		},
		Observability: &manifestpb.Observability{
			Metrics:           m.Observability.Metrics,
			Tracing:           m.Observability.Tracing,
			Health:            m.Observability.Health,
			TracingConfigFrom: m.Observability.TracingConfigFrom,
			LoggingConfigFrom: m.Observability.LoggingConfigFrom,
		},
	}
	if m.Transports.GRPC != nil || m.Transports.REST != nil {
		pb.Transports = &manifestpb.Transports{}
		if m.Transports.GRPC != nil {
			pb.Transports.Grpc = &manifestpb.GRPC{
				Port: portToInt32(m.Transports.GRPC.Port),
				Tls:  tlsconfig.ServerOptionsToProto(m.Transports.GRPC.TLS),
			}
		}
		if m.Transports.REST != nil {
			pb.Transports.Rest = &manifestpb.REST{
				Port:          portToInt32(m.Transports.REST.Port),
				Tls:           tlsconfig.ServerOptionsToProto(m.Transports.REST.TLS),
				GrpcClientTls: tlsconfig.ClientOptionsToProto(m.Transports.REST.GRPCClientTLS),
			}
		}
	}
	if m.DB != nil && (m.DB.Postgres != nil || m.DB.Cassandra != nil || m.DB.Oracle != nil) {
		pb.Db = &manifestpb.DB{}
		if m.DB.Postgres != nil {
			pb.Db.Postgres = &manifestpb.PostgresDB{
				ConfigFrom: m.DB.Postgres.ConfigFrom,
				Queries:    m.DB.Postgres.Queries,
			}
		}
		if m.DB.Cassandra != nil {
			pb.Db.Cassandra = &manifestpb.CassandraDB{
				ConfigFrom: m.DB.Cassandra.ConfigFrom,
				Keyspace:   m.DB.Cassandra.Keyspace,
				Cql:        m.DB.Cassandra.CQL,
			}
		}
		if m.DB.Oracle != nil {
			pb.Db.Oracle = &manifestpb.OracleDB{
				ConfigFrom: m.DB.Oracle.ConfigFrom,
				Queries:    m.DB.Oracle.Queries,
			}
		}
	}
	if m.Queue != nil && (m.Queue.Kafka != nil || m.Queue.Nats != nil || m.Queue.RabbitMQ != nil) {
		pb.Queue = &manifestpb.Queue{}
		if m.Queue.Kafka != nil {
			pb.Queue.Kafka = &manifestpb.Kafka{
				BrokersFromConfig: m.Queue.Kafka.BrokersFromConfig,
				Brokers:           append([]string(nil), m.Queue.Kafka.Brokers...),
			}
		}
		if m.Queue.Nats != nil {
			pb.Queue.Nats = &manifestpb.Nats{
				ConfigFrom: m.Queue.Nats.ConfigFrom,
				Urls:       append([]string(nil), m.Queue.Nats.URLs...),
			}
		}
		if m.Queue.RabbitMQ != nil {
			pb.Queue.Rabbitmq = &manifestpb.RabbitMQ{
				ConfigFrom: m.Queue.RabbitMQ.ConfigFrom,
				Urls:       append([]string(nil), m.Queue.RabbitMQ.URLs...),
			}
		}
	}
	if m.Cache != nil && m.Cache.Redis != nil {
		pb.Cache = &manifestpb.Cache{
			Redis: &manifestpb.Redis{ConfigFrom: m.Cache.Redis.ConfigFrom},
		}
	}
	if m.Storage != nil && m.Storage.S3 != nil {
		pb.Storage = &manifestpb.Storage{
			S3: &manifestpb.S3{ConfigFrom: m.Storage.S3.ConfigFrom},
		}
	}
	if m.Auth != nil {
		pb.Auth = &manifestpb.Auth{
			Enabled: m.Auth.Enabled,
			ApiKeys: cloneMap(m.Auth.APIKeys),
		}
	}
	if m.Compatibility != nil {
		pb.Compatibility = &manifestpb.Compatibility{
			LegacyVersions: compatibilityModeToProto(m.Compatibility.LegacyVersions),
		}
	}
	return pb
}

func compatibilityModeFromProto(mode manifestpb.Compatibility_Mode) CompatibilityMode {
	switch mode {
	case manifestpb.Compatibility_DISABLED:
		return CompatibilityDisabled
	case manifestpb.Compatibility_ENABLED:
		return CompatibilityEnabled
	default:
		return CompatibilityUnspecified
	}
}

func compatibilityModeToProto(mode CompatibilityMode) manifestpb.Compatibility_Mode {
	switch mode {
	case CompatibilityDisabled:
		return manifestpb.Compatibility_DISABLED
	case CompatibilityEnabled:
		return manifestpb.Compatibility_ENABLED
	default:
		return manifestpb.Compatibility_UNSPECIFIED
	}
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func portToInt32(port int) int32 {
	if port < 0 || port > math.MaxUint16 {
		return 0
	}
	return int32(port)
}
