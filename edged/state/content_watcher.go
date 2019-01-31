package state

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

func (s *State) startContentFileWatcher() {
	// creates a new file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("Can't add watcher to content directory")
		defer watcher.Close()
	}
	done := make(chan bool)
	go func() {
		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Remove == fsnotify.Remove ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					if !strings.Contains(event.Name, "temp") {
						// Get some info about the file (if it exists)
						fi, fErr := os.Stat(event.Name)
						if fErr == nil {
							if fi.IsDir() {
								if err := watcher.Add(event.Name); err != nil {
									log.Error().
										Err(err).
										Str("directory", event.Name).
										Msg("Can't add watcher to website directory")
								}
							} else {
								s.loadContentFromDisk()
							}
						}
					}
				}

			// watch for errors
			case watchErr := <-watcher.Errors:
				if watchErr != nil {
					log.Error().
						Err(watchErr).
						Msg("Error watching content directory")
				}
			}
		}
	}()

	// out of the box fsnotify can watch a single file, or a single directory
	filePath, err := getContentDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Error getting content dir")
	}
	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error when reading content dir")
	}
	if err := watcher.Add(filePath); err != nil {
		log.Error().
			Err(err).
			Str("directory", filePath).
			Msg("Can't add watcher to content directory")
	}
	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			if err := watcher.Add(path.Join(filePath, website)); err != nil {
				log.Error().
					Err(err).
					Str("directory", path.Join(filePath, website)).
					Msg("Can't add watcher to website directory")
			}
		}
	}

	<-done
}

// LoadContentFromDisk loads the content from the disk and stores it in the state
func (s *State) loadContentFromDisk() {
	filePath, err := getContentDir()
	if err != nil {
		log.Fatal().Err(err).Msg("Error getting content dir")
	}

	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error when reading content dir")
	}
	// map websites
	cs := &contentStore{make(map[string]*websiteContent)}

	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			// Create a website store
			wc := cs.createWebsite(website)

			// Get all of the files for that website
			websiteFiles, err := ioutil.ReadDir(path.Join(filePath, website))
			if err != nil {
				log.Fatal().Err(err).Msg("Error when reading content dir")
			}

			log.Debug().Str("website", website).Msg("Loading website: " + website)

			for _, websiteFile := range websiteFiles {
				// Ignore subdirecories
				if !websiteFile.IsDir() && !strings.Contains(websiteFile.Name(), "temp") {
					fileName := websiteFile.Name()

					// Pull the file
					b, err := ioutil.ReadFile(path.Join(filePath, website, fileName))
					if err != nil {
						log.Warn().
							Str("file_name", fileName).
							Err(err).
							Msg("Error loading asset")
						continue
					}
					// Create the asset in the website content
					wc.createAsset(fileName, []byte(b))
					log.Debug().Str("asset_name", fileName).Msg("Loaded new asset")
				}
			}
		}
	}
	go func() {
		// Wait until we have joined the network before we try to update our content
		s.p2p.BlockUntilJoined()

		s.mux.Lock()
		s.content = cs

		// Tell the controld about our new content
		err = s.p2p.UpdateField("disk_content", cs.getContentList()...)
		if err != nil {
			log.Warn().Err(err).Msg("Error updating disk content, trying again in a few seconds")
			time.Sleep(2 * time.Second)
			err = s.p2p.UpdateField("disk_content", cs.getContentList()...)
			if err != nil {
				log.Warn().Err(err).Msg("Error retrying updating disk content, not trying again.")
			} else {
				log.Info().Msg("Second disk content update worked!")
			}
		}
		s.mux.Unlock()
	}()
}

func (s *State) startContentSyncWatcher() {
	// Get the files we have on disk now
	s.loadContentFromDisk()
	go s.startContentFileWatcher()

	/* If there is new content we need, sleep for a random time then ask which
	nodes have it in the network, then download it from a random one. This allows
	a semi random	propogation so we can minimize individal load on nodes.*/
	go func() {
		// Wait until we have joined the p2p network
		s.p2p.BlockUntilJoined()
		for {
			time.Sleep(2 * time.Second)       // Sleep to give the controld a break
			siteContent := s.getContentList() // Fetch what we have on disk in a format that's understood by the controld
			contentNeeded := getNeededFromControld(siteContent)

			if len(contentNeeded) > 0 {
				r := rand.New(rand.NewSource(time.Now().Unix()))
				time.Sleep(time.Duration(r.Intn(10)) * time.Second) // Random sleep allow better propogation

				for _, nc := range getContentLocationsFromControld(contentNeeded) {
					if len(nc.contentLocations) > 0 {
						contentLocations := nc.contentLocations
						contentName := nc.contentName

						contentDir, err := getContentDir()
						if err != nil {
							log.Fatal().Err(err).Msg("Can't find content dir")
							return
						}

						contentURL := contentLocations[r.Intn(len(contentLocations))]
						log.Debug().Str("url", contentURL).Msg("Downloading file from peer")

						// Create a filepath location from the content name
						toDownload := filepath.Join(append([]string{contentDir}, strings.Split(contentName, "/")...)...)

						// Pass in the name so we can verify the hash (filename is the hash)
						err = downloadFile(toDownload, contentURL, strings.Split(contentName, "/")[1])
						if err != nil {
							log.Warn().
								Str("url", contentURL).
								Str("filename", contentName).
								Str("path", toDownload).
								Err(err).
								Msg("Error downloading file from peer")
						}
					}
				}
			}
		}
	}()
}

// downloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(toDownload, url, name string) error {
	err := os.MkdirAll(filepath.Dir(toDownload), os.ModePerm)
	if err != nil {
		log.Fatal().Err(err)
	}

	// Create the file
	out, err := os.Create(toDownload + "_temp")
	if err != nil {
		return err
	}

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		out.Close()
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		out.Close()
		return err
	}

	out.Close()

	f, err := os.Open(toDownload + "_temp")
	if err != nil {
		return err
	}
	defer f.Close()

	// Check the hash of the file
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actualHash := fmt.Sprintf("%X", h.Sum(nil))
	if actualHash != strings.ToUpper(name) {
		out.Close()
		//os.Remove(toDownload + "_temp")
		errorString := fmt.Sprintf("incoming file from peer did not match expected hash. Expecting: %s, got: %s", strings.ToUpper(name), actualHash)
		return errors.New(errorString)
	}

	os.Rename(toDownload+"_temp", toDownload)
	log.Debug().
		Str("url", url).
		Str("filename", name).
		Str("path", toDownload).
		Msg("A new file was downloaded from a peer")
	return nil
}
