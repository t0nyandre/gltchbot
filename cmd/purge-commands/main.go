package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/t0nyandre/gltchbot/internal/config"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

func main() {
	// Define command-line flags
	var (
		guildID = flag.String("guild", "", "Guild ID to purge commands from (optional)")
		global  = flag.Bool("global", false, "Purge global commands")
		all     = flag.Bool("all", false, "Purge all commands (global + all guilds)")
		dryRun  = flag.Bool("dry-run", false, "Show what would be deleted without actually deleting")
		force   = flag.Bool("force", false, "Skip confirmation prompts")
		help    = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Validate flags
	if !*global && !*all && *guildID == "" {
		logging.Fatal("Error: You must specify at least one of --guild, --global, or --all. Use --help for usage.")
	}

	if *all && (*global || *guildID != "") {
		logging.Fatal("Error: --all cannot be used with --guild or --global. Use --help for usage.")
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logging.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DiscordToken == "" {
		logging.Fatal("Error: DISCORD_TOKEN environment variable is required")
	}
	if cfg.DiscordAppID == "" {
		logging.Fatal("Error: DISCORD_APP_ID environment variable is required")
	}

	// Create Discord session
	s, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		logging.Fatalf("Failed to create Discord session: %v", err)
	}
	defer s.Close()

	// Open session (needed for API calls)
	if err := s.Open(); err != nil {
		logging.Fatalf("Failed to open Discord session: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// Determine what to purge
	if *all {
		purgeAllCommands(ctx, s, cfg.DiscordAppID, *dryRun, *force)
	} else if *global {
		purgeGlobalCommands(ctx, s, cfg.DiscordAppID, *dryRun, *force)
	} else if *guildID != "" {
		purgeGuildCommands(ctx, s, cfg.DiscordAppID, *guildID, *dryRun, *force)
	}
}

func purgeAllCommands(ctx context.Context, s *discordgo.Session, appID string, dryRun, force bool) {
	logging.Info("Purging ALL commands (global + all guilds)...")

	// Get all guilds the bot is in
	guilds, err := s.UserGuilds(100, "", "", false)
	if err != nil {
		logging.Fatalf("Failed to fetch guilds: %v", err)
	}

	// Purge global commands first
	purgeGlobalCommands(ctx, s, appID, dryRun, true) // force=true since we already confirmed

	// Purge commands from each guild
	for _, guild := range guilds {
		logging.Info("Purging commands from guild", "guild_name", guild.Name, "guild_id", guild.ID)
		purgeGuildCommands(ctx, s, appID, guild.ID, dryRun, true) // force=true
	}

	if !dryRun {
		logging.Info("✅ All commands purged successfully!")
	} else {
		logging.Info("✅ Dry run completed. No commands were actually deleted.")
	}
}

func purgeGlobalCommands(ctx context.Context, s *discordgo.Session, appID string, dryRun, force bool) {
	logging.Info("Purging global commands...")
	purgeCommands(ctx, s, appID, "", dryRun, force, "global")
}

func purgeGuildCommands(ctx context.Context, s *discordgo.Session, appID, guildID string, dryRun, force bool) {
	// Get guild name for better logging
	guild, err := s.Guild(guildID)
	guildName := guildID
	if err == nil && guild != nil {
		guildName = fmt.Sprintf("%s (%s)", guild.Name, guildID)
	}

	logging.Info("Purging commands from guild", "guild", guildName)
	purgeCommands(ctx, s, appID, guildID, dryRun, force, "guild")
}

func purgeCommands(ctx context.Context, s *discordgo.Session, appID, guildID string, dryRun, force bool, scope string) {
	// Fetch existing commands
	commands, err := s.ApplicationCommands(appID, guildID)
	if err != nil {
		logging.Fatalf("Failed to fetch %s commands: %v", scope, err)
	}

	if len(commands) == 0 {
		logging.Info("No commands found", "scope", scope)
		return
	}

	// Show what will be deleted
	logging.Info("Found commands", "command_count", len(commands), "scope", scope)
	for _, cmd := range commands {
		logging.Info("Command", "command_name", cmd.Name, "command_id", cmd.ID)
	}

	// Ask for confirmation unless force flag is set
	if !force && !dryRun {
		if !askForConfirmation(fmt.Sprintf("Delete %d %s command(s)?", len(commands), scope)) {
			logging.Info("Operation cancelled.")
			return
		}
	}

	// Delete commands
	if dryRun {
		logging.Info("Dry run: Would delete commands", "command_count", len(commands), "scope", scope)
		return
	}

	deletedCount := 0
	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(appID, guildID, cmd.ID)
		if err != nil {
			logging.Error("Failed to delete command", "command_name", cmd.Name, "error", err)
		} else {
			deletedCount++
			logging.Info("Deleted command", "command_name", cmd.Name)
		}
	}

	logging.Info("Deleted commands", "deleted_count", deletedCount, "total_count", len(commands), "scope", scope)
}

func askForConfirmation(question string) bool {
	fmt.Printf("%s [y/N]: ", question)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func printHelp() {
	fmt.Println(`Purge Discord slash commands using the Discord API.

Usage:
  go run ./cmd/purge-commands [flags]

Flags:
  --guild string   Guild ID to purge commands from (optional)
  --global         Purge global commands
  --all            Purge all commands (global + all guilds)
  --dry-run        Show what would be deleted without actually deleting
  --force          Skip confirmation prompts
  --help           Show this help message

Examples:
  # Show what would be deleted from global commands
  go run ./cmd/purge-commands --global --dry-run

  # Delete commands from a specific guild
  go run ./cmd/purge-commands --guild 123456789012345678 --force

  # Delete global commands (with confirmation)
  go run ./cmd/purge-commands --global

  # Delete all commands from all guilds and global (use with caution!)
  go run ./cmd/purge-commands --all --force

Environment variables required:
  DISCORD_TOKEN    Your bot token
  DISCORD_APP_ID   Your application ID
  DISCORD_DEV_GUILD_ID (optional) Default dev guild ID from config

Note: The --all flag will purge commands from ALL guilds the bot is in,
which may include production servers. Use with extreme caution!`)
}
