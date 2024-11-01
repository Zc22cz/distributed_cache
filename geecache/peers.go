package geecache

import pb "GeeCache/geecache/geecachepb"

// PeerPicker is the interface that must be implemented to locate
// the peer that owns a specific key.
type PeerPicker interface {
	//PickPeer() 方法用于根据传入的 key 选择相应节点 PeerGetter。
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter is the interface that must be implemented by a peer.
// PeerGetter 就对应于上述流程中的 HTTP 客户端。
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error //用于从对应 group 查找缓存值
}
