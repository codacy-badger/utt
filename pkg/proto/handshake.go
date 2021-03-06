package proto

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
)

var (
	helloMagic = []byte{'u', 't', 't', 0, 0, 0}
)

type Hello struct {
	Challenge [16]byte
}

func (h *Hello) Encode(buf []byte) []byte {
	buf = buf[0:0]
	buf = append(buf, helloMagic...)
	buf = append(buf, h.Challenge[:]...)
	return buf
}

func (h *Hello) Decode(buf []byte) error {
	if len(buf) < h.Len() || bytes.Compare(helloMagic, buf[:len(helloMagic)]) != 0 {
		return ErrInvalidPacket
	}
	copy(h.Challenge[:16], buf[len(helloMagic):16+len(helloMagic)])
	return nil
}

func (h *Hello) Len() int {
	return len(helloMagic) + len(h.Challenge)
}

func (h *Hello) Refresh() { rand.Read(h.Challenge[:]) }

type Connect struct {
	ACLKey    string
	Signature []byte
}

func (c *Connect) HMAC(challenge []byte, psk []byte) []byte {
	h := hmac.New(sha512.New, psk)
	h.Write([]byte("cha"))
	h.Write(challenge)
	h.Write([]byte("acl#" + c.ACLKey))
	return h.Sum(nil)
}

func (c *Connect) Len() int {
	return len([]byte(c.ACLKey)) + len(c.Signature)
}

func (c *Connect) Sign(challenge []byte, psk []byte) {
	c.Signature = c.HMAC(challenge, psk)
}

func (c *Connect) Verify(challenge []byte, psk []byte) bool {
	return bytes.Compare(c.Signature, c.HMAC(challenge, psk)) == 0
}

func (c *Connect) Encode(buf []byte) ([]byte, error) {
	var lengthBin [2]byte
	buf = buf[0:0]
	binACLKey := []byte(c.ACLKey)
	if len(binACLKey) > 0xFFFF || len(c.Signature) > 0xFFFF {
		return nil, ErrContentTooLong
	}
	binary.BigEndian.PutUint16(lengthBin[:], uint16(len(binACLKey)))
	buf = append(buf, lengthBin[:]...)
	binary.BigEndian.PutUint16(lengthBin[:], uint16(len(c.Signature)))
	buf = append(buf, lengthBin[:]...)

	buf = append(buf, binACLKey...)
	buf = append(buf, c.Signature...)
	return buf, nil
}

func (c *Connect) Decode(buf []byte) error {
	if len(buf) < 4 {
		return ErrInvalidPacket
	}
	msgLen, signLen := uint16(0), uint16(0)
	msgLen = binary.BigEndian.Uint16(buf[:2])
	signLen = binary.BigEndian.Uint16(buf[2:4])
	if int(msgLen)+int(signLen)+4 > len(buf) {
		return ErrInvalidPacket
	}
	c.ACLKey = string(buf[4 : 4+msgLen])
	c.Signature = buf[4+msgLen : 4+msgLen+signLen]
	return nil
}

type ConnectResult struct {
	Welcome bool
	RawMsg  [31]byte
	MsgLen  int
}

func (c *ConnectResult) Len() int { return 1 + c.MsgLen }

func (c *ConnectResult) EncodeMessage(msg string) error {
	raw := []byte(msg)
	if len(raw) > len(c.RawMsg) {
		return ErrBufferTooShort
	}
	copy(c.RawMsg[:len(raw)], raw)
	c.MsgLen = len(raw)
	return nil
}

func (c *ConnectResult) DecodeMessage() string {
	return string(c.RawMsg[:])
}

func (c *ConnectResult) Encode(buf []byte) []byte {
	buf = buf[0:0]
	if c.Welcome {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	buf = append(buf, byte(c.MsgLen&0xFF))
	buf = append(buf, c.RawMsg[:c.MsgLen]...)
	return buf
}

func (c *ConnectResult) Decode(buf []byte) error {
	if len(buf) < 2 {
		return ErrInvalidPacket
	}
	msgLen := uint8(buf[1])
	c.Welcome = buf[0] > 0
	if int(2+msgLen) > len(buf) {
		return ErrInvalidPacket
	}
	c.MsgLen = int(msgLen)
	copy(c.RawMsg[:c.MsgLen], buf[2:2+c.MsgLen])
	return nil
}
