package utils

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
