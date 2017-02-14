package main

import "path"

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

func (s Shuttle) Path() string {
	return path.Join(s.Route.Path, s.Filename)
}
