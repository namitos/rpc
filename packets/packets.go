//packet formatter for JSON-RPC over TCP implementation
package packets

import (
	"encoding/binary"
	"io"
	"net"
)

func Parse(connection net.Conn) ([]byte, uint64, uint64, uint64, error) {
	header := make([]byte, 24)
	_, err := io.ReadFull(connection, header)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	length := binary.BigEndian.Uint64(header[0:8])
	messageType := binary.BigEndian.Uint64(header[8:16])
	messageID := binary.BigEndian.Uint64(header[16:24])
	message := make([]byte, length)
	_, err = io.ReadFull(connection, message)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return message, messageType, messageID, length, nil
}

func Create(message []byte, messageType, messageID uint64) []byte {
	buf := make([]byte, 24+len(message))
	binary.BigEndian.PutUint64(buf[0:8], uint64(len(message)))
	binary.BigEndian.PutUint64(buf[8:16], messageType)
	binary.BigEndian.PutUint64(buf[16:24], messageID)
	copy(buf[24:], message)
	return buf
}

func Send(message []byte, messageType, messageID uint64, URL string) ([]byte, uint64, uint64, uint64, error) {
	connection, err := net.Dial("tcp", URL)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer connection.Close()
	connection.Write(Create(message, messageType, messageID))
	return Parse(connection)
}
