package root

import (
	"strings"
	"testing"
)

// ---------- builder unit tests ----------

func TestBuildChatCreateBody_RequiresTitle(t *testing.T) {
	if _, err := buildChatCreateBody(chatCreateOpts{}); err == nil {
		t.Fatal("expected missing title to error")
	}
}

func TestBuildChatCreateBody_TitleOnly(t *testing.T) {
	body, err := buildChatCreateBody(chatCreateOpts{Title: "Hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["title"] != "Hello" {
		t.Fatalf("title = %v", body["title"])
	}
	if _, ok := body["message"]; ok {
		t.Fatal("message should be omitted when empty")
	}
	if _, ok := body["model"]; ok {
		t.Fatal("model should be omitted when empty")
	}
}

func TestBuildChatCreateBody_AllFields(t *testing.T) {
	body, err := buildChatCreateBody(chatCreateOpts{Title: "T", Message: "hi", Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["title"] != "T" || body["message"] != "hi" || body["model"] != "gpt-4o" {
		t.Fatalf("body = %#v", body)
	}
}

func TestBuildChatCreateBody_RejectsWhitespaceTitle(t *testing.T) {
	if _, err := buildChatCreateBody(chatCreateOpts{Title: "   "}); err == nil {
		t.Fatal("expected whitespace-only title to be rejected (upstream validates presence after strip)")
	}
}

func TestBuildChatUpdateBody_RequiresTitle(t *testing.T) {
	if _, err := buildChatUpdateBody(chatUpdateOpts{}); err == nil {
		t.Fatal("expected missing title to error")
	}
}

func TestBuildChatUpdateBody_RejectsWhitespaceTitle(t *testing.T) {
	if _, err := buildChatUpdateBody(chatUpdateOpts{Title: "\t\n"}); err == nil {
		t.Fatal("expected whitespace-only title to be rejected")
	}
}

func TestBuildChatUpdateBody_OK(t *testing.T) {
	body, err := buildChatUpdateBody(chatUpdateOpts{Title: "renamed"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["title"] != "renamed" {
		t.Fatalf("title = %v", body["title"])
	}
}

func TestBuildMessageCreateBody_RequiresChatID(t *testing.T) {
	if _, err := buildMessageCreateBody(messageCreateOpts{Content: "hi"}); err == nil {
		t.Fatal("expected missing chat-id to error")
	}
}

func TestBuildMessageCreateBody_RequiresContent(t *testing.T) {
	if _, err := buildMessageCreateBody(messageCreateOpts{ChatID: "c1"}); err == nil {
		t.Fatal("expected missing content to error")
	}
}

func TestBuildMessageCreateBody_OptionalModel(t *testing.T) {
	body, err := buildMessageCreateBody(messageCreateOpts{ChatID: "c1", Content: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["content"] != "hello" {
		t.Fatalf("content = %v", body["content"])
	}
	if _, ok := body["model"]; ok {
		t.Fatal("model should be omitted when empty")
	}
}

func TestBuildMessageCreateBody_WithModel(t *testing.T) {
	body, err := buildMessageCreateBody(messageCreateOpts{ChatID: "c1", Content: "hi", Model: "claude"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["model"] != "claude" {
		t.Fatalf("model = %v", body["model"])
	}
}

func TestBuildMessageRetry_RequiresChatID(t *testing.T) {
	if err := validateMessageRetryOpts(messageRetryOpts{}); err == nil {
		t.Fatal("expected missing chat-id to error")
	}
}

// ---------- command-shape tests ----------

func TestChatsCommandShape(t *testing.T) {
	cmd := newChatsCmd()
	if cmd.Use != "chats" {
		t.Fatalf("Use = %q", cmd.Use)
	}

	list := findSub(t, cmd, "list")
	if list.Flags().Lookup("page") == nil {
		t.Fatal("chats list missing --page")
	}
	// Upstream uses a FIXED items: 20 — there is no per_page; do NOT expose --per-page.
	if list.Flags().Lookup("per-page") != nil {
		t.Fatal("chats list must not expose --per-page (upstream ignores it)")
	}

	show := findSub(t, cmd, "show")
	if show.Args == nil {
		t.Fatal("chats show should require an id")
	}
	if show.Flags().Lookup("page") == nil {
		t.Fatal("chats show missing --page for messages paging")
	}

	create := findSub(t, cmd, "create")
	for _, f := range []string{"title", "message", "model", "apply"} {
		if create.Flags().Lookup(f) == nil {
			t.Fatalf("chats create missing --%s", f)
		}
	}

	update := findSub(t, cmd, "update")
	for _, f := range []string{"title", "apply"} {
		if update.Flags().Lookup(f) == nil {
			t.Fatalf("chats update missing --%s", f)
		}
	}
	if update.Args == nil {
		t.Fatal("chats update should require an id")
	}

	del := findSub(t, cmd, "delete")
	if del.Flags().Lookup("apply") == nil {
		t.Fatal("chats delete missing --apply")
	}
	if del.Args == nil {
		t.Fatal("chats delete should require an id")
	}

	msgs := findSub(t, cmd, "messages")
	if msgs == nil {
		t.Fatal("chats messages subtree missing")
	}
	msgCreate := findSub(t, msgs, "create")
	for _, f := range []string{"chat-id", "content", "model", "apply"} {
		if msgCreate.Flags().Lookup(f) == nil {
			t.Fatalf("chats messages create missing --%s", f)
		}
	}
	msgRetry := findSub(t, msgs, "retry")
	for _, f := range []string{"chat-id", "apply"} {
		if msgRetry.Flags().Lookup(f) == nil {
			t.Fatalf("chats messages retry missing --%s", f)
		}
	}
}

func TestChatsRegistered(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		want string
	}{
		{[]string{"chats"}, "chats"},
		{[]string{"chats", "list"}, "list"},
		{[]string{"chats", "show"}, "show"},
		{[]string{"chats", "create"}, "create"},
		{[]string{"chats", "update"}, "update"},
		{[]string{"chats", "delete"}, "delete"},
		{[]string{"chats", "messages", "create"}, "create"},
		{[]string{"chats", "messages", "retry"}, "retry"},
	}
	for _, c := range cases {
		got, _, err := root.Find(c.path)
		if err != nil {
			t.Fatalf("path %v not registered: %v", c.path, err)
		}
		if got.Name() != c.want {
			t.Fatalf("path %v resolved to %q, want %q", c.path, got.Name(), c.want)
		}
	}
}

// Defense in depth: the upstream short description should mention the AI-enabled requirement
// so users get a hint before hitting a 403.
func TestChatsCommand_HelpMentionsAIEnabled(t *testing.T) {
	short := strings.ToLower(newChatsCmd().Short)
	if !strings.Contains(short, "ai") {
		t.Fatalf("chats short description should mention AI requirement, got: %q", short)
	}
}
