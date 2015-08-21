package helpers

import (
	"errors"
	"fmt"
	"reflect"
)

func EnsureDefaults(target map[string]interface{}, defaults map[string]interface{}) error {
	for dk, dv := range defaults {
		defaultType := reflect.TypeOf(dv)
		if tv, ok := target[dk]; ok {
			if reflect.TypeOf(tv) != defaultType {
				return errors.New(fmt.Sprintf("Field '%s': expected a value of type %T.", dk, dv))
			}
			defaultZero := reflect.New(defaultType)
			if tv == defaultZero {
				target[dk] = dv
			}
		} else {
			target[dk] = dv
		}
	}
	return nil
}
