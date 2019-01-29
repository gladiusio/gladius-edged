package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/buger/jsonparser"
	ipify "github.com/rdegges/go-ipify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// New returns a new P2PHandler object.
func New(controldBase, joinIP, joinPort, contentPort string) *P2PHandler {
	return &P2PHandler{
		controldBase: controldBase,
		connected:    false,
		joinIP:       joinIP,
		joinPort:     joinPort,
		contentPort:  contentPort,
		joined:       false,
		joinChan:     make(chan struct{}),
	}
}

// P2PHandler is a type that interfaces with the controld's p2p network
type P2PHandler struct {
	joined       bool
	contentPort  string
	joinIP       string
	joinPort     string
	controldBase string
	connected    bool
	ourIP        string
	joinChan     chan struct{}
}

// Connect connects to the p2p newtwork and starts the heartbeat once connected.
func (p2p *P2PHandler) Connect() {
	// Join the p2p network and handle any failures
	if !viper.GetBool("DisableAutoJoin") {
		joinString := `{"ip":"` + p2p.joinIP + ":" + p2p.joinPort + `"}`
		resp, err := p2p.post("/network/join", joinString)
		if success, _ := getSuccess(resp, err); !success {
			log.Warn().Err(err).Msg("Error joining p2p network, trying again in 10 seconds")
			time.Sleep(10 * time.Second)
			go p2p.Connect()
			return
		}
		p2p.joined = true
		close(p2p.joinChan)
	}

	// Once we have successfully connected, start the heartbeat
	p2p.startHearbeat()
}

// BlockUntilJoined blocks until we have joined the network
func (p2p *P2PHandler) BlockUntilJoined() {
	<-p2p.joinChan
}

// LeaveIfJoined will call the leave endpoint if we have joined
func (p2p *P2PHandler) LeaveIfJoined() {
	if p2p.joined {
		p2p.post("/network/leave", "")
	}
}

func getIP() (string, error) {
	return ipify.GetIp()
}

func (p2p *P2PHandler) postIP() (bool, error) {
	var myIP string
	var err error
	if viper.GetString("OverrideIP") == "" {
		myIP, err = getIP()
		if err != nil {
			log.Warn().Err(err).Msg("Error getting public IP address")
			return false, err
		}
	} else {
		myIP = viper.GetString("OverrideIP")
	}

	toUpdate := make(map[string]string)
	toUpdate["ip_address"] = myIP
	toUpdate["content_port"] = p2p.contentPort
	err = p2p.UpdateFields(toUpdate)
	if err != nil {
		return false, err
	}
	return true, nil
}

func getSuccess(resp *http.Response, err error) (bool, []byte) {
	if err != nil {
		return false, []byte{}
	}
	body, _ := ioutil.ReadAll(resp.Body)
	success, err := jsonparser.GetBoolean(body, "success")
	if !success || err != nil {
		url, err := resp.Location()
		if err == nil {
			log.Debug().Str("url", url.String()).Msg("Success was false, this was the body: " + string(body))
		} else {
			log.Debug().Str("url", "nil").Msg("Success was false, this was the body: " + string(body))
		}
		return false, []byte{}
	}
	return true, body
}

func (p2p *P2PHandler) post(endpoint, message string) (*http.Response, error) {
	byteMessage := []byte(message)
	return http.Post(p2p.controldBase+endpoint, "application/json", bytes.NewBuffer(byteMessage))
}

func (p2p *P2PHandler) startHearbeat() {
	log.Debug().Msg("Started heartbeat")
	go func() {
		for {
			time.Sleep(5 * time.Second)
			// Update the hearbeat with the current timestamp (in base 10)
			if !viper.GetBool("DisableHeartbeat") {
				err := p2p.UpdateField("heartbeat", strconv.FormatInt(time.Now().Unix(), 10))
				if err != nil {
					log.Warn().Err(err).Msg("Error posting heartbeat")
				}
			}
			// If we have detection on, tell the network our IP and handle any failures
			if !viper.GetBool("DisableIPDiscovery") {
				var myIP string
				var err error
				if viper.GetString("OverrideIP") == "" {
					myIP, err = getIP()
					if err != nil {
						log.Warn().Err(err).Msg("Error getting public IP address")
						break
					}
				} else {
					myIP = viper.GetString("OverrideIP")
				}
				// If the IP changed since last time, inform the network
				if myIP != p2p.ourIP && myIP != "" {
					success, err := p2p.postIP()
					if err != nil {
						log.Error().Str("detected_ip", myIP).Err(err).Msg("Error updating this node's public IP in network state")
					}
					if !success {
						log.Warn().Str("detected_ip", myIP).Err(err).Msg("Error updating this node's public IP in network state")
					} else {
						// If successfull, update the local state's IP so we can detect changes
						p2p.ourIP = myIP
					}
				}
			}
		}
	}()
}

func (p2p *P2PHandler) UpdateFields(toUpdate map[string]string) error {
	fields, err := json.Marshal(toUpdate)
	if err != nil {
		return errors.New("Input fields can't be marshalled")
	}
	updateString := `{"message": {"node": ` + string(fields) + `}}`
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

	resp, err := p2p.post("/message/sign", updateString)
	success, body := getSuccess(resp, err)
	if !success {
		return errors.New("Couldn't sign message with network gateway, wallet could be locked")
	}

	// Get the signed message
	signedMessageBytes, _, _, err := jsonparser.Get(body, "response")
	if err != nil {
		return errors.New("The network gateway returned a corrupted message")
	}

	// Send the signed message to the p2p network introducing ourselves
	resp, err = p2p.post("/state/push_message", string(signedMessageBytes))
	success, _ = getSuccess(resp, err)
	if !success {
		return errors.New("Couldn't push message")
	}

	return nil
}
