package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	webrtc "github.com/pion/webrtc/v3"
)

const (
	rtspIp     string = "rtsp://192.168.0.16"
	streamerIp string = "http://192.168.0.14:8080/api"
)

/*
	Action
	1. ice-server
	2. call
	3. hang-up
	4. get-ice-cand
	5. add-ice-cand
*/
func getApiUrl(action string, peerId string) string {
	switch action {
	case "ice-server":
		return streamerIp + "/getIceServers"
	case "call":
		return streamerIp + "/call?peerid=" + peerId + "&url=" + url.QueryEscape(rtspIp)
	case "hang-up":
		return streamerIp + "/hangup?peerid=" + peerId
	case "get-ice-cand":
		return streamerIp + "/getIceCandidate?peerid=" + peerId
	case "add-ice-cand":
		return streamerIp + "/addIceCandidate?peerid=" + peerId
	default:
		return ""
	}
}

func main() {

	earlyCandidates := make(map[string]webrtc.ICECandidate)

	// Set up peer connection config
	peerId, pc, err := CreatePeerConnection(earlyCandidates)

	if err != nil {
		panic(err)
	}

	defer func() {
		if err := pc.Close(); err != nil {
			fmt.Printf("cannot close peerConnection: %v\n", err)
		}
	}()

	/*
		Create an offer and set it to local description
	*/

	offer, _ := pc.CreateOffer(&webrtc.OfferOptions{
		OfferAnswerOptions: webrtc.OfferAnswerOptions{
			VoiceActivityDetection: true,
		},
		ICERestart: false,
	})

	// fmt.Println("This is offer: ", offer)

	if err := pc.SetLocalDescription(offer); err != nil {
		panic(err)
	}

	/*
		1. Send the offer to streamer through API
		2. API return the answer
		3. Set remote description with the answer returned
	*/

	answer := <-SendOffer(peerId, offer)

	// fmt.Println("This is answer from streamer: ", answer)

	if err := pc.SetRemoteDescription(answer); err != nil {
		log.Fatal(err)
	}

	gatherCompleted := webrtc.GatheringCompletePromise(pc)

	/*
		Get ice candidates from streamer, add each to peer connection
	*/

	fmt.Println("After get ice candidates")

	/*
		Create channel blocked until ice gathering completed
	*/

	<-gatherCompleted

	fmt.Println("")
	fmt.Println("======= This is after gathering completed  =======")

	iceCandidates := <-GetIceCandidate(peerId)

	fmt.Println(iceCandidates)

	for _, iceCandidate := range iceCandidates {
		pc.AddICECandidate(iceCandidate)
	}
	/*
		Empty select keeps the program opened
	*/
	select {}
}

// func CreatePeerConnection(candidates sync.Mutex, pendingCandidates []*webrtc.ICECandidate) (pc *webrtc.PeerConnection, err error) {
func CreatePeerConnection(earlyCandidates map[string]webrtc.ICECandidate) (peerId string, pc *webrtc.PeerConnection, err error) {
	// Set up peer connection config
	webrtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create unique id for this peer connection
	peerId = uuid.New().String()

	fmt.Printf("\n")
	fmt.Println("Peer id: ", peerId)

	// Setup webrtc media engine
	mediaEngine := webrtc.MediaEngine{}

	// Setup video codec, no audio for now (but supported by pkg)
	rtpCodec := webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeH264,
		ClockRate: 90000,
	}

	if err := mediaEngine.RegisterCodec(
		webrtc.RTPCodecParameters{
			RTPCodecCapability: rtpCodec,
			PayloadType:        96,
		},
		webrtc.RTPCodecTypeVideo,
	); err != nil {
		panic(err)
	}

	// Create API object with media engine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))

	// Create new pc
	pc, _ = api.NewPeerConnection(webrtcConfig)

	// Set up local track

	videoTrack, _ := webrtc.NewTrackLocalStaticRTP(rtpCodec, "video", "pion_video")

	if _, err = pc.AddTrack(videoTrack); err != nil {
		log.Fatal(err)
	}

	// Create data channel
	dataChannel, err := pc.CreateDataChannel("data", nil)
	if err != nil {
		log.Fatal(err)
	}

	dataChannel.OnOpen(func() {
		fmt.Print("Peer connection data channel: Open \n")
	})

	dataChannel.OnClose(func() {
		fmt.Print("Peer connection data channel: Close \n")
	})

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		log.Fatal(err)
	}

	/*
		Setup the following functions on pc:
		1. pc.onicecandidate = bind.onIceCandidate.call(bind, evt); };
		2. pc.onaddstream    = function(evt) { bind.onAddStream.call(bind,evt); };
		3. pc.oniceconnectionstatechange = callback
		4. pc.ondatachannel = callback
		5. pc.onicegatheringstatechange = callback
	*/

	// pc.onicecandidate
	pc.OnICECandidate(func(iceCandidate *webrtc.ICECandidate) {
		if iceCandidate == nil {
			return
		}

		remoteDes := pc.RemoteDescription()
		if remoteDes == nil {
			fmt.Printf("on ice candidate event: no remote desc, saving candidates to map for later use \n")
			uuid := uuid.New().String()
			earlyCandidates[uuid] = *iceCandidate
		} else {
			fmt.Printf("on ice candidate event: now add early candidates and new ice candidate to streamer\n")

			/*
				Add all accumulated candidates
			*/

			for index, earlyCandidate := range earlyCandidates {
				AddIceCandidate(pc, peerId, &earlyCandidate)
				delete(earlyCandidates, index)
			}

			/*
				Add the new ice candidate
			*/
			AddIceCandidate(pc, peerId, iceCandidate)
		}
	})

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Println("on track triggered")
	})

	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		fmt.Printf("Data to channel: %d \n", dataChannel.ID())

		dataChannel.OnOpen(func() {
			fmt.Printf("Data channel is opened \n")
		})

		dataChannel.OnMessage(func(message webrtc.DataChannelMessage) {
			fmt.Println("Data channel on message.isString: ", message.IsString)
			fmt.Println("Data channel on message.Data: ", string(message.Data))
		})
	})

	/*
		Print connection state to console
	*/

	pc.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State changed to: %s \n", pcs.String())

		if pcs == webrtc.PeerConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil {
				panic(closeErr)
			}
		}
	})

	// Trigger when connected, if connection failed, close pc and send panic
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection state has changed to: %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateNew {
			fmt.Printf("ICE connection state is new\n")
			GetIceCandidate(peerId)
		}

		// if connectionState == webrtc.ICEConnectionStateCompleted {
		// 	receivers := pc.GetReceivers()

		// 	for _, receiver := range receivers {
		// 		fmt.Println(receiver)
		// 	}
		// }

		if connectionState == webrtc.ICEConnectionStateChecking {
			fmt.Println("doing some checkings")
		}

		if connectionState == webrtc.ICEConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil {
				panic(closeErr)
			}
		}
	})

	// Show ice gathering state changes, triggered by set local and remote description
	pc.OnICEGatheringStateChange(func(iceGatherState webrtc.ICEGathererState) {
		fmt.Printf("ICE Gathering state has changed to: %s \n", iceGatherState.String())

		// if iceGatherState == webrtc.ICEGathererStateComplete {
		// 	receivers := pc.GetReceivers()

		// 	for _, receiver := range receivers {
		// 		fmt.Println(receiver.Track())
		// 	}
		// }
	})

	pc.OnSignalingStateChange(func(signalingState webrtc.SignalingState) {
		fmt.Printf("Signaling state has changed to: %s \n", signalingState.String())
	})

	pc.OnNegotiationNeeded(func() {
		fmt.Println("This is on negotiation needed")
	})

	return
}

func AddIceCandidate(pc *webrtc.PeerConnection, peerId string, iceCandidate *webrtc.ICECandidate) error {
	// Turn ICE to json
	iceJson := iceCandidate.ToJSON()

	fmt.Println("Adding ice candidate to streamer: ", iceJson)

	// call url
	callUrl := getApiUrl("add-ice-cand", peerId)

	body, _ := json.Marshal(iceJson)

	resp, err := http.Post(callUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Add ice candidate to streamer status: %s\n", resp.Status)

	return nil
}

func GetIceCandidate(peerId string) chan []webrtc.ICECandidateInit {
	/*
		Create a go channel for async operation
		1. make = initiate the channel
		2. chan = go channel type
		3. type = data type that the channel returns
	*/
	channel := make(chan []webrtc.ICECandidateInit)

	go func() {
		fmt.Println("")
		fmt.Println("======= This is function get ice candidate  =======")

		// Set get ice candidate url
		callUrl := getApiUrl("get-ice-cand", peerId)

		// Sync get request
		resp, err := http.Get(callUrl)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		fmt.Printf("Get ice candidate state: %s\n", resp.Status)
		fmt.Println("Get ice candidate response header: ", resp.Header)

		// Parse the binary
		body, _ := ioutil.ReadAll(resp.Body)

		// Create an array
		iceCandidates := []webrtc.ICECandidateInit{}

		// Fill the array with data
		if err = json.Unmarshal(body, &iceCandidates); err != nil {
			log.Fatal(err)
		}

		// Return the resulting json
		channel <- iceCandidates
	}()

	return channel
}

func SendOffer(peerId string, offer webrtc.SessionDescription) chan webrtc.SessionDescription {

	channel := make(chan webrtc.SessionDescription)

	go func() {
		fmt.Println("")
		fmt.Println("======= This is function send offer  =======")

		callUrl := getApiUrl("call", peerId)

		bodyJson, _ := json.Marshal(offer)

		response, err := http.Post(callUrl, "application/json;charset=UTF-8", bytes.NewBuffer(bodyJson))

		if err != nil {
			log.Fatal(err)
		}

		defer response.Body.Close()

		body, err := ioutil.ReadAll(response.Body)

		fmt.Println("Response status: ", response.Status)
		// fmt.Println("Response body: ", string(body))

		if err != nil {
			log.Fatal(err)
		}

		// This is the SDP description for the WebRTC answer
		callResponseJson := webrtc.SessionDescription{}

		jsonErr := json.Unmarshal(body, &callResponseJson)
		if jsonErr != nil {
			log.Fatal(jsonErr)
		}

		channel <- callResponseJson
	}()

	return channel
}
