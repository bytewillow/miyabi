// Package clouddrive talks to CloudDrive2's gRPC management API
// (default port 19798) to force-refresh its directory cache: folders
// created through the 115 API are invisible in the CloudDrive2 mount
// until the cached listing is refreshed, which breaks downstream
// consumers such as Plex that read through the mount.
// Authentication is a Bearer API token in the gRPC metadata
// (created in the CloudDrive2 settings UI).
package clouddrive

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ppxb/miyabi/internal/clouddrive/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client is a CloudDrive2 gRPC client.
type Client struct {
	endpoint string
	token    string
}

// New creates a client. endpoint looks like "192.168.1.10:19798".
func New(endpoint, token string) *Client {
	return &Client{endpoint: endpoint, token: token}
}

func (c *Client) dial(ctx context.Context) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, c.endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect CloudDrive2 %s: %w", c.endpoint, err)
	}
	return conn, nil
}

func (c *Client) authCtx(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token)
}

// SystemInfo tests connectivity; GetSystemInfo is a public method and
// needs no authorization.
func (c *Client) SystemInfo(ctx context.Context) (string, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	cli := pb.NewCloudDriveFileSrvClient(conn)
	info, err := cli.GetSystemInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return "", fmt.Errorf("GetSystemInfo: %w", err)
	}
	return fmt.Sprintf("user %s (%s)", info.GetUserName(), info.GetSystemMessage()), nil
}

// listNames lists the direct children of path. forceRefresh bypasses the
// directory cache and pulls the listing from the cloud drive.
func (c *Client) listNames(ctx context.Context, path string, forceRefresh bool) ([]string, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	cli := pb.NewCloudDriveFileSrvClient(conn)
	stream, err := cli.GetSubFiles(c.authCtx(ctx), &pb.ListSubFileRequest{
		Path:         path,
		ForceRefresh: forceRefresh,
	})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", path, err)
	}
	var names []string
	for {
		reply, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list %s: read stream: %w", path, err)
		}
		for _, f := range reply.GetSubFiles() {
			names = append(names, f.GetName())
		}
	}
	return names, nil
}

// RefreshDir force-refreshes the cached listing of path from the cloud.
func (c *Client) RefreshDir(ctx context.Context, path string) error {
	_, err := c.listNames(ctx, path, true)
	return err
}

// ListNames lists the direct children of path through the cache; useful
// to verify a configured path is written the way CloudDrive2 sees it.
func (c *Client) ListNames(ctx context.Context, path string) ([]string, error) {
	return c.listNames(ctx, path, false)
}

// NormalizePath canonicalizes a CloudDrive2 virtual path: leading slash,
// no trailing slash, no duplicate slashes.
func NormalizePath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	p = strings.TrimRight(p, "/")
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}
