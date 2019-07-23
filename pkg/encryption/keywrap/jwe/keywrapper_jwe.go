/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package jwe

import (
	"crypto/ecdsa"

	"github.com/containerd/containerd/pkg/encryption/keywrap"
	"github.com/containerd/containerd/pkg/encryption/utils"
	"github.com/pkg/errors"
	jose "gopkg.in/square/go-jose.v2"
)

type JWEConfig struct {
	PublicKeys           [][]byte
	PrivateKeys          [][]byte
	PrivateKeysPasswords [][]byte
}

type jweKeyWrapper struct {
}

func (kw *jweKeyWrapper) GetAnnotationID() string {
	return "org.opencontainers.image.enc.keys.jwe"
}

// NewKeyWrapper returns a new key wrapping interface using jwe
func NewKeyWrapper() keywrap.KeyWrapper {
	return &jweKeyWrapper{}
}

// WrapKeys wraps the session key for recpients and encrypts the optsData, which
// describe the symmetric key used for encrypting the layer
func (kw *jweKeyWrapper) WrapKeys(ic interface{}, optsData []byte) ([]byte, error) {
	ec, ok := ic.(*JWEConfig)
	if !ok {
		return nil, errors.New("unsupported encryption config")
	}

	var joseRecipients []jose.Recipient

	err := addPubKeys(&joseRecipients, ec.PublicKeys)
	if err != nil {
		return nil, err
	}
	// no recipients is not an error...
	if len(joseRecipients) == 0 {
		return nil, nil
	}

	encrypter, err := jose.NewMultiEncrypter(jose.A256GCM, joseRecipients, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "jose.NewMultiEncrypter failed")
	}
	jwe, err := encrypter.Encrypt(optsData)
	if err != nil {
		return nil, errors.Wrapf(err, "JWE Encrypt failed")
	}
	return []byte(jwe.FullSerialize()), nil
}

func (kw *jweKeyWrapper) UnwrapKey(ic interface{}, jweString []byte) ([]byte, error) {
	dc, ok := ic.(*JWEConfig)
	if !ok {
		return nil, errors.New("unsupported encryption config")
	}

	jwe, err := jose.ParseEncrypted(string(jweString))
	if err != nil {
		return nil, errors.New("jose.ParseEncrypted failed")
	}

	privKeys := dc.PrivateKeys
	if len(privKeys) == 0 {
		return nil, errors.New("No private keys found for JWE decryption")
	}
	privKeysPasswords := dc.PrivateKeysPasswords
	if len(privKeysPasswords) != len(privKeys) {
		return nil, errors.New("Private key password array length must be same as that of private keys")
	}

	for idx, privKey := range privKeys {
		key, err := utils.ParsePrivateKey(privKey, privKeysPasswords[idx], "JWE")
		if err != nil {
			return nil, err
		}
		_, _, plain, err := jwe.DecryptMulti(key)
		if err == nil {
			return plain, nil
		}
	}
	return nil, errors.New("JWE: No suitable private key found for decryption")
}

func (kw *jweKeyWrapper) GetPrivateKeys(ic interface{}) [][]byte {
	dc, ok := ic.(*JWEConfig)
	if !ok {
		return nil
	}
	return dc.PrivateKeys
}

func (kw *jweKeyWrapper) GetKeyIdsFromPacket(b64jwes string) ([]uint64, error) {
	return nil, nil
}

func (kw *jweKeyWrapper) GetRecipients(b64jwes string) ([]string, error) {
	return []string{"[jwe]"}, nil
}

func addPubKeys(joseRecipients *[]jose.Recipient, pubKeys [][]byte) error {
	if len(pubKeys) == 0 {
		return nil
	}
	for _, pubKey := range pubKeys {
		key, err := utils.ParsePublicKey(pubKey, "JWE")
		if err != nil {
			return err
		}

		alg := jose.RSA_OAEP
		switch key.(type) {
		case *ecdsa.PublicKey:
			alg = jose.ECDH_ES_A256KW
		}

		*joseRecipients = append(*joseRecipients, jose.Recipient{
			Algorithm: alg,
			Key:       key,
		})
	}
	return nil
}
