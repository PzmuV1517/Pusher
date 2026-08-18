package dash

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The client here is written out by hand, which means nothing has ever checked
// it against a server that speaks the protocol back. These tests are that
// server: they do the handshake and write real frames, so a mistake in the
// framing shows up as a failed test rather than as a robot that says nothing.

// fakeDashboard serves one connection the way FtcDashboard does: it accepts the
// upgrade and then volunteers messages without being asked.
func fakeDashboard(t *testing.T, messages ...string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		socket, err := listener.Accept()
		if err != nil {
			return
		}
		defer socket.Close()

		read := bufio.NewReader(socket)
		request, err := http.ReadRequest(read)
		if err != nil {
			return
		}

		sum := sha1.Sum([]byte(request.Header.Get("Sec-WebSocket-Key") + magic))
		io.WriteString(socket, "HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: "+base64.StdEncoding.EncodeToString(sum[:])+"\r\n\r\n")

		for _, message := range messages {
			if _, err := socket.Write(textFrame(message)); err != nil {
				return
			}
		}

		// Held open, so the client is the one that decides it has finished.
		time.Sleep(2 * time.Second)
	}()

	return listener.Addr().String()
}

// textFrame is one unmasked server frame, which is what a server sends.
func textFrame(payload string) []byte {
	frame := []byte{0x81}

	switch n := len(payload); {
	case n < 126:
		frame = append(frame, byte(n))
	default:
		frame = append(frame, 126, 0, 0)
		binary.BigEndian.PutUint16(frame[2:], uint16(n))
	}

	return append(frame, payload...)
}

func TestTheClientReadsWhatAServerSends(t *testing.T) {
	const list = `{"type":"RECEIVE_OP_MODE_LIST","opModeInfoList":` +
		`[{"name":"RST TUNING","group":"tuning2"},{"name":"TeleOp Red","group":"drive"}]}`

	addr := fakeDashboard(t, `{"type":"RECEIVE_TELEMETRY","log":[]}`, list)

	modes, err := OpModes(addr)
	if err != nil {
		t.Fatalf("OpModes: %v", err)
	}

	names := Names(modes)
	if len(names) != 2 || names[0] != "RST TUNING" {
		t.Errorf("names = %v", names)
	}
}

// Telemetry, images and gamepad state all arrive on the same socket whether or
// not anything asked, so the answer has to be waited for rather than read once.
func TestTheAnswerIsFoundAmongTheNoise(t *testing.T) {
	noise := make([]string, 0, 12)
	for i := 0; i < 10; i++ {
		noise = append(noise, `{"type":"RECEIVE_TELEMETRY","log":["`+strings.Repeat("x", 200)+`"]}`)
	}
	noise = append(noise, `{"type":"RECEIVE_OP_MODE_LIST","opModeInfoList":[{"name":"Only One","group":"g"}]}`)

	addr := fakeDashboard(t, noise...)

	modes, err := OpModes(addr)
	if err != nil {
		t.Fatalf("OpModes: %v", err)
	}
	if len(modes) != 1 || modes[0].Name != "Only One" {
		t.Errorf("modes = %+v", modes)
	}
}

func TestAConfigIsReadOffTheWireToo(t *testing.T) {
	const message = `{"type":"RECEIVE_CONFIG","configRoot":{"__type":"custom","__value":` +
		`{"Tuning":{"__type":"custom","__value":{"kP":{"__type":"double","__value":2.5}}}}}}`

	values, err := Fetch(fakeDashboard(t, message))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if values["Tuning.kP"] != "2.5" {
		t.Errorf("values = %v", values)
	}
}

// A robot that is there but has nothing to say must not hang the deploy.
func TestASilentDashboardGivesUp(t *testing.T) {
	addr := fakeDashboard(t)

	start := time.Now()
	if _, err := OpModes(addr); err == nil {
		t.Error("a dashboard that said nothing was reported as an answer")
	}

	if elapsed := time.Since(start); elapsed > ListTimeout+time.Second {
		t.Errorf("waited %s, which is longer than the timeout allows", elapsed)
	}
}

func TestAnUnreachableDashboardFailsQuickly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()

	start := time.Now()
	if _, err := OpModes(addr); err == nil {
		t.Error("a closed port was reported as a dashboard")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %s to notice nothing was listening", elapsed)
	}
}
