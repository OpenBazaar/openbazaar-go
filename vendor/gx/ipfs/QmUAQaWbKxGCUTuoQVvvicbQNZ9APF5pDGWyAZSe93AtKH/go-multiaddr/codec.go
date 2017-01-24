package multiaddr

import (
	"bytes"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	mh "gx/ipfs/QmYDds3421prZgqKbLpEK7T9Aa2eVdQ7o3YarX1LVLdP2J/go-multihash"
)

func stringToBytes(s string) ([]byte, error) {

	// consume trailing slashes
	s = strings.TrimRight(s, "/")

	b := new(bytes.Buffer)
	sp := strings.Split(s, "/")

	if sp[0] != "" {
		return nil, fmt.Errorf("invalid multiaddr, must begin with /")
	}

	// consume first empty elem
	sp = sp[1:]

	for len(sp) > 0 {
		p := ProtocolWithName(sp[0])
		if p.Code == 0 {
			return nil, fmt.Errorf("no protocol with name %s", sp[0])
		}
		b.Write(CodeToVarint(p.Code))
		sp = sp[1:]

		if p.Size == 0 { // no length.
			continue
		}

		if len(sp) < 1 {
			return nil, fmt.Errorf("protocol requires address, none given: %s", p.Name)
		}

		if p.Path {
			// it's a path protocol (terminal).
			// consume the rest of the address as the next component.
			sp = []string{"/" + strings.Join(sp, "/")}
		}

		a, err := addressStringToBytes(p, sp[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %s %s", p.Name, sp[0], err)
		}
		b.Write(a)
		sp = sp[1:]
	}

	return b.Bytes(), nil
}

func validateBytes(b []byte) (err error) {
	for len(b) > 0 {
		code, n, err := ReadVarintCode(b)
		if err != nil {
			return err
		}

		b = b[n:]
		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return fmt.Errorf("no protocol with code %d", code)
		}

		if p.Size == 0 {
			continue
		}

		size, err := sizeForAddr(p, b)
		if err != nil {
			return err
		}

		if len(b) < size || size < 0 {
			return fmt.Errorf("invalid value for size")
		}

		b = b[size:]
	}

	return nil
}

func bytesToString(b []byte) (ret string, err error) {
	s := ""

	for len(b) > 0 {
		code, n, err := ReadVarintCode(b)
		if err != nil {
			return "", err
		}

		b = b[n:]
		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return "", fmt.Errorf("no protocol with code %d", code)
		}
		s += "/" + p.Name

		if p.Size == 0 {
			continue
		}

		size, err := sizeForAddr(p, b)
		if err != nil {
			return "", err
		}

		if len(b) < size || size < 0 {
			return "", fmt.Errorf("invalid value for size")
		}

		a, err := addressBytesToString(p, b[:size])
		if err != nil {
			return "", err
		}
		if len(a) > 0 {
			s += "/" + a
		}
		b = b[size:]
	}

	return s, nil
}

func sizeForAddr(p Protocol, b []byte) (int, error) {
	switch {
	case p.Size > 0:
		return (p.Size / 8), nil
	case p.Size == 0:
		return 0, nil
	case p.Path:
		size, n, err := ReadVarintCode(b)
		if err != nil {
			return 0, err
		}
		return size + n, nil
	default:
		size, n, err := ReadVarintCode(b)
		if err != nil {
			return 0, err
		}
		return size + n, nil
	}
}

func bytesSplit(b []byte) ([][]byte, error) {
	var ret [][]byte
	for len(b) > 0 {
		code, n, err := ReadVarintCode(b)
		if err != nil {
			return nil, err
		}

		p := ProtocolWithCode(code)
		if p.Code == 0 {
			return nil, fmt.Errorf("no protocol with code %d", b[0])
		}

		size, err := sizeForAddr(p, b[n:])
		if err != nil {
			return nil, err
		}

		length := n + size
		ret = append(ret, b[:length])
		b = b[length:]
	}

	return ret, nil
}

func addressStringToBytes(p Protocol, s string) ([]byte, error) {
	switch p.Code {

	case P_IP4: // ipv4
		i := net.ParseIP(s).To4()
		if i == nil {
			return nil, fmt.Errorf("failed to parse ip4 addr: %s", s)
		}
		return i, nil

	case P_IP6: // ipv6
		i := net.ParseIP(s).To16()
		if i == nil {
			return nil, fmt.Errorf("failed to parse ip6 addr: %s", s)
		}
		return i, nil

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, err)
		}
		if i >= 65536 {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, "greater than 65536")
		}
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(i))
		return b, nil

	case P_ONION:
		addr := strings.Split(s, ":")
		if len(addr) != 2 {
			return nil, fmt.Errorf("failed to parse %s addr: %s does not contain a port number.", p.Name, s)
		}

		// onion address without the ".onion" substring
		if len(addr[0]) != 16 {
			return nil, fmt.Errorf("failed to parse %s addr: %s not a Tor onion address.", p.Name, s)
		}
		onionHostBytes, err := base32.StdEncoding.DecodeString(strings.ToUpper(addr[0]))
		if err != nil {
			return nil, fmt.Errorf("failed to decode base32 %s addr: %s %s", p.Name, s, err)
		}

		// onion port number
		i, err := strconv.Atoi(addr[1])
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, err)
		}
		if i >= 65536 {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, "port greater than 65536")
		}
		if i < 1 {
			return nil, fmt.Errorf("failed to parse %s addr: %s", p.Name, "port less than 1")
		}

		onionPortBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(onionPortBytes, uint16(i))
		bytes := []byte{}
		bytes = append(bytes, onionHostBytes...)
		bytes = append(bytes, onionPortBytes...)
		return bytes, nil

	case P_IPFS: // ipfs
		// the address is a varint prefixed multihash string representation
		m, err := mh.FromB58String(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ipfs addr: %s %s", s, err)
		}
		size := CodeToVarint(len(m))
		b := append(size, m...)
		return b, nil

	case P_UNIX:
		// the address is the whole remaining string, prefixed by a varint len
		size := CodeToVarint(len(s))
		b := append(size, []byte(s)...)
		return b, nil
	}

	return []byte{}, fmt.Errorf("failed to parse %s addr: unknown", p.Name)
}

func addressBytesToString(p Protocol, b []byte) (string, error) {
	switch p.Code {

	// ipv4,6
	case P_IP4, P_IP6:
		return net.IP(b).String(), nil

	// tcp udp dccp sctp
	case P_TCP, P_UDP, P_DCCP, P_SCTP:
		i := binary.BigEndian.Uint16(b)
		return strconv.Itoa(int(i)), nil

	case P_IPFS: // ipfs
		// the address is a varint-prefixed multihash string representation
		size, n, err := ReadVarintCode(b)
		if err != nil {
			return "", err
		}

		b = b[n:]
		if len(b) != size {
			return "", errors.New("inconsistent lengths")
		}
		m, err := mh.Cast(b)
		if err != nil {
			return "", err
		}
		return m.B58String(), nil

	case P_ONION:
		addr := strings.ToLower(base32.StdEncoding.EncodeToString(b[0:10]))
		port := binary.BigEndian.Uint16(b[10:12])
		return addr + ":" + strconv.Itoa(int(port)), nil

	case P_UNIX:
		// the address is a varint len prefixed string
		size, n, err := ReadVarintCode(b)
		if err != nil {
			return "", err
		}

		b = b[n:]
		if len(b) != size {
			return "", errors.New("inconsistent lengths")
		}
		if size == 0 {
			return "", errors.New("invalid length")
		}
		s := string(b)
		s = s[1:] // remove starting slash
		return s, nil

	default:
		return "", errors.New("unknown protocol")
	}
}
