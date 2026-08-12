package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zengo/platform/internal/manifest"
)

// Add enables an optional manifest/config component in an existing service.
func Add(dir, target string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	var manifestPath string
	manifestPath, err = manifest.FindPath(absDir)
	if err != nil {
		return err
	}
	var m *manifest.Manifest
	m, err = manifest.Load(manifestPath)
	if err != nil {
		return err
	}

	ctx := addContext{
		dir:          absDir,
		manifestPath: manifestPath,
		manifest:     m,
	}

	target = normalizeAddTarget(target)
	switch target {
	case "db/postgres":
		return addPostgres(ctx)
	case "queue/kafka":
		return addKafka(ctx)
	case "queue/nats":
		return addNats(ctx)
	case "queue/rabbitmq":
		return addRabbitMQ(ctx)
	case "transport/grpc":
		return addGRPC(ctx)
	case "transport/rest":
		return addREST(ctx)
	case "observability":
		return addObservability(ctx)
	case "auth":
		return addAuth(ctx)
	case "cache/redis":
		return addRedis(ctx)
	case "storage/s3":
		return addS3(ctx)
	case "db/cassandra":
		return addCassandra(ctx)
	default:
		return fmt.Errorf(
			"unknown add target %q (try: db/postgres, queue/kafka, transport/grpc, cache/redis, storage/s3, observability)",
			target,
		)
	}
}

type addContext struct {
	dir          string
	manifestPath string
	manifest     *manifest.Manifest
}

func (c addContext) replacer() *strings.Replacer {
	return strings.NewReplacer(
		"{{HANDLER_PKG}}", c.manifest.HandlerPackage(),
		"{{SERVICE_NAME}}", c.serviceName(),
	)
}

func (c addContext) serviceName() string {
	if c.manifest != nil && c.manifest.Service.Name != "" {
		return c.manifest.Service.Name
	}
	return "app"
}

func normalizeAddTarget(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "postgres", "db/postgres":
		return "db/postgres"
	case "kafka", "queue/kafka":
		return "queue/kafka"
	case "nats", "queue/nats":
		return "queue/nats"
	case "rabbitmq", "queue/rabbitmq":
		return "queue/rabbitmq"
	case "grpc", "transport/grpc":
		return "transport/grpc"
	case "rest", "transport/rest":
		return "transport/rest"
	case "observability", "obs", "metrics", "tracing", "health":
		return "observability"
	case "auth":
		return "auth"
	case "redis", "cache/redis", "config/redis":
		return "cache/redis"
	case "s3", "storage/s3", "config/s3":
		return "storage/s3"
	case "cassandra", "db/cassandra":
		return "db/cassandra"
	default:
		return raw
	}
}

func addPostgres(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.DB == nil {
			m.DB = &manifest.DB{}
		}
		if m.DB.Postgres == nil {
			m.DB.Postgres = &manifest.PostgresDB{Queries: "queries/"}
		}
		if m.DB.Postgres.Queries == "" {
			m.DB.Postgres.Queries = "queries/"
		}
		return nil
	})
	if err != nil {
		return err
	}
	replacer := ctx.replacer()
	err = writeTemplateIfMissing(ctx.dir, "configs/postgres.yaml", "configs/postgres.yaml.tmpl", replacer)
	if err != nil {
		return err
	}
	err = writeTemplateIfMissing(
		ctx.dir,
		filepath.Join("queries", ctx.manifest.HandlerPackage()+".sql"),
		"queries/_handler_.sql.tmpl",
		replacer,
	)
	if err != nil {
		return err
	}
	if !fileExists(filepath.Join(ctx.dir, "sqlc.yaml")) {
		return copyEmbeddedTemplate(ctx.dir, "sqlc.yaml", "sqlc.yaml", replacer)
	}
	return nil
}

func addKafka(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Queue == nil {
			m.Queue = &manifest.Queue{}
		}
		if m.Queue.Kafka == nil {
			m.Queue.Kafka = &manifest.Kafka{}
		}
		if m.Queue.Kafka.BrokersFromConfig == "" {
			m.Queue.Kafka.BrokersFromConfig = "kafka"
		}
		if len(m.Queue.Kafka.Brokers) == 0 {
			m.Queue.Kafka.Brokers = []string{"localhost:9092"}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/kafka.yaml", "configs/kafka.yaml.tmpl", ctx.replacer())
}

func addNats(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Queue == nil {
			m.Queue = &manifest.Queue{}
		}
		if m.Queue.Nats == nil {
			m.Queue.Nats = &manifest.Nats{ConfigFrom: "nats"}
		}
		if m.Queue.Nats.ConfigFrom == "" {
			m.Queue.Nats.ConfigFrom = "nats"
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/nats.textproto", "configs/nats.textproto.tmpl", ctx.replacer())
}

func addRabbitMQ(ctx addContext) error {
	return manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Queue == nil {
			m.Queue = &manifest.Queue{}
		}
		if m.Queue.RabbitMQ == nil {
			m.Queue.RabbitMQ = &manifest.RabbitMQ{ConfigFrom: "rabbitmq"}
		}
		if m.Queue.RabbitMQ.ConfigFrom == "" {
			m.Queue.RabbitMQ.ConfigFrom = "rabbitmq"
		}
		return nil
	})
}

func addGRPC(ctx addContext) error {
	return manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Transports.GRPC == nil {
			m.Transports.GRPC = &manifest.GRPC{Port: 9090}
		}
		if m.Transports.GRPC.Port == 0 {
			m.Transports.GRPC.Port = 9090
		}
		return nil
	})
}

func addREST(ctx addContext) error {
	return manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Transports.REST == nil {
			m.Transports.REST = &manifest.REST{Port: 8080}
		}
		if m.Transports.REST.Port == 0 {
			m.Transports.REST.Port = 8080
		}
		return nil
	})
}

func addObservability(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		m.Observability.Metrics = true
		m.Observability.Tracing = true
		m.Observability.Health = true
		if m.Observability.TracingConfigFrom == "" {
			m.Observability.TracingConfigFrom = "tracing"
		}
		if m.Observability.LoggingConfigFrom == "" {
			m.Observability.LoggingConfigFrom = "logging"
		}
		return nil
	})
	if err != nil {
		return err
	}
	replacer := ctx.replacer()
	err = writeTemplateIfMissing(ctx.dir, "configs/logging.yaml", "configs/logging.yaml.tmpl", replacer)
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/tracing.yaml", "configs/tracing.yaml.tmpl", replacer)
}

func addAuth(ctx addContext) error {
	return manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Auth == nil {
			m.Auth = &manifest.Auth{}
		}
		m.Auth.Enabled = true
		if len(m.Auth.APIKeys) == 0 {
			m.Auth.APIKeys = map[string]string{"dev": "dev-token"}
		}
		return nil
	})
}

func addCassandra(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.DB == nil {
			m.DB = &manifest.DB{}
		}
		if m.DB.Cassandra == nil {
			m.DB.Cassandra = &manifest.CassandraDB{Keyspace: "app", CQL: "cql/"}
		}
		if m.DB.Cassandra.Keyspace == "" {
			m.DB.Cassandra.Keyspace = "app"
		}
		if m.DB.Cassandra.CQL == "" {
			m.DB.Cassandra.CQL = "cql/"
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/cassandra.yaml", "configs/cassandra.yaml.tmpl", ctx.replacer())
}

func addRedis(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Cache == nil {
			m.Cache = &manifest.Cache{}
		}
		if m.Cache.Redis == nil {
			m.Cache.Redis = &manifest.Redis{ConfigFrom: "redis"}
		}
		if m.Cache.Redis.ConfigFrom == "" {
			m.Cache.Redis.ConfigFrom = "redis"
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/redis.yaml", "configs/redis.yaml.tmpl", ctx.replacer())
}

func addS3(ctx addContext) error {
	err := manifest.Update(ctx.manifestPath, func(m *manifest.Manifest) error {
		if m.Storage == nil {
			m.Storage = &manifest.Storage{}
		}
		if m.Storage.S3 == nil {
			m.Storage.S3 = &manifest.S3{ConfigFrom: "s3"}
		}
		if m.Storage.S3.ConfigFrom == "" {
			m.Storage.S3.ConfigFrom = "s3"
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeTemplateIfMissing(ctx.dir, "configs/s3.yaml", "configs/s3.yaml.tmpl", ctx.replacer())
}

func writeTemplateIfMissing(baseDir, relPath, templatePath string, replacer *strings.Replacer) error {
	target := filepath.Join(baseDir, relPath)
	if fileExists(target) {
		return nil
	}
	data, err := templates.ReadFile(filepath.Join("templates", templatePath))
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(target), 0o755)
	if err != nil {
		return err
	}
	content := replacer.Replace(string(data))
	return os.WriteFile(target, []byte(content), 0o644)
}

func copyEmbeddedTemplate(baseDir, name, target string, replacer *strings.Replacer) error {
	data, err := templates.ReadFile(filepath.Join("templates", name))
	if err != nil {
		return err
	}
	path := filepath.Join(baseDir, target)
	return os.WriteFile(path, []byte(replacer.Replace(string(data))), 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
