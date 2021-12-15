package utils

import "reflect"

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}
	switch reflect.TypeOf(v).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return reflect.ValueOf(v).IsNil()
	}
	return false
}
