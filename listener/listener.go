package listener

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"strconv"
)

// Listener is the interface for things that listen on file descriptors
// specified by Start::Server / server_starter 
type Listener interface {
	Fd() uintptr
	Listen() (net.Listener, error)
	String() string
}

// ListenerList holds a list of Listeners. This is here just for convenience
// so that you can do
//	list.String()
// to get a string compatible with SERVER_STARTER_PORT
type ListenerList []Listener

func (ll ListenerList) String() string {
	list := make([]string, len(ll))
	for i, l := range ll {
		list[i] = l.String()
	}
	return strings.Join(list, ";")
}

// TCPListener is a listener for ... tcp duh.
type TCPListener struct {
	Addr string
	Port int
	fd   uintptr
}

// UnixListener is a listener for unix sockets.
type UnixListener struct {
	Path string
	fd   uintptr
}

func (l TCPListener) String() string {
	if l.Addr == "0.0.0.0" {
		return fmt.Sprintf("%d=%d", l.Port, l.fd)
	}
	return fmt.Sprintf("%s:%d=%d", l.Addr, l.Port, l.fd)
}

// Fd returns the underlying file descriptor
func (l TCPListener) Fd() uintptr {
	return l.fd
}

// Listen creates a new Listener
func (l TCPListener) Listen() (net.Listener, error) {
	return net.FileListener(os.NewFile(l.Fd(), fmt.Sprintf("%s:%d", l.Addr, l.Port)))
}

func (l UnixListener) String() string {
	return fmt.Sprintf("%s=%d", l.Path, l.fd)
}

// Fd returns the underlying file descriptor
func (l UnixListener) Fd() uintptr {
	return l.fd
}

// Listen creates a new Listener
func (l UnixListener) Listen() (net.Listener, error) {
	return net.FileListener(os.NewFile(l.Fd(), l.Path))
}

// Being lazy here...
var reLooksLikeHostPort = regexp.MustCompile(`^(\d+):(\d+)$`)
var reLooksLikePort = regexp.MustCompile(`^\d+$`)

func parseListenTargets(str string) ([]Listener, error) {
	rawspec := strings.Split(str, ";")
	ret := make([]Listener, len(rawspec))

	for i, pairString := range rawspec {
		pair := strings.Split(pairString, "=")
		hostPort := strings.TrimSpace(pair[0])
		fdString := strings.TrimSpace(pair[1])
		fd, err := strconv.ParseUint(fdString, 10, 0)
		if err != nil {
			return nil, err
		}

		if matches := reLooksLikeHostPort.FindAllString(hostPort, -1); matches != nil {
			port, err := strconv.ParseInt(matches[1], 10, 0)
			if err != nil {
				return nil, err
			}

			ret[i] = TCPListener{
				Addr: matches[0],
				Port: int(port),
				fd:   uintptr(fd),
			}
		} else if match := reLooksLikePort.FindString(hostPort); match != "" {
			port, err := strconv.ParseInt(match, 10, 0)
			if err != nil {
				return nil, err
			}

			ret[i] = TCPListener{
				Addr: "0.0.0.0",
				Port: int(port),
				fd:   uintptr(fd),
			}
		} else {
			ret[i] = UnixListener{
				Path: hostPort,
				fd:   uintptr(fd),
			}
		}
	}

	return ret, nil
}

// Ports parses environment variable SERVER_STARTER_PORT
func Ports() ([]Listener, error) {
	return parseListenTargets(os.Getenv("SERVER_STARTER_PORT"))
}

// ListenAll parses environment variable SERVER_STARTER_PORT, and creates
// net.Listener objects
func ListenAll() ([]net.Listener, error) {
	targets, err := parseListenTargets(os.Getenv("SERVER_STARTER_PORT"))
	if err != nil {
		return nil, err
	}

	ret := make([]net.Listener, len(targets))
	for i, target := range targets {
		ret[i], err = target.Listen()
		if err != nil {
			// Close everything up to this listener
			for x := 0; x < i; x++ {
				ret[x].Close()
			}
			return nil, err
		}
	}
	return ret, nil
}
