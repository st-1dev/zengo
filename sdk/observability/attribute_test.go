package observability

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/grpc/metadata"
)

func TestAttribute_toOpenTelemetry_AllTypes(t *testing.T) {
	tests := []struct {
		expected attribute.KeyValue
		attr     Attribute
		name     string
	}{
		{
			name:     "bool",
			attr:     BoolAttribute("flag", true),
			expected: attribute.Bool("flag", true),
		},
		{
			name:     "bool slice",
			attr:     BoolSliceAttribute("flags", []bool{true, false}),
			expected: attribute.BoolSlice("flags", []bool{true, false}),
		},
		{
			name:     "bool slice nil",
			attr:     BoolSliceAttribute("flags", []bool(nil)),
			expected: attribute.BoolSlice("flags", []bool(nil)),
		},
		{
			name:     "null bool nil",
			attr:     NullBoolAttribute("flag", nil),
			expected: attribute.String("flag", "<nil>"),
		},
		{
			name:     "null bool value",
			attr:     NullBoolAttribute("flag", new(true)),
			expected: attribute.Bool("flag", true),
		},
		{
			name:     "int",
			attr:     IntAttribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "int slice",
			attr:     IntSliceAttribute("counts", []int{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "int8",
			attr:     Int8Attribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "int8 slice",
			attr:     Int8SliceAttribute("counts", []int8{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "null int8 nil",
			attr:     NullInt8Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null int8 value",
			attr:     NullInt8Attribute("count", new(int8(8))),
			expected: attribute.Int("count", 8),
		},
		{
			name:     "int16",
			attr:     Int16Attribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "int16 slice",
			attr:     Int16SliceAttribute("counts", []int16{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "null int16 nil",
			attr:     NullInt16Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null int16 value",
			attr:     NullInt16Attribute("count", new(int16(16))),
			expected: attribute.Int("count", 16),
		},
		{
			name:     "int32",
			attr:     Int32Attribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "int32 slice",
			attr:     Int32SliceAttribute("counts", []int32{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "null int32 nil",
			attr:     NullInt32Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null int32 value",
			attr:     NullInt32Attribute("count", new(int32(32))),
			expected: attribute.Int("count", 32),
		},
		{
			name:     "int64",
			attr:     Int64Attribute("big", int64(999999999)),
			expected: attribute.Int64("big", 999999999),
		},
		{
			name:     "int64 slice",
			attr:     Int64SliceAttribute("bigs", []int64{4, 5, 6}),
			expected: attribute.Int64Slice("bigs", []int64{4, 5, 6}),
		},
		{
			name:     "null int64 nil",
			attr:     NullInt64Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null int64 value",
			attr:     NullInt64Attribute("count", new(int64(64))),
			expected: attribute.Int64("count", 64),
		},
		{
			name:     "null int nil",
			attr:     NullIntAttribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null int value",
			attr:     NullIntAttribute("count", new(42)),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "uint8",
			attr:     Uint8Attribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "uint8 slice",
			attr:     Uint8SliceAttribute("counts", []uint8{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "null uint8 nil",
			attr:     NullUint8Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null uint8 value",
			attr:     NullUint8Attribute("count", new(uint8(8))),
			expected: attribute.Int("count", 8),
		},
		{
			name:     "uint16",
			attr:     Uint16Attribute("count", 42),
			expected: attribute.Int("count", 42),
		},
		{
			name:     "uint16 slice",
			attr:     Uint16SliceAttribute("counts", []uint16{1, 2, 3}),
			expected: attribute.IntSlice("counts", []int{1, 2, 3}),
		},
		{
			name:     "null uint16 nil",
			attr:     NullUint16Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null uint16 value",
			attr:     NullUint16Attribute("count", new(uint16(16))),
			expected: attribute.Int("count", 16),
		},
		{
			name:     "uint32",
			attr:     Uint32Attribute("count", 42),
			expected: attribute.Int64("count", 42),
		},
		{
			name:     "uint32 slice",
			attr:     Uint32SliceAttribute("counts", []uint32{1, 2, 3}),
			expected: attribute.Int64Slice("counts", []int64{1, 2, 3}),
		},
		{
			name:     "null uint32 nil",
			attr:     NullUint32Attribute("count", nil),
			expected: attribute.String("count", "<nil>"),
		},
		{
			name:     "null uint32 value",
			attr:     NullUint32Attribute("count", new(uint32(32))),
			expected: attribute.Int64("count", 32),
		},
		{
			name:     "float32",
			attr:     Float32Attribute("rate", 3),
			expected: attribute.Float64("rate", 3),
		},
		{
			name:     "float32 slice",
			attr:     Float32SliceAttribute("rates", []float32{1, 3}),
			expected: attribute.Float64Slice("rates", []float64{1, 3}),
		},
		{
			name:     "null float32 nil",
			attr:     NullFloat32Attribute("rate", nil),
			expected: attribute.String("rate", "<nil>"),
		},
		{
			name:     "null float32 value",
			attr:     NullFloat32Attribute("rate", new(float32(3.5))),
			expected: attribute.Float64("rate", 3.5),
		},
		{
			name:     "float64",
			attr:     Float64Attribute("rate", 3.14),
			expected: attribute.Float64("rate", 3.14),
		},
		{
			name:     "float64 slice",
			attr:     Float64SliceAttribute("rates", []float64{1.2, 3.4}),
			expected: attribute.Float64Slice("rates", []float64{1.2, 3.4}),
		},
		{
			name:     "null float64 nil",
			attr:     NullFloat64Attribute("rate", nil),
			expected: attribute.String("rate", "<nil>"),
		},
		{
			name:     "null float64 value",
			attr:     NullFloat64Attribute("rate", new(6.25)),
			expected: attribute.Float64("rate", 6.25),
		},
		{
			name:     "string",
			attr:     StringAttribute("name", "hello"),
			expected: attribute.String("name", "hello"),
		},
		{
			name:     "string slice",
			attr:     StringSliceAttribute("names", []string{"a", "b"}),
			expected: attribute.StringSlice("names", []string{"a", "b"}),
		},
		{
			name:     "null string nil",
			attr:     NullStringAttribute("name", nil),
			expected: attribute.String("name", "<nil>"),
		},
		{
			name:     "null string value",
			attr:     NullStringAttribute("name", new("hello")),
			expected: attribute.String("name", "hello"),
		},
		{
			name:     "stringer",
			attr:     StringerAttribute("status", url.UserPassword("user", "password")),
			expected: attribute.Stringer("status", url.UserPassword("user", "password")),
		},
		{
			name:     "stringer nil",
			attr:     StringerAttribute("status", nil),
			expected: attribute.String("status", "<nil>"),
		},
		{
			name:     "stringer typed nil",
			attr:     StringerAttribute("status", (*url.Userinfo)(nil)),
			expected: attribute.String("status", "<nil>"),
		},
		{
			name:     "stringer slice",
			attr:     StringerSliceAttribute("status", []*url.Userinfo{url.UserPassword("user", "password")}),
			expected: attribute.StringSlice("status", []string{"user:password"}),
		},
		{
			name:     "stringer slice nil",
			attr:     StringerSliceAttribute("status", []*url.Userinfo(nil)),
			expected: attribute.StringSlice("status", nil),
		},
		{
			name:     "stringer slice empty",
			attr:     StringerSliceAttribute("status", []*url.Userinfo{}),
			expected: attribute.StringSlice("status", []string{}),
		},
		{
			name:     "stringer slice with nil element",
			attr:     StringerSliceAttribute("status", []*url.Userinfo{nil, url.UserPassword("user", "password")}),
			expected: attribute.StringSlice("status", []string{"<nil>", "user:password"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.attr.transform())
		})
	}
}

func TestGRPCmetadataAttribute(t *testing.T) {
	t.Run("no metadata in context", func(t *testing.T) {
		a := GRPCmetadataAttribute(context.Background(), "x-request-id")
		assert.Equal(t, attribute.StringSlice("x-request-id", nil), a.transform())
	})

	t.Run("metadata key does not exist", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "123"))
		a := GRPCmetadataAttribute(ctx, "x-request-id")
		assert.Equal(t, attribute.StringSlice("x-request-id", nil), a.transform())
	})

	t.Run("metadata key exists", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs("x-request-id", "first", "x-request-id", "second"),
		)

		a := GRPCmetadataAttribute(ctx, "x-request-id")
		assert.Equal(t, attribute.StringSlice("x-request-id", []string{"first", "second"}), a.transform())
	})
}

func TestIsNil(t *testing.T) {
	type sample struct{}

	var userinfo *url.Userinfo

	assert.True(t, isNil(nil))
	assert.True(t, isNil(userinfo))
	assert.False(t, isNil(sample{}))
	assert.False(t, isNil(42))
}

func BenchmarkStringerSliceAttribute(b *testing.B) {
	ctx := context.TODO()

	v := []*url.Userinfo{
		url.UserPassword("user", "password"),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, endSpan := StartSpan(ctx, StringerSliceAttribute("key", v))
		endSpan()
	}
}
