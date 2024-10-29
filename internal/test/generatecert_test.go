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
	"fmt"
	"testing"
)

func Test_DropFile(t *testing.T) {
	t.Run("Drop File", func(t *testing.T) {
		_ = dropFile("/Volumes/ext/project/repo/device-opc-ua/internal/test/server_pk.pem")
	})
}

func Test_CreatePublicKeyPem(t *testing.T) {
	t.Run("Create Public Key PEM", func(t *testing.T) {
		certInfo, err := CreateCerts()
		if err != nil {
			t.Errorf("Create Public Key PEM failed, %v", err)
		}
		fmt.Printf("CertInfo: %v \n", certInfo)
		Clean(certInfo)
	})
}

func Test_GetIPList(t *testing.T) {
	t.Run("Get IP List", func(t *testing.T) {
		_, err := ip4AddrList()
		if err != nil {
			t.Errorf("Get IP List failed, %v", err)
		}
	})
}
