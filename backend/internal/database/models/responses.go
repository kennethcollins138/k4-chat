package models

/*
ChatWithMessagesResponse represents the response structure returned when fetching a
complete chat session along with its associated messages.

This type is typically used in API responses where both session metadata and the
full conversation history are needed in a single payload.

Key Design Notes:

- Embeds `ChatSession` directly to include all session-level metadata.
- Messages are returned as a flat list but can represent a tree via `ParentMessageID` and `Children`.

Scalability Considerations:

- This structure assumes that the full message history is returned at once.
  For large conversations, consider switching to:
    - Cursor-based pagination (`[]Message` → `[]MessagePage`)
    - Streaming response (e.g., for real-time UIs)
    - Filtered responses (e.g., only selected branches or latest N messages)

Future Enhancements:

- Add a `Summary` field for session previews.
- Include `UserPreferences` or `ViewMetadata` (e.g. which branch is selected).
- Add a `Stats` sub-struct for session analytics (e.g., token count, message count).
*/

type ChatWithMessagesResponse struct {
	ChatSession           // Embedded ChatSession metadata (ID, Title, Model, etc.)
	Messages    []Message `json:"messages"` // Flat array of all messages in the session
}
