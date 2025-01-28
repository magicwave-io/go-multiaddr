package multiaddr

import (
	"bytes"
	"fmt"
	"strings"

	multibase "github.com/multiformats/go-multibase"
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
		if sp[0] == "unknown" {
			if len(sp) < 2 {
				return nil, fmt.Errorf("unknown protocol requires an argument")
			}
			_, bts, err := multibase.Decode(sp[1])
			if err != nil {
				return nil, err
			}
			b.Write(bts)
			sp = sp[2:]
			continue
		}
		p := ProtocolWithName(sp[0])
		if p.Code == 0 {
			return nil, fmt.Errorf("no protocol with name %s", sp[0])
		}
		_, _ = b.Write(CodeToVarint(p.Code))
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

		if p.Transcoder == nil {
			return nil, fmt.Errorf("no transcoder for %s protocol", p.Name)
		}
		a, err := p.Transcoder.StringToBytes(sp[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %s %s", p.Name, sp[0], err)
		}
		if p.Size < 0 { // varint size.
			_, _ = b.Write(CodeToVarint(len(a)))
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
			return nil
		}

		if p.Size == 0 {
			continue
		}

		n, size, err := sizeForAddr(p, b)
		if err != nil {
			return err
		}

		b = b[n:]

		if len(b) < size || size < 0 {
			return fmt.Errorf("invalid value for size")
		}

		err = p.Transcoder.ValidateBytes(b[:size])
		if err != nil {
			return err
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

		p := ProtocolWithCode(code)
		if p.Code == 0 {
			encoded, err := multibase.Encode(multibase.Base32, b)
			if err != nil {
				panic(err)
			}
			return s + "/unknown/" + encoded, nil
		}
		b = b[n:]

		s += "/" + p.Name

		if p.Size == 0 {
			continue
		}

		n, size, err := sizeForAddr(p, b)
		if err != nil {
			return "", err
		}

		b = b[n:]

		if len(b) < size || size < 0 {
			return "", fmt.Errorf("invalid value for size")
		}

		if p.Transcoder == nil {
			return "", fmt.Errorf("no transcoder for %s protocol", p.Name)
		}
		a, err := p.Transcoder.BytesToString(b[:size])
		if err != nil {
			return "", err
		}
		if p.Path && len(a) > 0 && a[0] == '/' {
			a = a[1:]
		}
		if len(a) > 0 {
			s += "/" + a
		}
		b = b[size:]
	}

	return s, nil
}

func sizeForAddr(p Protocol, b []byte) (skip, size int, err error) {
	switch {
	case p.Size > 0:
		return 0, (p.Size / 8), nil
	case p.Size == 0:
		return 0, 0, nil
	default:
		size, n, err := ReadVarintCode(b)
		if err != nil {
			return 0, 0, err
		}
		return n, size, nil
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
		// Unknown protocol. Finish.
		if p.Code == 0 {
			ret = append(ret, b)
			break
		}

		n2, size, err := sizeForAddr(p, b[n:])
		if err != nil {
			return nil, err
		}

		length := n + n2 + size
		ret = append(ret, b[:length])
		b = b[length:]
	}

	return ret, nil
}
