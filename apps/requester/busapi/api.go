package busapi

import (
	"go.uber.org/fx"
)

var Module = fx.Options(
	fx.Provide(func() *Client {
		return NewClient(ClientConfig{})
	}),
)
