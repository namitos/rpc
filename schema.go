package rpc

import (
	"github.com/namitos/rpc/schema"
)

type Schema struct {
	Info struct {
		Description string `json:"description"`
		License     struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"license"`
		Title   string `json:"title"`
		Version string `json:"version"`
	} `json:"info"`
	OpenRPC string          `json:"openrpc"`
	Methods []*MethodSchema `json:"methods"`
}

type MethodSchema struct {
	Name    string              `json:"name"`
	Summary string              `json:"summary"`
	Params  []MethodSchemaParam `json:"params"`
	Result  MethodSchemaParam   `json:"result"`
}

type MethodSchemaParam struct {
	Name     string         `json:"name"`
	Required bool           `json:"required"`
	Schema   *schema.Schema `json:"schema"`
}
