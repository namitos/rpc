//JSON-RPC clent and server implementation; supports HTTP and TCP
package rpc

import (
	"context"
)

func CallSingle(client Client, ctx context.Context, method string, params interface{}, result interface{}) error {
	output := []Output{{
		Result: result,
	}}
	err := client.Call(ctx, &[]Input{{
		Method: method,
		Params: params,
	}}, &output)
	if err != nil {
		return err
	}
	if output[0].Error != nil {
		return output[0].Error
	}
	return nil
}
