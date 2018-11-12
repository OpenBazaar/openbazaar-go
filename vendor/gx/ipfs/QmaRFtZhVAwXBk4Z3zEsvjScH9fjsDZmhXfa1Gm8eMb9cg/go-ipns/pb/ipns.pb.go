// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: ipns.proto

package ipns_pb

import proto "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"
import fmt "fmt"
import math "math"

import github_com_gogo_protobuf_proto "gx/ipfs/QmdxUuburamoF6zF9qjeQC4WYcWGbWuRmdLacMEsW8ioD8/gogo-protobuf/proto"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type IpnsEntry_ValidityType int32

const (
	// setting an EOL says "this record is valid until..."
	IpnsEntry_EOL IpnsEntry_ValidityType = 0
)

var IpnsEntry_ValidityType_name = map[int32]string{
	0: "EOL",
}
var IpnsEntry_ValidityType_value = map[string]int32{
	"EOL": 0,
}

func (x IpnsEntry_ValidityType) Enum() *IpnsEntry_ValidityType {
	p := new(IpnsEntry_ValidityType)
	*p = x
	return p
}
func (x IpnsEntry_ValidityType) String() string {
	return proto.EnumName(IpnsEntry_ValidityType_name, int32(x))
}
func (x *IpnsEntry_ValidityType) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(IpnsEntry_ValidityType_value, data, "IpnsEntry_ValidityType")
	if err != nil {
		return err
	}
	*x = IpnsEntry_ValidityType(value)
	return nil
}
func (IpnsEntry_ValidityType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_ipns_02f6be73595bcc54, []int{0, 0}
}

type IpnsEntry struct {
	Value        []byte                  `protobuf:"bytes,1,req,name=value" json:"value,omitempty"`
	Signature    []byte                  `protobuf:"bytes,2,req,name=signature" json:"signature,omitempty"`
	ValidityType *IpnsEntry_ValidityType `protobuf:"varint,3,opt,name=validityType,enum=ipns.pb.IpnsEntry_ValidityType" json:"validityType,omitempty"`
	Validity     []byte                  `protobuf:"bytes,4,opt,name=validity" json:"validity,omitempty"`
	Sequence     *uint64                 `protobuf:"varint,5,opt,name=sequence" json:"sequence,omitempty"`
	Ttl          *uint64                 `protobuf:"varint,6,opt,name=ttl" json:"ttl,omitempty"`
	// in order for nodes to properly validate a record upon receipt, they need the public
	// key associated with it. For old RSA keys, its easiest if we just send this as part of
	// the record itself. For newer ed25519 keys, the public key can be embedded in the
	// peerID, making this field unnecessary.
	PubKey               []byte   `protobuf:"bytes,7,opt,name=pubKey" json:"pubKey,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *IpnsEntry) Reset()         { *m = IpnsEntry{} }
func (m *IpnsEntry) String() string { return proto.CompactTextString(m) }
func (*IpnsEntry) ProtoMessage()    {}
func (*IpnsEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_ipns_02f6be73595bcc54, []int{0}
}
func (m *IpnsEntry) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *IpnsEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_IpnsEntry.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *IpnsEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IpnsEntry.Merge(dst, src)
}
func (m *IpnsEntry) XXX_Size() int {
	return m.Size()
}
func (m *IpnsEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_IpnsEntry.DiscardUnknown(m)
}

var xxx_messageInfo_IpnsEntry proto.InternalMessageInfo

func (m *IpnsEntry) GetValue() []byte {
	if m != nil {
		return m.Value
	}
	return nil
}

func (m *IpnsEntry) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *IpnsEntry) GetValidityType() IpnsEntry_ValidityType {
	if m != nil && m.ValidityType != nil {
		return *m.ValidityType
	}
	return IpnsEntry_EOL
}

func (m *IpnsEntry) GetValidity() []byte {
	if m != nil {
		return m.Validity
	}
	return nil
}

func (m *IpnsEntry) GetSequence() uint64 {
	if m != nil && m.Sequence != nil {
		return *m.Sequence
	}
	return 0
}

func (m *IpnsEntry) GetTtl() uint64 {
	if m != nil && m.Ttl != nil {
		return *m.Ttl
	}
	return 0
}

func (m *IpnsEntry) GetPubKey() []byte {
	if m != nil {
		return m.PubKey
	}
	return nil
}

func init() {
	proto.RegisterType((*IpnsEntry)(nil), "ipns.pb.IpnsEntry")
	proto.RegisterEnum("ipns.pb.IpnsEntry_ValidityType", IpnsEntry_ValidityType_name, IpnsEntry_ValidityType_value)
}
func (m *IpnsEntry) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *IpnsEntry) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Value == nil {
		return 0, github_com_gogo_protobuf_proto.NewRequiredNotSetError("value")
	} else {
		dAtA[i] = 0xa
		i++
		i = encodeVarintIpns(dAtA, i, uint64(len(m.Value)))
		i += copy(dAtA[i:], m.Value)
	}
	if m.Signature == nil {
		return 0, github_com_gogo_protobuf_proto.NewRequiredNotSetError("signature")
	} else {
		dAtA[i] = 0x12
		i++
		i = encodeVarintIpns(dAtA, i, uint64(len(m.Signature)))
		i += copy(dAtA[i:], m.Signature)
	}
	if m.ValidityType != nil {
		dAtA[i] = 0x18
		i++
		i = encodeVarintIpns(dAtA, i, uint64(*m.ValidityType))
	}
	if m.Validity != nil {
		dAtA[i] = 0x22
		i++
		i = encodeVarintIpns(dAtA, i, uint64(len(m.Validity)))
		i += copy(dAtA[i:], m.Validity)
	}
	if m.Sequence != nil {
		dAtA[i] = 0x28
		i++
		i = encodeVarintIpns(dAtA, i, uint64(*m.Sequence))
	}
	if m.Ttl != nil {
		dAtA[i] = 0x30
		i++
		i = encodeVarintIpns(dAtA, i, uint64(*m.Ttl))
	}
	if m.PubKey != nil {
		dAtA[i] = 0x3a
		i++
		i = encodeVarintIpns(dAtA, i, uint64(len(m.PubKey)))
		i += copy(dAtA[i:], m.PubKey)
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintIpns(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *IpnsEntry) Size() (n int) {
	var l int
	_ = l
	if m.Value != nil {
		l = len(m.Value)
		n += 1 + l + sovIpns(uint64(l))
	}
	if m.Signature != nil {
		l = len(m.Signature)
		n += 1 + l + sovIpns(uint64(l))
	}
	if m.ValidityType != nil {
		n += 1 + sovIpns(uint64(*m.ValidityType))
	}
	if m.Validity != nil {
		l = len(m.Validity)
		n += 1 + l + sovIpns(uint64(l))
	}
	if m.Sequence != nil {
		n += 1 + sovIpns(uint64(*m.Sequence))
	}
	if m.Ttl != nil {
		n += 1 + sovIpns(uint64(*m.Ttl))
	}
	if m.PubKey != nil {
		l = len(m.PubKey)
		n += 1 + l + sovIpns(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovIpns(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozIpns(x uint64) (n int) {
	return sovIpns(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *IpnsEntry) Unmarshal(dAtA []byte) error {
	var hasFields [1]uint64
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowIpns
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: IpnsEntry: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: IpnsEntry: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthIpns
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Value = append(m.Value[:0], dAtA[iNdEx:postIndex]...)
			if m.Value == nil {
				m.Value = []byte{}
			}
			iNdEx = postIndex
			hasFields[0] |= uint64(0x00000001)
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Signature", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthIpns
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Signature = append(m.Signature[:0], dAtA[iNdEx:postIndex]...)
			if m.Signature == nil {
				m.Signature = []byte{}
			}
			iNdEx = postIndex
			hasFields[0] |= uint64(0x00000002)
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValidityType", wireType)
			}
			var v IpnsEntry_ValidityType
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (IpnsEntry_ValidityType(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.ValidityType = &v
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Validity", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthIpns
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Validity = append(m.Validity[:0], dAtA[iNdEx:postIndex]...)
			if m.Validity == nil {
				m.Validity = []byte{}
			}
			iNdEx = postIndex
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sequence", wireType)
			}
			var v uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Sequence = &v
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ttl", wireType)
			}
			var v uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Ttl = &v
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PubKey", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthIpns
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.PubKey = append(m.PubKey[:0], dAtA[iNdEx:postIndex]...)
			if m.PubKey == nil {
				m.PubKey = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipIpns(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthIpns
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}
	if hasFields[0]&uint64(0x00000001) == 0 {
		return github_com_gogo_protobuf_proto.NewRequiredNotSetError("value")
	}
	if hasFields[0]&uint64(0x00000002) == 0 {
		return github_com_gogo_protobuf_proto.NewRequiredNotSetError("signature")
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipIpns(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowIpns
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowIpns
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthIpns
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowIpns
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipIpns(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthIpns = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowIpns   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("ipns.proto", fileDescriptor_ipns_02f6be73595bcc54) }

var fileDescriptor_ipns_02f6be73595bcc54 = []byte{
	// 221 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xca, 0x2c, 0xc8, 0x2b,
	0xd6, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x87, 0xb0, 0x93, 0x94, 0xfe, 0x33, 0x72, 0x71,
	0x7a, 0x16, 0xe4, 0x15, 0xbb, 0xe6, 0x95, 0x14, 0x55, 0x0a, 0x89, 0x70, 0xb1, 0x96, 0x25, 0xe6,
	0x94, 0xa6, 0x4a, 0x30, 0x2a, 0x30, 0x69, 0xf0, 0x04, 0x41, 0x38, 0x42, 0x32, 0x5c, 0x9c, 0xc5,
	0x99, 0xe9, 0x79, 0x89, 0x25, 0xa5, 0x45, 0xa9, 0x12, 0x4c, 0x60, 0x19, 0x84, 0x80, 0x90, 0x33,
	0x17, 0x4f, 0x59, 0x62, 0x4e, 0x66, 0x4a, 0x66, 0x49, 0x65, 0x48, 0x65, 0x41, 0xaa, 0x04, 0xb3,
	0x02, 0xa3, 0x06, 0x9f, 0x91, 0xbc, 0x1e, 0xd4, 0x06, 0x3d, 0xb8, 0xe9, 0x7a, 0x61, 0x48, 0xca,
	0x82, 0x50, 0x34, 0x09, 0x49, 0x71, 0x71, 0xc0, 0xf8, 0x12, 0x2c, 0x0a, 0x8c, 0x1a, 0x3c, 0x41,
	0x70, 0x3e, 0x48, 0xae, 0x38, 0xb5, 0xb0, 0x34, 0x35, 0x2f, 0x39, 0x55, 0x82, 0x55, 0x81, 0x51,
	0x83, 0x25, 0x08, 0xce, 0x17, 0x12, 0xe0, 0x62, 0x2e, 0x29, 0xc9, 0x91, 0x60, 0x03, 0x0b, 0x83,
	0x98, 0x42, 0x62, 0x5c, 0x6c, 0x05, 0xa5, 0x49, 0xde, 0xa9, 0x95, 0x12, 0xec, 0x60, 0x73, 0xa0,
	0x3c, 0x25, 0x71, 0x2e, 0x1e, 0x64, 0xfb, 0x85, 0xd8, 0xb9, 0x98, 0x5d, 0xfd, 0x7d, 0x04, 0x18,
	0x9c, 0x78, 0x4e, 0x3c, 0x92, 0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1, 0x23, 0x39, 0x46, 0x40,
	0x00, 0x00, 0x00, 0xff, 0xff, 0x32, 0x35, 0xc7, 0xf2, 0x25, 0x01, 0x00, 0x00,
}
