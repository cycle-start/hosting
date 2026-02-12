package request

type UploadCertificate struct {
	CertPEM  string `json:"cert_pem" validate:"required"`
	KeyPEM   string `json:"key_pem" validate:"required"`
	ChainPEM string `json:"chain_pem"`
}
