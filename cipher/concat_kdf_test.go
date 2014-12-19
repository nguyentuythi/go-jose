/*-
 * Copyright 2014 Square Inc.
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

package josecipher

import (
	"bytes"
	"crypto"
	"testing"
)

// Taken from: https://tools.ietf.org/id/draft-ietf-jose-json-web-algorithms-38.txt
func TestVectorConcatKDF(t *testing.T) {
	z := []byte{
		158, 86, 217, 29, 129, 113, 53, 211, 114, 131, 66, 131, 191, 132,
		38, 156, 251, 49, 110, 163, 218, 128, 106, 72, 246, 218, 167, 121,
		140, 254, 144, 196}

	algID := []byte{0, 0, 0, 7, 65, 49, 50, 56, 71, 67, 77}

	ptyUInfo := []byte{0, 0, 0, 5, 65, 108, 105, 99, 101}
	ptyVInfo := []byte{0, 0, 0, 3, 66, 111, 98}

	supPubInfo := []byte{0, 0, 0, 128}
	supPrivInfo := []byte{}

	expected := []byte{
		86, 170, 141, 234, 248, 35, 109, 32, 92, 34, 40, 205, 113, 167, 16, 26}

	ckdf := NewConcatKDF(crypto.SHA256, z, algID, ptyUInfo, ptyVInfo, supPubInfo, supPrivInfo)

	out0 := make([]byte, 9)
	out1 := make([]byte, 7)

	read0, err := ckdf.Read(out0)
	if err != nil {
		t.Error("error when reading from concat kdf reader", err)
		return
	}

	read1, err := ckdf.Read(out1)
	if err != nil {
		t.Error("error when reading from concat kdf reader", err)
		return
	}

	if read0+read1 != len(out0)+len(out1) {
		t.Error("did not receive enough bytes from concat kdf reader")
		return
	}

	out := []byte{}
	out = append(out, out0...)
	out = append(out, out1...)

	if bytes.Compare(out, expected) != 0 {
		t.Error("did not receive expected output from concat kdf reader")
		return
	}
}

func TestCache(t *testing.T) {
	z := []byte{
		158, 86, 217, 29, 129, 113, 53, 211, 114, 131, 66, 131, 191, 132,
		38, 156, 251, 49, 110, 163, 218, 128, 106, 72, 246, 218, 167, 121,
		140, 254, 144, 196}

	algID := []byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4}

	ptyUInfo := []byte{1, 2, 3, 4}
	ptyVInfo := []byte{4, 3, 2, 1}

	supPubInfo := []byte{}
	supPrivInfo := []byte{}

	outputs := [][]byte{}

	// Read the same amount of data in different chunk sizes
	for i := 10; i <= 100; i++ {
		out := make([]byte, 1024)
		reader := NewConcatKDF(crypto.SHA256, z, algID, ptyUInfo, ptyVInfo, supPubInfo, supPrivInfo)

		for j := 0; j < 1024/i; j++ {
			_, _ = reader.Read(out[j*i:])
		}

		outputs = append(outputs, out)
	}

	for i := range outputs {
		if bytes.Compare(outputs[i], outputs[i%len(outputs)]) != 0 {
			t.Error("not all outputs from KDF matched")
		}
	}
}

func benchmarkKDF(b *testing.B, total, chunksize int) {
	z := []byte{
		158, 86, 217, 29, 129, 113, 53, 211, 114, 131, 66, 131, 191, 132,
		38, 156, 251, 49, 110, 163, 218, 128, 106, 72, 246, 218, 167, 121,
		140, 254, 144, 196}

	algID := []byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4}

	ptyUInfo := []byte{1, 2, 3, 4}
	ptyVInfo := []byte{4, 3, 2, 1}

	supPubInfo := []byte{}
	supPrivInfo := []byte{}

	out := make([]byte, total)
	reader := NewConcatKDF(crypto.SHA256, z, algID, ptyUInfo, ptyVInfo, supPubInfo, supPrivInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < total/chunksize; j++ {
			_, _ = reader.Read(out[j*chunksize:])
		}
	}
}

func BenchmarkConcatKDF_4k_64(b *testing.B) {
	benchmarkKDF(b, 4096, 64)
}

func BenchmarkConcatKDF_4k_1024(b *testing.B) {
	benchmarkKDF(b, 4096, 1024)
}
