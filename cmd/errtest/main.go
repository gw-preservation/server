package main

import (
	"fmt"
	"io"
)

func throws() error {
	return fmt.Errorf("1/2 bytes %w", io.ErrUnexpectedEOF)
}

func calls2() error {
	// something
	err := throws()
	if err != nil {
		return fmt.Errorf("throws(): %w", err)
	}
	return nil
}

func calls1() error {
	// something
	err := calls2()
	if err != nil {
		return fmt.Errorf("calls2(): %w", err)
	}
	return nil
}

func main() {
	err := calls1()
	if err != nil {
		//if errors.Is(errors.Unwrap(err), io.ErrUnexpectedEOF) {
		//	fmt.Printf("Custom type")
		//} else {
		panic(err)
		//}
	}
	fmt.Printf("OK")
}
