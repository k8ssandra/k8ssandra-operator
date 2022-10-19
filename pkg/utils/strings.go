package utils

// FirstNonEmptyString returns the first non-empty string in the provided list of strings. If all
// strings are empty, an empty string is returned.
func FirstNonEmptyString(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}
