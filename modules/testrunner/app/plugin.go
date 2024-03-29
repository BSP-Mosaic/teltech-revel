package app

import (
	"fmt"
	"github.com/BSP-Mosaic/revel"
)

func init() {
	revel.OnAppStart(func() {
		fmt.Println("Go to /@tests to run the tests.")
	})
}
