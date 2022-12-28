package rpc

import (
	"github.com/namitos/rpc/schema"
)

type SchemaRoot struct {
	Info    SchemaRootInfo  `json:"info,omitempty"`
	OpenRPC string          `json:"openrpc"`
	Methods []*MethodSchema `json:"methods,omitempty"`
	Servers []*SchemaServer `json:"servers,omitempty"`
}

type SchemaRootInfo struct {
	Description string `json:"description"`
	License     struct {
		Name string `json:"name,omitempty"`
		URL  string `json:"url,omitempty"`
	} `json:"license,omitempty"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type SchemaServer struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type MethodSchema struct {
	Name        string              `json:"name,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Params      []MethodSchemaParam `json:"params"`
	Result      MethodSchemaParam   `json:"result"`
	Examples    []MethodExample     `json:"examples,omitempty"`
}

type MethodExample struct {
	Name        string                  `json:"name,omitempty"`
	Summary     string                  `json:"summary,omitempty"`
	Description string                  `json:"description,omitempty"`
	Params      []MethodExampleVariable `json:"params,omitempty"`
	Result      MethodExampleVariable   `json:"result,omitempty"`
}

type MethodExampleVariable struct {
	Name  string      `json:"name,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

type MethodSchemaParam struct {
	Name     string         `json:"name,omitempty"`
	Required bool           `json:"required,omitempty"`
	Schema   *schema.Schema `json:"schema,omitempty"`
}
