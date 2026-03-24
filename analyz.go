package main

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"
)

type Packet struct {
	Time    string
	Length  string
	Opcode  string
	Body    string
	FullHex string
}

func ParsePacket(data []byte) Packet {
	full := hex.EncodeToString(data)
	p := Packet{
		Time:    time.Now().Format("15:04:05.000"),
		FullHex: full,
	}
	// У нас уже нарезанный пакет нужной длины
	if len(data) >= 4 {
		// Для отображения в "Длина" переводим первые 2 байта в число (Little Endian)
		pLen := binary.LittleEndian.Uint16(data[:2])
		p.Length = string(hex.EncodeToString(data[:2])) + " (" + string(rune(pLen)) + ")"
		// Или просто hex, как ты просил:
		p.Length = hex.EncodeToString(data[:2])
		p.Opcode = hex.EncodeToString(data[2:4])
		p.Body = hex.EncodeToString(data[4:])
	} else {
		p.Body = full
	}
	return p
}

func HexToBytes(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	b, _ := hex.DecodeString(s)
	return b
}
