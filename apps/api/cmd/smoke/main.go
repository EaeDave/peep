package main
import ("fmt";"os";"time";lksdk "github.com/livekit/server-sdk-go/v2")
func main(){
 r,err:=lksdk.ConnectToRoomWithToken(os.Args[1],os.Args[2],&lksdk.RoomCallback{})
 if err!=nil{fmt.Println("ERR",err);os.Exit(1)}
 fmt.Println("connected room",r.Name(),"identity",r.LocalParticipant.Identity(),"name",r.LocalParticipant.Name())
 time.Sleep(8*time.Second); r.Disconnect()}
