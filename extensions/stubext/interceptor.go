package stubext

import "context"
import "google.golang.org/grpc"
import "google.golang.org/grpc/metadata"
import "os"
import "strings"


var DownStreamHeaderName = "x-down-stream-name"

// DownSteamNameHeaderInterceptorForK8sPod
// 给k8s pod用的Interceptor，把pod的HOSTNAME前缀（deployment name 或cronjob name）放到x-down-stream-name header里
// 主要用于服务方识别调用方
func DownSteamNameHeaderInterceptorForK8sPod(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error{
		hostName := os.Getenv("HOSTNAME")
		if hostName != "" {
			sps := strings.Split(hostName, "-")
			if len(sps) >= 3 {
				downStreamName := strings.Join(sps[:len(sps) - 2], "-")
				ctx = metadata.AppendToOutgoingContext(ctx, DownStreamHeaderName, downStreamName)
			}
		}
		return invoker(ctx, method, req, reply, cc, opts...)
} 

