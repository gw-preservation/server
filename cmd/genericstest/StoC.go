//go:generate go run ../codegen/main.go s2c
//go:generate go fmt

package main

// opcode: 0x0003
type RequestResponse struct {
	reqNumber    uint32
	responseCode uint16
}
