package watchdog

import "testing"
import "fmt"
import "time"

func TestExtract(t *testing.T) {
	d, _ := time.ParseDuration("19s")
	if cmpMsg("TYPE|message|Action|19",
		&Message{"TYPE", "message", "Action", d, "", 0, false, nil}) != true {
		t.Fatalf("comparison failed")
	}
}

func cmpMsg(s string, b *Message) (res bool) {
	a := new(Message)
	a.extract([]byte(s))
	res = true
	if a.Action != b.Action {
		res = false
	}
	if a.Timeout != b.Timeout {
		res = false
	}
	if a.Type != b.Type {
		res = false
	}
	if a.Text != b.Text {
		res = false
	}
	if res == false {
		fmt.Printf("%s != %s\n", a, b)
	}
	return
}

func TestDefaults(t *testing.T) {
	if cmpMsg("", &Message{Type: "default"}) != true {
		t.Fatalf("comparison failed to set defaults correctly")
	}

}

func TestGetSet(t *testing.T) {
	d, _ := time.ParseDuration("30s")
	def := Message{"default", "abc", "action", d, "", 100, false, make(map[int]bool)}
	b := new(Message)
	b.Type = "alpha"
	b.dirtyfields = make(map[int]bool)
	b.dirtyfields[0] = true
	def.change(*b)
	if def.Type != "alpha" {
		t.Fatalf("failed to change defaults")
	}
	if def.Text != "abc" {
		t.Fatalf("shouldn't have updated msg")
	}
}
