package rules

func isTypeParameter(name string) bool {
	return len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z'
}
