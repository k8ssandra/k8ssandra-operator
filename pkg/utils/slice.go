package utils

func SliceContains(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}

func RemoveValue(slice []string, value string) []string {
	newSlice := make([]string, 0)
	for _, s := range slice {
		if s != value {
			newSlice = append(newSlice, s)
		}
	}
	return newSlice
}
