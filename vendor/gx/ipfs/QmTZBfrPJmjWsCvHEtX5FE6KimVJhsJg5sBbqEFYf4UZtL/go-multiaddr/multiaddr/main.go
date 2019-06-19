package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	maddr "github.com/multiformats/go-multiaddr"
)

var (
	flagHelp bool
)

func main() {
	flag.Usage = func() {
		usage := `usage: %s [options] ADDR

Print details about the given multiaddr.

Options:
`
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}

	flag.BoolVar(&flagHelp, "h", false, "display help message")
	flag.Parse()

	if flagHelp || len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(0)
	}

	addrStr := flag.Args()[0]
	var addr maddr.Multiaddr
	var err error
	if strings.HasPrefix(addrStr, "0x") {
		addrBytes, err := hex.DecodeString(addrStr[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
			os.Exit(1)
		}
		addr, err = maddr.NewMultiaddrBytes(addrBytes)
	} else {
		addr, err = maddr.NewMultiaddr(addrStr)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		os.Exit(1)
	}

	infoCommand(addr)
}

func infoCommand(addr maddr.Multiaddr) {
	var compsJson []string
	maddr.ForEach(addr, func(comp maddr.Component) bool {
		lengthPrefix := ""
		if comp.Protocol().Size == maddr.LengthPrefixedVarSize {
			lengthPrefix = "0x" + hex.EncodeToString(maddr.CodeToVarint(len(comp.RawValue())))
		}

		compsJson = append(compsJson, `{`+
			fmt.Sprintf(`"string": "%s", `, comp.String())+
			fmt.Sprintf(`"stringSize": "%d", `, len(comp.String()))+
			fmt.Sprintf(`"packed": "0x%x", `, comp.Bytes())+
			fmt.Sprintf(`"packedSize": "%d", `, len(comp.Bytes()))+
			fmt.Sprintf(`"value": %#v, `, comp.Value())+
			fmt.Sprintf(`"rawValue": "0x%x", `, comp.RawValue())+
			fmt.Sprintf(`"valueSize": "%d", `, len(comp.RawValue()))+
			fmt.Sprintf(`"protocol": "%s", `, comp.Protocol().Name)+
			fmt.Sprintf(`"codec": "%d", `, comp.Protocol().Code)+
			fmt.Sprintf(`"uvarint": "0x%x", `, comp.Protocol().VCode)+
			fmt.Sprintf(`"lengthPrefix": "%s"`, lengthPrefix)+
			`}`)
		return true
	})

	addrJson := `{
  "string": "%[1]s",
  "stringSize": "%[2]d",
  "packed": "0x%[3]x",
  "packedSize": "%[4]d",
  "components": [
    %[5]s
  ]
}`
	fmt.Fprintf(os.Stdout, addrJson+"\n",
		addr.String(), len(addr.String()), addr.Bytes(), len(addr.Bytes()), strings.Join(compsJson, ",\n    "))
}
