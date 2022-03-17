package cassandra

import (
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
)

// preMarshalConfig expects val to be a struct, or a pointer thereto. It walks through the struct
// fields and looks for cass-config tags; each field annotated with such tags will be processed and
// included in the resulting map, which is then returned.
func preMarshalConfig(val reflect.Value, version *semver.Version) (map[string]interface{}, error) {
	t := val.Type()
	// if val is a pointer: if nil, return immediately; otherwise dereference it.
	if t.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, nil
		}
		val = val.Elem()
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got: %v", t.String())
	}
	out := make(map[string]interface{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if tagStr, tagFound := field.Tag.Lookup(cassConfigTagName); tagFound {
			if tag, err := parseCassConfigTag(tagStr); err != nil {
				return nil, fmt.Errorf("cannot parse %v tag on %v.%v: %w", cassConfigTagName, t.String(), field.Name, err)
			} else if path := tag.pathForVersion(version); path != nil {
				fieldVal := val.Field(i)
				if !fieldVal.IsZero() || tag.retainZero {
					if tag.recurse {
						if fieldOut, err := preMarshalConfig(fieldVal, version); err != nil {
							return nil, fmt.Errorf("field %v.%v: recurse failed: %w", t.String(), field.Name, err)
						} else if path.isInline() {
							if out, err = utils.MergeMapNested(false, out, fieldOut); err != nil {
								return nil, fmt.Errorf("field %v.%v: cannot merge map: %w", t.String(), field.Name, err)
							}
						} else if existingOut, found := utils.GetMapNested(out, path.segments[0], path.segments[1:]...); !found {
							_ = utils.PutMapNested(false, out, fieldOut, path.segments[0], path.segments[1:]...)
						} else if existingOutMap, existingOutIsMap := existingOut.(map[string]interface{}); !existingOutIsMap {
							return nil, fmt.Errorf("field %v.%v: cannot merge map: path %v exists but its value is of type %T", t.String(), field.Name, path.path, existingOut)
						} else if mergedOut, err := utils.MergeMapNested(false, existingOutMap, fieldOut); err != nil {
							return nil, fmt.Errorf("field %v.%v: cannot merge map: %w", t.String(), field.Name, err)
						} else {
							_ = utils.PutMapNested(true, out, mergedOut, path.segments[0], path.segments[1:]...)
						}
					} else {
						if v, err := getFieldValue(fieldVal, t, field); err != nil {
							return nil, err
						} else if err := utils.PutMapNested(false, out, v, path.segments[0], path.segments[1:]...); err != nil {
							return nil, fmt.Errorf("field %v.%v: cannot put value: %w", t.String(), field.Name, err)
						}
					}
				}
			}
		}
	}
	return out, nil
}

func getFieldValue(fieldVal reflect.Value, structType reflect.Type, field reflect.StructField) (interface{}, error) {
	v := fieldVal.Interface()
	switch vv := v.(type) {
	case resource.Quantity:
		var ok bool
		if v, ok = vv.AsInt64(); !ok {
			return nil, fmt.Errorf("field %v.%v: cannot convert quantity to int64", structType.String(), field.Name)
		}
	case *resource.Quantity:
		var ok bool
		if v, ok = vv.AsInt64(); !ok {
			return nil, fmt.Errorf("field %v.%v: cannot convert quantity to int64", structType.String(), field.Name)
		}
	}
	return v, nil
}
