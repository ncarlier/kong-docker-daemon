package toolkit

// Diff get b items that are not in a
func Diff(a []string, b []string) []string {
	var diff []string
	bMap := make(map[string]struct{}, len(a))
	for _, val := range b {
		bMap[val] = struct{}{}
	}

	for _, val := range a {
		if _, ok := bMap[val]; !ok {
			diff = append(diff, val)
		}
	}
	return diff
}
