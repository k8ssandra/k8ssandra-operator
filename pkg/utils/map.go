package utils

import (
	"fmt"
)

// MergeMap will take two or more maps, merging the entries of the sources map into a destination map. If both maps
// share the same key then destination's value for that key will be overwritten with what's in source.
func MergeMap(sources ...map[string]string) map[string]string {
	destination := make(map[string]string, 0)
	for _, source := range sources {
		if source != nil {
			for k, v := range source {
				destination[k] = v
			}
		}
	}
	return destination
}

// MergeMapNested will take two or more maps, merging the entries of the sources map into a
// destination map. If both maps share the same key then destination's value for that key will be
// merged with what's in source. The source maps are not modified.
func MergeMapNested(allowOverwrite bool, sources ...map[string]interface{}) (map[string]interface{}, error) {
	destination := make(map[string]interface{}, 0)
	for _, source := range sources {
		if source != nil {
			for k, sourceVal := range source {
				destVal, destValFound := destination[k]
				if destValFound {
					sourceMap, sourceValIsMap := sourceVal.(map[string]interface{})
					destMap, destValIsMap := destVal.(map[string]interface{})
					if sourceValIsMap && destValIsMap {
						if merged, err := MergeMapNested(allowOverwrite, destMap, sourceMap); err != nil {
							return nil, err
						} else {
							destination[k] = merged
						}
					} else if allowOverwrite {
						destination[k] = sourceVal
					} else {
						return nil, fmt.Errorf("key %v already exists", k)
					}
				} else {
					destination[k] = sourceVal
				}
			}
		}
	}
	return destination, nil
}

// GetMapNested gets the value at the given keys in the given map. It returns nil and false if the
// map does not contain any of the keys.
func GetMapNested(m map[string]interface{}, key string, keys ...string) (interface{}, bool) {
	if len(keys) == 0 {
		val, found := m[key]
		return val, found
	} else {
		v, found := m[key]
		if !found {
			return nil, false
		} else if _, ok := v.(map[string]interface{}); !ok {
			return nil, false
		}
		return GetMapNested(v.(map[string]interface{}), keys[0], keys[1:]...)
	}
}

// PutMapNested puts the given value in the given map at the given keys. When a key is not present,
// the entry is created on the fly; created entries are always of type map[string]interface{} to
// allow for nested entries to be further inserted.
func PutMapNested(allowOverwrite bool, m map[string]interface{}, val interface{}, key string, keys ...string) error {
	v, found := m[key]
	if len(keys) == 0 {
		if found && v != nil && !allowOverwrite {
			return fmt.Errorf("key %v already exists", key)
		}
		m[key] = val
		return nil
	} else {
		if !found {
			v = make(map[string]interface{})
			m[key] = v
		} else if _, ok := v.(map[string]interface{}); !ok {
			if v != nil && !allowOverwrite {
				return fmt.Errorf("key %v already exists", key)
			}
			v = make(map[string]interface{})
			m[key] = v
		}
		return PutMapNested(allowOverwrite, v.(map[string]interface{}), val, keys[0], keys[1:]...)
	}
}
