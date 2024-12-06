// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v4.24.3
// source: reconfigure.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ReconfigureClient is the client API for Reconfigure service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ReconfigureClient interface {
	StopContainer(ctx context.Context, in *StopContainerRequest, opts ...grpc.CallOption) (*StopContainerResponse, error)
	OnlineUpgradeParams(ctx context.Context, in *OnlineUpgradeParamsRequest, opts ...grpc.CallOption) (*OnlineUpgradeParamsResponse, error)
}

type reconfigureClient struct {
	cc grpc.ClientConnInterface
}

func NewReconfigureClient(cc grpc.ClientConnInterface) ReconfigureClient {
	return &reconfigureClient{cc}
}

func (c *reconfigureClient) StopContainer(ctx context.Context, in *StopContainerRequest, opts ...grpc.CallOption) (*StopContainerResponse, error) {
	out := new(StopContainerResponse)
	err := c.cc.Invoke(ctx, "/proto.Reconfigure/StopContainer", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *reconfigureClient) OnlineUpgradeParams(ctx context.Context, in *OnlineUpgradeParamsRequest, opts ...grpc.CallOption) (*OnlineUpgradeParamsResponse, error) {
	out := new(OnlineUpgradeParamsResponse)
	err := c.cc.Invoke(ctx, "/proto.Reconfigure/OnlineUpgradeParams", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReconfigureServer is the server API for Reconfigure service.
// All implementations must embed UnimplementedReconfigureServer
// for forward compatibility
type ReconfigureServer interface {
	StopContainer(context.Context, *StopContainerRequest) (*StopContainerResponse, error)
	OnlineUpgradeParams(context.Context, *OnlineUpgradeParamsRequest) (*OnlineUpgradeParamsResponse, error)
	mustEmbedUnimplementedReconfigureServer()
}

// UnimplementedReconfigureServer must be embedded to have forward compatible implementations.
type UnimplementedReconfigureServer struct {
}

func (UnimplementedReconfigureServer) StopContainer(context.Context, *StopContainerRequest) (*StopContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopContainer not implemented")
}
func (UnimplementedReconfigureServer) OnlineUpgradeParams(context.Context, *OnlineUpgradeParamsRequest) (*OnlineUpgradeParamsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method OnlineUpgradeParams not implemented")
}
func (UnimplementedReconfigureServer) mustEmbedUnimplementedReconfigureServer() {}

// UnsafeReconfigureServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ReconfigureServer will
// result in compilation errors.
type UnsafeReconfigureServer interface {
	mustEmbedUnimplementedReconfigureServer()
}

func RegisterReconfigureServer(s grpc.ServiceRegistrar, srv ReconfigureServer) {
	s.RegisterService(&Reconfigure_ServiceDesc, srv)
}

func _Reconfigure_StopContainer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconfigureServer).StopContainer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Reconfigure/StopContainer",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconfigureServer).StopContainer(ctx, req.(*StopContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Reconfigure_OnlineUpgradeParams_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OnlineUpgradeParamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ReconfigureServer).OnlineUpgradeParams(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/proto.Reconfigure/OnlineUpgradeParams",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ReconfigureServer).OnlineUpgradeParams(ctx, req.(*OnlineUpgradeParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Reconfigure_ServiceDesc is the grpc.ServiceDesc for Reconfigure service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Reconfigure_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "proto.Reconfigure",
	HandlerType: (*ReconfigureServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "StopContainer",
			Handler:    _Reconfigure_StopContainer_Handler,
		},
		{
			MethodName: "OnlineUpgradeParams",
			Handler:    _Reconfigure_OnlineUpgradeParams_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "reconfigure.proto",
}
