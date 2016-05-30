package main

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestHandshake(t *testing.T) {
	assert := assert.New(t)
	config.serverKeys = testServerSecret
	peerAddr := Sockaddr{IP: net.ParseIP("127.0.0.1"), Port: 8755}

	InitPeers()
	peer := &Peer{
		ourHandshakeKey: testHandshakeKey,
	}
	peers[string(peerAddr.Raw())] = peer

	// Handshake request (0x01)
	msg := readTestmsg("null-request.dat")

	// Handle request and build response (0x02)
	reply := handlePacket(msg)
	assert.NotNil(reply)
	assert.NotNil(reply.SignKey)
	assert.Equal([]byte{2}, reply.Records[RECORD_HANDSHAKE_TYPE])
	assert.Equal([]byte{0}, reply.Records[RECORD_REPLY_CODE])
	assert.Equal(msg.Records[RECORD_SENDER_KEY], reply.Records[RECORD_RECIPIENT_KEY])
	assert.Equal(msg.Records[RECORD_PROTOCOL_NAME], reply.Records[RECORD_PROTOCOL_NAME])

	// Handshake finish (0x03)
	msg = readTestmsg("null-finish.dat")

	// Handle finish
	reply = handlePacket(msg)
	assert.Nil(reply)
	assert.Nil(peer.sharedKey)
}