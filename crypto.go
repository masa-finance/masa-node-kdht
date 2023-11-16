package main

import (
	"encoding/hex"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/sirupsen/logrus"
)

func CreatePrivateKey() (privKey crypto.PrivKey, err error) {
	// Check if the private key file is set in the environment
	envKey := os.Getenv("PRIVATE_KEY")
	if envKey != "" {
		rawKey, err := hex.DecodeString(envKey)
		if err != nil {
			logrus.Errorf("Error decoding private key: %s\n", err)
			return nil, err
		}
		privKey, err = crypto.UnmarshalPrivateKey(rawKey)
		if err != nil {
			logrus.Errorf("Error unmarshalling private key: %s\n", err)
			return nil, err
		}
	} else {
		// Generate a new private key
		privKey, _, err = crypto.GenerateKeyPair(crypto.Secp256k1, 2048)
		if err != nil {
			return nil, err
		}
	}
	return privKey, nil
}
