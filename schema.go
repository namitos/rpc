package rpc

import (
	"github.com/namitos/rpc/schema"
)

type Schema struct {
	Info struct {
		Description string `json:"description,omitempty"`
		License     struct {
			Name string `json:"name,omitempty"`
			URL  string `json:"url,omitempty"`
		} `json:"license,omitempty"`
		Title   string `json:"title,omitempty"`
		Version string `json:"version,omitempty"`
	} `json:"info,omitempty"`
	OpenRPC string          `json:"openrpc,omitempty"`
	Methods []*MethodSchema `json:"methods,omitempty"`
}

type MethodSchema struct {
	Name    string              `json:"name,omitempty"`
	Summary string              `json:"summary,omitempty"`
	Params  []MethodSchemaParam `json:"params,omitempty"`
	Result  MethodSchemaParam   `json:"result,omitempty"`
}

type MethodSchemaParam struct {
	Name     string         `json:"name,omitempty"`
	Required bool           `json:"required,omitempty"`
	Schema   *schema.Schema `json:"schema,omitempty"`
}
