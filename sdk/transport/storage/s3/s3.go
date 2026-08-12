package s3

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"zengo/platform/sdk/observability"
	"zengo/platform/sdk/tlsconfig"

	s3cfg "zengo/platform/api/config/storage/s3"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

// Client combines an instrumented AWS S3 client with its default object location.
type Client struct {
	// S3 is the instrumented AWS SDK client.
	S3 *s3.Client
	// Bucket is the default bucket configured for the client.
	Bucket string
	// Prefix is the default object key prefix configured for the client.
	Prefix string
}

// NewClient opens an instrumented S3 client from typed config.
//
// The returned Client is safe to reuse across requests.
func NewClient(ctx context.Context, cfg *s3cfg.Config) (*Client, error) {
	if cfg == nil || cfg.GetSpec() == nil {
		return nil, fmt.Errorf("s3 config is nil")
	}
	spec := cfg.GetSpec()
	bucket := spec.GetBucket()
	spanCtx, endSpan := observability.StartSpan(
		ctx,
		observability.StringAttribute("cloud.storage.provider", "aws_s3"),
		observability.StringAttribute("cloud.storage.bucket", bucket),
	)
	ctx = spanCtx
	defer endSpan()

	awsCfg, err := loadAWSConfig(ctx, spec)
	if err != nil {
		observability.RecordException(ctx, err, "load s3 aws config")
		return nil, err
	}
	otelaws.AppendMiddlewares(&awsCfg.APIOptions)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		endpoint := spec.GetEndpoint()
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		if spec.ForcePathStyle != nil {
			o.UsePathStyle = spec.GetForcePathStyle()
		}
	})
	return &Client{
		S3:     client,
		Bucket: spec.GetBucket(),
		Prefix: spec.GetPrefix(),
	}, nil
}

func loadAWSConfig(ctx context.Context, spec *s3cfg.Spec) (aws.Config, error) {
	region := spec.GetRegion()
	if region == "" {
		region = "us-east-1"
	}
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	key := spec.GetAccessKeyId()
	if key != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			key,
			spec.GetSecretAccessKey(),
			spec.GetSessionToken(),
		)))
	}
	clientTLS, err := tlsconfig.ClientConfig(tlsconfig.ClientOptionsFromProto(spec.GetTls()))
	if err != nil {
		return aws.Config{}, fmt.Errorf("s3 tls: %w", err)
	}
	httpClient := &http.Client{}
	if spec.RequestTimeout != nil {
		httpClient.Timeout = spec.GetRequestTimeout().AsDuration()
	}
	if clientTLS != nil {
		httpClient.Transport = &http.Transport{TLSClientConfig: clientTLS}
	}
	if httpClient.Timeout != 0 || httpClient.Transport != nil {
		opts = append(opts, awsconfig.WithHTTPClient(httpClient))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

// ClientFromMinIO builds a client for local MinIO development.
func ClientFromMinIO(ctx context.Context, host string, port int, bucket string) (*Client, error) {
	if port <= 0 || port > math.MaxUint16 {
		return nil, fmt.Errorf("s3 port must be between 1 and 65535")
	}
	if host == "" {
		host = "localhost"
	}
	if bucket == "" {
		bucket = "zengo"
	}
	region := "us-east-1"
	endpoint := fmt.Sprintf("http://%s:%d", host, port)
	accessKeyID := "minioadmin"
	secretAccessKey := "minioadmin"
	forcePathStyle := true
	return NewClient(ctx, &s3cfg.Config{
		Kind: "s3",
		Spec: &s3cfg.Spec{
			Bucket:          &bucket,
			Region:          &region,
			Endpoint:        &endpoint,
			AccessKeyId:     &accessKeyID,
			SecretAccessKey: &secretAccessKey,
			ForcePathStyle:  &forcePathStyle,
		},
	})
}

// PortFromEnv parses a development port override and falls back on invalid input.
func PortFromEnv(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port <= 0 || port > math.MaxUint16 {
		return fallback
	}
	return port
}
