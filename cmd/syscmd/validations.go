package syscmd

import (
	"encoding/json"
	"fmt"
)

type Validation interface {
	validate(value string) (any, error)
}

type IntValidations struct {
	Required *bool  `json:"required"`
	Type     string `json:"type"`
	MinVal   *int   `json:"minVal"`
	MaxVal   *int   `json:"maxVal"`
}

type FloatValidations struct {
	Required *bool    `json:"required"`
	Type     string   `json:"type"`
	MinVal   *float64 `json:"minVal"`
	MaxVal   *float64 `json:"maxVal"`
}

type StrValidations struct {
	Required  *bool   `json:"required"`
	Type      string  `json:"type"`
	MinLength *int    `json:"minLength"`
	MaxLength *int    `json:"maxLength"`
	Regex     *string `json:"regex"`
}

type IterableVld interface {
	ArrValidation | ObjValidation
}

type ArrValidation struct {
	Type string
	arr  []Validation
}

type ObjValidation struct {
	Type   string
	fields ValidationSchema
}

func buildObjValidation(cfg json.RawMessage) (*ObjValidation, error) {
	var obj struct {
		Schema map[string]json.RawMessage `json:"schema"`
	}

	if err := json.Unmarshal(cfg, &obj); err != nil {
		return nil, err
	}

	fields, err := constructValidationSchema(obj.Schema)
	if err != nil {
		return nil, err
	}

	return &ObjValidation{fields: fields}, nil
}

func newValidator(t string, cfg json.RawMessage) (Validation, error) {
	switch t {
	case "int":
		return &IntValidations{}, nil
	case "float":
		return &FloatValidations{}, nil
	case "string":
		return &StrValidations{}, nil
	case "array":
		return &ArrValidation{}, nil
	case "object", "json":
		return buildObjValidation(cfg)
	default:
		return nil, fmt.Errorf(`invalid parameter type "%s"`, t)
	}
}

func getValidation(cfg json.RawMessage) (Validation, error) {
	var meta struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(cfg, &meta)

	isShortHand := false
	if meta.Type == "" { // In case if it's empty, maybe it's a shorthand
		if err := json.Unmarshal(cfg, &meta.Type); err == nil && meta.Type != "" {
			isShortHand = true
		}
	}

	vld, err := newValidator(meta.Type, cfg)
	if err != nil {
		return nil, err
	}

	if isShortHand {
		return vld, nil
	}

	if err := json.Unmarshal(cfg, vld); err != nil {
		return nil, err
	}

	return vld, nil
}

func constructValidationSchema(cfg map[string]json.RawMessage) (ValidationSchema, error) {
	schema := make(ValidationSchema)

	for key, value := range cfg {
		if vld, err := getValidation(value); err != nil {
			return nil, err
		} else {
			schema[key] = vld
		}
	}

	return schema, nil
}
