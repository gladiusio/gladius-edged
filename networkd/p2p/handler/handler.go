package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/buger/jsonparser"
	ipify "github.com/rdegges/go-ipify"
	log "github.com/sirupsen/logrus"
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
		log.WithFields(log.Fields{
			"err": err.Error(),
		}).Warn("Error getting public IP address")
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
		log.WithFields(log.Fields{
			"err": err.Error(),
		}).Warn("Error getting signed message, controld returned something wrong.")
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
		log.Debug("Success was false, this was the body: " + string(body))
		return false, []byte{}
	}
	return true, body
}

func (p2p *P2PHandler) post(endpoint, message string) (*http.Response, error) {
	byteMessage := []byte(message)
	return http.Post(p2p.controldBase+endpoint, "application/json", bytes.NewBuffer(byteMessage))
}

func (p2p *P2PHandler) startHearbeat() {
	log.Debug("Started heartbeat")
	go func() {
		for {
			time.Sleep(5 * time.Second)
			// Update the hearbeat with the current timestamp (in base 10)
			err := p2p.UpdateField("heartbeat", strconv.FormatInt(time.Now().Unix(), 10))
			log.WithFields(log.Fields{
				"err": err.Error(),
			}).Warn("Error posting heartbeat")
		}
	}()
}

// UpdateField updates the specified node field with the value
func (p2p *P2PHandler) UpdateField(key string, value ...string) error {
	var updateString string

	// Check for the disk content field so we always send the controld a list here
	if key == "disk_content" {
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return errors.New("Input values for the method UpdateField couldn't be parsed")
		}
		updateString = `{"message": {"node": {"` + key + `": ` + string(valueJSON) + `}}}`

		// Send a string if we only have one
	} else if len(value) == 1 {
		updateString = `{"message": {"node": {"` + key + `": "` + value[0] + `"}}}`

		// If we have more than one value send a list
	} else if len(value) > 0 {
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return errors.New("Input values for the method UpdateField couldn't be parsed")
		}
		updateString = `{"message": {"node": {"` + key + `": ` + string(valueJSON) + `}}}`

	} else {
		return errors.New("UpdateField needs at least one value")
	}

	fmt.Println(updateString)

	resp, err := p2p.post("/message/sign", updateString)
	success, body := getSuccess(resp, err)
	if !success {
		return errors.New("Couldn't sign message with contorld, wallet could be locked")
	}

	// Get the signed message
	signedMessageBytes, _, _, err := jsonparser.Get(body, "response")
	if err != nil {
		return errors.New("Controld returned a corrupted message")
	}

	// Send the signed message to the p2p network introducing ourselves
	resp, err = p2p.post("/state/push_message", string(signedMessageBytes))
	success, _ = getSuccess(resp, err)
	if !success {
		return errors.New("Couldn't push message")
	}

	return nil
}
