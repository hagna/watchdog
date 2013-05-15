package watchdog

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"time"
)

var Longdesc = `This program is a list of timers controlled by a UDP client (or clients).

If the client sends any Message (of length > 0) the server will setup a default
timer, but a Message of the form "[type]|[message]|[action]|[timeout]" will
make the server start a timer of the specified type.  All the parameters are
optional, so the Message "DWN||acton" is valid and would either start a new
timer or update an existing timer with the specified parameters.  A client can
start multiple timers with multiple actions and other parameters.  Also
"typeA|this is a message" followed by "typeA||newaction" would update the
action but not the message, and enables the client to set the message once.

After alerting once or alerting sufficiently this program will remove the
corresponding timer from its list. 
`

var Examples = `To send data with python do:

import socket

sock = socket.socket()
sock.connect(('watchdog.sm', 3212))
sock.sendall('messageType|Nice long explanation|action1|900ms')
sock.close()

`

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s: %s [OPTIONS]\n\n", os.Args[0], os.Args[0])
	fmt.Fprintf(os.Stderr, Longdesc)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
}

const (
	RESET = 100 + iota
	STOP
)

type Handler interface {
	Starve(m Message)
	Feed(m Message)

}

type Server struct {
	Addr       string
	Timeout    string
	From       bool
	Fromstrict bool
	Handler    Handler // handler to invoke, default if nil
	Alertonce  bool
	Alertlimit int
	AlertText  string
}

type Message struct {
	Type        string
	Msg         string
	Action      string
	Timeout     time.Duration
	From        string
	Alertlimit  int
	Alertonce   bool
	dirtyfields map[int]bool
}

type timer struct {
	Message
	Handler Handler
	Fuse    <-chan time.Time
	ctrl    chan int
	cupdate chan Message
}

// return the ith elementh of a slice
// error is set if it doesn't exist
func get(i int, b [][]byte) (res []byte, err error) {
	err = nil
	if len(b) > i {
		res = b[i]
		return
	}
	err = errors.New("invalid index")
	return
}

func (m *Message) setField(i int, val interface{}) {
	// wow
	f := reflect.ValueOf(m).Elem().Field(i)
	f.Set(reflect.ValueOf(val))
	_, ok := m.dirtyfields[i]
	if !ok {
		m.dirtyfields[i] = true
	}
}

func (m *Message) getField(i int) (res interface{}) {
	res = reflect.ValueOf(m).Elem().Field(i).Interface()
	return
}

// extract a Message from bytes
func (m *Message) extract(b []byte) {
	slices := bytes.Split(b, []byte{'|'})
	i := bytes.Index(b, []byte{'|'})
	m.Type = "default"
	if i != -1 {
		// well I know how I'd do it in python
		// this seems a little awkward
		m.dirtyfields = make(map[int]bool)
		if res, err := get(0, slices); err == nil {
			m.setField(0, string(res))
		}
	}
	if res, err := get(1, slices); err == nil {
		if string(res) != "" {
			m.setField(1, string(res))
		}
	}
	if res, err := get(2, slices); err == nil {
		m.setField(2, string(res))
	}
	if res, err := get(3, slices); err == nil {
		if t, err2 := strconv.Atoi(string(res)); err2 == nil {
			Timeout, _ := time.ParseDuration(fmt.Sprintf("%ds", t))
			m.setField(3, Timeout)
		} else {
			if d, err3 := time.ParseDuration(string(res)); err3 == nil {
				m.setField(3, d)
			}
		}
	}
}

func (t *timer) relight() {
	t.Fuse = time.After(t.Timeout)
}

func (m *Message) setDefault(srv *Server) {
	d, _ := time.ParseDuration(srv.Timeout)
	def := Message{"default", srv.AlertText, "", d, "", srv.Alertlimit, srv.Alertonce, make(map[int]bool)}
	def.change(*m)
	*m = def
	m.dirtyfields = make(map[int]bool)
}

func (t *timer) watchdog(remove chan Message) {
	alertcount := 0
loop:
	for {
		select {
		case ctrl := <-t.ctrl:
			switch ctrl {
			case RESET:
				t.Handler.Feed(t.Message)
				t.relight()
				alertcount = 0
			case STOP:
				break loop
			}
		case b := <-t.cupdate:
			log.Println("got update Message")
			t.Message.change(b)
		case <-t.Fuse:
			log.Println("Timeout reached", t.Message)
			t.Handler.Starve(t.Message)
			t.relight()
			alertcount++
			if alertcount > t.Alertlimit || t.Alertonce {
				log.Printf("Alert limit %d reached", t.Alertlimit)
				break loop
			}
		}
	}
	remove <- t.Message
}

func (srv *Server) newtimer(m Message, remove chan Message) (res timer) {
	res = *new(timer)
	res.Message = m
	res.Message.setDefault(srv)
	res.relight()
	res.ctrl = make(chan int)
	res.cupdate = make(chan Message)
	res.Handler = srv.Handler
	go res.watchdog(remove)
	return
}

// update all the fields in m that are dirty or not default
// values in b and always update From
func (m *Message) change(b Message) {
	for k := range b.dirtyfields {
		val := b.getField(k)
		m.setField(k, val)
	}
	m.From = b.From
}

func (t *timer) update(m Message) {
	t.cupdate <- m
}

func (t *timer) reset() {
	t.ctrl <- RESET
}

func messageReceiver(srv *Server, newmsg <-chan Message) {
	remove := make(chan Message)
	alltimers := make(map[string]timer)
	for {
		select {
		case s := <-newmsg:
			v, ok := alltimers[s.Type]
			if !ok {
				log.Println("first one of type", s.Type)
				nt := srv.newtimer(s, remove)
				alltimers[s.Type] = nt
				v = nt
			} else {
				log.Println("another one of type", s.Type)
				v.update(s)
				v.reset()
			}
		case s := <-remove:
			delete(alltimers, s.Type)
			log.Printf("removed %+v\n", s)
		}
	}
}

func ListenAndServe(addr string, handler Handler) error {
	s := Server{Addr: addr, Handler: handler}
	return s.ListenAndServe()
}

func (s *Server) ListenAndServe() error {
	uaddr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Println("listening on", uaddr)
	newmsg := make(chan Message)
	go messageReceiver(s, newmsg)
	for {
		b := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(b)
		if err != nil {
			log.Println("error %v", err)
			continue
		}
		heartbeat := Message{From: addr.String()}
		heartbeat.extract(b[:n-1]) // remove newline
		newmsg <- heartbeat
	}

}
