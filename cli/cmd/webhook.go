package cmd

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"kuro/cli/internal/client"
	"kuro/cli/internal/config"
)

// RunWebhook handles the `kuro webhook` command family.
// Subcommands: list, add, delete, toggle, help
func RunWebhook(args []string) {
	if len(args) < 1 {
		printWebhookUsage()
		os.Exit(1)
	}

	sub := args[0]
	subArgs := args[1:]

	switch sub {
	case "list":
		runWebhookList()
	case "add":
		runWebhookAdd(subArgs)
	case "delete":
		runWebhookDelete(subArgs)
	case "toggle":
		runWebhookToggle(subArgs)
	case "help", "--help", "-h":
		printWebhookUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown webhook subcommand %q\n", sub)
		fmt.Fprintln(os.Stderr, "Run 'kuro webhook help' for usage.")
		os.Exit(1)
	}
}

func printWebhookUsage() {
	fmt.Println(strings.TrimSpace(`
kuro webhook — Manage notification channels

Usage:
  kuro webhook list                         List all webhooks
  kuro webhook add                          Add a webhook (interactive)
  kuro webhook add <flags>                  Add a webhook (non-interactive)
    --name <name>          Display name (required)
    --url <url>            Webhook URL (required)
    --type <type>          Type: slack, discord, or telegram (required)
    --event <event>        Event: blocked, review (repeatable, default: all)
  kuro webhook delete <id>                  Delete a webhook
  kuro webhook toggle <id>                  Activate/deactivate a webhook
  kuro webhook help                         Show this help

Examples:
  kuro webhook list
  kuro webhook add
  kuro webhook add --name "Devs" --url "https://hooks.slack.com/..." --type slack --event blocked
  kuro webhook delete abc123-def456
  kuro webhook toggle abc123-def456
`))
}

// ── Helper ───────────────────────────────────────────────────────────────────

func newClient() *client.Client {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "No API key configured. Run 'kuro auth <key>' first.")
		os.Exit(1)
	}
	return client.New(cfg.APIURL, cfg.APIKey)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

// ── Scanner helpers ──────────────────────────────────────────────────────────

var scanner = bufio.NewScanner(os.Stdin)

func prompt(label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}

	if !scanner.Scan() {
		fmt.Println()
		os.Exit(0)
	}

	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return def
	}
	return val
}

func promptRequired(label string) string {
	for {
		val := prompt(label, "")
		if val != "" {
			return val
		}
		fmt.Println("  Este campo es obligatorio.")
	}
}

// ── List ─────────────────────────────────────────────────────────────────────

func runWebhookList() {
	cl := newClient()

	webhooks, err := cl.ListWebhooks()
	if err != nil {
		fatal(err)
	}

	if len(webhooks) == 0 {
		fmt.Println("No webhooks configured.")
		fmt.Println("Add one with: kuro webhook add")
		return
	}

	fmt.Println("Notification channels:")
	for _, wh := range webhooks {
		status := "✅ active"
		if !wh.Active {
			status = "⏸ inactive"
		}
		id := wh.ID
		if len(id) > 8 {
			id = id[:8]
		}
		fmt.Printf("  %s  %s\n", id, wh.Name)
		fmt.Printf("      Type: %s   Events: %s   %s\n", wh.Type, strings.Join(wh.Events, ", "), status)
	}
}

// ── Add (interactive + non-interactive) ──────────────────────────────────────

func runWebhookAdd(args []string) {
	if len(args) > 0 && strings.HasPrefix(args[0], "--") {
		addWithFlags(args)
	} else {
		addInteractive()
	}
}

func addWithFlags(args []string) {
	fs := flag.NewFlagSet("webhook add", flag.ContinueOnError)
	name := fs.String("name", "", "Display name for the webhook")
	url := fs.String("url", "", "Webhook URL")
	whType := fs.String("type", "", "Type: slack, discord, or telegram")
	events := fs.String("event", "", "Event (blocked, review). Repeatable: --event blocked --event review")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *name == "" || *url == "" || *whType == "" {
		fmt.Fprintln(os.Stderr, "Usage: kuro webhook add --name <name> --url <url> --type <type> [--event <event>]")
		os.Exit(1)
	}

	eventList := []string{"blocked", "review"}
	if *events != "" {
		eventList = strings.Split(*events, ",")
		for i := range eventList {
			eventList[i] = strings.TrimSpace(eventList[i])
		}
	}

	cl := newClient()
	wh, err := cl.CreateWebhook(client.WebhookInput{
		Name:   *name,
		URL:    *url,
		Type:   *whType,
		Events: eventList,
	})
	if err != nil {
		fatal(err)
	}

	fmt.Printf("✅ Webhook created: %s (%s, %s)\n", wh.Name, wh.Type, strings.Join(wh.Events, ", "))
	fmt.Printf("   ID: %s\n", wh.ID)
}

func addInteractive() {
	fmt.Println()
	fmt.Println("Add a new notification channel")
	fmt.Println(strings.Repeat("─", 40))

	// Basic info
	name := promptRequired("Name")
	whType := promptRequired("Type (slack, discord, telegram)")

	// Validate type
	validTypes := map[string]bool{"slack": true, "discord": true, "telegram": true}
	for !validTypes[whType] {
		fmt.Printf("  Invalid type %q. Must be slack, discord, or telegram.\n", whType)
		whType = promptRequired("Type (slack, discord, telegram)")
	}

	// URL (Slack/Discord) o Token + Chat ID (Telegram)
	input := client.WebhookInput{
		Name:   name,
		Type:   whType,
		Events: nil, // set below
	}

	switch whType {
	case "telegram":
		token := promptRequired("Bot token (from @BotFather)")
		chatID := promptRequired("Chat ID (from @userinfobot or group info)")
		input.URL = "https://api.telegram.org/bot" + token + "/sendMessage"
		input.Metadata = []byte(fmt.Sprintf(`{"chat_id":%q}`, chatID))
	default:
		input.URL = promptRequired("Webhook URL")
	}

	// Events
	eventsStr := prompt("Events (comma-separated, Enter = blocked,review)", "blocked,review")
	eventList := strings.Split(eventsStr, ",")
	for i := range eventList {
		eventList[i] = strings.TrimSpace(eventList[i])
	}
	input.Events = eventList

	// Summary
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Name:    %s\n", input.Name)
	fmt.Printf("  Type:    %s\n", input.Type)
	if whType == "telegram" {
		fmt.Printf("  Bot URL: %s\n", input.URL)
		var meta struct{ ChatID string `json:"chat_id"` }
		json.Unmarshal(input.Metadata, &meta)
		fmt.Printf("  Chat ID: %s\n", meta.ChatID)
	} else {
		fmt.Printf("  URL:     %s\n", input.URL)
	}
	fmt.Printf("  Events:  %s\n", strings.Join(input.Events, ", "))

	confirm := prompt("Create? (Y/n)", "Y")
	if confirm != "Y" && confirm != "y" && confirm != "" {
		fmt.Println("Cancelled.")
		return
	}

	cl := newClient()
	wh, err := cl.CreateWebhook(input)
	if err != nil {
		fatal(err)
	}

	fmt.Printf("\n✅ Webhook created: %s (%s)\n", wh.Name, wh.Type)
	fmt.Printf("   ID: %s\n", wh.ID)
}

// ── Delete ───────────────────────────────────────────────────────────────────

func runWebhookDelete(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kuro webhook delete <id>")
		os.Exit(1)
	}

	id := args[0]
	cl := newClient()

	if err := cl.DeleteWebhook(id); err != nil {
		fatal(err)
	}

	fmt.Printf("Webhook %s deleted.\n", id)
}

// ── Toggle ───────────────────────────────────────────────────────────────────

func runWebhookToggle(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: kuro webhook toggle <id>")
		os.Exit(1)
	}

	id := args[0]
	cl := newClient()

	active, err := cl.ToggleWebhook(id)
	if err != nil {
		fatal(err)
	}

	status := "✅ active"
	if !active {
		status = "⏸ inactive"
	}
	fmt.Printf("Webhook %s is now %s\n", id, status)
}
