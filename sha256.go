/*
 * Minio Cloud Storage, (C) 2016 Minio, Inc.
 * Aidos Developer, 2017
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

package sha256

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

// Size - The size of a SHA256 checksum in bytes.
const Size = 32

// BlockSize - The blocksize of SHA256 in bytes.
const BlockSize = 64

const (
	chunk = 64
)

//initial values for SHA-256
const (
	Init0 = 0x6A09E667
	Init1 = 0xBB67AE85
	Init2 = 0x3C6EF372
	Init3 = 0xA54FF53A
	Init4 = 0x510E527F
	Init5 = 0x9B05688C
	Init6 = 0x1F83D9AB
	Init7 = 0x5BE0CD19
)

// digest represents the partial evaluation of a checksum.
type digest struct {
	h   [8]uint32
	x   [chunk]byte
	nx  int
	len uint64
}

// Reset digest back to default
func (d *digest) Reset() {
	d.h[0] = Init0
	d.h[1] = Init1
	d.h[2] = Init2
	d.h[3] = Init3
	d.h[4] = Init4
	d.h[5] = Init5
	d.h[6] = Init6
	d.h[7] = Init7
	d.nx = 0
	d.len = 0
}

//Block calcs 1 block in sha256
func Block(h []uint32, p []byte) {
	switch true {
	case avx2:
		blockAvx2GoDirect(h, p)
	case avx:
		blockAvxGoDirect(h, p)
	case ssse3:
		blockSsseGoDirect(h, p)
	case armSha:
		blockArmGoDirect(h, p)
	default:
		blockGenericDirect(h, p)
	}
}

func block(dig *digest, p []byte) {
	switch true {
	case avx2:
		blockAvx2Go(dig, p)
	case avx:
		blockAvxGo(dig, p)
	case ssse3:
		blockSsseGo(dig, p)
	case armSha:
		blockArmGo(dig, p)
	default:
		blockGeneric(dig, p)
	}
}

//Int2Bytes converts internal states of SHA-256 ([]uint32)  to bytes array
func Int2Bytes(h []uint32, digest []byte) {
	for i, s := range h {
		binary.BigEndian.PutUint32(digest[i<<2:], s)
	}
}

// New returns a new hash.Hash computing the SHA256 checksum.
func New() hash.Hash {
	if avx2 || avx || ssse3 || armSha {
		d := new(digest)
		d.Reset()
		return d
	}
	// Fallback to the standard golang implementation
	// if no features were found.
	return sha256.New()
}

// Sum256 - single caller sha256 helper
func Sum256(data []byte) [Size]byte {
	var d digest
	d.Reset()
	d.Write(data)
	return d.checkSum()
}

//Sum256D32 returns sha256 of 256 bytes data.
func Sum256D32(data []byte) [Size]byte {
	stat := []uint32{
		Init0,
		Init1,
		Init2,
		Init3,
		Init4,
		Init5,
		Init6,
		Init7,
	}
	buf := make([]byte, 64)
	copy(buf, data)
	buf[32] = 0x80
	buf[62] = 0x01
	// buf[63] = 0x00
	Block(stat, buf)
	var out [Size]byte
	Int2Bytes(stat, out[:])
	return out
}

// Return size of checksum
func (d *digest) Size() int { return Size }

// Return blocksize of checksum
func (d *digest) BlockSize() int { return BlockSize }

// Write to digest
func (d *digest) Write(p []byte) (nn int, err error) {
	nn = len(p)
	d.len += uint64(nn)
	if d.nx > 0 {
		n := copy(d.x[d.nx:], p)
		d.nx += n
		if d.nx == chunk {
			block(d, d.x[:])
			d.nx = 0
		}
		p = p[n:]
	}
	if len(p) >= chunk {
		n := len(p) &^ (chunk - 1)
		block(d, p[:n])
		p = p[n:]
	}
	if len(p) > 0 {
		d.nx = copy(d.x[:], p)
	}
	return
}

// Return sha256 sum in bytes
func (d *digest) Sum(in []byte) []byte {
	// Make a copy of d0 so that caller can keep writing and summing.
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

// Intermediate checksum function
func (d *digest) checkSum() [Size]byte {
	len := d.len
	// Padding.  Add a 1 bit and 0 bits until 56 bytes mod 64.
	var tmp [64]byte
	tmp[0] = 0x80
	if len%64 < 56 {
		d.Write(tmp[0 : 56-len%64])
	} else {
		d.Write(tmp[0 : 64+56-len%64])
	}

	// Length in bits.
	len <<= 3
	for i := uint(0); i < 8; i++ {
		tmp[i] = byte(len >> (56 - 8*i))
	}
	d.Write(tmp[0:8])

	if d.nx != 0 {
		panic("d.nx != 0")
	}

	h := d.h[:]

	var digest [Size]byte
	for i, s := range h {
		digest[i*4] = byte(s >> 24)
		digest[i*4+1] = byte(s >> 16)
		digest[i*4+2] = byte(s >> 8)
		digest[i*4+3] = byte(s)
	}

	return digest
}
