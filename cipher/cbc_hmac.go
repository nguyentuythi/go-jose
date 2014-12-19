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
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"github.com/apexskier/cryptoPadding"
	"hash"
)

const (
	nonceBytes = 16
)

// NewCBCHMAC instantiates a new AEAD based on CBC+HMAC.
func NewCBCHMAC(key []byte, newBlockCipher func([]byte) (cipher.Block, error)) (cipher.AEAD, error) {
	keySize := len(key) / 2
	integrityKey := key[:keySize]
	encryptionKey := key[keySize:]

	blockCipher, err := newBlockCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	return &cbcAEAD{
		encryptionKey: encryptionKey,
		integrityKey:  integrityKey,
		blockCipher:   blockCipher,
		authtagBytes:  keySize,
	}, nil
}

// An AEAD based on CBC+HMAC
type cbcAEAD struct {
	encryptionKey []byte
	integrityKey  []byte
	authtagBytes  int
	blockCipher   cipher.Block
}

func (ctx *cbcAEAD) NonceSize() int {
	return nonceBytes
}

func (ctx *cbcAEAD) Overhead() int {
	// Maximum overhead is block size (for padding) plus auth tag length
	return ctx.blockCipher.BlockSize() + ctx.authtagBytes
}

// Seal encrypts and authenticates the plaintext.
func (ctx *cbcAEAD) Seal(dst, nonce, plaintext, data []byte) []byte {
	// Output buffer -- must take care not to mangle plaintext input.
	ciphertext := make([]byte, len(plaintext)+ctx.Overhead())[:len(plaintext)]
	copy(ciphertext, plaintext)

	cbc := cipher.NewCBCEncrypter(ctx.blockCipher, nonce)
	padding := new(cryptoPadding.PKCS7)
	ciphertext, err := padding.Pad(ciphertext, ctx.blockCipher.BlockSize())
	if err != nil {
		panic(err)
	}

	cbc.CryptBlocks(ciphertext, ciphertext)
	authtag := ctx.computeAuthTag(data, nonce, ciphertext)

	ret, out := resize(dst, len(dst)+len(ciphertext)+len(authtag))
	copy(out, ciphertext)
	copy(out[len(ciphertext):], authtag)

	return ret
}

// Open decrypts and authenticates the ciphertext.
func (ctx *cbcAEAD) Open(dst, nonce, ciphertext, data []byte) ([]byte, error) {
	if len(ciphertext) < ctx.authtagBytes {
		return nil, errors.New("square/go-jose: invalid ciphertext (too short)")
	}

	offset := len(ciphertext) - ctx.authtagBytes
	expectedTag := ctx.computeAuthTag(data, nonce, ciphertext[:offset])
	match := subtle.ConstantTimeCompare(expectedTag, ciphertext[offset:])
	if match != 1 {
		return nil, errors.New("square/go-jose: invalid ciphertext (auth tag mismatch)")
	}

	cbc := cipher.NewCBCDecrypter(ctx.blockCipher, nonce)
	buffer := []byte(ciphertext[:offset])
	cbc.CryptBlocks(buffer, buffer)

	// Remove padding
	padding := new(cryptoPadding.PKCS7)
	plaintext, err := padding.Unpad(buffer, ctx.blockCipher.BlockSize())
	if err != nil {
		return nil, err
	}

	ret, out := resize(dst, len(dst)+len(plaintext))
	copy(out, plaintext)

	return ret, nil
}

// Compute an authentication tag
func (ctx *cbcAEAD) computeAuthTag(aad, nonce, ciphertext []byte) []byte {
	buffer := []byte(aad)
	buffer = append(buffer, nonce...)
	buffer = append(buffer, ciphertext...)
	buffer = append(buffer, bitLen(aad)...)

	var hash func() hash.Hash
	switch len(ctx.integrityKey) {
	case 16:
		hash = sha256.New
	case 24:
		hash = sha512.New384
	case 32:
		hash = sha512.New
	}

	hmac := hmac.New(hash, ctx.integrityKey)

	// According to documentation, Write() on hash.Hash never fails.
	_, _ = hmac.Write(buffer)

	return hmac.Sum(nil)[:ctx.authtagBytes]
}

// Helper function for serializing bit length into array
func bitLen(input []byte) []byte {
	encodedLen := make([]byte, 8)
	binary.BigEndian.PutUint64(encodedLen, uint64(len(input)*8))
	return encodedLen
}

// resize ensures the the given slice has a capacity of at least n bytes.
// If the capacity of the slice is less than n, a new slice is allocated
// and the existing data will be copied.
func resize(in []byte, n int) (head, tail []byte) {
	if cap(in) >= n {
		head = in[:n]
	} else {
		head = make([]byte, n)
		copy(head, in)
	}

	tail = head[len(in):]
	return
}
