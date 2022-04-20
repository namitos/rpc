package schema

import (
	"reflect"
	"strconv"
	"strings"
)

type Schema struct {
	Type           string             `json:"type,omitempty"`
	Label          string             `json:"label,omitempty"`
	Properties     map[string]*Schema `json:"properties,omitempty"`
	Items          *Schema            `json:"items,omitempty"`
	Required       bool               `json:"required,omitempty"`
	Weight         int64              `json:"weight,omitempty"`
	WidgetSettings WidgetSettings     `json:"widgetSettings,omitempty"` //for Object.assign to component
}

type WidgetSettings map[string]interface{}

func Get(v reflect.Value) *Schema {
	return getSchema(v, map[string]string{})
}

func getSchema(v reflect.Value, tags map[string]string) *Schema {
	if !v.IsValid() {
		return nil
	}
	typeOfS := v.Type()
	kind := v.Kind()

	var weight int64
	if tags["weight"] != "" {
		weight, _ = strconv.ParseInt(tags["weight"], 10, 64)
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

	if kind == reflect.Int64 || kind == reflect.Float64 || kind == reflect.String || kind == reflect.Bool {
		typeName := typeOfS.String()
		return &Schema{
			Type:           typeName,
			Label:          tags["label"],
			Required:       required,
			Weight:         weight,
			WidgetSettings: widgetSettings,
		}
	} else if kind == reflect.Map {
		keys := v.MapKeys()
		if len(keys) > 0 {
			return &Schema{
				Type:           "map",
				Label:          tags["label"],
				Required:       required,
				Weight:         weight,
				WidgetSettings: widgetSettings,
				Items:          getSchema(v.MapIndex(keys[0]), map[string]string{}),
			}
		}
	} else if kind == reflect.Ptr {
		return getSchema(v.Elem(), tags)
	} else if kind == reflect.Array || kind == reflect.Slice {
		if v.Len() > 0 {
			schema := &Schema{
				Type:           "array",
				Label:          tags["label"],
				Required:       required,
				Weight:         weight,
				WidgetSettings: widgetSettings,
				Items:          getSchema(v.Index(0), map[string]string{}),
			}
			return schema
		}
	} else if kind == reflect.Struct {
		fieldsCount := v.NumField()
		schema := &Schema{
			Type:           "object",
			Label:          tags["label"],
			Required:       required,
			Weight:         weight,
			WidgetSettings: widgetSettings,
			Properties:     map[string]*Schema{},
		}
		for i := 0; i < fieldsCount; i++ {
			f := typeOfS.Field(i)
			fieldName := f.Name
			fieldNameTag := strings.Split(f.Tag.Get("json"), ",")
			if len(fieldNameTag) > 0 && fieldNameTag[0] != "" {
				fieldName = fieldNameTag[0]
			}
			label := f.Tag.Get("label")
			vocabulary := f.Tag.Get("vocabulary")
			widget := f.Tag.Get("widget")
			weight := f.Tag.Get("weight")
			validate := f.Tag.Get("validate")
			fieldValue := v.Field(i)
			if fieldValue.Kind() == reflect.Ptr && !fieldValue.Elem().IsValid() {
				fieldValue = reflect.New(f.Type.Elem())
			}
			schema.Properties[fieldName] = getSchema(fieldValue, map[string]string{
				"label":      label,
				"vocabulary": vocabulary,
				"widget":     widget,
				"weight":     weight,
				"validate":   validate,
			})
		}
		return schema
	}
	return nil
}
