package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type WebRTCSessions struct {
	mu       sync.Mutex
	sessions map[string]*webrtcSession
	scanner  *Scanner
}

type webrtcSession struct {
	pc     *webrtc.PeerConnection
	ffmpeg *exec.Cmd
	udp    []net.PacketConn
	cancel context.CancelFunc
	done   <-chan error
}

type WebRTCOffer struct {
	SessionID   string             `json:"session_id"`
	MediaKey    string             `json:"media_key"`
	SDP         string             `json:"sdp"`
	Type        string             `json:"type"`
	ICEServers  []webrtc.ICEServer `json:"ice_servers"`
	AllowDirect bool               `json:"allow_direct"`
}

type WebRTCAnswer struct {
	SessionID string `json:"session_id"`
	SDP       string `json:"sdp"`
	Type      string `json:"type"`
}

func NewWebRTCSessions(scanner *Scanner) *WebRTCSessions {
	return &WebRTCSessions{sessions: make(map[string]*webrtcSession), scanner: scanner}
}

func (n *Node) webrtcOffer(w http.ResponseWriter, r *http.Request) {
	if !n.authorized(r) {
		writeError(w, http.StatusUnauthorized, "node authentication required")
		return
	}
	var offer WebRTCOffer
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&offer); err != nil || offer.SessionID == "" || offer.MediaKey == "" || offer.SDP == "" {
		writeError(w, http.StatusUnprocessableEntity, "invalid WebRTC offer")
		return
	}
	answer, err := n.webrtc.create(offer)
	if err != nil {
		log.Printf("WebRTC offer failed: %v", err)
		writeError(w, http.StatusBadGateway, "WebRTC playback setup failed")
		return
	}
	writeJSON(w, http.StatusOK, answer)
}

func (n *Node) webrtcClose(w http.ResponseWriter, r *http.Request) {
	if !n.authorized(r) {
		writeError(w, http.StatusUnauthorized, "node authentication required")
		return
	}
	var input struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil || input.SessionID == "" {
		writeError(w, 422, "invalid session")
		return
	}
	n.webrtc.close(input.SessionID)
	w.WriteHeader(http.StatusNoContent)
}

func (m *WebRTCSessions) create(offer WebRTCOffer) (WebRTCAnswer, error) {
	path, ok := m.scanner.ResolveMedia(offer.MediaKey)
	if !ok {
		return WebRTCAnswer{}, errors.New("media is unavailable")
	}
	m.close(offer.SessionID)
	ctx, cancel := newWebRTCSessionContext(6 * time.Hour)
	profile, level, _ := probeH264Profile(ctx, path)
	videoCapability, videoArguments := selectH264Delivery(profile, level, offer.SDP)
	pc, err := newPeerConnection(offer.ICEServers, offer.AllowDirect)
	if err != nil {
		cancel()
		return WebRTCAnswer{}, err
	}
	videoTrack, err := webrtc.NewTrackLocalStaticRTP(videoCapability, "video", "deer")
	if err != nil {
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "deer")
	if err != nil {
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	if _, err = pc.AddTrack(videoTrack); err != nil {
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	if _, err = pc.AddTrack(audioTrack); err != nil {
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	udpVideo, videoPort, err := listenRTP()
	if err != nil {
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	udpAudio, audioPort, err := listenRTP()
	if err != nil {
		udpVideo.Close()
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	ffmpegArguments := []string{"-hide_banner", "-loglevel", "error", "-re", "-i", path, "-map", "0:v:0"}
	ffmpegArguments = append(ffmpegArguments, videoArguments...)
	ffmpegArguments = append(ffmpegArguments, "-an", "-f", "rtp", fmt.Sprintf("rtp://127.0.0.1:%d?pkt_size=1200&payload_type=96", videoPort), "-map", "0:a:0?", "-vn", "-c:a", "libopus", "-b:a", "96k", "-f", "rtp", fmt.Sprintf("rtp://127.0.0.1:%d?pkt_size=1200&payload_type=111", audioPort))
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArguments...)
	if err := cmd.Start(); err != nil {
		udpVideo.Close()
		udpAudio.Close()
		pc.Close()
		cancel()
		return WebRTCAnswer{}, err
	}
	done := make(chan error, 1)
	session := &webrtcSession{pc: pc, ffmpeg: cmd, udp: []net.PacketConn{udpVideo, udpAudio}, cancel: cancel, done: done}
	m.mu.Lock()
	m.sessions[offer.SessionID] = session
	m.mu.Unlock()
	go func() {
		done <- cmd.Wait()
		close(done)
		m.close(offer.SessionID)
	}()
	go func() {
		<-ctx.Done()
		m.close(offer.SessionID)
	}()
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateDisconnected {
			m.close(offer.SessionID)
		}
	})
	go forwardRTP(ctx, udpVideo, videoTrack)
	go forwardRTP(ctx, udpAudio, audioTrack)
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.NewSDPType(offer.Type), SDP: offer.SDP}); err != nil {
		m.close(offer.SessionID)
		return WebRTCAnswer{}, err
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		m.close(offer.SessionID)
		return WebRTCAnswer{}, err
	}
	gatheringComplete := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		m.close(offer.SessionID)
		return WebRTCAnswer{}, err
	}
	select {
	case <-time.After(5 * time.Second):
	case <-gatheringComplete:
	}
	local := pc.LocalDescription()
	if local == nil {
		m.close(offer.SessionID)
		return WebRTCAnswer{}, errors.New("missing local description")
	}
	return WebRTCAnswer{SessionID: offer.SessionID, SDP: local.SDP, Type: local.Type.String()}, nil
}

func probeH264Profile(ctx context.Context, path string) (string, int, error) {
	output, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=profile,level", "-of", "json", path).Output()
	if err != nil {
		return "", 0, err
	}
	var body struct {
		Streams []struct {
			Profile string `json:"profile"`
			Level   int    `json:"level"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &body); err != nil || len(body.Streams) == 0 {
		return "", 0, errors.New("H.264 profile is unavailable")
	}
	return body.Streams[0].Profile, body.Streams[0].Level, nil
}

func selectH264Delivery(profile string, level int, offerSDP string) (webrtc.RTPCodecCapability, []string) {
	prefixes := map[string]string{"Baseline": "4200", "Constrained Baseline": "42e0", "Main": "4d00"}
	prefix := prefixes[profile]
	if prefix != "" && level > 0 && level < 256 && strings.Contains(strings.ToLower(offerSDP), "profile-level-id="+prefix) {
		profileLevelID := fmt.Sprintf("%s%02x", prefix, level)
		return webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=" + profileLevelID}, []string{"-c:v", "copy"}
	}
	return webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000, SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e02a"}, []string{"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-profile:v", "baseline", "-level:v", "4.2", "-pix_fmt", "yuv420p", "-crf", "23", "-bf", "0", "-g", "60", "-keyint_min", "60", "-sc_threshold", "0"}
}

func (m *WebRTCSessions) close(id string) {
	m.mu.Lock()
	session := m.sessions[id]
	delete(m.sessions, id)
	m.mu.Unlock()
	if session == nil {
		return
	}
	session.cancel()
	for _, conn := range session.udp {
		_ = conn.Close()
	}
	if session.pc != nil {
		_ = session.pc.Close()
	}
	if session.ffmpeg != nil && session.ffmpeg.Process != nil {
		_ = session.ffmpeg.Process.Kill()
	}
	if session.done != nil {
		select {
		case <-session.done:
		case <-time.After(2 * time.Second):
		}
	}
}

func newWebRTCSessionContext(ttl time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ttl)
}

func peerConfiguration(servers []webrtc.ICEServer, allowDirect bool) webrtc.Configuration {
	policy := webrtc.ICETransportPolicyRelay
	if allowDirect {
		policy = webrtc.ICETransportPolicyAll
	}
	return webrtc.Configuration{ICEServers: servers, ICETransportPolicy: policy}
}

func newPeerConnection(servers []webrtc.ICEServer, allowDirect bool) (*webrtc.PeerConnection, error) {
	setting := webrtc.SettingEngine{}
	setting.SetICEMulticastDNSMode(ice.MulticastDNSModeQueryAndGather)
	api := webrtc.NewAPI(webrtc.WithSettingEngine(setting))
	return api.NewPeerConnection(peerConfiguration(servers, allowDirect))
}

func listenRTP() (net.PacketConn, int, error) {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	return conn, conn.LocalAddr().(*net.UDPAddr).Port, nil
}

func forwardRTP(ctx context.Context, conn net.PacketConn, track *webrtc.TrackLocalStaticRTP) {
	buffer := make([]byte, 2048)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		packet := &rtp.Packet{}
		if packet.Unmarshal(buffer[:n]) == nil {
			_ = track.WriteRTP(packet)
		}
	}
}
