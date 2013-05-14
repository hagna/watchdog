package main

import (
	"flag"
	"github.com/hagna/watchdog"
	"log"
	"os/exec"
)

var msg = flag.String("m", "timeout", "Message to display when alerting")
var timeout = flag.String("t", "30s", "Wait this number of seconds for a message to arrive")
var from = flag.Bool("from", false, "Only accept messages from the same IP")
var fromstrict = flag.Bool("fs", false, "Only accept message from the same IP and port")
var action = flag.String("action", "", "A script to spawn on alert")
var alertonce = flag.Bool("alertonce", false, "Only alert once")
var alertlimit = flag.Int("limit", 10, "Number of times to alert before giving up (-1 will make this experience more memorable)")
var endpoint = flag.String("ep", "0.0.0.0:3212", "Network interface and port on which to listen")

type world string

func (w world) On(m watchdog.Message) {
	log.Println("ON", m,)
	out, _ := exec.Command(*action).CombinedOutput()
	// comment out the following for great security
	// out, _ := exec.Command(m.Action).CombinedOutput()
	log.Println("output is: ", string(out))
}

func (w world) Off(m watchdog.Message) {
	log.Println("OFF", m)
}

func main() {
	flag.Usage = watchdog.Usage
	flag.Parse()
	wd := watchdog.Server{
		Addr:       *endpoint,
		Timeout:    *timeout,
		From:       *from,
		Fromstrict: *fromstrict,
		Handler:    world("It's the end of the world as we know it"), // why the ( instead of { ?
		Alertonce:  *alertonce,
		Alertlimit: *alertlimit,
		AlertText:  *msg,
	}
	log.Fatal(wd.ListenAndServe())

}
