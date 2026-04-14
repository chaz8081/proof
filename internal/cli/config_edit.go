// internal/cli/config_edit.go
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaz8081/proof/internal/config"
	"github.com/chaz8081/proof/internal/review"
	"github.com/spf13/cobra"
)

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Interactively edit configuration",
		Example: `  proof config edit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := filepath.Join(config.ConfigDir(), "config.yaml")

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("no config found — run 'proof setup' to create one")
			}

			cmd.Printf("\nCurrent config: %s\n", cfgPath)

			reader := bufio.NewReader(os.Stdin)

			// ── Repos ──────────────────────────────────────────────────────────
			cmd.Println("\n=== Repos ===")
			cmd.Printf("  Current: %s\n", strings.Join(cfg.RepoNames(), ", "))
			cmd.Print("\n  ? Edit repos? (y/N): ")
			if promptYesNo(os.Stdin, false) {
				cfg.Repos = editRepos(cmd, reader, cfg.Repos)
				updated := cfg.RepoNames()
				cmd.Printf("\n  Updated repos: %s\n", strings.Join(updated, ", "))
			}

			// ── Auth ───────────────────────────────────────────────────────────
			cmd.Println("\n=== Auth ===")
			cmd.Printf("  Current: reviewer=%s, copilot=%s\n",
				orDefault(cfg.Auth.Reviewer, "(default)"),
				orDefault(cfg.Auth.Copilot, "(default)"),
			)
			cmd.Print("\n  ? Edit auth? (y/N): ")
			if promptYesNo(os.Stdin, false) {
				accounts, _ := listGHAccounts()
				if len(accounts) > 1 {
					cmd.Println()
					cmd.Println("  GitHub accounts detected:")
					for i, a := range accounts {
						cmd.Printf("    %d. %s\n", i+1, a)
					}

					cmd.Printf("\n  ? Which account has Copilot? [1]: ")
					copilotInput, _ := reader.ReadString('\n')
					copilotIdx := parseIndex(copilotInput, len(accounts))
					newCopilot := accounts[copilotIdx]

					cmd.Printf("  ? Which account should post reviews? [1]: ")
					reviewerInput, _ := reader.ReadString('\n')
					reviewerIdx := parseIndex(reviewerInput, len(accounts))
					newReviewer := accounts[reviewerIdx]

					if newCopilot == newReviewer {
						cfg.Auth.Copilot = ""
						cfg.Auth.Reviewer = ""
					} else {
						cfg.Auth.Copilot = newCopilot
						cfg.Auth.Reviewer = newReviewer
					}
				} else if len(accounts) == 1 {
					cmd.Printf("  Only one account detected (%s) — nothing to change.\n", accounts[0])
				} else {
					cmd.Println("  No gh accounts found — run 'gh auth login' first.")
				}
			}

			// ── Review ─────────────────────────────────────────────────────────
			cmd.Println("\n=== Review ===")
			cmd.Printf("  Model: %s\n", cfg.Review.Model)
			cmd.Printf("  Default verdict: %s\n", cfg.Review.DefaultVerdict)
			cmd.Print("\n  ? Edit review settings? (y/N): ")
			if promptYesNo(os.Stdin, false) {
				// Model picker
				copilotToken := resolveCopilotToken(cfg, "")
				models, modelsErr := review.ListModels(cmd.Context(), copilotToken)
				if modelsErr == nil && len(models) > 0 {
					cmd.Println("\n  Available models:")
					currentModel := cfg.Review.Model
					for i, m := range models {
						marker := ""
						if m.ID == currentModel {
							marker = " ✓ (current)"
						}
						name := m.Name
						if name == "" {
							name = m.ID
						}
						cmd.Printf("    %d. %s%s\n", i+1, name, marker)
					}
					cmd.Printf("  ? Select model [1]: ")
					modelInput, _ := reader.ReadString('\n')
					modelIdx := parseIndex(modelInput, len(models))
					cfg.Review.Model = models[modelIdx].ID
				} else {
					cmd.Printf("\n  ? AI model [%s]: ", cfg.Review.Model)
					modelInput, _ := reader.ReadString('\n')
					if m := strings.TrimSpace(modelInput); m != "" {
						cfg.Review.Model = m
					}
				}

				// Verdict picker
				cmd.Printf("\n  ? Default verdict (COMMENT/APPROVE/REQUEST_CHANGES) [%s]: ", cfg.Review.DefaultVerdict)
				verdictInput, _ := reader.ReadString('\n')
				verdict := strings.TrimSpace(strings.ToUpper(verdictInput))
				if verdict != "" {
					cfg.Review.DefaultVerdict = verdict
				}
			}

			// ── Poll ───────────────────────────────────────────────────────────
			cmd.Println("\n=== Poll ===")
			ignoreDrafts := cfg.Poll.IgnoreDrafts != nil && *cfg.Poll.IgnoreDrafts
			cmd.Printf("  ignore_drafts: %v, ignore_wip: %v, include_own: %v\n",
				ignoreDrafts, cfg.Poll.IgnoreWIP, cfg.Poll.IncludeOwn)
			cmd.Print("\n  ? Edit poll settings? (y/N): ")
			if promptYesNo(os.Stdin, false) {
				ignoreDrafts = editBool(reader, "ignore_drafts", ignoreDrafts)
				b := ignoreDrafts
				cfg.Poll.IgnoreDrafts = &b
				cfg.Poll.IgnoreWIP = editBool(reader, "ignore_wip", cfg.Poll.IgnoreWIP)
				cfg.Poll.IncludeOwn = editBool(reader, "include_own", cfg.Poll.IncludeOwn)
			}

			// ── Write config ───────────────────────────────────────────────────
			cfgContent := marshalConfig(cfg)
			if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			cmd.Printf("\n✓ Config updated at %s\n", cfgPath)
			return nil
		},
	}
}

// editRepos runs the interactive add/remove/search loop for the repos section.
func editRepos(cmd *cobra.Command, reader *bufio.Reader, current []config.RepoEntry) []config.RepoEntry {
	repos := make([]config.RepoEntry, len(current))
	copy(repos, current)

	for {
		cmd.Println()
		cmd.Println("  Current repos:")
		for i, r := range repos {
			cmd.Printf("    %d. %s\n", i+1, r.Name)
		}

		cmd.Print("\n  ? (a)dd / (r)emove / (s)earch / (d)one [done]: ")
		line, _ := reader.ReadString('\n')
		action := strings.ToLower(strings.TrimSpace(line))

		switch action {
		case "a", "add":
			cmd.Print("  ? Enter repo (owner/repo): ")
			input, _ := reader.ReadString('\n')
			repo := strings.TrimSpace(input)
			if repo != "" {
				if !repoExists(repos, repo) {
					repos = append(repos, config.RepoEntry{Name: repo})
					cmd.Printf("  Added %s.\n", repo)
				} else {
					cmd.Printf("  %s is already in the list.\n", repo)
				}
			}

		case "s", "search":
			cmd.Print("  ? Search: ")
			input, _ := reader.ReadString('\n')
			query := strings.TrimSpace(input)
			if query == "" {
				continue
			}
			results, err := searchGitHubRepos(query)
			if err != nil || len(results) == 0 {
				cmd.Println("  No results found.")
				continue
			}
			cmd.Println()
			for i, r := range results {
				marker := ""
				if repoExists(repos, r) {
					marker = " (already added)"
				}
				cmd.Printf("    %d. %s%s\n", i+1, r, marker)
			}
			cmd.Printf("  ? Add (comma-separated numbers, e.g. 1,3): ")
			selInput, _ := reader.ReadString('\n')
			for _, part := range strings.Split(selInput, ",") {
				idx := parseIndex(strings.TrimSpace(part)+"\n", len(results))
				r := results[idx]
				// parseIndex defaults to 0 on bad input — only add if user picked something real
				if strings.TrimSpace(part) == "" {
					continue
				}
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil || n < 1 || n > len(results) {
					continue
				}
				r = results[n-1]
				if !repoExists(repos, r) {
					repos = append(repos, config.RepoEntry{Name: r})
					cmd.Printf("  Added %s.\n", r)
				} else {
					cmd.Printf("  %s already added.\n", r)
				}
			}

		case "r", "remove":
			if len(repos) == 0 {
				cmd.Println("  No repos to remove.")
				continue
			}
			cmd.Printf("  ? Remove (comma-separated numbers 1-%d): ", len(repos))
			selInput, _ := reader.ReadString('\n')
			toRemove := map[int]bool{}
			for _, part := range strings.Split(selInput, ",") {
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil || n < 1 || n > len(repos) {
					continue
				}
				toRemove[n-1] = true
			}
			var updated []config.RepoEntry
			for i, r := range repos {
				if !toRemove[i] {
					updated = append(updated, r)
				} else {
					cmd.Printf("  Removed %s.\n", r.Name)
				}
			}
			repos = updated

		case "", "d", "done":
			return repos

		default:
			cmd.Printf("  Unknown action %q — use a, r, s, or d.\n", action)
		}
	}
}

// editBool shows the current boolean value and prompts to toggle it.
func editBool(reader *bufio.Reader, name string, current bool) bool {
	defaultLabel := "y"
	if !current {
		defaultLabel = "n"
	}
	fmt.Printf("  ? %s (y/N) [%s]: ", name, defaultLabel)
	line, _ := reader.ReadString('\n')
	input := strings.TrimSpace(strings.ToLower(line))
	if input == "" {
		return current
	}
	return input == "y" || input == "yes"
}

// orDefault returns s if non-empty, otherwise def.
func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// repoExists checks whether name is already in the list.
func repoExists(repos []config.RepoEntry, name string) bool {
	for _, r := range repos {
		if r.Name == name {
			return true
		}
	}
	return false
}

// searchGitHubRepos searches for repos matching a query via gh CLI.
func searchGitHubRepos(query string) ([]string, error) {
	cmd := exec.Command("gh", "search", "repos", query,
		"--json", "fullName",
		"--jq", ".[].fullName",
		"--limit", "10",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos, nil
}

// listUserRepos lists repos owned by username via gh CLI.
func listUserRepos(username string) ([]string, error) {
	cmd := exec.Command("gh", "repo", "list", username,
		"--json", "nameWithOwner",
		"--jq", ".[].nameWithOwner",
		"--limit", "30",
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var repos []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			repos = append(repos, line)
		}
	}
	return repos, nil
}

// marshalConfig serialises the config back to YAML text.
func marshalConfig(cfg *config.Config) string {
	var b strings.Builder

	b.WriteString("# Proof configuration\n\n")

	b.WriteString("repos:\n")
	for _, r := range cfg.Repos {
		if r.Instructions != "" {
			fmt.Fprintf(&b, "  - name: %s\n    instructions: %s\n", r.Name, r.Instructions)
		} else {
			fmt.Fprintf(&b, "  - %s\n", r.Name)
		}
	}

	b.WriteString("\npoll:\n")
	if cfg.Poll.IgnoreDrafts != nil {
		fmt.Fprintf(&b, "  ignore_drafts: %v\n", *cfg.Poll.IgnoreDrafts)
	}
	fmt.Fprintf(&b, "  ignore_wip: %v\n", cfg.Poll.IgnoreWIP)
	if cfg.Poll.IncludeOwn {
		b.WriteString("  include_own: true\n")
	}
	if cfg.Poll.MaxFiles > 0 {
		fmt.Fprintf(&b, "  max_files: %d\n", cfg.Poll.MaxFiles)
	}
	if cfg.Poll.MaxDiffBytes > 0 {
		fmt.Fprintf(&b, "  max_diff_bytes: %d\n", cfg.Poll.MaxDiffBytes)
	}

	b.WriteString("\nreview:\n")
	fmt.Fprintf(&b, "  default_verdict: %s\n", cfg.Review.DefaultVerdict)
	fmt.Fprintf(&b, "  model: %s\n", cfg.Review.Model)
	if cfg.Review.Instructions != "" {
		fmt.Fprintf(&b, "  instructions: %s\n", cfg.Review.Instructions)
	}

	if cfg.Auth.Reviewer != "" || cfg.Auth.Copilot != "" {
		b.WriteString("\nauth:\n")
		if cfg.Auth.Reviewer != "" {
			fmt.Fprintf(&b, "  reviewer: %s\n", cfg.Auth.Reviewer)
		}
		if cfg.Auth.Copilot != "" {
			fmt.Fprintf(&b, "  copilot: %s\n", cfg.Auth.Copilot)
		}
	}

	return b.String()
}
