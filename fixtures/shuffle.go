// shuffles events in an ics file
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"

	ics "github.com/arran4/golang-ical"
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("Please provide only one ics file to shuffle")
	}

	icsFile, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalf("Error opening %q: %s", flag.Arg(0), err)
	}
	defer icsFile.Close()

	cal, err := ics.ParseCalendar(icsFile)
	if err != nil {
		log.Fatalf("Error parsing %q: %s", flag.Arg(0), err)
	}

	rand.Shuffle(len(cal.Components), func(i, j int) {
		cal.Components[i], cal.Components[j] = cal.Components[j], cal.Components[i]
	})

	fmt.Print(cal.Serialize())
}
