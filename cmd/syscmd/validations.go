package syscmd

import u "github.com/nodding-noddy/repl-reqs/util"

type Validation interface {
	validate(value string) (any, error)
	InitializeParams(map[string]any) Validation
}

type NumValidateable interface {
}

type GenericValidations struct {
	Required bool
}

type IntValidations struct {
	GenericValidations
	MinVal *int
	MaxVal *int
}

type FloatValidations struct {
	GenericValidations
	MinVal *float64
	MaxVal *float64
}

type StrValidations struct {
	GenericValidations
	MinLength *int
	MaxLength *int
	regex     *string
}

type IterableVld interface {
	ArrValidation | ObjValidation
}

type ArrValidation []Validation

type ObjValidation map[string]Validation

func (iv *IntValidations) InitializeParams(params map[string]interface{}) Validation {
	iv.Required = u.Is("required", params)
	if v, ok := params["minVal"].(float64); ok {
		minVal := int(v)
		iv.MinVal = &minVal
	}
	if v, ok := params["maxVal"].(float64); ok {
		maxVal := int(v)
		iv.MaxVal = &maxVal
	}
	return iv
}

func (fv *FloatValidations) InitializeParams(params map[string]interface{}) Validation {
	fv.Required = u.Is("required", params)
	if v, ok := params["minVal"].(float64); ok {
		fv.MinVal = &v
	}
	if v, ok := params["maxVal"].(float64); ok {
		fv.MaxVal = &v
	}
	return fv
}

func (sv *StrValidations) InitializeParams(params map[string]interface{}) Validation {
	sv.Required = u.Is("required", params)
	if v, ok := params["minLength"].(float64); ok {
		i := int(v)
		sv.MinLength = &i
	}
	if v, ok := params["maxLength"].(float64); ok {
		i := int(v)
		sv.MaxLength = &i
	}
	return sv
}

func (arr *ArrValidation) InitializeParams(params map[string]interface{}) Validation {
	return arr
}

func (vld *ObjValidation) InitializeParams(params map[string]interface{}) Validation {
	return nil
}
