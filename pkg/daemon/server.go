package daemon

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// ServerConfig wires up the listeners and middleware.
type ServerConfig struct {
	SocketPath      string
	TCPAddr         string
	Token           string
	Log             Logger
	Mux             http.Handler
	ShutdownTimeout time.Duration
	BodyLimitBytes  int64
}

// Server runs the HTTP listener(s) until ctx is cancelled.
type Server struct {
	cfg  ServerConfig
	srv  *http.Server
	wg   sync.WaitGroup
	stop chan struct{}
}

func NewServer(cfg ServerConfig) *Server {
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.BodyLimitBytes == 0 {
		cfg.BodyLimitBytes = 1 << 20
	}
	return &Server{cfg: cfg, stop: make(chan struct{})}
}

func (s *Server) Run(ctx context.Context) error {
	chain := func(t Transport) http.Handler {
		h := s.cfg.Mux
		h = LogMW(s.cfg.Log)(h)
		h = BodyLimitMW(s.cfg.BodyLimitBytes)(h)
		h = AuthMW(s.cfg.Token)(h)
		h = RecoverMW(s.cfg.Log)(h)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(WithTransport(r.Context(), t)))
		})
	}

	unixSrv := &http.Server{Handler: chain(TransportUnix)}
	tcpSrv := &http.Server{Handler: chain(TransportTCP)}

	_ = os.Remove(s.cfg.SocketPath)
	unixL, err := net.Listen("unix", s.cfg.SocketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.cfg.SocketPath, 0o600); err != nil {
		_ = unixL.Close()
		return err
	}

	errCh := make(chan error, 2)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := unixSrv.Serve(unixL)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	if s.cfg.TCPAddr != "" {
		tcpL, err := net.Listen("tcp", s.cfg.TCPAddr)
		if err != nil {
			_ = unixSrv.Close()
			s.wg.Wait()
			return err
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := tcpSrv.Serve(tcpL)
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	s.srv = unixSrv

	select {
	case <-ctx.Done():
	case err := <-errCh:
		s.cfg.Log.Error("http", "listener: "+err.Error())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	_ = unixSrv.Shutdown(shutdownCtx)
	_ = tcpSrv.Shutdown(shutdownCtx)
	_ = os.Remove(s.cfg.SocketPath)
	close(s.stop)
	return nil
}

func (s *Server) Wait() {
	s.wg.Wait()
}
