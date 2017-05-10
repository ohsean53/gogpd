package main

import (
	"github.com/ohsaean/gogpd/lib"
	"github.com/ohsaean/gogpd/protobuf"
	"math"
	"math/rand"
	"net"
	"runtime"
	"time"
)

// server config
const (
	maxRoom = math.MaxInt32
)

// global variable
var (
	rooms lib.SharedMap
)

type Message struct {
	userID    int64 // sender
	msgType   gs_protocol.Type
	timestamp int    // send time
	contents  []byte // serialized google protocol-buffer message
}

func NewMessage(userID int64, eventType gs_protocol.Type, msg []byte) *Message {
	return &Message{
		userID,
		eventType,
		int(time.Now().Unix()),
		msg,
	}
}

func InitRooms() {
	rooms = lib.NewSMap(lib.Channel)
	rand.Seed(time.Now().UTC().UnixNano())
}

func onClientWrite(user *User, c net.Conn) {

	defer user.Leave()

	for {
		select {
		case <-user.exit:
			// when receive signal then finish the program
			if DEBUG {
				lib.Log("Leave user id :" + lib.Itoa64(user.userID))
			}
			return
		case m := <-user.recv:
			// on receive message
			msgTypeBytes := lib.WriteMsgType(m.msgType)

			// msg header + msg type
			msg := append(msgTypeBytes, m.contents...) // '...' need when concat between slice+slice
			if DEBUG {
				lib.Log("Client recv, user id : " + lib.Itoa64(user.userID))
			}
			_, err := c.Write(msg) // send data to client
			if err != nil {
				if DEBUG {
					lib.Log(err)
				}
				return
			}
		}
	}
}

func onClientRead(user *User, c net.Conn) {

	data := make([]byte, 4096) // 4096 byte slice (dynamic resize)

	c.SetReadDeadline(time.Now().Add(1 * time.Second))
	defer c.Close() // reserve tcp connection close
	for {
		n, err := c.Read(data)
		if err != nil {

			lib.Log("Fail Stream read, err : ", err)
			break
		}

		// header - body format (header + body in single line)
		messageType := gs_protocol.Type(lib.ReadInt32(data[0:4]))

		lib.Log("Decoding type : ", messageType)

		rawData := data[4:n] // 4~ end of line <--if fail read rawData, need calculated body size data (field)
		handler, ok := msgHandler[messageType]

		if ok {
			handler(user, rawData) // calling proper handler function
		} else {

			lib.Log("Fail no function defined for type", messageType)

			break
		}
	}

	// fail read
	user.exit <- struct{}{}
}

func onConnect(c net.Conn) {
	if DEBUG {
		lib.Log("New connection: ", c.RemoteAddr())
	}

	user := NewUser(0, nil) // empty user data

	go onClientRead(user, c)
	go onClientWrite(user, c)
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())
	ln, err := net.Listen("tcp", ":8000") // using TCP protocol over 8000 port
	defer ln.Close()                      // reserve listen wait close
	if err != nil {
		if DEBUG {
			lib.Log(err)
		}
		return
	}

	InitRooms()

	for {
		conn, err := ln.Accept() // server accept client connection -> return connection
		if err != nil {
			lib.Log("Fail Accept err : ", err)
			continue
		}

		onConnect(conn)
	}
}
