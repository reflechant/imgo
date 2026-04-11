package persistent

type measurable interface {
	Len() int
}

func Len(c measurable) int {
	return c.Len()
}
