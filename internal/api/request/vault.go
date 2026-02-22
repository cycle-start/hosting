package request

import "fmt"

const maxVaultPlaintextSize = 64 * 1024 // 64KB

type VaultEncrypt struct {
	Plaintext string `json:"plaintext" validate:"required"`
}

func (r *VaultEncrypt) Validate() error {
	if len(r.Plaintext) > maxVaultPlaintextSize {
		return fmt.Errorf("plaintext too large: max %d bytes", maxVaultPlaintextSize)
	}
	return nil
}

type VaultDecrypt struct {
	Token string `json:"token" validate:"required"`
}

func (r *VaultDecrypt) Validate() error {
	if len(r.Token) < len("vault:v1:") {
		return fmt.Errorf("invalid vault token format")
	}
	if r.Token[:9] != "vault:v1:" {
		return fmt.Errorf("invalid vault token format: must start with vault:v1:")
	}
	return nil
}
