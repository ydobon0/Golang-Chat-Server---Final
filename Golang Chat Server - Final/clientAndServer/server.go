package ClientAndServer

/*
To run:
Open the Chat Server directory in the terminal
Run main.go
connect to http://localhost:8080/
*/
import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var counter int = 0                      // counter to ensure each username is unique
var clients = make(map[*Client]string)   // map of all the clients in the server. This is used when sending messages to other users
var usernames = make(map[string]*Client) // map of all the usernames in the server. This is used to make sure all usernames are unique and when searching for a specific user
var broadcast = make(chan Message)       // channel that handles the sending of messages

var upgrader = websocket.Upgrader{}

var channelSli = make([]ChannelMap, 0) //slice containing all the different channels. This refers to the different channels that users can create and join when in the chat server, NOT golang channels

type ChannelMap struct {
	chl_clients map[*Client]string // map of all clients connected to a channel
	chl_names   map[string]*Client // map of all usernames of clients connected to the channel
}

func HandleClients(w http.ResponseWriter, r *http.Request) { //handles the client's messages and commands
	go broadcastMessagesToClients() // start goroutine for the client to broadcast its messages to all other users

	websocket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal("error upgrading GET request to a websocket :: ", err)
	}
	defer websocket.Close()
	cc := NewClient(websocket)
	clients[&cc] = ""

	//log.Printf("*** HandleClients%d 1", funcID)
	for {
		var message Message

		err := websocket.ReadJSON(&message)
		//log.Printf("*** HandleClients%d 2", funcID)

		if err != nil {
			log.Printf("error occurred while reading message : %v", err)
			deleteUser(&cc)
			updateUserList()
			break
		}
		// implement unique name and then add the unique name into maps of clients, usernames
		if cc.name == "" && message.Name != "" {
			message.Name += strconv.Itoa(counter) //counter is appended to each new username until the username is unique then it increments after the process is finished
			_, found := usernames[message.Name]
			for {
				if !found {
					cc.name = message.Name        // set the client's name value to the correct value
					usernames[message.Name] = &cc // update the username map
					break
				}
				message.Name += strconv.Itoa(counter)
				_, found = usernames[message.Name]
			}
			counter += 1
			clients[&cc] = message.Name // update the client map

			if channelSli[0].chl_clients[&cc] == "" { // add user and websocket to channel 1
				channelSli[0].chl_clients[&cc] = cc.name
				channelSli[0].chl_names[cc.name] = &cc
			}

			updateUserList()
		}

		if message.Type != "update" {
			updateUserList()
		}

		message.Name = cc.name // set the sender's name to the correct value

		// a command starts with / and ends with :: if the message starts with / and contains :: then it gets interpreted as a command and we call the processCommand() function to handle the command
		if message.Type == "message" && message.Message[0:1] == "/" && strings.Contains(message.Message, "::") {
			processCommand(&message)
		}

		if message.Type == "message" {
			extra := ""
			if message.Channel != "0" {
				inx, _ := strconv.Atoi(message.Channel)
				extra = "(to channel" + strconv.Itoa(inx+1) + ")"
			}
			if message.Target != "ALL" {
				extra = "(private message to " + message.Target + ")"
			}
			message.Message = time.Now().Format("15:04:05") + " *" + cc.name + extra + "*     " + message.Message
			broadcast <- message
		} else if message.Type == "channel" {
			addChannel()
			updateChannel()
		} else if message.Type == "update" {
			updateUserList()
			updateChannel()
		}
	}
}

func broadcastMessagesToClients() { //broadcasts client's messages to other users
	for {
		message := <-broadcast

		if message.Type == "message" {
			_, found := usernames[message.Target] //check if the target is a valid user
			if found {                            //Send message only to the target and the sender
				usernames[message.Name].conn.WriteJSON(message)
				if message.Name != message.Target {
					usernames[message.Target].conn.WriteJSON(message)
				}
			} else if message.Channel == "0" { // send  message to ALL connected clients
				for client := range clients {
					err := client.conn.WriteJSON(message)
					if err != nil {
						log.Printf("error occurred while writing message to client: %v", err)
						client.conn.Close()
						delete(usernames, client.name)
						delete(clients, client)
					}
				}
			} else { // send message to clients in a channel
				inx, _ := strconv.Atoi(message.Channel)
				_, found := channelSli[inx].chl_names[message.Name] // make sure the sender is in the channel
				if found {
					for client := range channelSli[inx].chl_clients {
						err := client.conn.WriteJSON(message)
						if err != nil {
							log.Printf("error occurred while writing message to client: %v", err)
							client.conn.Close()
							delete(clients, client)
						}
					}
				} else {
					message.Message = time.Now().Format("15:04:05") + " *" + message.Name + "*     You are not in channel " + strconv.Itoa(inx+1)
					err := usernames[message.Name].conn.WriteJSON(message) // send the message to the sender
					if err != nil {
						log.Printf("error occurred while writing message to client: %v", err)
					}
				}
			}
		}
	}
}

func RunServer() {
	//log.Printf("### main")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/echo", HandleClients)

	http.HandleFunc("/help", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "help.html")
	})

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("error starting http server :: ", err)

		return
	}
}

func init() { // called before RunServer()
	addChannel() // add the verfirst channel
}

// notify all connected clients that to update "To" drop down list
func updateUserList() {
	var message Message
	message.Message = "ALL"
	message.Type = "user"

	for user := range usernames {
		message.Message += "," + user
	}

	for client := range clients {
		err := client.conn.WriteJSON(message) //.Message)
		if err != nil {
			log.Printf("updateList: error occurred while writing message to client: %v", err)
			client.conn.Close()
			deleteUser(client)
		}
	}
}

// delete the user from maps, including all maps in channel slices
func deleteUser(cli *Client) {
	for ii := 0; ii < len(channelSli); ii++ {
		name, exist := channelSli[ii].chl_clients[cli]
		if exist {
			_, find := channelSli[ii].chl_names[name]
			if find {
				delete(channelSli[ii].chl_names, name)
			}
			delete(channelSli[ii].chl_clients, cli)
		}
	}

	_, found := usernames[clients[cli]]
	if found {
		delete(usernames, clients[cli])
	}
	delete(clients, cli)

}

// add a ChannelMap to "channelSli"
func addChannel() {
	chl := new(ChannelMap)
	chl.chl_clients = make(map[*Client]string)
	chl.chl_names = make(map[string]*Client)

	channelSli = append(channelSli, *chl)
}

// notify all connected clients that to update "Channel" drop down list
func updateChannel() {
	var message Message
	message.Channel = "0"
	message.Type = "channel"

	for ii := 1; ii < len(channelSli); ii++ {
		message.Channel += "," + strconv.Itoa(ii)
	}

	for client := range clients {
		err := client.conn.WriteJSON(message)
		if err != nil {
			log.Printf("updateChannel: error occurred while writing message to client: %v", err)
			client.conn.Close()
			deleteUser(client)
		}
	}
}

func processCommand(message *Message) { //interprets command if message is in the command format
	str := strings.Split(message.Message, "::")
	message.Message = str[1]
	strTemp := strings.ToLower(str[0][1:])
	strTemp = strings.TrimSpace(strTemp)
	command := strings.Split(strTemp, " ")

	switch command[0] {
	case "time": //command for getting the current time
		message.Message = time.Now().Format("15:04:05")
		message.Target = message.Name

	case "join": //command for join a channel
		if len(command) > 1 {
			strTemp = strings.TrimSpace(command[1])
			chl := strings.Split(strTemp, "channel")
			nn, _ := strconv.Atoi(chl[len(chl)-1])
			if nn > 0 && nn <= len(channelSli) {
				nn--
				cli, find := usernames[message.Name]
				if find && channelSli[nn].chl_clients[cli] == "" { // add user and websocket to channel slices
					channelSli[nn].chl_clients[cli] = message.Name
					channelSli[nn].chl_names[message.Name] = cli
				}
				message.Message = message.Name + " joined " + command[1]
				message.Target = message.Name
			} else {
				message.Message = command[1] + " does not exist"
				message.Target = message.Name
			}
		} else {
			message.Message = "Invalid join command! Please make sure to write out the channel you want to join!"
			message.Target = message.Name
		}
	case "leave":
		if len(command) > 1 {
			strTemp = strings.TrimSpace(command[1])
			chl := strings.Split(strTemp, "channel")
			nn, _ := strconv.Atoi(chl[len(chl)-1])
			if nn > 0 && nn <= len(channelSli) {
				ii := nn - 1
				websocket, find := channelSli[ii].chl_names[message.Name]
				if find {
					name, exist := channelSli[ii].chl_clients[websocket]
					if exist {
						delete(channelSli[ii].chl_names, name)
					}
					delete(channelSli[ii].chl_clients, websocket)
					message.Message = message.Name + " left " + command[1]
					message.Channel = "0"
					message.Target = message.Name
				} else {
					message.Message = message.Name + " is not in " + command[1]
					message.Channel = "0"
					message.Target = message.Name
				}
			} else {
				message.Message = command[1] + " does not exist"
				message.Target = message.Name
			}
		} else {
			message.Message = "Invalid leave command! Please make sure to write out the channel you want to leave!"
			message.Target = message.Name
		}

	default: //command for sending a private message to another user
		user := strings.TrimSpace(str[0][1:])
		_, found := usernames[user]
		if found {
			message.Target = user
		} else {
			message.Message = user + " does not exist. Please make sure you spelled your command correctly"
			message.Target = message.Name
		}
	}
}
