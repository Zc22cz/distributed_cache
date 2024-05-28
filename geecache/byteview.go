package geecache

// 缓存值的抽象与封装
// A ByteView holds an immutable view of bytes.
type ByteView struct {
	// byte 类型是能够支持任意的数据类型的存储，例如字符串、图片等。
	b []byte
}

// Len returns the view's length
// 在 lru.Cache 的实现中，要求被缓存对象必须实现 Value 接口
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice returns a copy of the data as a byte slice.
// b 是只读的，使用 ByteSlice() 方法返回一个拷贝，防止缓存值被外部程序修改。
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String returns the data as a string, making a copy if necessary.
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
