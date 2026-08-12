package policy_test

import (
	"context"
	"time"
	"zengo/platform/sdk/policy"
)

func ExampleNewExecutor() {
	exec := policy.NewExecutor(policy.Options{
		Timeout: time.Second,
		Retry: policy.Retry{
			Attempts: 2,
		},
	})
	_ = exec.Do(context.Background(), func(context.Context) error {
		return nil
	})
}
