package root

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-promise/sure-cli/internal/output"
)

func newChatsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "chats", Short: "AI chat sessions (requires AI enabled on the account)"}

	cmd.AddCommand(newChatsListCmd())
	cmd.AddCommand(newChatsShowCmd())
	cmd.AddCommand(newChatsCreateCmd())
	cmd.AddCommand(newChatsUpdateCmd())
	cmd.AddCommand(newChatsDeleteCmd())
	cmd.AddCommand(newChatsMessagesCmd())

	return cmd
}

func newChatsListCmd() *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List chats",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			printGet(pathWithQuery("/api/v1/chats", q))
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "page number (upstream uses a fixed page size of 20)")
	return cmd
}

func newChatsShowCmd() *cobra.Command {
	var page int
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a chat with its messages (paged at 50/page)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			q := url.Values{}
			if page > 0 {
				q.Set("page", fmt.Sprintf("%d", page))
			}
			printGet(pathWithQuery(fmt.Sprintf("/api/v1/chats/%s", url.PathEscape(args[0])), q))
		},
	}
	cmd.Flags().IntVar(&page, "page", 0, "messages page number")
	return cmd
}

// ---------- create ----------

type chatCreateOpts struct {
	Title   string
	Message string
	Model   string
	Apply   bool
}

func buildChatCreateBody(o chatCreateOpts) (map[string]any, error) {
	if strings.TrimSpace(o.Title) == "" {
		return nil, errors.New("title is required (upstream validates presence)")
	}
	body := map[string]any{"title": o.Title}
	if o.Message != "" {
		body["message"] = o.Message
	}
	if o.Model != "" {
		body["model"] = o.Model
	}
	return body, nil
}

func newChatsCreateCmd() *cobra.Command {
	var o chatCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a chat (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := buildChatCreateBody(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "POST", "/api/v1/chats", body)
		},
	}
	cmd.Flags().StringVar(&o.Title, "title", "", "chat title (required)")
	cmd.Flags().StringVar(&o.Message, "message", "", "optional first user message")
	cmd.Flags().StringVar(&o.Model, "model", "", "optional AI model identifier")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

// ---------- update ----------

type chatUpdateOpts struct {
	Title string
	Apply bool
}

func buildChatUpdateBody(o chatUpdateOpts) (map[string]any, error) {
	if strings.TrimSpace(o.Title) == "" {
		return nil, errors.New("title is required")
	}
	return map[string]any{"title": o.Title}, nil
}

func newChatsUpdateCmd() *cobra.Command {
	var o chatUpdateOpts
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Rename a chat (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			body, err := buildChatUpdateBody(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "PATCH", fmt.Sprintf("/api/v1/chats/%s", url.PathEscape(args[0])), body)
		},
	}
	cmd.Flags().StringVar(&o.Title, "title", "", "new chat title (required)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the update (otherwise dry-run)")
	return cmd
}

// ---------- delete ----------

func newChatsDeleteCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a chat (default dry-run; use --apply to execute)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dispatchWrite(apply, "DELETE", fmt.Sprintf("/api/v1/chats/%s", url.PathEscape(args[0])), nil)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "execute the delete (otherwise dry-run)")
	return cmd
}

// ---------- messages ----------

func newChatsMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "messages", Short: "Messages within a chat"}
	cmd.AddCommand(newChatsMessagesCreateCmd())
	cmd.AddCommand(newChatsMessagesRetryCmd())
	return cmd
}

type messageCreateOpts struct {
	ChatID  string
	Content string
	Model   string
	Apply   bool
}

func buildMessageCreateBody(o messageCreateOpts) (map[string]any, error) {
	if o.ChatID == "" {
		return nil, errors.New("chat-id is required")
	}
	if strings.TrimSpace(o.Content) == "" {
		return nil, errors.New("content is required")
	}
	body := map[string]any{"content": o.Content}
	if o.Model != "" {
		body["model"] = o.Model
	}
	return body, nil
}

func newChatsMessagesCreateCmd() *cobra.Command {
	var o messageCreateOpts
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Send a user message in a chat (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			body, err := buildMessageCreateBody(o)
			if err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "POST", fmt.Sprintf("/api/v1/chats/%s/messages", url.PathEscape(o.ChatID)), body)
		},
	}
	cmd.Flags().StringVar(&o.ChatID, "chat-id", "", "chat id (required)")
	cmd.Flags().StringVar(&o.Content, "content", "", "message content (required)")
	cmd.Flags().StringVar(&o.Model, "model", "", "optional AI model identifier")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the create (otherwise dry-run)")
	return cmd
}

type messageRetryOpts struct {
	ChatID string
	Apply  bool
}

func validateMessageRetryOpts(o messageRetryOpts) error {
	if o.ChatID == "" {
		return errors.New("chat-id is required")
	}
	return nil
}

func newChatsMessagesRetryCmd() *cobra.Command {
	var o messageRetryOpts
	cmd := &cobra.Command{
		Use:   "retry",
		Short: "Retry the last assistant response in a chat (default dry-run; use --apply to execute)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := validateMessageRetryOpts(o); err != nil {
				output.Fail("validation_failed", err.Error(), nil)
				return
			}
			dispatchWrite(o.Apply, "POST", fmt.Sprintf("/api/v1/chats/%s/messages/retry", url.PathEscape(o.ChatID)), map[string]any{})
		},
	}
	cmd.Flags().StringVar(&o.ChatID, "chat-id", "", "chat id (required)")
	cmd.Flags().BoolVar(&o.Apply, "apply", false, "execute the retry (otherwise dry-run)")
	return cmd
}
