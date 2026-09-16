package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
)

const authReplyBufferSize = 512

type noChallengeError struct {
	server string
}

func (nce noChallengeError) Error() string {
	return nce.server + " no challenge code, secret-file disabled"
}

func varnishAuth(server, secret string, conn net.Conn) error {
	// I want to allocate enough bytes to read the varnish help output.
	reply := make([]byte, authReplyBufferSize)

	_, err := conn.Read(reply)
	if err != nil {
		return fmt.Errorf("%s %w", server, err)
	}

	rp := regexp.MustCompile("[a-z]{32}") // find challenge string

	challenge := rp.FindString(string(reply))
	if challenge == "" {
		return noChallengeError{server: server}
	}

	// time to authenticate
	hash := sha256.New()
	hash.Write([]byte(challenge + "\n" + secret + "\n" + challenge + "\n"))
	md := hash.Sum(nil)
	mdStr := hex.EncodeToString(md)

	_, err = conn.Write([]byte("auth " + mdStr + "\n"))
	if err != nil {
		return fmt.Errorf("%s %w", server, err)
	}

	authReply := make([]byte, authReplyBufferSize)

	_, err = conn.Read(authReply)
	if err != nil {
		return fmt.Errorf("%s %w", server, err)
	}

	return nil
}
