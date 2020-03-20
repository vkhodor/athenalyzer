package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "0.0.1"

func main() {

	argVersion := flag.Bool("version", false, "show version")
	argFromTime := flag.String("from-time", "", "from time (format: 0000-00-00T00:00:00Z)")
	argToTime := flag.String("to-time", "", "from time (format: 0000-00-00T00:00:00Z)")
	flag.Parse()

	if *argVersion {
		fmt.Println("AthenAlyzer " + version)
		os.Exit(0)
	} else {
		fmt.Println("sdfs")
	}
	fmt.Println("From:\t" + *argFromTime)
	fmt.Println("To:\t" + *argToTime)

}
