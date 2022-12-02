package main

import (
	"chat-demo/conf"
	"chat-demo/router"
	"chat-demo/service"
)

func main() {
	conf.Init()
	go service.Manager.Start()
	r := router.NewRouter()
	_ = r.Run(conf.HttpPort)
}
