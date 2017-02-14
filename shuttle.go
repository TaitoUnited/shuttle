package main

type Shuttle struct {
	Route    Route
	Filename string
}

func NewShuttle(route Route, filename string) Shuttle {
	return Shuttle{
		Route:    route,
		Filename: filename,
	}
}
