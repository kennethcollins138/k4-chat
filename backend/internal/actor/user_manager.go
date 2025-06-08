package actor

import "github.com/anthdm/hollywood/actor"

type UserManagerActor struct {
	actor.Receiver
	users map[string]*actor.PID // map of userID to user actor PID
}

func (a *UserManagerActor) Receive(ctx *actor.Context) {
	switch ctx.Message().(type) {
	}
}
