package utils

import "fmt"

// PanicOnError panic errors
func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}

// LogOnError log errors
func LogOnError(err error) {
	if err != nil {
		fmt.Printf("ERROR - %s\n", err)
	}
}
