package model

type List struct {
	Delim rune
	Boxes []string
}

type Thread struct {
	From    []string
	Subject string
	Count   int
}
