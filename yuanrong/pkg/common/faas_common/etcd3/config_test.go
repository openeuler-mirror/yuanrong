/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package etcd3 implements crud and watch operations based etcd clientv3
package etcd3

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"

	commonCrypto "yuanrong/pkg/common/crypto"
	"yuanrong/pkg/common/faas_common/crypto"
	"yuanrong/pkg/common/faas_common/localauth"
	"yuanrong/pkg/common/faas_common/sts/cert"
	commontls "yuanrong/pkg/common/faas_common/tls"
	mockUtils "yuanrong/pkg/common/faas_common/utils"
)

func TestGetEtcdAuthType(t *testing.T) {
	convey.Convey("tlsAuth", t, func() {
		etcdConfig := EtcdConfig{
			SslEnable: true,
		}
		etcdAuth := GetEtcdAuthType(etcdConfig)
		convey.So(etcdAuth, convey.ShouldResemble, &tlsAuth{
			certFile: etcdConfig.CertFile,
			keyFile:  etcdConfig.KeyFile,
			caFile:   etcdConfig.CaFile,
		})
	})
	convey.Convey("tlsAuth", t, func() {
		etcdConfig := EtcdConfig{
			SslEnable: false,
			Password:  "",
		}
		etcdAuth := GetEtcdAuthType(etcdConfig)
		convey.So(etcdAuth, convey.ShouldResemble, &noAuth{})
	})
	convey.Convey("tlsAuth", t, func() {
		etcdConfig := EtcdConfig{
			SslEnable: false,
			Password:  "p123",
		}
		etcdAuth := GetEtcdAuthType(etcdConfig)
		convey.So(etcdAuth, convey.ShouldResemble, &pwdAuth{
			user:     etcdConfig.User,
			password: etcdConfig.Password,
		})
	})
	convey.Convey("clientTLSAuth", t, func() {
		etcdConfig := EtcdConfig{
			AuthType:       "TLS",
			CaFile:         "CaFile",
			CertFile:       "CertFile",
			KeyFile:        "KeyFile",
			PassphraseFile: "PassphraseFile",
		}
		etcdAuth := GetEtcdAuthType(etcdConfig)
		convey.So(etcdAuth, convey.ShouldResemble, &clientTLSAuth{
			cerfile:        []byte("CertFile"),
			keyfile:        []byte("KeyFile"),
			cafile:         []byte("CaFile"),
			passphrasefile: []byte("PassphraseFile"),
		})
	})
}

func TestGetEtcdConfig(t *testing.T) {
	defer gomonkey.ApplyFunc(localauth.Decrypt, func(src string) ([]byte, error) {
		return []byte(strings.Clone(src)), nil
	}).Reset()
	convey.Convey("noAuth", t, func() {
		noAuth := &noAuth{}
		cfg, err := noAuth.GetEtcdConfig()
		convey.So(cfg, convey.ShouldResemble, &clientv3.Config{})
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("tlsAuth", t, func() {
		defer gomonkey.ApplyFunc(tls.X509KeyPair, func(certPEMBlock []byte, keyPEMBlock []byte) (tls.Certificate, error) {
			return tls.Certificate{}, nil
		}).Reset()
		defer gomonkey.ApplyFunc(ioutil.ReadFile, func(filename string) ([]byte, error) {
			return []byte{}, nil
		}).Reset()
		defer gomonkey.ApplyFunc(commontls.LoadServerTLSCertificate, func(certFile, keyFile, passPhase, decryptTool string,
			isHTTPS bool) ([]tls.Certificate, error) {
			return nil, nil
		}).Reset()
		tlsAuth := &tlsAuth{
			user:     "root",
			password: string([]byte("123")),
		}
		cfg, err := tlsAuth.GetEtcdConfig()
		convey.So(err, convey.ShouldBeNil)
		convey.So(cfg, convey.ShouldNotBeNil)
	})
	convey.Convey("tlsAuth error", t, func() {
		defer gomonkey.ApplyFunc(localauth.Decrypt, func(src string) ([]byte, error) {
			return nil, errors.New("some error")
		}).Reset()
		tlsAuth := &tlsAuth{}
		_, err := tlsAuth.GetEtcdConfig()
		convey.So(err, convey.ShouldNotBeNil)
		tlsAuth.user, tlsAuth.password = "root", "123"
		_, err = tlsAuth.GetEtcdConfig()
		convey.So(err, convey.ShouldNotBeNil)
	})
	convey.Convey("pwdAuth", t, func() {
		pwdAuth := &pwdAuth{
			user:     "root",
			password: string([]byte("123")),
		}
		cfg, err := pwdAuth.GetEtcdConfig()
		convey.So(cfg.Password, convey.ShouldEqual, "123")
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("pwdAuth error", t, func() {
		defer gomonkey.ApplyFunc(localauth.Decrypt, func(src string) ([]byte, error) {
			return nil, errors.New("some error")
		}).Reset()
		pwdAuth := &pwdAuth{
			password: string([]byte("123")),
		}
		_, err := pwdAuth.GetEtcdConfig()
		convey.So(err, convey.ShouldNotBeNil)
		pwdAuth.user = "root"
		convey.So(err, convey.ShouldNotBeNil)
	})
}

func TestGetPassphrase(t *testing.T) {
	patches := []*Patches{
		ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
			return nil, nil
		}),
		ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
			return []byte("dummyPassphrase"), nil
		}),
		ApplyFunc(crypto.SCCInitialized, func() bool {
			return false
		}),
	}
	defer func() {
		for _, patch := range patches {
			time.Sleep(100 * time.Millisecond)
			patch.Reset()
		}
	}()

	c := &clientTLSAuth{
		passphrasefile: []byte("path/to/passphrasefile"),
	}

	passphrase, err := c.getPassphrase()
	assert.Nil(t, err)
	assert.Equal(t, []byte("dummyPassphrase"), passphrase)
}

func TestBuildStsCfg(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		patchesFunc mockUtils.PatchesFunc
	}{
		{"case1", false, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cert.LoadCerts, func() (*x509.CertPool, *tls.Certificate,
					error) {
					return &x509.CertPool{}, &tls.Certificate{}, nil
				})})
			return patches
		}},
		{"case2 LoadCerts error", true, func() mockUtils.PatchSlice {
			patches := mockUtils.InitPatchSlice()
			patches.Append(mockUtils.PatchSlice{
				gomonkey.ApplyFunc(cert.LoadCerts, func() (*x509.CertPool, *tls.Certificate,
					error) {
					return &x509.CertPool{}, &tls.Certificate{}, errors.New("error")
				})})
			return patches
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.patchesFunc()
			_, err := BuildStsCfg()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildStsCfg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			patches.ResetAll()
		})
	}
}

func TestGetTLSConfig(t *testing.T) {
	// Create test data
	testKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	keyDER := x509.MarshalPKCS1PrivateKey(testKey)
	encryptedPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "ENCRYPTED PRIVATE KEY",
		Bytes: keyDER,
	})
	certPEM := []byte("test cert")
	caCertPEM := []byte("test CA cert")
	keyPwd := []byte("test password")

	tests := []struct {
		name          string
		mockSetups    func() []*gomonkey.Patches
		expectedError bool
		errorContains string
	}{
		{
			name: "decrypt PEM block failure",
			mockSetups: func() []*gomonkey.Patches {
				var patches []*gomonkey.Patches

				p1 := gomonkey.ApplyFunc(pem.Decode, func(data []byte) (*pem.Block, []byte) {
					return &pem.Block{}, nil
				})
				patches = append(patches, p1)

				p2 := gomonkey.ApplyFunc(commonCrypto.DecryptPEMBlock, func(block *pem.Block, password []byte) ([]byte, error) {
					return nil, fmt.Errorf("decrypt error")
				})
				patches = append(patches, p2)

				return patches
			},
			expectedError: true,
			errorContains: "decrypt error",
		},
		{
			name: "parse private key failure",
			mockSetups: func() []*gomonkey.Patches {
				var patches []*gomonkey.Patches

				p1 := gomonkey.ApplyFunc(pem.Decode, func(data []byte) (*pem.Block, []byte) {
					return &pem.Block{}, nil
				})
				patches = append(patches, p1)

				p2 := gomonkey.ApplyFunc(commonCrypto.DecryptPEMBlock, func(block *pem.Block, password []byte) ([]byte, error) {
					return []byte("invalid key"), nil
				})
				patches = append(patches, p2)

				p3 := gomonkey.ApplyFunc(x509.ParsePKCS1PrivateKey, func(der []byte) (*rsa.PrivateKey, error) {
					return nil, fmt.Errorf("parse error")
				})
				patches = append(patches, p3)

				return patches
			},
			expectedError: true,
			errorContains: "parse error",
		},
		{
			name: "append CA cert failure",
			mockSetups: func() []*gomonkey.Patches {
				var patches []*gomonkey.Patches

				p1 := gomonkey.ApplyFunc(pem.Decode, func(data []byte) (*pem.Block, []byte) {
					return &pem.Block{}, nil
				})
				patches = append(patches, p1)

				p2 := gomonkey.ApplyFunc(commonCrypto.DecryptPEMBlock, func(block *pem.Block, password []byte) ([]byte, error) {
					return keyDER, nil
				})
				patches = append(patches, p2)

				p3 := gomonkey.ApplyFunc(x509.ParsePKCS1PrivateKey, func(der []byte) (*rsa.PrivateKey, error) {
					return testKey, nil
				})
				patches = append(patches, p3)

				p4 := gomonkey.ApplyMethod((*x509.CertPool)(nil), "AppendCertsFromPEM",
					func(_ *x509.CertPool, _ []byte) bool {
						return false
					})
				patches = append(patches, p4)

				return patches
			},
			expectedError: true,
			errorContains: "failed to append CA certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.mockSetups()
			defer func() {
				for _, p := range patches {
					p.Reset()
				}
			}()

			c := &clientTLSAuth{}
			config, err := c.getTLSConfig(encryptedPEM, keyPwd, certPEM, caCertPEM)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func mockTls(keyDER []byte, testKey *rsa.PrivateKey) []*Patches {
	var patches []*gomonkey.Patches

	// Mock pem.Decode
	p1 := gomonkey.ApplyFunc(pem.Decode, func(data []byte) (*pem.Block, []byte) {
		return &pem.Block{
			Type:  "ENCRYPTED PRIVATE KEY",
			Bytes: keyDER,
		}, nil
	})
	patches = append(patches, p1)

	// Mock commonCrypto.DecryptPEMBlock
	p2 := gomonkey.ApplyFunc(commonCrypto.DecryptPEMBlock, func(block *pem.Block, password []byte) ([]byte, error) {
		return keyDER, nil
	})
	patches = append(patches, p2)

	// Mock x509.ParsePKCS1PrivateKey
	p3 := gomonkey.ApplyFunc(x509.ParsePKCS1PrivateKey, func(der []byte) (*rsa.PrivateKey, error) {
		return testKey, nil
	})
	patches = append(patches, p3)

	// Mock certPool.AppendCertsFromPEM
	p4 := gomonkey.ApplyMethod((*x509.CertPool)(nil), "AppendCertsFromPEM",
		func(_ *x509.CertPool, _ []byte) bool {
			return true
		})
	patches = append(patches, p4)

	// Mock tls.X509KeyPair
	p5 := gomonkey.ApplyFunc(tls.X509KeyPair, func(certPEM, keyPEM []byte) (tls.Certificate, error) {
		return tls.Certificate{}, nil
	})
	patches = append(patches, p5)

	return patches
}

func TestTlsAuthGetEtcdConfig(t *testing.T) {
	patches := []*Patches{
		ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
			return nil, nil
		}),
		ApplyFunc(os.ReadFile, func(string) ([]byte, error) {
			return []byte("dummyPassphrase"), nil
		}),
		ApplyFunc(crypto.SCCInitialized, func() bool {
			return false
		}),
	}
	defer func() {
		for _, patch := range patches {
			time.Sleep(100 * time.Millisecond)
			patch.Reset()
		}
	}()
	tests := []struct {
		name          string
		mockSetups    func() []*gomonkey.Patches
		expectedError bool
		errorContains string
	}{
		{
			name: "successful case",
			mockSetups: func() []*gomonkey.Patches {
				var patches []*gomonkey.Patches

				// Mock ioutil.ReadFile
				p2 := gomonkey.ApplyFunc(ioutil.ReadFile, func(filename string) ([]byte, error) {
					return []byte("test data"), nil
				})
				patches = append(patches, p2)

				testKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				keyDER := x509.MarshalPKCS1PrivateKey(testKey)
				tlsMocks := mockTls(keyDER, testKey)
				patches = append(patches, tlsMocks...)

				return patches
			},
			expectedError: false,
		},
		{
			name: "getPassphrase failure",
			mockSetups: func() []*gomonkey.Patches {
				p1 := gomonkey.ApplyFunc(pem.Decode, func(data []byte) (*pem.Block, []byte) {
					return nil, nil
				})
				return []*gomonkey.Patches{p1}
			},
			expectedError: true,
			errorContains: "failed to decode key PEM block",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := tt.mockSetups()
			defer func() {
				for _, p := range patches {
					p.Reset()
				}
			}()

			c := &clientTLSAuth{
				cerfile: []byte("cert.pem"),
				cafile:  []byte("ca.pem"),
				keyfile: []byte("key.pem"),
			}
			config, err := c.GetEtcdConfig()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}
