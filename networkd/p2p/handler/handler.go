package handler

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/buger/jsonparser"
	ipify "github.com/rdegges/go-ipify"
)

// New returns a new P2PHandler object.
func New(controldBase, joinIP string) *P2PHandler {
	return &P2PHandler{controldBase: controldBase, connected: false, joinIP: joinIP}
}

// P2PHandler is a type that interfaces with the controld's p2p network
type P2PHandler struct {
	joinIP       string
	controldBase string
	connected    bool
}

// Connect connects to the p2p newtwork and starts the heartbeat once connected.
func (p2p *P2PHandler) Connect() {
	// Join the p2p network and handle any failures
	joinString := `{"ip":"` + p2p.joinIP + `"}`
	resp, err := p2p.post("/network/join", joinString)
	if success, _ := getSuccess(resp, err); !success {
		time.Sleep(10 * time.Second)
		go p2p.Connect()
		return
	}

	// Tell the network our IP and handle any failures
	myIP, err := ipify.GetIp()
	if err != nil {
		fmt.Println("Couldn't get the public IP address:", err)
	}
	ipString := `{"message": {"node": {"ip_address": "` + myIP + `"}}}`
	resp, err = p2p.post("/message/sign", ipString)
	success, body := getSuccess(resp, err)
	if !success {
		time.Sleep(10 * time.Second)
		go p2p.Connect()
		return
	}

	// Get the signed message
	signedMessageBytes, _, _, err := jsonparser.Get(body, "response")
	if err != nil {
		return
	}

	// Send the signed message to the p2p network introducing ourselves
	resp, err = p2p.post("/state/push_message", string(signedMessageBytes))
	success, body = getSuccess(resp, err)
	if !success {
		time.Sleep(10 * time.Second)
		go p2p.Connect()
		return
	}

	// Once we have successfully connected, start the heartbeat
	p2p.startHearbeat()
}

func getSuccess(resp *http.Response, err error) (bool, []byte) {
	if err != nil {
		return false, []byte{}
	}
	body, _ := ioutil.ReadAll(resp.Body)
	success, err := jsonparser.GetBoolean(body, "success")
	if !success || err != nil {
		return false, []byte{}
	}
	return true, body
}

func (p2p *P2PHandler) post(endpoint, message string) (*http.Response, error) {
	byteMessage := []byte(message)
	return http.Post(p2p.controldBase+endpoint, "application/json", bytes.NewBuffer(byteMessage))
}

func (p2p *P2PHandler) startHearbeat() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			// Update the hearbeat with the current timestamp (in base 10)
			p2p.UpdateField("heartbeat", strconv.FormatInt(time.Now().Unix(), 10))
		}
	}()
}

// UpdateField updates the specified node field with the value
func (p2p *P2PHandler) UpdateField(key, value string) {

}
