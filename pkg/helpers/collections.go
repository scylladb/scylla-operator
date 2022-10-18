package helpers

// MergeMaps merges several maps into a new map. If there are conflicting keys, the last one wins and overwrites the value.
func MergeMaps[Key comparable, Value any](maps ...map[Key]Value) map[Key]Value {
	res := map[Key]Value{}
	for _, m := range maps {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}
