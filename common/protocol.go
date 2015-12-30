package common
import (
    "os"
    "fmt"
    "net"
)

const (
    // UidLength is the length of the UID that the puncher server issues.
    UidLength = 16
)

type Packet byte

const (
    Ping           Packet = 0x00
    Pong           Packet = 0x01
    ClientType     Packet = 0x02
    FileInfo       Packet = 0x03
    ChecksumStatus Packet = 0x04
)

var (
    bodilessPackets = []Packet{Ping, Pong}
)

type Message struct {
    packet Packet
    body   []byte
}

func MessageChannel(conn net.Conn) (ch chan Message) {
    ch = make(chan Message)

    go func() {
        for {
            packetBuff := make([]byte, 1)

            if _, err := conn.Read(packetBuff); err != nil {
                fmt.Fprintf(os.Stderr, "Read error for '%s': %s", conn.RemoteAddr(), err)
                return
            }

            packet := Packet(packetBuff[0])

            if isBodiless(packet) {
                ch <- Message{
                    packet: packet,
                }
                continue
            }

            lenBuff := make([]byte, 1)

            if _, err := conn.Read(lenBuff); err != nil {
                fmt.Fprintf(os.Stderr, "Read error for '%s': %s", conn.RemoteAddr(), err)
                return
            }

            len := uint8(lenBuff[0])
            bodyBuff := make([]byte, len)

            if _, err := conn.Read(bodyBuff); err != nil {
                fmt.Fprintf(os.Stderr, "Read error for '%s': %s", conn.RemoteAddr(), err)
                return
            }

            ch <- Message{
                packet: packet,
                body:   bodyBuff,
            }
        }
    }()

    return
}

func isBodiless(p Packet) bool {
    for _, bp := range bodilessPackets {
        if p == bp {
            return true
        }
    }

    return false
}

func Mtob(msg Packet) []byte {
    return []byte{byte(msg)}
}
