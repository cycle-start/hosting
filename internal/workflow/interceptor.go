package workflow

import (
	"context"
	"errors"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/temporal"
)

// ErrorTypingInterceptor is a Temporal worker interceptor that wraps activity
// errors with the activity name as the error type. This makes errors much more
// visible in the Temporal UI — each failed activity shows its name as the
// error type instead of a generic "ApplicationError".
type ErrorTypingInterceptor struct {
	interceptor.WorkerInterceptorBase
}

func (e *ErrorTypingInterceptor) InterceptActivity(
	ctx context.Context,
	next interceptor.ActivityInboundInterceptor,
) interceptor.ActivityInboundInterceptor {
	return &errorTypingActivityInterceptor{
		ActivityInboundInterceptorBase: interceptor.ActivityInboundInterceptorBase{},
		next:                          next,
	}
}

type errorTypingActivityInterceptor struct {
	interceptor.ActivityInboundInterceptorBase
	next interceptor.ActivityInboundInterceptor
}

func (e *errorTypingActivityInterceptor) Init(outbound interceptor.ActivityOutboundInterceptor) error {
	return e.next.Init(outbound)
}

func (e *errorTypingActivityInterceptor) ExecuteActivity(
	ctx context.Context,
	in *interceptor.ExecuteActivityInput,
) (interface{}, error) {
	result, err := e.next.ExecuteActivity(ctx, in)
	if err != nil {
		// Don't double-wrap errors that already have a type.
		var appErr *temporal.ApplicationError
		if errors.As(err, &appErr) && appErr.Type() != "" {
			return result, err
		}

		actName := activity.GetInfo(ctx).ActivityType.Name
		return result, temporal.NewApplicationError(err.Error(), actName, err)
	}
	return result, nil
}
