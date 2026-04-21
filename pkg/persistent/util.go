package persistent

type measurable interface {
	Len() int
}

// Len returns the length of any measurable collection (Map or List).
func Len(c measurable) int {
	return c.Len()
}
