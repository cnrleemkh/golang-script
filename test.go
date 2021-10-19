package main

import (
	"fmt"

	"github.com/cnrleemkh/golang-script/pion-webrtc/webrtc-streamer/helper"
)

func main() {
	helper := helper.Helper{
		StreamerIp: "http://192.168.0.14:8080/api",
		RtspIp:     "rtsp://192.168.0.16",
	}

	fmt.Println(helper)
}
