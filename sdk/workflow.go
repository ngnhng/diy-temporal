package sdk

import "context"

type (
	// Future represents the result of an asynchronous computation.
	Future interface {
		// Get blocks until the future is ready. When ready it either returns non nil error or assigns result value to
		// the provided pointer.
		// Example:
		//  var v string
		//  if err := f.Get(ctx, &v); err != nil {
		//      return err
		//  }
		//
		// The valuePtr parameter can be nil when the encoded result value is not needed.
		// Example:
		//  err = f.Get(ctx, nil)
		//
		// Note, values should not be reused for extraction here because merging on
		// top of existing values may result in unexpected behavior similar to
		// json.Unmarshal.
		Get(ctx context.Context, valuePtr interface{}) error

		// When true Get is guaranteed to not block
		IsReady() bool
	}
)

func ExecuteActivity(ctx context.Context, fn any, args ...any) Future {

}
