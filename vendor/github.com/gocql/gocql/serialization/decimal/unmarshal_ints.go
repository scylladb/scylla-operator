package decimal

import (
	"gopkg.in/inf.v0"
)

const (
	neg8     = int64(-1) << 8
	neg16    = int64(-1) << 16
	neg24    = int64(-1) << 24
	neg32    = int64(-1) << 32
	neg40    = int64(-1) << 40
	neg48    = int64(-1) << 48
	neg56    = int64(-1) << 56
	neg32Int = int(-1) << 32
)

func decScale(p []byte) inf.Scale {
	return inf.Scale(p[0])<<24 | inf.Scale(p[1])<<16 | inf.Scale(p[2])<<8 | inf.Scale(p[3])
}

func decScaleInt64(p []byte) int64 {
	if p[0] > 127 {
		return neg32 | int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
	}
	return int64(p[0])<<24 | int64(p[1])<<16 | int64(p[2])<<8 | int64(p[3])
}

func dec1toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg8 | int64(p[4])
	}
	return int64(p[4])
}

func dec2toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg16 | int64(p[4])<<8 | int64(p[5])
	}
	return int64(p[4])<<8 | int64(p[5])
}

func dec3toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg24 | int64(p[4])<<16 | int64(p[5])<<8 | int64(p[6])
	}
	return int64(p[4])<<16 | int64(p[5])<<8 | int64(p[6])
}

func dec4toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg32 | int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
	}
	return int64(p[4])<<24 | int64(p[5])<<16 | int64(p[6])<<8 | int64(p[7])
}

func dec5toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg40 | int64(p[4])<<32 | int64(p[5])<<24 | int64(p[6])<<16 | int64(p[7])<<8 | int64(p[8])
	}
	return int64(p[4])<<32 | int64(p[5])<<24 | int64(p[6])<<16 | int64(p[7])<<8 | int64(p[8])
}

func dec6toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg48 | int64(p[4])<<40 | int64(p[5])<<32 | int64(p[6])<<24 | int64(p[7])<<16 | int64(p[8])<<8 | int64(p[9])
	}
	return int64(p[4])<<40 | int64(p[5])<<32 | int64(p[6])<<24 | int64(p[7])<<16 | int64(p[8])<<8 | int64(p[9])
}

func dec7toInt64(p []byte) int64 {
	if p[4] > 127 {
		return neg56 | int64(p[4])<<48 | int64(p[5])<<40 | int64(p[6])<<32 | int64(p[7])<<24 | int64(p[8])<<16 | int64(p[9])<<8 | int64(p[10])
	}
	return int64(p[4])<<48 | int64(p[5])<<40 | int64(p[6])<<32 | int64(p[7])<<24 | int64(p[8])<<16 | int64(p[9])<<8 | int64(p[10])
}

func dec8toInt64(p []byte) int64 {
	return int64(p[4])<<56 | int64(p[5])<<48 | int64(p[6])<<40 | int64(p[7])<<32 | int64(p[8])<<24 | int64(p[9])<<16 | int64(p[10])<<8 | int64(p[11])
}
