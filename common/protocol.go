package common

import (
    "os"
    "fmt"
    "net"
    "bytes"
)

const (
    // UidLength is the length of the UID that the puncher server issues.
    UidLength = 16
)

// Packet is a description of the data sent from one endpoint to another.
type Packet           byte

// ClientTypeB is a body of ClientType indicating the type of client connecting
// to the puncher.
type ClientTypeBody   byte

// VerificationB is a body of Verification indicating the status of verification
// for the received file (checksum verification).
type VerificationBody byte

const (
    // ClientType is sent from the uploader or downloader to the puncher
    // followed by a known length byte indicating the type of client
    // (ClientType).
    ClientType    Packet = 0x00

    // UidAssignment is sent from the puncher to the downloader to indicate the
    // UID of the downloader.
    UidAssignment Packet = 0x01

    // UidRequest is sent from the uploader to the puncher to indicate the
    // UID of the downloader it would like to connect to.
    UidRequest    Packet = 0x02

    // PeerNotFound is sent from the puncher to the uploader to indicate that
    // the requested peer (identified by its UID) could not be found.
    PeerNotFound  Packet = 0x03

    // UploaderReady is sent from the puncher to the downloader to indicate that
    // the uploader is ready to send files.
    UploaderReady Packet = 0x04

    // FileName is sent from the uploader to the downloader indicating the name
    // of the file about to be sent.
    FileName      Packet = 0x05

    // FileSize is sent from the uploader to the downloader indicating the size
    // of the file about to be sent.
    FileSize      Packet = 0x06

    // FileHash is sent from the uploader to the downloader indicating the hash
    // of the file about to be sent.
    FileHash      Packet = 0x07

    // Verification is sent from the downloader to the uploader indicating the
    // status of the hash of the received file (Verification).
    Verification  Packet = 0x08

    // ProtocolError is sent from any peer to another indicating that the sender
    // of this message received unexpected or invalid data. The body contains an
    // error string.
    ProtocolError Packet = 0x09

    // InternalError is sent from any peer to another indicating that the sender
    // of this message encountered an internal error. The body contains an error
    // string.
    InternalError Packet = 0x0A

    // Halt is sent from any peer to another indicating that all current
    // connections should be closed and that no future communications will take
    // place. The body contains a message describing the reason.
    Halt          Packet = 0x0B
)

const (
    // DownloaderClientType is a body of the ClientType Packet.
    DownloaderClientType ClientTypeBody   = 0x00

    // UploaderClientType is a body of the ClientType Packet.
    UploaderClientType   ClientTypeBody   = 0x01

    // GoodVerification is the body of the Verification Packet indicating
    // verification has succeeded.
    GoodVerification     VerificationBody = 0x00

    // BadVerification is the body of the Verification Packet indicating
    // verification has failed.
    BadVerification      VerificationBody = 0x01
)

var (
    // bodilessPackets is the set of all Packets that do not have a body.
    bodilessPackets = []Packet{
        PeerNotFound,
        UploaderReady,
    }

    // fixedLengthPackets is the map of all Packets that have a fixed length
    // body.
    fixedLengthPackets = map[Packet]uint8{
        ClientType:    1,
        UidAssignment: UidLength,
        UidRequest:    UidLength,
        FileSize:      8,  // uint64
        FileHash:      32, // sha256
        Verification:  1,
    }
)

// Message is a message from one endpoint to another with a packet and body.
// Some messages may be bodiless, where body will therefore be nil.
type Message struct {
    // Packet is the Packet that describes the body, if present.
    Packet Packet

    // Body is the bytes that the packet describes. May be nil if bodiless.
    Body   []byte
}

func NewMesssageWithByte(packet Packet, body byte) *Message {
    return &Message{ packet, []byte{body} }
}

func (m Message) MarshalBinary() (data []byte, err error) {
    var buff bytes.Buffer

    buff.WriteByte(byte(m.Packet))

    if ! isBodiless(m.Packet) {
        if _, fixed := fixedLengthPackets[m.Packet]; ! fixed {
            bodyLen := len(m.Body)

            if bodyLen > 0xFF {
                return nil, fmt.Errorf("length of body cannot fit in 1 byte (got %d bytes)", bodyLen)
            }

            buff.WriteByte(byte(len(m.Body)))
        }

        buff.Write(m.Body)
    }

    return buff.Bytes(), nil
}

// MessageChannel returns a 2 channels of Messages for the given Conn. Closes
// both channels upon error or closure.
func MessageChannel(conn net.Conn) (in chan Message, out chan Message) {
    in = make(chan Message)
    out = make(chan Message)

    go func() {
        defer close(in)
        defer close(out)

        for {
            packetBuff := make([]byte, 1)

            if _, err := conn.Read(packetBuff); err != nil {
                handleReadError(conn, err)
                return
            }

            packet := Packet(packetBuff[0])

            if isBodiless(packet) {
                in <- Message{
                    Packet: packet,
                }
                continue
            }

            len, known := fixedLengthPackets[packet]

            if ! known {
                lenBuff := make([]byte, 1)

                if _, err := conn.Read(lenBuff); err != nil {
                    handleReadError(conn, err)
                    return
                }

                len = uint8(lenBuff[0])
            }

            bodyBuff := make([]byte, len)

            if _, err := conn.Read(bodyBuff); err != nil {
                handleReadError(conn, err)
                break
            }

            in <- Message{
                Packet: packet,
                Body:   bodyBuff,
            }
        }
    }()

    go func() {
        defer close(in)
        defer close(out)

        for {
            data, err := (<- out).MarshalBinary()

            if err != nil {
                handleWriteError(conn, err)
                break
            }

            if _, err := conn.Write(data); err != nil {
                handleWriteError(conn, err)
                break
            }
        }
    }()

    return
}

func handleReadError(conn net.Conn, err error) {
    fmt.Fprintf(os.Stderr, "%s -- Read error: %s", conn.RemoteAddr(), err)
}

func handleWriteError(conn net.Conn, err error) {
    fmt.Fprintf(os.Stderr, "%s -- Write error: %s", conn.RemoteAddr(), err)
}

func isBodiless(p Packet) bool {
    for _, v := range bodilessPackets {
        if v == p {
            return true
        }
    }

    return false
}
