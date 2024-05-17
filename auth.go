package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"regexp"
)

func varnishAuth(server string, secret string, conn net.Conn) error {
	// I want to allocate 512 bytes, enough to read the varnish help output.
	reply := make([]byte, 512)
	_, err := conn.Read(reply)
	if err != nil {
		return errors.New(server + " " + err.Error())
	}
	rp := regexp.MustCompile("[a-z]{32}") //find challenge string
	challenge := rp.FindString(string(reply))
	if challenge != "" {
		// time to authenticate
		hash := sha256.New()
		hash.Write([]byte(challenge + "\n" + secret + "\n" + challenge + "\n"))
		md := hash.Sum(nil)
		mdStr := hex.EncodeToString(md)
		_, err := conn.Write([]byte("auth " + mdStr + "\n"))
		if err != nil {
			return errors.New(server + " " + err.Error())
		}
		auth_reply := make([]byte, 512)
		_, err = conn.Read(auth_reply)
		if err != nil {
			return errors.New(server + " " + err.Error())
		}
		return nil
	} else {
		return errors.New(server + " no challenge code, secret-file disabled.")
	}
}
