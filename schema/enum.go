package schema

type Enum []string

func (e Enum) Includes(s string) bool {
	for _, str := range e {
		if str == s {
			return true
		}
	}
	return false
}
