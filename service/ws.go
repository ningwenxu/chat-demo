package service

import (
	"chat-demo/cache"
	"chat-demo/conf"
	"chat-demo/pkg/e"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"strconv"
	"time"
)

const month = 60 * 60 * 24 * 30 //一个月30天

type SendMsg struct {
	Type    int    `json:"type"`
	Content string `json:"content"`
}
type ReplyMsg struct {
	From    string `json:"from"`
	Code    int    `json:"code"`
	Content string `json:"content"`
}
type Client struct {
	ID     string
	SendID string
	Socket *websocket.Conn
	Send   chan []byte
}
type Broadcast struct {
	Client  *Client
	Message []byte
	Type    int
}
type ClientManager struct {
	Clients    map[string]*Client
	Broadcast  chan *Broadcast
	Reply      chan *Client
	Register   chan *Client
	Unregister chan *Client
}
type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

var Manager = ClientManager{
	Clients:    make(map[string]*Client), //参与连接的用户，出于性能考虑，需要设置的最大连接数
	Broadcast:  make(chan *Broadcast),
	Register:   make(chan *Client),
	Reply:      make(chan *Client),
	Unregister: make(chan *Client),
}

func CreateID(uid, toUid string) string {
	return uid + "->" + toUid //1->2

}
func Handler(c *gin.Context) {
	uid := c.Query("uid")
	toUid := c.Query("toUid")
	conn, err := (&websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}).Upgrade(c.Writer, c.Request, nil) //升级为ws协议
	if err != nil {
		http.NotFound(c.Writer, c.Request)
		return
	}
	client := &Client{
		ID:     CreateID(uid, toUid), //1->2
		SendID: CreateID(toUid, uid), //2->1
		Socket: conn,
		Send:   make(chan []byte), //消息内容
	}
	//用户注册到用户管理上
	Manager.Register <- client
	go client.Read()
	go client.Write()
}

func (c *Client) Read() {
	defer func() {
		Manager.Unregister <- c
		_ = c.Socket.Close()
	}()
	for {
		c.Socket.PongHandler()
		sendMsg := new(SendMsg)
		err := c.Socket.ReadJSON(&sendMsg)
		if err != nil {
			fmt.Println("数据格式不正确", err)
			Manager.Unregister <- c
			_ = c.Socket.Close()
			break
		}
		if sendMsg.Type == 1 {
			//发送消息
			r1, _ := cache.RedisClient.Get(c.ID).Result()     //1->2
			r2, _ := cache.RedisClient.Get(c.SendID).Result() //2->1
			if r1 > "3" && r2 == "" {
				//1给2发消息3条  但是2没有回  或者是没有看到  就停止发送
				replyMsg := ReplyMsg{
					Code:    e.WebsocketLimit,
					Content: "到底了",
				}
				msg, _ := json.Marshal(replyMsg) //序列化
				_ = c.Socket.WriteMessage(websocket.TextMessage, msg)
				continue
			} else {
				cache.RedisClient.Incr(c.ID)
				_, _ = cache.RedisClient.Expire(c.ID, time.Hour*24*30*3).Result()
				//防止过快 “分手”  建立连接三个月过期
			}
			Manager.Broadcast <- &Broadcast{
				Client:  c,
				Message: []byte(sendMsg.Content), //发送过来的信息
			}
		} else if sendMsg.Type == 2 {
			//获取历史消息
			timeT, err := strconv.Atoi(sendMsg.Content) /// string to int
			if err != nil {
				timeT = 999999
			}
			results, _ := FindMany(conf.MongoDBName, c.SendID, c.ID, int64(timeT), 10) //获取10条历史消息
			if len(results) > 10 {
				results = results[:10]

			} else if len(results) == 0 {
				replyMsg := ReplyMsg{
					Code:    e.WebsocketEnd,
					Content: "1",
				}
				msg, _ := json.Marshal(replyMsg)
				_ = c.Socket.WriteMessage(websocket.TextMessage, msg)
				continue
			}
			for _, result := range results {
				replyMsg := ReplyMsg{
					From:    result.From,
					Content: result.Msg,
				}
				msg, _ := json.Marshal(replyMsg)
				_ = c.Socket.WriteMessage(websocket.TextMessage, msg)
			}
		}
	}
}
func (c *Client) Write() {
	defer func() {
		_ = c.Socket.Close()

	}()
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				_ = c.Socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			replayMsg := ReplyMsg{
				Code:    e.WebsocketSuccessMessage,
				Content: fmt.Sprintf("%s", string(message)),
			}
			msg, _ := json.Marshal(replayMsg)
			_ = c.Socket.WriteMessage(websocket.TextMessage, msg)
		}
	}
}
