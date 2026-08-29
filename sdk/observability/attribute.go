package observability

import (
	"context"
	"fmt"
	"reflect"

	"google.golang.org/grpc/metadata"

	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/constraints"
)

// Attribute represents a lazily-built key-value attribute for a span.
type Attribute struct {
	// transform converts the attribute into the OpenTelemetry representation.
	transform func() (kv attribute.KeyValue)
}

// BoolAttribute returns a bool attribute.
func BoolAttribute(key string, value bool) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Bool(key, value)
		},
	}
}

// NullBoolAttribute returns a *bool attribute.
func NullBoolAttribute(key string, value *bool) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return BoolAttribute(key, *value)
}

// BoolSliceAttribute returns a []bool attribute.
func BoolSliceAttribute(key string, value []bool) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.BoolSlice(key, value)
		},
	}
}

// Int8Attribute returns a int8 attribute.
func Int8Attribute(key string, value int8) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, int(value))
		},
	}
}

// NullInt8Attribute returns a *int8 attribute.
func NullInt8Attribute(key string, value *int8) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Int8Attribute(key, *value)
}

// Int8SliceAttribute returns a []int8 attribute.
func Int8SliceAttribute(key string, value []int8) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, transformNumberSlice[int8, int](value))
		},
	}
}

// Int16Attribute returns a int16 attribute.
func Int16Attribute(key string, value int16) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, int(value))
		},
	}
}

// NullInt16Attribute returns a *int16 attribute.
func NullInt16Attribute(key string, value *int16) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Int16Attribute(key, *value)
}

// Int16SliceAttribute returns a []int16 attribute.
func Int16SliceAttribute(key string, value []int16) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, transformNumberSlice[int16, int](value))
		},
	}
}

// Int32Attribute returns a int32 attribute.
func Int32Attribute(key string, value int32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, int(value))
		},
	}
}

// NullInt32Attribute returns a *int32 attribute.
func NullInt32Attribute(key string, value *int32) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Int32Attribute(key, *value)
}

// Int32SliceAttribute returns a []int32 attribute.
func Int32SliceAttribute(key string, value []int32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, transformNumberSlice[int32, int](value))
		},
	}
}

// Int64Attribute returns a int64 attribute.
func Int64Attribute(key string, value int64) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int64(key, value)
		},
	}
}

// NullInt64Attribute returns a *int64 attribute.
func NullInt64Attribute(key string, value *int64) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Int64Attribute(key, *value)
}

// Int64SliceAttribute returns a []int64 attribute.
func Int64SliceAttribute(key string, value []int64) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int64Slice(key, value)
		},
	}
}

// IntAttribute returns a int attribute.
func IntAttribute(key string, value int) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, value)
		},
	}
}

// NullIntAttribute returns a *int attribute.
func NullIntAttribute(key string, value *int) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return IntAttribute(key, *value)
}

// IntSliceAttribute returns a []int attribute.
func IntSliceAttribute(key string, value []int) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, value)
		},
	}
}

// Uint8Attribute returns a uint8 attribute.
func Uint8Attribute(key string, value uint8) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, int(value))
		},
	}
}

// NullUint8Attribute returns a *uint8 attribute.
func NullUint8Attribute(key string, value *uint8) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Uint8Attribute(key, *value)
}

// Uint8SliceAttribute returns a []uint8 attribute.
func Uint8SliceAttribute(key string, value []uint8) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, transformNumberSlice[uint8, int](value))
		},
	}
}

// Uint16Attribute returns a uint16 attribute.
func Uint16Attribute(key string, value uint16) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int(key, int(value))
		},
	}
}

// NullUint16Attribute returns a *uint16 attribute.
func NullUint16Attribute(key string, value *uint16) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Uint16Attribute(key, *value)
}

// Uint16SliceAttribute returns a []uint16 attribute.
func Uint16SliceAttribute(key string, value []uint16) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.IntSlice(key, transformNumberSlice[uint16, int](value))
		},
	}
}

// Uint32Attribute returns a uint32 attribute.
func Uint32Attribute(key string, value uint32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int64(key, int64(value))
		},
	}
}

// NullUint32Attribute returns a *uint32 attribute.
func NullUint32Attribute(key string, value *uint32) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Uint32Attribute(key, *value)
}

// Uint32SliceAttribute returns a []uint32 attribute.
func Uint32SliceAttribute(key string, value []uint32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Int64Slice(key, transformNumberSlice[uint32, int64](value))
		},
	}
}

// Float32Attribute returns a float32 attribute.
func Float32Attribute(key string, value float32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Float64(key, float64(value))
		},
	}
}

// NullFloat32Attribute returns a *float32 attribute.
func NullFloat32Attribute(key string, value *float32) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Float32Attribute(key, *value)
}

// Float32SliceAttribute returns a []float32 attribute.
func Float32SliceAttribute(key string, value []float32) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Float64Slice(key, transformNumberSlice[float32, float64](value))
		},
	}
}

// Float64Attribute returns a float64 attribute.
func Float64Attribute(key string, value float64) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Float64(key, value)
		},
	}
}

// NullFloat64Attribute returns a *float64 attribute.
func NullFloat64Attribute(key string, value *float64) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return Float64Attribute(key, *value)
}

// Float64SliceAttribute returns a []float64 attribute.
func Float64SliceAttribute(key string, value []float64) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Float64Slice(key, value)
		},
	}
}

// StringAttribute returns a string attribute.
func StringAttribute(key, value string) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.String(key, value)
		},
	}
}

// NullStringAttribute returns a *string attribute.
func NullStringAttribute(key string, value *string) (attr Attribute) {
	if value == nil {
		return StringAttribute(key, "<nil>")
	}
	return StringAttribute(key, *value)
}

// StringSliceAttribute returns a []string attribute.
func StringSliceAttribute(key string, value []string) (attr Attribute) {
	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.StringSlice(key, value)
		},
	}
}

// StringerAttribute returns a fmt.Stringer attribute.
func StringerAttribute(key string, value fmt.Stringer) (attr Attribute) {
	if isNil(value) {
		return StringAttribute(key, "<nil>")
	}

	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			return attribute.Stringer(key, value)
		},
	}
}

// StringerSliceAttribute returns a []fmt.Stringer attribute.
func StringerSliceAttribute[T fmt.Stringer](key string, value []T) (attr Attribute) {
	if value == nil {
		return StringSliceAttribute(key, nil)
	}

	return Attribute{
		transform: func() (kv attribute.KeyValue) {
			out := make([]string, 0, len(value))
			for _, it := range value {
				if isNil(it) {
					out = append(out, "<nil>")
					continue
				}
				out = append(out, it.String())
			}
			return attribute.StringSlice(key, out)
		},
	}
}

// GRPCmetadataAttribute returns a gRPC metadata attribute.
func GRPCmetadataAttribute(ctx context.Context, key string) (attr Attribute) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return StringSliceAttribute(key, nil)
	}

	values := md.Get(key)
	return StringSliceAttribute(key, values)
}

// isNil reports whether x is a nil pointer-like value.
func isNil(x any) (flag bool) {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	switch v.Kind() { //nolint:exhaustive // only pointer-like kinds can be nil here
	case reflect.Pointer, reflect.UnsafePointer, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// transformNumberSlice converts one numeric slice type into another.
func transformNumberSlice[
	T constraints.Integer | constraints.Float,
	S constraints.Integer | constraints.Float,
](in []T) (out []S) {
	if in == nil {
		return nil
	}

	out = make([]S, 0, len(in))
	for _, it := range in {
		out = append(out, S(it))
	}
	return out
}
