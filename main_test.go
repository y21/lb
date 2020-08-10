package lb

import (
	"fmt"
	"log"
	"testing"
)

func Test(t *testing.T) {
	lb := NewFrom(
		[]*Node{
			{
				URI: "http://localhost:6666",
				Fs: map[string]*F{
					"memory": {
						Mod: 5,
					},
				},
			},
			{
				URI: "http://localhost:6667",
				Fs: map[string]*F{
					"memory": {
						Mod: 5,
					},
				},
			},
		},
		Options{
			CacheOptimalNode: true,
		},
	)

	lb.Ping()
	optimal := lb.GetOptimalNode(true)

	if optimal == nil {
		log.Fatal("no node found")
		return
	}

	fmt.Println(optimal.GetScore(), optimal.URI)
}
