package magicsockets_test

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	TEST_SERVER_ADDRESS = "127.0.0.1:8080"
	DEFAULT_TOPIC       = "default-topic"
)

var (
	EVENTUALLY_TIMEOUT = func() time.Duration {
		isUsingDebugger := os.Getenv("GOLANG_DEBUGGER") == "on"
		if isUsingDebugger {
			return 100000000 * time.Second
		} else {
			return 5 * time.Second
		}
	}()
)

func TestMagicsockets(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(EVENTUALLY_TIMEOUT)
	RunSpecs(t, "Magicsockets Suite")
}

func newTestClient(address string) (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: address, Path: "/"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}
	return c, nil
}
