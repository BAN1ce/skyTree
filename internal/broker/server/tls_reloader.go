package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/fsnotify/fsnotify"
)

type tlsReloader struct {
	certFile string
	keyFile  string
	caFile   string

	reloadInterval time.Duration

	current atomic.Value // stores *tls.Certificate

	mux sync.Mutex
}

func newTLSReloader(certFile, keyFile, caFile string, reloadInterval time.Duration) (*tlsReloader, *tls.Config, error) {
	r := &tlsReloader{
		certFile:       certFile,
		keyFile:        keyFile,
		caFile:         caFile,
		reloadInterval: reloadInterval,
	}
	cert, err := r.loadCertificate()
	if err != nil {
		return nil, nil, err
	}
	r.current.Store(cert)

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			v := r.current.Load()
			if v == nil {
				return nil, fmt.Errorf("tls certificate is not loaded")
			}
			c, ok := v.(*tls.Certificate)
			if !ok || c == nil {
				return nil, fmt.Errorf("tls certificate type is invalid")
			}
			return c, nil
		},
	}
	return r, tlsCfg, nil
}

func (r *tlsReloader) Start(ctx context.Context) {
	if r == nil {
		return
	}

	// Always try to watch; poll is controlled by ReloadInterval (0 disables poll).
	w, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Logger.Error().Err(err).Msg("tls reloader: create fsnotify watcher failed; fallback to poll only")
		r.startPoll(ctx)
		return
	}

	dirs := uniqueStrings([]string{
		filepath.Dir(r.certFile),
		filepath.Dir(r.keyFile),
	})
	if r.caFile != "" {
		dirs = uniqueStrings(append(dirs, filepath.Dir(r.caFile)))
	}

	// Watch directories to support atomic replace patterns (e.g. K8s Secret volume updates).
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := w.Add(d); err != nil {
			logger.Logger.Warn().Err(err).Str("dir", d).Msg("tls reloader: watch dir failed")
		}
	}

	go r.runWatchLoop(ctx, w)
	r.startPoll(ctx)
}

func (r *tlsReloader) startPoll(ctx context.Context) {
	if r == nil {
		return
	}
	if r.reloadInterval <= 0 {
		return
	}
	go func() {
		tk := time.NewTicker(r.reloadInterval)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				r.reloadNow("poll")
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *tlsReloader) runWatchLoop(ctx context.Context, w *fsnotify.Watcher) {
	defer func() {
		_ = w.Close()
	}()

	var (
		timer   *time.Timer
		timerCh <-chan time.Time
	)
	resetDebounce := func() {
		if timer == nil {
			timer = time.NewTimer(300 * time.Millisecond)
			timerCh = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(300 * time.Millisecond)
		timerCh = timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-timerCh:
			r.reloadNow("watch")
			// Disable until next event.
			timerCh = nil
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !r.isRelevantEvent(ev) {
				continue
			}
			resetDebounce()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			logger.Logger.Warn().Err(err).Msg("tls reloader: watcher error")
		}
	}
}

func (r *tlsReloader) isRelevantEvent(ev fsnotify.Event) bool {
	// Keep it conservative: any event in watched dirs may indicate cert rotation.
	// Still, ignore chmod-only noise and irrelevant paths when possible.
	if ev.Op&fsnotify.Chmod == fsnotify.Chmod {
		return false
	}
	name := ev.Name
	if name == "" {
		return true
	}
	// Fast path: if event touches files we care about, it's relevant.
	if samePath(name, r.certFile) || samePath(name, r.keyFile) || (r.caFile != "" && samePath(name, r.caFile)) {
		return true
	}
	// K8s Secret volume updates often touch ..data or symlinks; accept any event in the same dir.
	base := filepath.Base(name)
	if strings.HasPrefix(base, "..") {
		return true
	}
	return true
}

func (r *tlsReloader) reloadNow(trigger string) {
	if r == nil {
		return
	}
	r.mux.Lock()
	defer r.mux.Unlock()

	cert, err := r.loadCertificate()
	if err != nil {
		logger.Logger.Error().Err(err).Str("trigger", trigger).Msg("tls reloader: reload certificate failed; keep old certificate")
		return
	}
	r.current.Store(cert)
	logger.Logger.Info().Str("trigger", trigger).Msg("tls reloader: certificate reloaded")
}

func (r *tlsReloader) loadCertificate() (*tls.Certificate, error) {
	if r.certFile == "" || r.keyFile == "" {
		return nil, fmt.Errorf("tls cert_file/key_file is empty")
	}
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return nil, fmt.Errorf("load tls key pair failed: %w", err)
	}
	return &cert, nil
}

func uniqueStrings(in []string) []string {
	m := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 == nil && err2 == nil {
		return aa == bb
	}
	return false
}
