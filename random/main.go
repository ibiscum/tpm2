package main

import (
	//"fmt"
	"fmt"
	"log"
	"os"

	"github.com/google/go-tpm/legacy/tpm2"
)

func main() {

	f, err := os.OpenFile("/dev/tpmrm0", os.O_RDWR, 0)
	if err != nil {
		log.Fatalf("opening tpm: %v", err)
	}
	defer f.Close()

	out, err := tpm2.GetRandom(f, 16)
	if err != nil {
		log.Fatalf("getting random bytes: %v", err)
	}
	fmt.Printf("%x\n", out)
}
