package models

import (
	"time"

	"github.com/google/uuid"
)

/*
ImageGeneration represents a single AI-generated image tied to a user message.

This model is designed with **future extensibility** and **auditability** in mind. Although the
initial implementation focuses on basic prompt-to-image generation, additional fields have been
included (or reserved for later) to support scaling to:

- Multiple image results per generation
- Image generation settings (e.g. style, quality, seed)
- Async/scheduled generation
- User feedback and moderation
- Status/lifecycle tracking

Key Design Notes:

- MessageID:
    Ties the image to a specific user message in the chat, ensuring contextual linkage.

- ThumbnailURL:
    Optimized preview or low-res version, ideal for galleries or mobile displays.

- Width / Height:
    Helps with layout and filtering in UIs or when enforcing resolution constraints.

- ModelName:
    Tracks which model (e.g. DALL·E, Stable Diffusion) generated the image — allows stats,
    audits, or cost tracking if multiple providers are used.

- Prompt:
    Stored verbatim to ensure traceability and reproduceability.

Planned additions once again, if I have time:

- Status:
    Tracks whether the generation is pending, completed, failed, etc.

- Settings (map or structured type):
    Stores style options, guidance scale, seed, negative prompts, etc.

- Feedback (e.g. thumbs up/down or tags):
    Allows users to rate or annotate generated images for refinement and analytics.
*/

type ImageGeneration struct {
	ID uuid.UUID `json:"id" db:"id"` // Unique image generation ID

	MessageID uuid.UUID `json:"message_id" db:"message_id"` // Associated message in chat session

	Prompt string `json:"prompt" db:"prompt"` // Text prompt used for generation

	ModelName string `json:"model_name" db:"model_name"` // Name of the generation model (DALLE Sora ya know)

	ImageURL string `json:"image_url" db:"image_url"` // Direct URL to the generated image

	ThumbnailURL *string `json:"thumbnail_url,omitempty" db:"thumbnail_url"` // Optional low-res preview URL (TODO: WE WILL SEE IF I HAVE TIME FOR THIS)

	Width  *int `json:"width,omitempty" db:"width"`   // Optional image width in pixels (Priority: 1)
	Height *int `json:"height,omitempty" db:"height"` // Optional image height in pixels (Priority: 1)

	CreatedAt time.Time `json:"created_at" db:"created_at"` // Time of image generation
}
