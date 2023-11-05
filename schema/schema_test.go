package schema

import (
	"encoding/json"
	"log"
	"reflect"
	"testing"
)

//example:

type TestType1 struct {
	Zzz  string
	Aaa  []string
	Aaa1 []Pair
	Xxx  map[string]float64
}

type Pair [2]string

type TestType struct {
	Location   [2]float64         `label:"Location label" json:"location" weight:"123" validate:"required" enum:"adfs,fsdf,gdfgdh,"`
	ExampleMap map[string]float64 `label:"exampleMap label" json:"exampleMap" widget:"custom-map-input,images,sortable,URLPrefix=/asdasd/,cols=5,zz=2.2"`
	Child      *TestType1         `json:"child"`
	Child1     []*TestType1
}

func TestGet(t *testing.T) {
	s := TestType{
		Location:   [2]float64{1, 2},
		ExampleMap: map[string]float64{"z": 123},
	}

	schemaItem := Get(reflect.TypeOf(s))
	schemaItemBytes, _ := json.MarshalIndent(schemaItem, "  ", "  ")
	log.Println(string(schemaItemBytes))
	//t.Fatal(string(schemaItemBytes))
}
