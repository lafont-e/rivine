package gateway

import (
	"net"

	"github.com/rivine/rivine/build"
	"github.com/xtaci/smux"
)

// A streamSession is a multiplexed transport that can accept or initiate
// streams.
type streamSession interface {
	Accept() (net.Conn, error)
	Open() (net.Conn, error)
	Close() error
}

// smux's Session methods do not return a net.Conn, but rather a
// smux.Stream, necessitating an adaptor.
type smuxSession struct {
	sess *smux.Session
}

func (s smuxSession) Accept() (net.Conn, error) { return s.sess.AcceptStream() }
func (s smuxSession) Open() (net.Conn, error)   { return s.sess.OpenStream() }
func (s smuxSession) Close() error {
	err := s.sess.Close()
	// errBrokenPipe is returned in the Close function,
	// in case the stream is already closed,
	// in which case we simply wish to ignore the error.
	if err == nil || err.Error() == "broken pipe" {
		return nil
	}
	return err
}

func newSmuxServer(conn net.Conn) streamSession {
	sess, err := smux.Server(conn, nil) // default config means no error is possible
	if err != nil {
		build.Critical("smux should not fail with default config:", err)
	}
	return smuxSession{sess}
}

func newSmuxClient(conn net.Conn) streamSession {
	sess, err := smux.Client(conn, nil) // default config means no error is possible
	if err != nil {
		build.Critical("smux should not fail with default config:", err)
	}
	return smuxSession{sess}
}
