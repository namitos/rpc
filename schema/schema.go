package schema

import (
	"reflect"
	"strconv"
	"strings"
)

const (
	TypeNameMap    = "map"
	TypeNameObject = "object"
	TypeNameArray  = "array"
)

var complexTypeNames = Enum{TypeNameMap, TypeNameObject, TypeNameArray}

var primitiveNumberKinds = []reflect.Kind{
	reflect.Int,
	reflect.Int8,
	reflect.Int16,
	reflect.Int32,
	reflect.Int64,
	reflect.Uint,
	reflect.Uint8,
	reflect.Uint16,
	reflect.Uint32,
	reflect.Uint64,
	reflect.Float32,
	reflect.Float64,
}

type Schema struct {
	Type           string             `json:"type,omitempty"`
	TypeName       string             `json:"typeName,omitempty"`
	Label          string             `json:"label,omitempty"`
	Properties     map[string]*Schema `json:"properties,omitempty"`
	Items          *Schema            `json:"items,omitempty"`
	Weight         int64              `json:"weight,omitempty"`
	Enum           Enum               `json:"enum,omitempty"`
	Required       bool               `json:"required,omitempty"`
	WidgetSettings WidgetSettings     `json:"widgetSettings,omitempty"` //for Object.assign to component
}

type WidgetSettings map[string]interface{}

func Get(t reflect.Type) *Schema {
	schema, _ := fillValue(t, map[string]string{}, nil)
	return schema
}

func getKindPrimitiveType(kind reflect.Kind) string {
	if kind == reflect.String {
		return "string"
	}
	if kind == reflect.Bool {
		return "boolean"
	}
	for _, pnk := range primitiveNumberKinds {
		if kind == pnk {
			return "number"
		}
	}
	return ""
}

func fillValue(t reflect.Type, tags map[string]string, parentTypes []reflect.Type) (*Schema, reflect.Value) {
	var v reflect.Value
	var baseType reflect.Type
	if t.Kind() == reflect.Ptr {
		v = reflect.New(t.Elem())
		baseType = t.Elem()
	} else {
		v = reflect.New(t).Elem()
		baseType = t
	}

	var weight int64
	if tags["weight"] != "" {
		weight, _ = strconv.ParseInt(tags["weight"], 10, 64)
	}
	var enum Enum
	if tags["enum"] != "" {
		enum = strings.Split(tags["enum"], ",")
	}
	var required bool
	if tags["validate"] != "" {
		validations := strings.Split(tags["validate"], ",")
		for _, str := range validations {
			if str == "required" {
				required = true
			}
		}
	}

	widgetSettingsSplitted := strings.Split(tags["widget"], ",")
	widgetSettings := WidgetSettings{}
	if widgetSettingsSplitted[0] != "" {
		widgetSettings["name"] = widgetSettingsSplitted[0]
	}
	if tags["vocabulary"] != "" {
		widgetSettings["vocabulary"] = tags["vocabulary"]
	}
	for i, setting := range widgetSettingsSplitted { //for now only flags
		if i == 0 || setting == "" {
			continue
		}
		settingKV := strings.Split(setting, "=")
		if len(settingKV) == 1 {
			widgetSettings[settingKV[0]] = true
		}
		if len(settingKV) == 2 {
			v := settingKV[1]
			vInt, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				widgetSettings[settingKV[0]] = vInt
				continue
			}
			vFloat, err := strconv.ParseFloat(v, 64)
			if err == nil {
				widgetSettings[settingKV[0]] = vFloat
				continue
			}
			widgetSettings[settingKV[0]] = v
		}
	}
	if len(widgetSettings) == 0 {
		widgetSettings = nil
	}

	schemaOut := &Schema{
		Type:           getKindPrimitiveType(baseType.Kind()),
		TypeName:       t.String(),
		Label:          tags["label"],
		Weight:         weight,
		Enum:           enum,
		Required:       required,
		WidgetSettings: widgetSettings,
	}

	for _, pt := range parentTypes {
		if pt == t {
			//no recursy for types
			return nil, reflect.Value{}
		}
	}
	parentTypes = append(parentTypes, t)

	if baseType.Kind() == reflect.Map {
		schemaOut.Type = TypeNameMap
		v = reflect.MakeMap(t)
		keyKind := baseType.Key().Kind()
		schema, filledValue := fillValue(baseType.Elem(), nil, parentTypes)
		if keyKind == reflect.String {
			v.SetMapIndex(reflect.ValueOf("q"), filledValue)
		}
		for _, pnk := range primitiveNumberKinds {
			if keyKind == pnk {
				v.SetMapIndex(reflect.ValueOf(0), filledValue)
			}
		}
		schemaOut.Items = schema

	} else if baseType.Kind() == reflect.Slice {
		schemaOut.Type = TypeNameArray
		var vSlice reflect.Value
		if v.Kind() == reflect.Ptr {
			vSlice = v.Elem()
		} else {
			vSlice = v
		}
		schema, filledValue := fillValue(baseType.Elem(), nil, parentTypes)
		vSlice = reflect.Append(vSlice, filledValue)
		if v.Kind() == reflect.Ptr {
			v.Elem().Set(vSlice)
		} else {
			v.Set(vSlice)
		}
		schemaOut.Items = schema

	} else if baseType.Kind() == reflect.Struct {
		schemaOut.Type = TypeNameObject
		schemaOut.Properties = map[string]*Schema{}
		fieldsCount := baseType.NumField()
		for i := 0; i < fieldsCount; i++ {
			f := baseType.Field(i)
			if !f.IsExported() {
				continue
			}
			widgetTag := f.Tag.Get("widget")
			if widgetTag == "hidden" {
				continue
			}
			jsonTag := f.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}
			fieldName := f.Name
			fieldNameTag := strings.Split(jsonTag, ",")
			if len(fieldNameTag) > 0 && fieldNameTag[0] != "" {
				fieldName = fieldNameTag[0]
			}
			schema, _ := fillValue(f.Type, map[string]string{
				"label":      f.Tag.Get("label"),
				"vocabulary": f.Tag.Get("vocabulary"),
				"weight":     f.Tag.Get("weight"),
				"validate":   f.Tag.Get("validate"),
				"enum":       f.Tag.Get("enum"),
				"widget":     widgetTag,
			}, parentTypes)
			schemaOut.Properties[fieldName] = schema
		}

	}

	return schemaOut, v
}
