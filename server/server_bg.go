package server

import (
	"log"
	"time"

	"github.com/fsnotify/fsnotify"
)

func (s *Server) backgroundRoutines() {

	go s.fetchSearchConfig(s.state.Config.ScraperURL)

	//poll torrents and files
	go func() {
		// initial state
		s.state.Lock()
		s.state.Torrents = s.engine.GetTorrents()
		s.state.Downloads = s.listFiles()
		s.state.Unlock()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&(fsnotify.Create|fsnotify.Remove) > 0 && s.state.NumConnections() > 0 {
						log.Println("event:", event)
						s.state.Lock()
						s.state.Downloads = s.listFiles()
						s.state.Unlock()
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				}
			}
		}()

		err = watcher.Add(s.state.Config.DownloadDirectory)
		if err != nil {
			log.Fatal(err)
		}

		for range time.Tick(time.Second) {
			if s.state.NumConnections() > 0 {
				// only update the state object if user connected
				s.state.Lock()
				s.state.Torrents = s.engine.GetTorrents()
				s.state.Unlock()
				s.state.Push()
			}
		}
	}()

	//start collecting stats
	go func() {
		for range time.Tick(5 * time.Second) {
			if s.state.NumConnections() > 0 {
				s.state.Lock()
				c := s.engine.Config()
				s.state.Stats.System.loadStats(c.DownloadDirectory)
				s.state.Stats.ConnStat = s.engine.ConnStat()
				s.state.Unlock()
				s.state.Push()
			}
		}
	}()

	// rss updater
	go func() {
		for range time.Tick(30 * time.Minute) {
			s.updateRSS()
		}
	}()

	go s.engine.UpdateTrackers()
	go s.RestoreTorrent()
	s.engine.StartTorrentWatcher()
}

func (s *Server) RestoreTorrent() error {
	s.engine.RestoreTorrent("*.torrent")
	s.engine.RestoreMagnet("*.info")
	return nil
}
