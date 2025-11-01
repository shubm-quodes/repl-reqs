package syscmd

type Validation interface {
	validate(value string) (any, error)
	InitializeParams(map[string]any) Validation
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

type ArrValidation []Validation

type ObjValidation map[string]Validation

func (iv *IntValidations) InitializeParams(params map[string]interface{}) Validation {
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
	if v, ok := params["minVal"].(float64); ok {
		fv.MinVal = &v
	}
	if v, ok := params["maxVal"].(float64); ok {
		fv.MaxVal = &v
	}
	return fv
}

func (sv *StrValidations) InitializeParams(params map[string]interface{}) Validation {
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

func (arr ArrValidation) InitializeParams(params map[string]interface{}) Validation {
	return arr
}

func (vld ObjValidation) InitializeParams(params map[string]interface{}) Validation {
	return nil
}
