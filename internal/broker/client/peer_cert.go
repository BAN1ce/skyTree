package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
)

type peerCertCtxKey struct{}

// PeerCertificatesFromContext 从 ctx 中读取 mTLS 握手时的客户端证书链。
// 没有 mTLS / 客户端未提供证书时返回 nil。
func PeerCertificatesFromContext(ctx context.Context) []*x509.Certificate {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(peerCertCtxKey{})
	certs, _ := v.([]*x509.Certificate)
	return certs
}

// withPeerCertificates 将客户端证书链塞入 ctx，供下游插件 / ACL 使用。
func withPeerCertificates(ctx context.Context, certs []*x509.Certificate) context.Context {
	if len(certs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, peerCertCtxKey{}, certs)
}

// peerCertificatesFromConn 在 conn 是 *tls.Conn 时返回握手得到的客户端证书链。
// 调用方必须保证 TLS 握手已完成（mqtt over tcp 的第一次 Read 之后即满足）。
func peerCertificatesFromConn(conn net.Conn) []*x509.Certificate {
	tlsConn, ok := conn.(*tls.Conn)
	if !ok || tlsConn == nil {
		return nil
	}
	state := tlsConn.ConnectionState()
	return state.PeerCertificates
}
