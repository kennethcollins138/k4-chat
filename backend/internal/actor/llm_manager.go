package actor

import "github.com/anthdm/hollywood/actor"

// Will definitely be stateful, need to specify model, BYOK or api etc.
// Will need to manage the LLM connections, sessions, sync operations, tools operations, user operations, chat operations, sync operations.
// Will need to manage the LLM connections, sessions, sync operations, tools operations, user operations, chat operations, sync operations.
// Message bus will be used to cooordinate everything specifically one to many return on sync across user sessions

/*
LLMManagerActor is a actor that manages the LLM.
LLMManagerActor is responsible for:
  - Managing LLM operations
  - Managing LLM connections
  - Managing LLM sessions
  - Managing LLM sync operations
  - Managing LLM tools operations
  - Managing LLM user operations
  - Managing LLM chat operations
  - Managing LLM sync operations
*/
type LLMManagerActor struct {
	actor.Receiver
}

func (a *LLMManagerActor) Receive(ctx *actor.Context) {
	switch ctx.Message().(type) {
	}
}
