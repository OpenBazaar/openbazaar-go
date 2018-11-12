// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package expfmt

import (
	"fmt"
	"io"
	"net/http"

	"gx/ipfs/QmQzJ5r1Ku82GvPwMEPtFhq3S3ZtC5LnEsS8zhTMPBPYw3/golang_protobuf_extensions/pbutil"
	"gx/ipfs/QmTWEDbLX2d4NiMgPks6J2crRz47BamBtP16WiFuTL6Ydm/common/internal/bitbucket.org/ww/goautoneg"
	"gx/ipfs/QmZHU2gx42NPTYXzw6pJkuX6xCE7bKECp6e8QcPdoLx8sx/protobuf/proto"

	dto "gx/ipfs/QmYaVovLzgcdBpCLEAnW41p8ujvCUxe3TFpfJxjK5qbzn7/client_model/go"
)

// Encoder types encode metric families into an underlying wire protocol.
type Encoder interface {
	Encode(*dto.MetricFamily) error
}

type encoder func(*dto.MetricFamily) error

func (e encoder) Encode(v *dto.MetricFamily) error {
	return e(v)
}

// Negotiate returns the Content-Type based on the given Accept header.
// If no appropriate accepted type is found, FmtText is returned.
func Negotiate(h http.Header) Format {
	for _, ac := range goautoneg.ParseAccept(h.Get(hdrAccept)) {
		// Check for protocol buffer
		if ac.Type+"/"+ac.SubType == ProtoType && ac.Params["proto"] == ProtoProtocol {
			switch ac.Params["encoding"] {
			case "delimited":
				return FmtProtoDelim
			case "text":
				return FmtProtoText
			case "compact-text":
				return FmtProtoCompact
			}
		}
		// Check for text format.
		ver := ac.Params["version"]
		if ac.Type == "text" && ac.SubType == "plain" && (ver == TextVersion || ver == "") {
			return FmtText
		}
	}
	return FmtText
}

// NewEncoder returns a new encoder based on content type negotiation.
func NewEncoder(w io.Writer, format Format) Encoder {
	switch format {
	case FmtProtoDelim:
		return encoder(func(v *dto.MetricFamily) error {
			_, err := pbutil.WriteDelimited(w, v)
			return err
		})
	case FmtProtoCompact:
		return encoder(func(v *dto.MetricFamily) error {
			_, err := fmt.Fprintln(w, v.String())
			return err
		})
	case FmtProtoText:
		return encoder(func(v *dto.MetricFamily) error {
			_, err := fmt.Fprintln(w, proto.MarshalTextString(v))
			return err
		})
	case FmtText:
		return encoder(func(v *dto.MetricFamily) error {
			_, err := MetricFamilyToText(w, v)
			return err
		})
	}
	panic("expfmt.NewEncoder: unknown format")
}
