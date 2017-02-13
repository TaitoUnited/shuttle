package main

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type Launchpad struct{}

func (p Launchpad) DiscoverRoutes(base string) ([]Route, error) {
	routes := []Route{}

	files, err := ioutil.ReadDir(base)
	if err != nil {
		return routes, nil
	}

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		endpointPath := path.Join(base, file.Name(), ".endpoint")
		if _, err := os.Stat(endpointPath); err == nil {
			endpoint, err := ioutil.ReadFile(endpointPath)
			if err != nil {
				return routes, err
			}

			route := Route{
				Path:     path.Join(base, file.Name(), "files"),
				Endpoint: strings.TrimSpace(string(endpoint)),
			}

			routes = append(routes, route)
		}
	}

	return routes, nil
}
