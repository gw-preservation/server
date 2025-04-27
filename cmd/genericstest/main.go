package main

import (
	"fmt"
	GwPacket "gw1/server/gwpacket"
	"math"
)

var inBytes = []byte{
	0x80, 0x8f,
	0x01, 0x00, 0x00, 0x00,
	0x02, 0x00,
	72, 0x00, 105, 0x00,
}

func unmarshal() {
	in := GwPacket.NewIn(inBytes)
	op, _ := in.Uint16()
	i1, _ := in.Uint32()
	s1, _ := in.UTF16WithLengthPrefix()
	fmt.Printf("unmarshal: op=%x, i1=%d, s1=%s\n", op, i1, s1)
}

func generic[T any](data []byte, t *T) {
}

func main() {
	v := uint32(1025651612)
	f := math.Float32frombits(v)
	fmt.Printf("f: %f\n", f)
}
