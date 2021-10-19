package webrtcstreamer

import (
	"log"

	"github.com/pion/webrtc/v3"
)

type WebRtcStreamer struct {
	serverUrl string
	pc        *webrtc.PeerConnection
	iceServer interface{}
}

type WebRtcStreamerMethods interface {
	New()
	Connect()
	Disconnect()
	OnReceiveGetIceServers()
	GetIceCandidate()
	CreatePeerConnection()
	OnIceCandidate()
	AddIceCandidate()
	OnAddStream()
}

func (streamer WebRtcStreamer) New(rtspUrl string) WebRtcStreamer {
	streamer = WebRtcStreamer{
		serverUrl: rtspUrl,
	}

	return streamer
}

// Driver function
func (streamer WebRtcStreamer) Connect(rtspUrl string) {

}

func (streamer WebRtcStreamer) Disconnect(peerId string) {

	if err := streamer.pc.Close(); err != nil {
		log.Fatal(err)
	}
}

func (streamer WebRtcStreamer) OnReceiveGetIceServers() {

}

func (streamer WebRtcStreamer) GetIceCandidate() {

}

func (streamer WebRtcStreamer) CreatePeerConnection() {

}

func (streamer WebRtcStreamer) OnIceCandidate() {

}

func (streamer WebRtcStreamer) AddIceCandidate() {

}

func (streamer WebRtcStreamer) OnAddStream() {

}
