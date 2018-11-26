// Package state contains a thread safe state struct that stores information
// about the edged
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/buger/jsonparser"
	"github.com/gladiusio/gladius-edged/edged/p2p/handler"
	"github.com/gladiusio/gladius-utils/config"
	log "github.com/sirupsen/logrus"
)

// New returns a new state struct
func New(p2pHandler *handler.P2PHandler) *State {
	state := &State{running: true, content: &contentStore{make(map[string]*websiteContent)}, runChannel: make(chan bool), p2p: p2pHandler}
	state.startContentSyncWatcher()
	return state
}

// State is a thread safe struct for keeping information about the edged
type State struct {
	p2p        *handler.P2PHandler
	running    bool
	content    *contentStore
	runChannel chan (bool)
	mux        sync.Mutex
}

type contentStore struct {
	websites map[string]*websiteContent
}

func (c contentStore) getContentList() []string {
	contentList := make([]string, 0)
	for websiteName, wc := range c.websites {
		for assetName := range wc.assets {
			contentList = append(contentList, strings.Join([]string{websiteName, assetName}, "/"))
		}

	}
	return contentList
}

func (c contentStore) getWebsite(name string) *websiteContent {
	return c.websites[name]
}

func (c contentStore) createWebsite(name string) *websiteContent {
	wc := &websiteContent{make(map[string][]byte)}
	c.websites[name] = wc
	return wc
}

type websiteContent struct {
	assets map[string][]byte
}

func (w websiteContent) getAsset(name string) []byte {
	return w.assets[name]
}

func (w *websiteContent) createAsset(name string, content []byte) {
	w.assets[name] = content
}

type status struct {
	Running bool
	Version string
}

func (s *State) GetAsset(website, asset string) []byte {
	s.mux.Lock()
	// Lock so only one goroutine at a time can access the map
	defer s.mux.Unlock()
	return s.content.getWebsite(website).getAsset(asset)
}

func (s *State) Info() string {
	s.mux.Lock()
	defer s.mux.Unlock()

	status := &status{Running: s.running}

	jsonString, _ := json.Marshal(status)
	return string(jsonString)
}

type networkContent struct {
	contentName      string
	contentLocations []string
}

type contentList struct {
	Content []string `json:"content"`
}

func (c *contentList) Marshal() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// getNeededFromControld asks the controld what we need
func getNeededFromControld(content []string) []string {
	c := &contentList{Content: content}
	resp, err := postToControld("/p2p/state/content_diff", c.Marshal())
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Warn("Problem getting needed content list from control daemon")
		return []string{}
	}
	body, _ := ioutil.ReadAll(resp.Body)
	contentNeeded := make([]string, 0)
	// Get every string in the response (our needed content)
	jsonparser.ArrayEach(body, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		contentNeeded = append(contentNeeded, string(value))
	}, "response")

	return contentNeeded
}

// getContentLocationsFromControld gets a list of networkContent objects
func getContentLocationsFromControld(content []string) []*networkContent {
	c := &contentList{Content: content}
	resp, err := postToControld("/p2p/state/content_links", c.Marshal())
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Warn("Problem getting links for needed content from control daemon")
		return []*networkContent{&networkContent{}}
	}
	body, _ := ioutil.ReadAll(resp.Body)

	ncList := make([]*networkContent, 0)

	// Get all of the files
	jsonparser.ObjectEach(body, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		nc := &networkContent{contentName: string(key), contentLocations: make([]string, 0)}

		// Get all of the links for that file
		jsonparser.ArrayEach(value, func(v []byte, dataType jsonparser.ValueType, offset int, err error) {
			nc.contentLocations = append(nc.contentLocations, string(v))
		})
		// Add this to the network content list
		ncList = append(ncList, nc)
		return nil
	}, "response")

	return ncList
}

func postToControld(endpoint, message string) (*http.Response, error) {
	controldBase := config.GetString("ControldProtocol") + "://" + config.GetString("ControldHostname") + ":" + config.GetString("ControldPort") + "/api"
	byteMessage := []byte(message)
	return http.Post(controldBase+endpoint, "application/json", bytes.NewBuffer(byteMessage))
}

// getContentList returns a list of the content we have on disk in the format of:
// <website name>/<fileName>
func (s *State) getContentList() []string {
	s.mux.Lock()
	defer s.mux.Unlock()

	return s.content.getContentList()
}

func getContentDir() (string, error) {
	contentDir := config.GetString("ContentDirectory")
	if contentDir == "" {
		return contentDir, errors.New("No content directory specified")
	}
	return contentDir, nil
}
