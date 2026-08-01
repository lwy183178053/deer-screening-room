package media

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestWebRTCConfigurationForcesRelayWhenDirectP2PDisabled(t *testing.T) {
	servers := []webrtc.ICEServer{{URLs: []string{"turn:turn.example.test:3478"}, Username: "user", Credential: "secret"}}
	relayOnly := peerConfiguration(servers, false)
	if relayOnly.ICETransportPolicy != webrtc.ICETransportPolicyRelay {
		t.Fatalf("relay-only policy=%s", relayOnly.ICETransportPolicy.String())
	}
	direct := peerConfiguration(servers, true)
	if direct.ICETransportPolicy != webrtc.ICETransportPolicyAll {
		t.Fatalf("direct policy=%s", direct.ICETransportPolicy.String())
	}
}

func TestH264DeliveryCopiesOnlyProfilesOfferedByBrowser(t *testing.T) {
	mainOffer := "a=fmtp:96 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=4d001f"
	capability, arguments := selectH264Delivery("Main", 31, mainOffer)
	if !containsArgumentPair(arguments, "-c:v", "copy") || !strings.Contains(capability.SDPFmtpLine, "profile-level-id=4d001f") {
		t.Fatalf("main delivery capability=%+v args=%v", capability, arguments)
	}
	capability, arguments = selectH264Delivery("High", 50, mainOffer)
	if !containsArgumentPair(arguments, "-c:v", "libx264") || !strings.Contains(capability.SDPFmtpLine, "profile-level-id=42e02a") {
		t.Fatalf("high delivery capability=%+v args=%v", capability, arguments)
	}
}

func containsArgumentPair(arguments []string, name, value string) bool {
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name && arguments[index+1] == value {
			return true
		}
	}
	return false
}

func TestWebRTCSessionContextOutlivesRequestContext(t *testing.T) {
	requestContext, cancelRequest := context.WithCancel(context.Background())
	sessionContext, cancelSession := newWebRTCSessionContext(50 * time.Millisecond)
	defer cancelSession()

	cancelRequest()
	select {
	case <-sessionContext.Done():
		t.Fatal("session was canceled with the HTTP request")
	default:
	}
	if requestContext.Err() != context.Canceled {
		t.Fatalf("request context err=%v", requestContext.Err())
	}
	select {
	case <-sessionContext.Done():
	case <-time.After(time.Second):
		t.Fatal("session TTL did not cancel its context")
	}
}

func TestWebRTCOfferHidesNodeFailureDetails(t *testing.T) {
	node, err := NewNode(NodeConfig{
		Name: "node", PublicURL: "http://node", GatewayURL: "http://gateway", NodeAPIToken: "node-token",
		MediaRoot: t.TempDir(), PosterRoot: t.TempDir(), HTTPClient: &http.Client{},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/internal/webrtc/offer", bytes.NewBufferString(`{"session_id":"session","media_key":"missing-media-key","sdp":"secret-node-path","type":"offer"}`))
	request.Header.Set("Authorization", "Bearer node-token")
	response := httptest.NewRecorder()
	node.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "media is unavailable") || strings.Contains(response.Body.String(), "secret-node-path") {
		t.Fatalf("internal failure leaked: %s", response.Body.String())
	}
}

func TestWebRTCWithRealMedia(t *testing.T) {
	root := os.Getenv("TEST_MEDIA_ROOT")
	if root == "" {
		t.Skip("TEST_MEDIA_ROOT is not set")
	}
	probeScanner, err := NewScanner(root, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var mediaPath string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || mediaPath != "" {
			return walkErr
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mp4" && ext != ".m4v" && ext != ".mov" {
			return nil
		}
		probe, probeErr := probeScanner.ffprobe(path)
		if probeErr == nil && probe.VideoCodec == "h264" && probe.AudioCodec == "aac" {
			mediaPath = path
		}
		return nil
	})
	if err != nil || mediaPath == "" {
		t.Fatalf("ready H.264/AAC media not found: %v", err)
	}
	scanner := &Scanner{paths: map[string]string{"real-media": mediaPath}}
	sessions := NewWebRTCSessions(scanner)

	client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}
	tracks := make(chan webrtc.RTPCodecType, 2)
	var once sync.Map
	client.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, loaded := once.LoadOrStore(track.Kind(), true); loaded {
			return
		}
		go func() {
			if _, _, readErr := track.ReadRTP(); readErr == nil {
				tracks <- track.Kind()
			}
		}()
	})
	offer, err := client.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathering := webrtc.GatheringCompletePromise(client)
	if err := client.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-gathering
	answer, err := sessions.create(WebRTCOffer{SessionID: "real-session", MediaKey: "real-media", SDP: client.LocalDescription().SDP, Type: client.LocalDescription().Type.String(), AllowDirect: true})
	if err != nil {
		t.Fatal(err)
	}
	defer sessions.close("real-session")
	if err := client.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.NewSDPType(answer.Type), SDP: answer.SDP}); err != nil {
		t.Fatal(err)
	}
	received := map[webrtc.RTPCodecType]bool{}
	deadline := time.After(30 * time.Second)
	for len(received) < 2 {
		select {
		case kind := <-tracks:
			received[kind] = true
		case <-deadline:
			t.Fatalf("RTP tracks received=%v", received)
		}
	}
}
