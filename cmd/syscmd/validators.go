package syscmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nodding-noddy/repl-reqs/util"
)

// Validators
func (iv *IntValidations) validate(value string) (interface{}, error) {
	var (
		intVal int
		err    error
	)
	if intVal, err = strconv.Atoi(value); err != nil {
		return intVal, fmt.Errorf("\"%v\": Received invalid integer value", value)
	}
	return intVal, validateNum(&intVal, iv.MinVal, iv.MaxVal)
}

func (fv *FloatValidations) validate(value string) (interface{}, error) {
	var (
		floatVal float64
		err      error
	)
	if floatVal, err = strconv.ParseFloat(value, 64); err != nil {
		return floatVal, fmt.Errorf("\"%v\": Received invalid float value", value)
	}
	return floatVal, validateNum(&floatVal, fv.MinVal, fv.MaxVal)
}

func (sv *StrValidations) validate(value string) (interface{}, error) {
	strLen := len(value)
	if err := validateNum(&strLen, sv.MinLength, sv.MaxLength); err != nil {
		return nil, err
	}

	if matched, err := regexp.MatchString(*sv.regex, value); !matched || err != nil {
		return nil, fmt.Errorf(
			"%v: Provided string does not match the pattern %v",
			value, sv.regex,
		)
	}
	return value, nil
}

func (arVld *ArrValidation) validate(value string) (interface{}, error) {
	str := []byte(fmt.Sprintf(`{"arr": %s}`, value)) // Hehehehuhuhu, am I Evil?
	var arrWrapper map[string]interface{}
	json.Unmarshal(str, &arrWrapper)
	return arrWrapper["arr"], arVld.validateArr(arrWrapper["arr"].([]interface{}))
}

func (objVlds *ObjValidation) validate(value string) (interface{}, error) {
	var obj map[string]interface{}
	json.Unmarshal([]byte(value), &obj)
	return obj, objVlds.validateObj(obj)
}

func (objVlds *ObjValidation) validateObj(obj map[string]interface{}) error {
	for key, vld := range *objVlds {
		return inferAndVld(vld, obj[key])
	}
	return nil
}

func (arrVlds *ArrValidation) validateArr(arr []interface{}) error {
	for idx, vld := range *arrVlds {
		return inferAndVld(vld, arr[idx])
	}
	return nil
}

func inferAndVld(vld Validation, value interface{}) error {
	switch vld := vld.(type) {
	case *ObjValidation:
		if v, ok := value.(map[string]interface{}); ok {
			return vld.validateObj(v)
		}
		return errors.New("invalid object type value")
	case *ArrValidation:
		if v, ok := value.([]interface{}); ok {
			return vld.validateArr(v)
		}
		return errors.New("invalid value for array type")
	default:
		_, err := vld.validate(value.(string))
		return err
	}
}

func strToArr(str string) []string {
	str = strings.Trim(str, " ")
	str = strings.TrimLeft(str, "[")
	str = strings.TrimRight(str, "]")
	return strings.Split(str, ",")
}

func validateNum[num util.Number](n, min, max *num) error {
	if min != nil {
		if *n < *min {
			return fmt.Errorf(
				`Error: Value "%v" not in range. It should atleast be %v`,
				n, min)
		}
	}

	if max != nil {
		if *n > *max {
			return fmt.Errorf(
				`Error: Value "%v" not in range. It can atmost be %v`,
				n, max)
		}
	}

	if min != nil && max != nil {
		if *n < *min || *n > *max {
			return fmt.Errorf(
				`Error: Value "%v" not in range. Expected to be between %v & %v`,
				n, min, max)
		}
	}
	return nil
}
