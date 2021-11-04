package utils

func SliceContains(slice []string, s string) bool {
	for _, elem := range slice {
		if elem == s {
			return true
		}
	}
	return false
}

func SliceRemove(slice []string, s string) []string {
	newSlice := make([]string, 0)
	for _, v := range slice {
		if v != s {
			newSlice = append(newSlice, s)
		}
	}
	return newSlice
}

func SlicesContainSameElementsInAnyOrder(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for _, s := range slice1 {
		if !SliceContains(slice2, s) {
			return false
		}
	}
	return true
}
