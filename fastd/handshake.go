package fastd

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/digineo/fastd/ifconfig"
	"log"
	"reflect"
	"time"
)

func (srv *Server) handlePacket(msg *Message) (reply *Message) {
	records := msg.Records
	var handshakeType byte

	if val := records[RECORD_HANDSHAKE_TYPE]; len(val) != 1 {
		log.Printf("%v handshake type missing", msg.Src)
		return
	} else {
		handshakeType = val[0]
	}

	senderKey := records[RECORD_SENDER_KEY]
	recipientKey := records[RECORD_RECIPIENT_KEY]
	senderHandshakeKey := records[RECORD_SENDER_HANDSHAKE_KEY]

	log.Printf("%v received handshake type=%x version=%s hostname=%s pubkey=%s", msg.Src, handshakeType, records[RECORD_VERSION_NAME], records[RECORD_HOSTNAME], hex.EncodeToString(senderKey))

	if reflect.DeepEqual(msg.Src, msg.Dst) {
		log.Printf("%v source address equals destination address", msg.Src)
		return
	}

	reply = msg.NewReply()

	if recipientKey == nil {
		log.Printf("%v recipient key missing", msg.Src)
		reply.SetError(REPLY_RECORD_MISSING, RECORD_RECIPIENT_KEY)
		return
	}

	if !bytes.Equal(recipientKey, srv.config.serverKeys.public[:]) {
		log.Printf("%v recipient key invalid: %s", msg.Src, hex.EncodeToString(recipientKey))
		reply.SetError(REPLY_UNACCEPTABLE_VALUE, RECORD_RECIPIENT_KEY)
		return
	}

	if senderKey == nil {
		log.Printf("%v sender key missing", msg.Src)
		reply.SetError(REPLY_RECORD_MISSING, RECORD_SENDER_KEY)
		return
	}

	if senderHandshakeKey == nil {
		log.Printf("%v sender handshake key missing", msg.Src)
		reply.SetError(REPLY_RECORD_MISSING, RECORD_SENDER_HANDSHAKE_KEY)
		return
	}

	peer := srv.GetPeer(msg.Src)
	peer.PublicKey = senderKey
	peer.peerHandshakeKey = senderHandshakeKey
	peer.lastSeen = time.Now()

	if !peer.makeSharedHandshakeKey(srv.config.serverKeys) {
		log.Printf("%v unable to make shared handshake key", msg.Src)
		return nil
	}

	reply.SignKey = peer.sharedKey
	reply.Records[RECORD_REPLY_CODE] = []byte{REPLY_SUCCESS}
	reply.Records[RECORD_METHOD_LIST] = []byte("null")
	reply.Records[RECORD_VERSION_NAME] = []byte("v18")
	reply.Records[RECORD_MTU] = records[RECORD_MTU]
	reply.Records[RECORD_SENDER_KEY] = srv.config.serverKeys.public[:]
	reply.Records[RECORD_SENDER_HANDSHAKE_KEY] = peer.ourHandshakeKey.public[:]
	reply.Records[RECORD_RECIPIENT_KEY] = senderKey
	reply.Records[RECORD_RECIPIENT_HANDSHAKE_KEY] = senderHandshakeKey

	switch handshakeType {
	case 1:
		if peer.Ifname != "" {
			log.Printf("%v already connected to %s", msg.Src, peer.Ifname)
			return nil
		}

		if err := srv.verifyPeer(peer); err != nil {
			log.Printf("%v verify failed: %s", msg.Src, err)
			return nil
		}

		// Assign interface and addresses
		var err error
		peer.Ifname, err = Clone(msg.Src, senderKey)
		if err != nil {
			log.Printf("%v cloning failed: %s", msg.Src, err)
			return nil
		}

		if f := srv.config.AssignAddresses; f != nil {
			f(peer)
		}

		// Copy Vars
		if peer.Vars != nil {
			reply.Records[RECORD_VARS] = peer.Vars
		}

		// Copy IPv4 addresses into response
		if peer.IPv4.LocalAddr != nil && peer.IPv4.DestAddr != nil {
			reply.Records[RECORD_IPV4_ADDR] = []byte(peer.IPv4.DestAddr.To4())
			reply.Records[RECORD_IPV4_DSTADDR] = []byte(peer.IPv4.LocalAddr.To4())
		}

		// Copy IPv6 addresses into response
		if peer.IPv6.LocalAddr != nil && peer.IPv6.DestAddr != nil {
			reply.Records[RECORD_IPV6_ADDR] = []byte(peer.IPv6.DestAddr.To16())
			reply.Records[RECORD_IPV6_DSTADDR] = []byte(peer.IPv6.LocalAddr.To16())
		}
	case 3:
		msg.SignKey = peer.sharedKey
		if !srv.handleFinishHandshake(msg, reply, peer) {
			return nil
		}

		// Decode and set MTU
		if val := records[RECORD_MTU]; len(val) == 0 {
			log.Printf("%v mtu missing", msg.Src)
		} else if len(val) != 2 {
			log.Printf("%v mtu invalid: %v", msg.Src, val)
		} else {
			if mtu := binary.BigEndian.Uint16(val); mtu < 576 {
				log.Printf("%v mtu invalid: %d", msg.Src, mtu)
			} else {
				ifconfig.SetMTU(peer.Ifname, mtu)
			}
		}

	default:
		log.Printf("%v unsupported handshake type", msg.Src)
	}

	return
}

func (srv *Server) handleFinishHandshake(msg *Message, reply *Message, peer *Peer) bool {
	methodName := msg.Records[RECORD_METHOD_NAME]

	if methodName == nil {
		log.Println("method name missing")
		reply.SetError(REPLY_RECORD_MISSING, RECORD_METHOD_NAME)
		return true
	}
	if string(methodName) != "null" {
		log.Println("method name invalid:", methodName)
		reply.SetError(REPLY_UNACCEPTABLE_VALUE, RECORD_METHOD_NAME)
		return true
	}

	if !msg.VerifySignature() {
		log.Println("invalid signature")
		return false
	}

	if !srv.establishPeer(peer) {
		return false
	}

	// Clear handshake keys
	peer.sharedKey = nil
	peer.peerHandshakeKey = nil
	peer.assignAddresses()

	// Established hook
	if f := srv.config.OnEstablished; f != nil {
		f(peer)
	}

	return false
}
