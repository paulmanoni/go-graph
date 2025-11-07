package main

import (
	"github.com/paulmanoni/graph"
)

func main() {
	graph.NewHTTP(graph.GraphContext{
		Playground: true,
		DEBUG:      true,
		UserDetailsFn: func(token string) (interface{}, error) {
			return token, nil
		},
	})
}
