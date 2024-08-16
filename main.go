// main package
//
// this package is the main package, starting the implementation of the Kubo-DTS
// package dts
package main

import (
	"flag"

	"github.com/ntt-nilab/kubo-dts/dts"
)

func main() {
	var (
		name     = flag.Int("n", 0, "name of the node")
		mesh     = flag.Bool("m", false, "mesh network")
		testnode = flag.Bool("t", false, "mass-publish slave node")
		address  = flag.String("a", "", "address of blockchain")
	)
	flag.Parse()
	dts.Run(*name, *mesh, *testnode, *address)
}
