package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	webrtc "github.com/pion/webrtc/v3"
)

type Helper struct {
	StreamerIp string
	RtspIp     string
}

func (helper *Helper) GetApiUrl(action string, StreamerIp string, peerId string, RtspIp string) string {
	switch action {
	case "ice-server":
		return helper.StreamerIp + "/getIceServers"
	case "call":
		return helper.StreamerIp + "/call?peerid=" + peerId + "&url=" + url.QueryEscape(helper.RtspIp)
	case "hang-up":
		return helper.StreamerIp + "/hangup?peerid=" + peerId
	case "get-ice-cand":
		return helper.StreamerIp + "/getIceCandidate?peerid=" + peerId
	case "add-ice-cand":
		return helper.StreamerIp + "/addIceCandidate?peerid=" + peerId
	default:
		return ""
	}
}

func (helper *Helper) AddIceCandidate(pc *webrtc.PeerConnection, peerId string, iceCandidate *webrtc.ICECandidate) error {
	// Turn ICE to json
	iceJson := iceCandidate.ToJSON()

	fmt.Println("Adding ice candidate to streamer: ", iceJson)

	// call url
	callUrl := helper.GetApiUrl("add-ice-cand", helper.StreamerIp, peerId, helper.RtspIp)

	body, _ := json.Marshal(iceJson)

	resp, err := http.Post(callUrl, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Add ice candidate to streamer status: %s\n", resp.Status)

	return nil
}

func (helper *Helper) GetIceCandidate(peerId string) chan []webrtc.ICECandidateInit {
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
		callUrl := helper.GetApiUrl("add-ice-cand", helper.StreamerIp, peerId, helper.RtspIp)

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

func (helper *Helper) SendOffer(peerId string, offer webrtc.SessionDescription) chan webrtc.SessionDescription {

	channel := make(chan webrtc.SessionDescription)

	go func() {
		fmt.Println("")
		fmt.Println("======= This is function send offer  =======")

		callUrl := helper.GetApiUrl("add-ice-cand", helper.StreamerIp, peerId, helper.RtspIp)

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
