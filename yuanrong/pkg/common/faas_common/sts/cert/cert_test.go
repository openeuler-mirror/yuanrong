package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"golang.org/x/crypto/pkcs12"
	"huawei.com/wisesecurity/sts-sdk/pkg/cryptosts"
	"huawei.com/wisesecurity/sts-sdk/pkg/stsgoapi"

	mockUtils "yuanrong/pkg/common/faas_common/utils"
)

func TestLoadCerts(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1 succeed to load certificates", false, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(pkcs12.ToPEM, func(pfxData []byte, password string) ([]*pem.Block, error) {
					return []*pem.Block{}, nil
				}),
				gomonkey.ApplyFunc(parseSTSCerts, func(pemBlocks []*pem.Block) ([][]byte, []byte, []byte, error) {
					return [][]byte{[]byte("1")}, []byte("a"), []byte("b"), nil
				}),
				gomonkey.ApplyFunc(tls.X509KeyPair, func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
					return tls.Certificate{}, nil
				}),
				gomonkey.ApplyMethod(reflect.TypeOf(&x509.CertPool{}), "AppendCertsFromPEM",
					func(_ *x509.CertPool, pemCerts []byte) (ok bool) {
						return true
					})})
			return patches
		}},
		{"case2 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(pkcs12.ToPEM, func(pfxData []byte, password string) ([]*pem.Block, error) {
					return []*pem.Block{}, nil
				}),
				gomonkey.ApplyFunc(parseSTSCerts, func(pemBlocks []*pem.Block) ([][]byte, []byte, []byte, error) {
					return [][]byte{[]byte("1")}, []byte("a"), []byte("b"), nil
				}),
				gomonkey.ApplyFunc(tls.X509KeyPair, func(certPEMBlock, keyPEMBlock []byte) (tls.Certificate, error) {
					return tls.Certificate{}, errors.New("error")
				}),
				gomonkey.ApplyMethod(reflect.TypeOf(&x509.CertPool{}), "AppendCertsFromPEM",
					func(_ *x509.CertPool, pemCerts []byte) (ok bool) {
						return true
					})})
			return patches
		}},
		{"case3 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(pkcs12.ToPEM, func(pfxData []byte, password string) ([]*pem.Block, error) {
					return []*pem.Block{}, nil
				}),
				gomonkey.ApplyFunc(parseSTSCerts, func(pemBlocks []*pem.Block) ([][]byte, []byte, []byte, error) {
					return [][]byte{[]byte("1")}, []byte("a"), []byte("b"), errors.New("error")
				})})
			return patches
		}},
		{"case4 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(pkcs12.ToPEM, func(pfxData []byte, password string) ([]*pem.Block, error) {
					return []*pem.Block{}, errors.New("error")
				})})
			return patches
		}},
		{"case5 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, nil
				}),
				gomonkey.ApplyFunc(os.ReadFile, func(name string) ([]byte, error) {
					return []byte{}, errors.New("error")
				})})
			return patches
		}},
		{"case6 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", nil
				}),
				gomonkey.ApplyFunc(x509.NewCertPool, func() *x509.CertPool {
					return &x509.CertPool{}
				}),
				gomonkey.ApplyFunc(stsgoapi.GetPassphrase, func() (passphrase []byte, err error) {
					return []byte{}, errors.New("error")
				})})
			return patches
		}},
		{"case7 failed to load certificates", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cryptosts.GetKeyStorePath, func() (string, error) {
					return "", errors.New("error")
				})})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			_, _, err := LoadCerts()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCerts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			patches.ResetAll()
		})
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
