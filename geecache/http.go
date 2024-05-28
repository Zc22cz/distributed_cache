package geecache

import (
	"GeeCache/geecache/consistenthash"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// 提供被其他节点访问的能力(基于http)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// HTTPPool implements PeerPicker for a pool of HTTP peers.
type HTTPPool struct {
	// this peer's base URL, e.g. "https://example.net:8000"
	self        string                 //记录自己的地址，包括主机名/IP 和端口
	basePath    string                 //作为节点间通讯地址的前缀
	mu          sync.Mutex             //guards peers and httpGetters
	peers       *consistenthash.Map    //用来根据具体的 key 选择节点
	httpGetters map[string]*httpGetter //keyed by e.g. "http://10.0.0.2:8008", 映射远程节点与对应的httpGetter
}

// NewHTTPPool initializes an HTTP pool of peers.
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log info with server name
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Sever %s] %s", p.self, fmt.Sprintf(format, v...))
}

// ServeHTTP handle all http requests
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool seving unexcepted path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// 约定访问路径格式为 /<basepath>/<groupname>/<key>
	//过 groupname 得到 group 实例,
	//再使用 group.Get(key) 获取缓存数据。
	//最终使用 w.Write() 将缓存值作为 httpResponse 的 body 返回。

	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2) //n:分割的次数，即最多将字符串分割成n个子串
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]

	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group"+groupName, http.StatusNotFound)
		return
	}

	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice())
}

// Set updates the pool's list of peers.
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseURL: peer + p.basePath}
	}
}

// PickPeer picks a peer according to key
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

// 确保HTTPPool类型实现了PeerPicker接口，即实现PickPeer。如果没有实现会报错的
var _ PeerPicker = (*HTTPPool)(nil)

// HTTP 客户端类 httpGetter
type httpGetter struct {
	baseURL string //表示将要访问的远程节点的地址，例如 http://example.com/_geecache/
}

func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%s%s%s",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}

	//ioutil.ReadAll 在处理大文件时可能会导致内存消耗过大，因为它会一次性将整个文件内容读入内存，被弃用
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body:%v", err)
	}

	return bytes, nil
}

// _ 用来表明定义了这个变量但不使用它，将 nil 转换为 *httpGetter 类型的指针，并将其赋值给该变量。
// 这样做的目的是，在编译时检查 *httpGetter 类型是否实现了 PeerGetter 接口。
// *httpGetter 类型需要实现 PeerGetter 接口，即Get，如果没有编译器会报错，从而帮助开发者发现潜在的问题。
var _ PeerGetter = (*httpGetter)(nil)
