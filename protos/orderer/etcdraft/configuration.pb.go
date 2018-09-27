// Code generated by protoc-gen-go. DO NOT EDIT.
// source: orderer/etcdraft/configuration.proto

package etcdraft // import "github.com/hyperledger/fabric/protos/orderer/etcdraft"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// Metadata is serialized and set as the value of ConsensusType.Metadata in
// a channel configuration when the ConsensusType.Type is set "etcdraft".
type Metadata struct {
	Consenters           []*Consenter `protobuf:"bytes,1,rep,name=consenters" json:"consenters,omitempty"`
	Options              *Options     `protobuf:"bytes,2,opt,name=options" json:"options,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Metadata) Reset()         { *m = Metadata{} }
func (m *Metadata) String() string { return proto.CompactTextString(m) }
func (*Metadata) ProtoMessage()    {}
func (*Metadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_5f2e30437520fe7a, []int{0}
}
func (m *Metadata) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Metadata.Unmarshal(m, b)
}
func (m *Metadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Metadata.Marshal(b, m, deterministic)
}
func (dst *Metadata) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Metadata.Merge(dst, src)
}
func (m *Metadata) XXX_Size() int {
	return xxx_messageInfo_Metadata.Size(m)
}
func (m *Metadata) XXX_DiscardUnknown() {
	xxx_messageInfo_Metadata.DiscardUnknown(m)
}

var xxx_messageInfo_Metadata proto.InternalMessageInfo

func (m *Metadata) GetConsenters() []*Consenter {
	if m != nil {
		return m.Consenters
	}
	return nil
}

func (m *Metadata) GetOptions() *Options {
	if m != nil {
		return m.Options
	}
	return nil
}

// Consenter represents a consenting node (i.e. replica).
type Consenter struct {
	Host                 string   `protobuf:"bytes,1,opt,name=host" json:"host,omitempty"`
	Port                 uint32   `protobuf:"varint,2,opt,name=port" json:"port,omitempty"`
	ClientTlsCert        []byte   `protobuf:"bytes,3,opt,name=client_tls_cert,json=clientTlsCert,proto3" json:"client_tls_cert,omitempty"`
	ServerTlsCert        []byte   `protobuf:"bytes,4,opt,name=server_tls_cert,json=serverTlsCert,proto3" json:"server_tls_cert,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Consenter) Reset()         { *m = Consenter{} }
func (m *Consenter) String() string { return proto.CompactTextString(m) }
func (*Consenter) ProtoMessage()    {}
func (*Consenter) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_5f2e30437520fe7a, []int{1}
}
func (m *Consenter) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Consenter.Unmarshal(m, b)
}
func (m *Consenter) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Consenter.Marshal(b, m, deterministic)
}
func (dst *Consenter) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Consenter.Merge(dst, src)
}
func (m *Consenter) XXX_Size() int {
	return xxx_messageInfo_Consenter.Size(m)
}
func (m *Consenter) XXX_DiscardUnknown() {
	xxx_messageInfo_Consenter.DiscardUnknown(m)
}

var xxx_messageInfo_Consenter proto.InternalMessageInfo

func (m *Consenter) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

func (m *Consenter) GetPort() uint32 {
	if m != nil {
		return m.Port
	}
	return 0
}

func (m *Consenter) GetClientTlsCert() []byte {
	if m != nil {
		return m.ClientTlsCert
	}
	return nil
}

func (m *Consenter) GetServerTlsCert() []byte {
	if m != nil {
		return m.ServerTlsCert
	}
	return nil
}

// Options to be specified for all the etcd/raft nodes. These can be modified on a
// per-channel basis.
type Options struct {
	// specified in miliseconds
	TickInterval         uint64   `protobuf:"varint,1,opt,name=tick_interval,json=tickInterval" json:"tick_interval,omitempty"`
	ElectionTick         uint32   `protobuf:"varint,2,opt,name=election_tick,json=electionTick" json:"election_tick,omitempty"`
	HeartbeatTick        uint32   `protobuf:"varint,3,opt,name=heartbeat_tick,json=heartbeatTick" json:"heartbeat_tick,omitempty"`
	MaxInflightMsgs      uint32   `protobuf:"varint,4,opt,name=max_inflight_msgs,json=maxInflightMsgs" json:"max_inflight_msgs,omitempty"`
	MaxSizePerMsg        uint64   `protobuf:"varint,5,opt,name=max_size_per_msg,json=maxSizePerMsg" json:"max_size_per_msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Options) Reset()         { *m = Options{} }
func (m *Options) String() string { return proto.CompactTextString(m) }
func (*Options) ProtoMessage()    {}
func (*Options) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_5f2e30437520fe7a, []int{2}
}
func (m *Options) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Options.Unmarshal(m, b)
}
func (m *Options) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Options.Marshal(b, m, deterministic)
}
func (dst *Options) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Options.Merge(dst, src)
}
func (m *Options) XXX_Size() int {
	return xxx_messageInfo_Options.Size(m)
}
func (m *Options) XXX_DiscardUnknown() {
	xxx_messageInfo_Options.DiscardUnknown(m)
}

var xxx_messageInfo_Options proto.InternalMessageInfo

func (m *Options) GetTickInterval() uint64 {
	if m != nil {
		return m.TickInterval
	}
	return 0
}

func (m *Options) GetElectionTick() uint32 {
	if m != nil {
		return m.ElectionTick
	}
	return 0
}

func (m *Options) GetHeartbeatTick() uint32 {
	if m != nil {
		return m.HeartbeatTick
	}
	return 0
}

func (m *Options) GetMaxInflightMsgs() uint32 {
	if m != nil {
		return m.MaxInflightMsgs
	}
	return 0
}

func (m *Options) GetMaxSizePerMsg() uint64 {
	if m != nil {
		return m.MaxSizePerMsg
	}
	return 0
}

type BlockMetadata struct {
	Index                uint64   `protobuf:"varint,1,opt,name=index" json:"index,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BlockMetadata) Reset()         { *m = BlockMetadata{} }
func (m *BlockMetadata) String() string { return proto.CompactTextString(m) }
func (*BlockMetadata) ProtoMessage()    {}
func (*BlockMetadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_5f2e30437520fe7a, []int{3}
}
func (m *BlockMetadata) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockMetadata.Unmarshal(m, b)
}
func (m *BlockMetadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockMetadata.Marshal(b, m, deterministic)
}
func (dst *BlockMetadata) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockMetadata.Merge(dst, src)
}
func (m *BlockMetadata) XXX_Size() int {
	return xxx_messageInfo_BlockMetadata.Size(m)
}
func (m *BlockMetadata) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockMetadata.DiscardUnknown(m)
}

var xxx_messageInfo_BlockMetadata proto.InternalMessageInfo

func (m *BlockMetadata) GetIndex() uint64 {
	if m != nil {
		return m.Index
	}
	return 0
}

func init() {
	proto.RegisterType((*Metadata)(nil), "etcdraft.Metadata")
	proto.RegisterType((*Consenter)(nil), "etcdraft.Consenter")
	proto.RegisterType((*Options)(nil), "etcdraft.Options")
	proto.RegisterType((*BlockMetadata)(nil), "etcdraft.BlockMetadata")
}

func init() {
	proto.RegisterFile("orderer/etcdraft/configuration.proto", fileDescriptor_configuration_5f2e30437520fe7a)
}

var fileDescriptor_configuration_5f2e30437520fe7a = []byte{
	// 408 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0x92, 0x41, 0x6f, 0xd4, 0x30,
	0x10, 0x85, 0x15, 0x76, 0x4b, 0x5b, 0x77, 0x43, 0xa9, 0xe1, 0x90, 0xe3, 0x6a, 0xa1, 0xb0, 0x02,
	0x29, 0x91, 0x5a, 0xf1, 0x07, 0xda, 0x53, 0x0f, 0x2b, 0x50, 0xe8, 0x89, 0x4b, 0xe4, 0x75, 0x66,
	0x1d, 0x6b, 0x9d, 0x38, 0x1a, 0x4f, 0xab, 0xa5, 0x57, 0x7e, 0x20, 0x7f, 0x09, 0xd9, 0x4e, 0xd2,
	0x8a, 0x9b, 0xf3, 0xde, 0xf7, 0x46, 0x2f, 0x9a, 0x61, 0x1f, 0x2d, 0xd6, 0x80, 0x80, 0x05, 0x90,
	0xac, 0x51, 0xec, 0xa8, 0x90, 0xb6, 0xdb, 0x69, 0xf5, 0x80, 0x82, 0xb4, 0xed, 0xf2, 0x1e, 0x2d,
	0x59, 0x7e, 0x32, 0xba, 0x2b, 0xc3, 0x4e, 0x36, 0x40, 0xa2, 0x16, 0x24, 0xf8, 0x35, 0x63, 0xd2,
	0x76, 0x0e, 0x3a, 0x02, 0x74, 0x59, 0xb2, 0x9c, 0xad, 0xcf, 0xae, 0xde, 0xe5, 0x23, 0x9a, 0xdf,
	0x8e, 0x5e, 0xf9, 0x02, 0xe3, 0x5f, 0xd9, 0xb1, 0xed, 0xfd, 0x68, 0x97, 0xbd, 0x5a, 0x26, 0xeb,
	0xb3, 0xab, 0x8b, 0xe7, 0xc4, 0xf7, 0x68, 0x94, 0x23, 0xb1, 0xfa, 0x93, 0xb0, 0xd3, 0x69, 0x0c,
	0xe7, 0x6c, 0xde, 0x58, 0x47, 0x59, 0xb2, 0x4c, 0xd6, 0xa7, 0x65, 0x78, 0x7b, 0xad, 0xb7, 0x48,
	0x61, 0x56, 0x5a, 0x86, 0x37, 0xff, 0xc4, 0xce, 0xa5, 0xd1, 0xd0, 0x51, 0x45, 0xc6, 0x55, 0x12,
	0x90, 0xb2, 0xd9, 0x32, 0x59, 0x2f, 0xca, 0x34, 0xca, 0xf7, 0xc6, 0xdd, 0x42, 0xe4, 0x1c, 0xe0,
	0x23, 0xe0, 0x33, 0x37, 0x8f, 0x5c, 0x94, 0x07, 0x6e, 0xf5, 0x37, 0x61, 0xc7, 0x43, 0x35, 0xfe,
	0x81, 0xa5, 0xa4, 0xe5, 0xbe, 0xd2, 0xbe, 0xd1, 0xa3, 0x30, 0xa1, 0xcc, 0xbc, 0x5c, 0x78, 0xf1,
	0x6e, 0xd0, 0x3c, 0x04, 0x06, 0xa4, 0x4f, 0x54, 0xde, 0x18, 0xda, 0x2d, 0x46, 0xf1, 0x5e, 0xcb,
	0x3d, 0xbf, 0x64, 0x6f, 0x1a, 0x10, 0x48, 0x5b, 0x10, 0x14, 0xa9, 0x59, 0xa0, 0xd2, 0x49, 0x0d,
	0xd8, 0x17, 0x76, 0xd1, 0x8a, 0x43, 0xa5, 0xbb, 0x9d, 0xd1, 0xaa, 0xa1, 0xaa, 0x75, 0xca, 0x85,
	0x9a, 0x69, 0x79, 0xde, 0x8a, 0xc3, 0xdd, 0xa0, 0x6f, 0x9c, 0x72, 0xfc, 0x33, 0x7b, 0xeb, 0x59,
	0xa7, 0x9f, 0xa0, 0xea, 0x01, 0x3d, 0x9b, 0x1d, 0x85, 0x7e, 0x69, 0x2b, 0x0e, 0x3f, 0xf5, 0x13,
	0xfc, 0x00, 0xdc, 0x38, 0xb5, 0xba, 0x64, 0xe9, 0x8d, 0xb1, 0x72, 0x3f, 0xad, 0xf2, 0x3d, 0x3b,
	0xd2, 0x5d, 0x0d, 0x87, 0xe1, 0x77, 0xe2, 0xc7, 0x8d, 0x62, 0xb9, 0x45, 0x95, 0x37, 0xbf, 0x7b,
	0x40, 0x03, 0xb5, 0x02, 0xcc, 0x77, 0x62, 0x8b, 0x5a, 0xc6, 0xb3, 0x70, 0xf9, 0x70, 0x3c, 0xd3,
	0x06, 0x7f, 0x7d, 0x53, 0x9a, 0x9a, 0x87, 0x6d, 0x2e, 0x6d, 0x5b, 0xbc, 0x88, 0x15, 0x31, 0x56,
	0xc4, 0x58, 0xf1, 0xff, 0xcd, 0x6d, 0x5f, 0x07, 0xe3, 0xfa, 0x5f, 0x00, 0x00, 0x00, 0xff, 0xff,
	0xea, 0xf1, 0xe5, 0x0f, 0x8e, 0x02, 0x00, 0x00,
}
