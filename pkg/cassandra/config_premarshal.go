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
func preMarshalConfig(val reflect.Value, version *semver.Version, serverType string) (map[string]interface{}, error) {
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
			} else if path := tag.pathForVersion(version, serverType); path != nil {
				fieldVal := val.Field(i)
				if !fieldVal.IsZero() || tag.retainZero {
					if tag.recurse {
						if fieldOut, err := getFieldValueRecursive(fieldVal, version, serverType, t, field); err != nil {
							return nil, err
						} else if path.isInline() {
							if fieldOutMap, fieldOutIsMap := fieldOut.(map[string]interface{}); !fieldOutIsMap {
								return nil, fmt.Errorf("field %v.%v: cannot inline value at path %v: expected map but found type %T", t.String(), field.Name, path.path, fieldOut)
							} else if out, err = utils.MergeMapNested(false, out, fieldOutMap); err != nil {
								return nil, fmt.Errorf("field %v.%v: cannot merge map: %w", t.String(), field.Name, err)
							}
						} else {
							if existingOut, found := utils.GetMapNested(out, path.segments[0], path.segments[1:]...); !found {
								_ = utils.PutMapNested(false, out, fieldOut, path.segments[0], path.segments[1:]...)
							} else if existingOutMap, existingOutIsMap := existingOut.(map[string]interface{}); !existingOutIsMap {
								return nil, fmt.Errorf("field %v.%v: cannot merge map: path %v exists but its value is of type %T", t.String(), field.Name, path.path, existingOut)
							} else if fieldOutMap, fieldOutIsMap := fieldOut.(map[string]interface{}); !fieldOutIsMap {
								return nil, fmt.Errorf("field %v.%v: cannot merge map: path %v exists but field value is of type %T", t.String(), field.Name, path.path, fieldOut)
							} else if mergedOut, err := utils.MergeMapNested(false, existingOutMap, fieldOutMap); err != nil {
								return nil, fmt.Errorf("field %v.%v: cannot merge map: %w", t.String(), field.Name, err)
							} else {
								_ = utils.PutMapNested(true, out, mergedOut, path.segments[0], path.segments[1:]...)
							}
						}
					} else {
						if fieldOut, err := getFieldValue(fieldVal); err != nil {
							return nil, err
						} else if err := utils.PutMapNested(false, out, fieldOut, path.segments[0], path.segments[1:]...); err != nil {
							return nil, fmt.Errorf("field %v.%v: cannot put value: %w", t.String(), field.Name, err)
						}
					}
				}
			}
		}
	}
	return out, nil
}

// getFieldValueRecursive returns the value of a complex field annotated with the "recurse" option,
// converting it to a value that can be put in a map destined to be marshalled for cass-config.
// Currently, it handles the following types: structs, slices of structs, maps of string keys and
// struct values, and pointers thereto.
func getFieldValueRecursive(
	fieldVal reflect.Value,
	version *semver.Version,
	serverType string,
	parentStructType reflect.Type,
	field reflect.StructField,
) (interface{}, error) {
	fieldKind := fieldVal.Kind()
	if fieldKind == reflect.Ptr {
		fieldKind = fieldVal.Type().Elem().Kind()
	}
	switch fieldKind {
	case reflect.Struct:
		if fieldOut, err := preMarshalConfig(fieldVal, version, serverType); err != nil {
			return nil, fmt.Errorf("field %v.%v: recurse failed: %w", parentStructType.String(), field.Name, err)
		} else {
			return fieldOut, nil
		}
	case reflect.Slice:
		fieldOutSlice := make([]interface{}, 0, fieldVal.Len())
		for i := 0; i < fieldVal.Len(); i++ {
			if elemOut, err := preMarshalConfig(fieldVal.Index(i), version, serverType); err != nil {
				return nil, fmt.Errorf("field %v.%v: recurse failed: %w", parentStructType.String(), field.Name, err)
			} else {
				fieldOutSlice = append(fieldOutSlice, elemOut)
			}
		}
		return fieldOutSlice, nil
	case reflect.Map:
		if fieldVal.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("cannot marshal map with non-string key: %v", fieldVal.Type().String())
		}
		fieldOutMap := make(map[string]interface{})
		for _, key := range fieldVal.MapKeys() {
			if elemOut, err := preMarshalConfig(fieldVal.MapIndex(key), version, serverType); err != nil {
				return nil, fmt.Errorf("field %v.%v: recurse failed: %w", parentStructType.String(), field.Name, err)
			} else {
				fieldOutMap[key.String()] = elemOut
			}
		}
		return fieldOutMap, nil
	}
	return nil, fmt.Errorf("field %v.%v: cannot recurse field, expected struct, slice or map type, got: %v", parentStructType.String(), field.Name, fieldVal.Type().String())
}

// getFieldValue returns the value of a scalar field (that is, not annotated with the "recurse"
// option), converting it to a value that can be marshaled to JSON for cass-config. Currently, it
// applies a special conversion to resource.Quantity values and pointers thereto. More special
// conversions could be added in the future, e.g. to handle the new throughput/rate syntax
// introduced in Cassandra 4.1.
func getFieldValue(fieldVal reflect.Value) (interface{}, error) {
	v := fieldVal.Interface()
	switch vv := v.(type) {
	case resource.Quantity:
		v = convertQuantity(&vv)
	case *resource.Quantity:
		v = convertQuantity(vv)
	}
	return v, nil
}

// convertQuantity converts a resource.Quantity to an int64 or float64, depending on the value. If
// the scale is zero or negative, it is assumed that the value has no decimal part, and it is
// converted to an int64. When the scale is positive, or if the conversion to int64 fails, it is
// assumed that the value has a decimal part, and it is converted to a float64. This is intended to
// make it possible for fields in the CRD to distinguish between e.g. 1 or 1.0: the former will be
// serialized as integer, the latter as decimal.
func convertQuantity(v *resource.Quantity) interface{} {
	if v == nil {
		return nil
	}
	copied := v.DeepCopy()
	scale := copied.AsDec().Scale() // AsDec mutates the quantity
	if scale <= 0 {
		if converted, ok := v.AsInt64(); ok {
			return converted
		}
	}
	return v.AsApproximateFloat64()
}
