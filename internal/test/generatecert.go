/*
 * Copyright (c) 2024.  liushenglong_8597@outlook.com.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	serverPkFileName      = "server_pk.pem"
	serverCertPemFileName = "server_cert.pem"
	serverCertDerFileName = "server_cert.der"
	clientPkFileName      = "client_pk.pem"
	clientCertPemFileName = "client_cert.pem"
)

type CertInfo struct {
	ServerPKPath      string
	ServerPEMCertPath string
	ServerDERCertPath string
	ClientPKPath      string
	ClientPEMCertPath string
}

func Clean(info *CertInfo) {
	_ = dropFile(info.ServerPKPath)
	_ = dropFile(info.ServerPEMCertPath)
	_ = dropFile(info.ServerDERCertPath)
	_ = dropFile(info.ClientPKPath)
	_ = dropFile(info.ClientPEMCertPath)
}

func dropFile(path string) error {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	err = os.Remove(path)
	if err != nil {
		panic(err)
	}
	return nil
}

func CreateCerts() (*CertInfo, error) {
	addrList, err := ip4AddrList()
	if err != nil {
		return nil, err
	}
	clientAltName := fmt.Sprintf("URI:urn:%v", addrList[0])
	clientTemplate := createTemplate(
		[]string{clientAltName},
		pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{"cn.brk2outside"},
			OrganizationalUnit: []string{"edgex-opcua"},
			Locality:           []string{"Wuhan"},
			Province:           []string{"Hubei"},
		},
		addrList,
		x509.KeyUsageDataEncipherment|x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign|x509.KeyUsageCRLSign,
		x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
	)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, clientTemplate, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	sprintf := fmt.Sprintf("URI:urn:%v:FreeOpaUA:python-opcua", addrList[0])
	serverTemplate := createTemplate(
		[]string{sprintf},
		pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{"cn.brk2outside"},
			OrganizationalUnit: []string{"edgex-opcua"},
			Locality:           []string{"Wuhan"},
			Province:           []string{"Hubei"},
		},
		addrList,
		x509.KeyUsageDataEncipherment|x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign|x509.KeyUsageCRLSign,
		x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
	)
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serverCertBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, serverTemplate, &serverKey.PublicKey, serverKey)
	if err != nil {
		return nil, err
	}
	serverKeyAbsPath, err := savePkcs1PrivateKey(serverKey, serverPkFileName)
	if err != nil {
		return nil, err
	}
	serverCertPemAbsPath, err := savePublicKeyPEM(serverCertBytes, serverCertPemFileName)
	if err != nil {
		return nil, err
	}
	serverCertDerAbsPath, err := savePublicKeyDer(serverCertBytes, serverCertDerFileName)
	if err != nil {
		return nil, err
	}
	clientPkAbsPath, err := savePkcs1PrivateKey(key, clientPkFileName)
	if err != nil {
		return nil, err
	}
	clientCertAbsPath, err := savePublicKeyPEM(clientCertBytes, clientCertPemFileName)
	if err != nil {
		return nil, err
	}
	return &CertInfo{
		ServerPKPath:      serverKeyAbsPath,
		ServerPEMCertPath: serverCertPemAbsPath,
		ServerDERCertPath: serverCertDerAbsPath,
		ClientPKPath:      clientPkAbsPath,
		ClientPEMCertPath: clientCertAbsPath,
	}, nil
}

func savePublicKeyDer(bytes []byte, path string) (absPath string, err error) {
	if _, err = os.Stat(path); errors.Is(err, os.ErrExist) {
		err = os.Remove(path)
		if err != nil {
			return
		}
	}
	pubkeyFile, err := os.Create(path)
	if err != nil {
		return
	}
	_, err = pubkeyFile.Write(bytes)
	if err != nil {
		return
	}
	absPath, err = filepath.Abs(path)
	if err != nil {
		return
	}
	return
}

func savePublicKeyPEM(bytes []byte, path string) (absPath string, err error) {
	if _, err = os.Stat(path); errors.Is(err, os.ErrExist) {
		err = os.Remove(path)
		if err != nil {
			return
		}
	}
	pubkeyFile, err := os.Create(path)
	if err != nil {
		return
	}
	err = pem.Encode(pubkeyFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: bytes,
	})
	if err != nil {
		return
	}
	absPath, err = filepath.Abs(path)
	if err != nil {
		return
	}
	return
}

func savePkcs1PrivateKey(key *rsa.PrivateKey, path string) (absPath string, err error) {
	if _, err = os.Stat(path); errors.Is(err, os.ErrExist) {
		err = os.Remove(path)
		if err != nil {
			return
		}
	}
	pkFile, err := os.Create(path)
	if err != nil {
		return
	}

	err = pem.Encode(pkFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err != nil {
		return
	}
	absPath, err = filepath.Abs(path)
	if err != nil {
		return
	}
	return
}

func createTemplate(subjectAltName []string, subj pkix.Name, addresses []net.IP, keyUsage x509.KeyUsage, extKeyUsage ...x509.ExtKeyUsage) *x509.Certificate {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject:      subj,
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		IPAddresses:  addresses,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		IsCA:         true,
		ExtKeyUsage:  extKeyUsage,
		KeyUsage:     keyUsage,
	}
	if len(subjectAltName) > 0 {
		extension := pkix.Extension{
			Id:       asn1.ObjectIdentifier{2, 5, 29, 17},
			Critical: false,
			Value:    []byte(strings.Join(subjectAltName, ", ")),
		}
		template.Extensions = []pkix.Extension{extension}
	}
	return template
}

// ip4AddrList walk through local address
func ip4AddrList() ([]net.IP, error) {
	ret := make([]net.IP, 0)

	iFaces, err := net.Interfaces()
	if err != nil {
		return ret, err
	}
	for _, iface := range iFaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				if v.IP.IsLoopback() || v.IP.To4() == nil {
					continue
				}
				ip = v.IP
			default:
				continue
			}
			ret = append(ret, ip)
		}
	}
	return ret, nil
}
