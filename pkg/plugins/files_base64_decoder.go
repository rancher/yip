package plugins

import (
	"encoding/base64"
	"fmt"
)

type base64Decoder struct{}

func (base64Decoder) Decode(content string) ([]byte, error) {
	output, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64: %q", err)
	}
	return output, nil
}

type base64GZip struct{}

func (base64GZip) Decode(content string) ([]byte, error) {
	b := base64Decoder{}
	c, err := b.Decode(content)
	if err != nil {
		return []byte{}, fmt.Errorf("Error while reading base64 data: %s", err.Error())
	}

	g := gzipDecoder{}

	return g.Decode(string(c))
}
