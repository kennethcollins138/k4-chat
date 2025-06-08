package actor

import "github.com/anthdm/hollywood/actor"

// Notes: will more than likely be stateful. need constructurs websearch/image-gen/file managing
/*
ToolsManagerActor is a actor that manages the tools.
ToolsManagerActor is responsible for:
  - Managing tools operations
  - Managing tools connections
  - Managing tools sessions
  - Managing tools sync operations
*/
type ToolsManagerActor struct {
	actor.Receiver
}

func (a *ToolsManagerActor) Receive(ctx *actor.Context) {
	switch ctx.Message().(type) {
	}
}
