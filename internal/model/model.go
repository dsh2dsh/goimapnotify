package model

type List struct {
	Delim rune
	Boxes []string
}

type Thread struct {
	From    map[string]string
	Subject string
	Count   int
}
