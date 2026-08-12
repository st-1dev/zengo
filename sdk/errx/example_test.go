package errx_test

import (
	"errors"
	"zengo/platform/sdk/errx"

	"google.golang.org/grpc/codes"
)

func ExampleNew() {
	err := errx.New(
		codes.InvalidArgument,
		"email is required",
		errx.Public("invalid request"),
		errx.Fields(errx.Field{Key: "field", Value: "email"}),
	)
	_ = err
}

func ExampleWrap() {
	err := errx.Wrap(
		errors.New("dial tcp: timeout"),
		codes.Unavailable,
		"connect postgres",
		errx.Public("service temporarily unavailable"),
	)
	_ = err
}
