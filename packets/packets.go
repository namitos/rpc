//packet formatter for JSON-RPC over TCP implementation
package packets

import (
	"encoding/binary"
	"io"
	"net"
)

func Parse(connection net.Conn) ([]byte, uint64, uint64, uint64, error) {
	lBytes := make([]byte, 8) //8*4=32;8*8=64
	_, err := io.ReadFull(connection, lBytes)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	tBytes := make([]byte, 8) //8*4=32;8*8=64
	_, err = io.ReadFull(connection, tBytes)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	IDBytes := make([]byte, 8) //8*4=32;8*8=64
	_, err = io.ReadFull(connection, IDBytes)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	length := binary.BigEndian.Uint64(lBytes)
	messageType := binary.BigEndian.Uint64(tBytes)
	messageID := binary.BigEndian.Uint64(IDBytes)
	message := make([]byte, uint32(length))
	_, err = io.ReadFull(connection, message)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return message, messageType, messageID, length, nil
}

func Create(message []byte, messageType, messageID uint64) []byte {
	lBytes := make([]byte, 8)
	length := uint64(len(message))
	binary.BigEndian.PutUint64(lBytes, length)
	tBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tBytes, messageType)
	IDBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(IDBytes, messageID)
	return append(append(append(lBytes, tBytes...), IDBytes...), message...)
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
