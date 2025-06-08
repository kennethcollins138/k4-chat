package actor

import "github.com/anthdm/hollywood/actor"

/*
UserActor is a actor that represents a user.
UserActor is responsible for:
  - Managing connections
  - Managing chat sessions
  - Managing sync operations
*/
type UserActor struct {
	actor.Receiver
	userID       string
	connections  map[string]*actor.PID // map of connectionID to connection actor PID
	chatSessions map[string]*actor.PID // map of chatSessionID to chatSession actor PID
	syncActor    *actor.PID            // actor that handles sync operations for the user
}

func (a *UserActor) Receive(ctx *actor.Context) {
	switch ctx.Message().(type) {
	}
}
