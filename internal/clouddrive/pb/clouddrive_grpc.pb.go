// Hand-written gRPC client stub for CloudDriveFileSrv, mirroring what
// protoc-gen-go-grpc would generate for the trimmed clouddrive.proto.
// Only the two RPCs Miyabi uses are included.

package pb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CloudDriveFileSrvClient is the client API for CloudDriveFileSrv service.
type CloudDriveFileSrvClient interface {
	GetSystemInfo(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CloudDriveSystemInfo, error)
	GetSubFiles(ctx context.Context, in *ListSubFileRequest, opts ...grpc.CallOption) (CloudDriveFileSrv_GetSubFilesClient, error)
}

type cloudDriveFileSrvClient struct {
	cc grpc.ClientConnInterface
}

// NewCloudDriveFileSrvClient creates a CloudDriveFileSrv client.
func NewCloudDriveFileSrvClient(cc grpc.ClientConnInterface) CloudDriveFileSrvClient {
	return &cloudDriveFileSrvClient{cc}
}

func (c *cloudDriveFileSrvClient) GetSystemInfo(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CloudDriveSystemInfo, error) {
	out := new(CloudDriveSystemInfo)
	err := c.cc.Invoke(ctx, "/clouddrive.CloudDriveFileSrv/GetSystemInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

var getSubFilesStreamDesc = grpc.StreamDesc{
	StreamName:    "GetSubFiles",
	ServerStreams: true,
}

// CloudDriveFileSrv_GetSubFilesClient is the streaming client for GetSubFiles.
type CloudDriveFileSrv_GetSubFilesClient interface {
	Recv() (*SubFilesReply, error)
	grpc.ClientStream
}

type cloudDriveFileSrvGetSubFilesClient struct {
	grpc.ClientStream
}

func (x *cloudDriveFileSrvGetSubFilesClient) Recv() (*SubFilesReply, error) {
	m := new(SubFilesReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *cloudDriveFileSrvClient) GetSubFiles(ctx context.Context, in *ListSubFileRequest, opts ...grpc.CallOption) (CloudDriveFileSrv_GetSubFilesClient, error) {
	stream, err := c.cc.NewStream(ctx, &getSubFilesStreamDesc, "/clouddrive.CloudDriveFileSrv/GetSubFiles", opts...)
	if err != nil {
		return nil, err
	}
	x := &cloudDriveFileSrvGetSubFilesClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}
