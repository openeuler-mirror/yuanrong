package cert

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	mockUtils "yuanrong.org/kernel/pkg/common/faas_common/utils"
)

func TestLoadCerts(t *testing.T) {
	_, _, err := LoadCerts()
	if err != nil {
		t.Errorf("LoadCerts() error = %v", err)
		return
	}
}

func Test_parseSTSCerts(t *testing.T) {
	type args struct {
		pemBlocks []*pem.Block
	}
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 succeed to parse", args{pemBlocks: []*pem.Block{
			&pem.Block{Type: "PRIVATE KEY"}, &pem.Block{}, &pem.Block{Bytes: []byte("a")}}},
			false, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(pem.EncodeToMemory, func(b *pem.Block) []byte {
					return []byte("a")
				}),
				gomonkey.ApplyFunc(x509.ParseCertificate, func(der []byte) (*x509.Certificate, error) {
					if string(der) == "a" {
						return &x509.Certificate{}, nil
					}
					return &x509.Certificate{IsCA: true}, nil
				}),
			})
			return patches
		}},
		{"case2 failed to parse", args{pemBlocks: []*pem.Block{
			&pem.Block{Type: "PRIVATE KEY"}}},
			true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(pem.EncodeToMemory, func(b *pem.Block) []byte {
					return []byte("a")
				}),
				gomonkey.ApplyFunc(x509.ParseCertificate, func(der []byte) (*x509.Certificate, error) {
					if string(der) == "a" {
						return &x509.Certificate{}, nil
					}
					return &x509.Certificate{IsCA: true}, nil
				}),
			})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			_, _, _, err := parseSTSCerts(tt.args.pemBlocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSTSCerts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			patches.ResetAll()
		})
	}
}
