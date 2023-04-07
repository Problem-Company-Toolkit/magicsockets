package magicsocket

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsAll(slice []string, items []string) bool {
	for _, item := range items {
		if !contains(slice, item) {
			return false
		}
	}
	return true
}
