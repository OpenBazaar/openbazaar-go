package cidlink

import "fmt"

var (
	multicodecDecodeTable MulticodecDecodeTable
	multicodecEncodeTable MulticodecEncodeTable
)

func init() {
	multicodecEncodeTable = make(MulticodecEncodeTable)
	multicodecDecodeTable = make(MulticodecDecodeTable)
}

// RegisterMulticodecDecoder is used to register multicodec features.
// It adjusts a global registry and may only be used at program init time;
// it is meant to provide a plugin system, not a configuration mechanism.
func RegisterMulticodecDecoder(hook uint64, fn MulticodecDecoder) {
	_, exists := multicodecDecodeTable[hook]
	if exists {
		panic(fmt.Errorf("multicodec decoder already registered for %x", hook))
	}
	multicodecDecodeTable[hook] = fn
}

// RegisterMulticodecEncoder is used to register multicodec features.
// It adjusts a global registry and may only be used at program init time;
// it is meant to provide a plugin system, not a configuration mechanism.
func RegisterMulticodecEncoder(hook uint64, fn MulticodecEncoder) {
	_, exists := multicodecEncodeTable[hook]
	if exists {
		panic(fmt.Errorf("multicodec encoder already registered for %x", hook))
	}
	multicodecEncodeTable[hook] = fn
}
