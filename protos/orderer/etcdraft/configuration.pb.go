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
	Consenters           []*Consenter `protobuf:"bytes,1,rep,name=consenters,proto3" json:"consenters,omitempty"`
	Options              *Options     `protobuf:"bytes,2,opt,name=options,proto3" json:"options,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Metadata) Reset()         { *m = Metadata{} }
func (m *Metadata) String() string { return proto.CompactTextString(m) }
func (*Metadata) ProtoMessage()    {}
func (*Metadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_868d446aa2970746, []int{0}
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
	Host                 string   `protobuf:"bytes,1,opt,name=host,proto3" json:"host,omitempty"`
	Port                 uint32   `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
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
	return fileDescriptor_configuration_868d446aa2970746, []int{1}
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
	TickInterval         uint64   `protobuf:"varint,1,opt,name=tick_interval,json=tickInterval,proto3" json:"tick_interval,omitempty"`
	ElectionTick         uint32   `protobuf:"varint,2,opt,name=election_tick,json=electionTick,proto3" json:"election_tick,omitempty"`
	HeartbeatTick        uint32   `protobuf:"varint,3,opt,name=heartbeat_tick,json=heartbeatTick,proto3" json:"heartbeat_tick,omitempty"`
	MaxInflightMsgs      uint32   `protobuf:"varint,4,opt,name=max_inflight_msgs,json=maxInflightMsgs,proto3" json:"max_inflight_msgs,omitempty"`
	MaxSizePerMsg        uint64   `protobuf:"varint,5,opt,name=max_size_per_msg,json=maxSizePerMsg,proto3" json:"max_size_per_msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Options) Reset()         { *m = Options{} }
func (m *Options) String() string { return proto.CompactTextString(m) }
func (*Options) ProtoMessage()    {}
func (*Options) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_868d446aa2970746, []int{2}
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

// Maps of the Raft consenters membership to
// cluster node ids to be stored in block metadata.
// Keeps track on conseters ids and the next id
type RaftMetadata struct {
	Consenters           map[uint64]*Consenter `protobuf:"bytes,1,rep,name=consenters,proto3" json:"consenters,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	NextConsenterID      uint64                `protobuf:"varint,2,opt,name=nextConsenterID,proto3" json:"nextConsenterID,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *RaftMetadata) Reset()         { *m = RaftMetadata{} }
func (m *RaftMetadata) String() string { return proto.CompactTextString(m) }
func (*RaftMetadata) ProtoMessage()    {}
func (*RaftMetadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_configuration_868d446aa2970746, []int{3}
}
func (m *RaftMetadata) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RaftMetadata.Unmarshal(m, b)
}
func (m *RaftMetadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RaftMetadata.Marshal(b, m, deterministic)
}
func (dst *RaftMetadata) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RaftMetadata.Merge(dst, src)
}
func (m *RaftMetadata) XXX_Size() int {
	return xxx_messageInfo_RaftMetadata.Size(m)
}
func (m *RaftMetadata) XXX_DiscardUnknown() {
	xxx_messageInfo_RaftMetadata.DiscardUnknown(m)
}

var xxx_messageInfo_RaftMetadata proto.InternalMessageInfo

func (m *RaftMetadata) GetConsenters() map[uint64]*Consenter {
	if m != nil {
		return m.Consenters
	}
	return nil
}

func (m *RaftMetadata) GetNextConsenterID() uint64 {
	if m != nil {
		return m.NextConsenterID
	}
	return 0
}

func init() {
	proto.RegisterType((*Metadata)(nil), "etcdraft.Metadata")
	proto.RegisterType((*Consenter)(nil), "etcdraft.Consenter")
	proto.RegisterType((*Options)(nil), "etcdraft.Options")
	proto.RegisterType((*RaftMetadata)(nil), "etcdraft.RaftMetadata")
	proto.RegisterMapType((map[uint64]*Consenter)(nil), "etcdraft.RaftMetadata.ConsentersEntry")
}

func init() {
	proto.RegisterFile("orderer/etcdraft/configuration.proto", fileDescriptor_configuration_868d446aa2970746)
}

var fileDescriptor_configuration_868d446aa2970746 = []byte{
	// 469 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x93, 0x41, 0x6f, 0xd3, 0x4e,
	0x10, 0xc5, 0xe5, 0x26, 0xfd, 0xb7, 0xdd, 0xc6, 0xff, 0xb4, 0xcb, 0x25, 0xe2, 0x14, 0x05, 0x28,
	0x01, 0x24, 0x5b, 0x6a, 0x85, 0x84, 0x38, 0x52, 0x40, 0xca, 0x21, 0x02, 0x2d, 0x3d, 0x71, 0xb1,
	0x36, 0x9b, 0xb1, 0xbd, 0x8a, 0xed, 0xb5, 0x76, 0x27, 0x51, 0xd2, 0x2b, 0x1f, 0x90, 0x0b, 0x1f,
	0x08, 0xed, 0xae, 0xed, 0x84, 0xa8, 0xb7, 0xd1, 0x9b, 0xdf, 0x8c, 0x9e, 0xfd, 0x66, 0xc9, 0x4b,
	0xa5, 0x97, 0xa0, 0x41, 0xc7, 0x80, 0x62, 0xa9, 0x79, 0x8a, 0xb1, 0x50, 0x55, 0x2a, 0xb3, 0xb5,
	0xe6, 0x28, 0x55, 0x15, 0xd5, 0x5a, 0xa1, 0xa2, 0xe7, 0x6d, 0x77, 0x52, 0x90, 0xf3, 0x39, 0x20,
	0x5f, 0x72, 0xe4, 0xf4, 0x8e, 0x10, 0xa1, 0x2a, 0x03, 0x15, 0x82, 0x36, 0xa3, 0x60, 0xdc, 0x9b,
	0x5e, 0xde, 0x3e, 0x8b, 0x5a, 0x34, 0xba, 0x6f, 0x7b, 0xec, 0x00, 0xa3, 0xef, 0xc8, 0x99, 0xaa,
	0xed, 0x6a, 0x33, 0x3a, 0x19, 0x07, 0xd3, 0xcb, 0xdb, 0xeb, 0xfd, 0xc4, 0x37, 0xdf, 0x60, 0x2d,
	0x31, 0xf9, 0x15, 0x90, 0x8b, 0x6e, 0x0d, 0xa5, 0xa4, 0x9f, 0x2b, 0x83, 0xa3, 0x60, 0x1c, 0x4c,
	0x2f, 0x98, 0xab, 0xad, 0x56, 0x2b, 0x8d, 0x6e, 0x57, 0xc8, 0x5c, 0x4d, 0x6f, 0xc8, 0x50, 0x14,
	0x12, 0x2a, 0x4c, 0xb0, 0x30, 0x89, 0x00, 0x8d, 0xa3, 0xde, 0x38, 0x98, 0x0e, 0x58, 0xe8, 0xe5,
	0x87, 0xc2, 0xdc, 0x83, 0xe7, 0x0c, 0xe8, 0x0d, 0xe8, 0x3d, 0xd7, 0xf7, 0x9c, 0x97, 0x1b, 0x6e,
	0xf2, 0x3b, 0x20, 0x67, 0x8d, 0x35, 0xfa, 0x82, 0x84, 0x28, 0xc5, 0x2a, 0x91, 0xd6, 0xd1, 0x86,
	0x17, 0xce, 0x4c, 0x9f, 0x0d, 0xac, 0x38, 0x6b, 0x34, 0x0b, 0x41, 0x01, 0xc2, 0x4e, 0x24, 0xb6,
	0xd1, 0xb8, 0x1b, 0xb4, 0xe2, 0x83, 0x14, 0x2b, 0xfa, 0x8a, 0xfc, 0x9f, 0x03, 0xd7, 0xb8, 0x00,
	0x8e, 0x9e, 0xea, 0x39, 0x2a, 0xec, 0x54, 0x87, 0xbd, 0x25, 0xd7, 0x25, 0xdf, 0x26, 0xb2, 0x4a,
	0x0b, 0x99, 0xe5, 0x98, 0x94, 0x26, 0x33, 0xce, 0x66, 0xc8, 0x86, 0x25, 0xdf, 0xce, 0x1a, 0x7d,
	0x6e, 0x32, 0x43, 0x5f, 0x93, 0x2b, 0xcb, 0x1a, 0xf9, 0x08, 0x49, 0x0d, 0xda, 0xb2, 0xa3, 0x53,
	0xe7, 0x2f, 0x2c, 0xf9, 0xf6, 0x87, 0x7c, 0x84, 0xef, 0xa0, 0xe7, 0x26, 0x9b, 0xfc, 0x09, 0xc8,
	0x80, 0xf1, 0x14, 0xbb, 0x28, 0xbf, 0x3e, 0x11, 0xe5, 0xcd, 0x3e, 0x98, 0x43, 0x76, 0x9f, 0xab,
	0xf9, 0x52, 0xa1, 0xde, 0xfd, 0x93, 0xee, 0x94, 0x0c, 0x2b, 0xd8, 0x62, 0x87, 0xcc, 0x3e, 0xbb,
	0x6f, 0xef, 0xb3, 0x63, 0xf9, 0x39, 0x23, 0xc3, 0xa3, 0x45, 0xf4, 0x8a, 0xf4, 0x56, 0xb0, 0x6b,
	0xfe, 0xa8, 0x2d, 0xe9, 0x1b, 0x72, 0xba, 0xe1, 0xc5, 0x1a, 0x9a, 0x53, 0x79, 0xf2, 0xb8, 0x3c,
	0xf1, 0xf1, 0xe4, 0x43, 0xf0, 0x29, 0x23, 0x91, 0xd2, 0x59, 0x94, 0xef, 0x6a, 0xd0, 0x05, 0x2c,
	0x33, 0xd0, 0x51, 0xca, 0x17, 0x5a, 0x0a, 0x7f, 0xc6, 0x26, 0x6a, 0x8e, 0xbd, 0x5b, 0xf3, 0xf3,
	0x7d, 0x26, 0x31, 0x5f, 0x2f, 0x22, 0xa1, 0xca, 0xf8, 0x60, 0x2c, 0xf6, 0x63, 0xb1, 0x1f, 0x8b,
	0x8f, 0xdf, 0xc8, 0xe2, 0x3f, 0xd7, 0xb8, 0xfb, 0x1b, 0x00, 0x00, 0xff, 0xff, 0xa9, 0xe5, 0x95,
	0x3a, 0x3e, 0x03, 0x00, 0x00,
}
