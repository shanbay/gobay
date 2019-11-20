package encoderext

import (
	"strings"
)

const (
	defaultAlphabet  = "asdfghjklURLEncoderConfigqwertyui"
	defaultBlockSize = uint(24)
	minLength        = 5
	one              = uint64(1)
)

type UrlEncoder struct {
	opt *Options
}

type Options struct {
	Alphabet  string
	BlockSize uint
}

func (opt *Options) init() {
	if opt.Alphabet == "" {
		opt.Alphabet = defaultAlphabet
	}
	if opt.BlockSize == 0 {
		opt.BlockSize = defaultBlockSize
	}
}

func NewURLEncoder(opt *Options) *UrlEncoder {
	opt.init()
	return &UrlEncoder{opt: opt}
}

// pos 1, 101 & 001 = 001
// pos 2, 101 & 010 = 000
// pos 3, 101 & 100 = 100
func getBit(n uint64, pos uint) int {
	if (n & (one << pos)) != 0 {
		return 1
	}
	return 0
}

func (encoder *UrlEncoder) encode(n uint64) uint64 {
	for i, j := uint(0), encoder.opt.BlockSize-1; i < j; i, j = i+1, j-1 {
		if getBit(n, i) != getBit(n, j) {
			n ^= (one << i) | (one << j)
		}
	}
	return n
}

func reverseSlice(a []byte) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
}

func (encoder *UrlEncoder) enbase(x uint64) string {
	n := uint64(len(encoder.opt.Alphabet))
	result := make([]byte, 0, minLength)
	for {
		ch := encoder.opt.Alphabet[x%n]
		result = append(result, ch)
		x = x / n
		if x == 0 && len(result) >= minLength {
			break
		}
	}
	reverseSlice(result)
	return string(result)
}

func (encoder *UrlEncoder) debase(x string) uint64 {
	n := uint64(len(encoder.opt.Alphabet))
	result := uint64(0)
	bits := []byte(x)
	for _, bitValue := range bits {
		result = result*n + uint64(strings.IndexByte(encoder.opt.Alphabet, bitValue))
	}
	return result
}

func (encoder *UrlEncoder) EncodeURL(n uint64) string {
	return encoder.enbase(encoder.encode(n))
}

func (encoder *UrlEncoder) DecodeURL(n string) uint64 {
	return encoder.encode(encoder.debase(n))
}
