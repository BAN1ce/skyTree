package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// applyMTLSAuthMode 根据配置在 *tls.Config 上启用 mTLS 客户端证书验证。
// mtlsAuthMode 取值：
//   - ""        : 不启用（兼容缺省值）
//   - "off"     : 不请求客户端证书
//   - "optional": 请求但不强制；缺失证书也允许握手成功
//   - "required": 强制要求；客户端必须出示且通过 CA 校验
func applyMTLSAuthMode(tlsCfg *tls.Config, caFile, mode string) error {
	switch mode {
	case "", "off":
		tlsCfg.ClientAuth = tls.NoClientCert
		return nil
	case "optional":
		tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
	case "required":
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		return fmt.Errorf("invalid mtls_auth_mode %q (expected off|optional|required)", mode)
	}
	if caFile == "" {
		// optional/required 都需要 CA 校验链；缺 CA 文件时回落到 optional 校验所有可信 root，
		// 但更稳妥的做法是直接报错让用户显式配置。
		return fmt.Errorf("mtls_auth_mode %q requires ca_file", mode)
	}
	pool, err := loadCAPool(caFile)
	if err != nil {
		return err
	}
	tlsCfg.ClientCAs = pool
	return nil
}

func loadCAPool(caFile string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read ca_file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("ca_file %q does not contain valid PEM certificates", caFile)
	}
	return pool, nil
}
