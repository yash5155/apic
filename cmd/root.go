package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/yash5155/apic/internal/httpx"
	"github.com/yash5155/apic/internal/spec"
	"github.com/yash5155/apic/internal/ui"
)

// version is stamped at build time with -ldflags "-X .../cmd.version=vX.Y.Z".
var version = "dev"

var (
	serverURL   string
	listOnly    bool
	headerFlags []string
	timeout     time.Duration
	insecure    bool
	maxBodyMB   int
)

var rootCmd = &cobra.Command{
	Use:   "apic <spec-file-or-url>",
	Short: "Browse and call an OpenAPI/Swagger service from the terminal",
	Long: `apic reads an OpenAPI 3.x or Swagger 2.0 document (JSON or YAML, local
file or http(s) URL), lists its endpoints, and builds an input form from each
endpoint's declared parameters so you can call it interactively.

Examples:
  apic openapi.json
  apic https://api.example.com/openapi.json --server https://api.example.com
  apic swagger2.yaml -H "Authorization: Bearer $TOKEN"
  apic openapi.json --list`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	Version:      version,
	RunE:         run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	f := rootCmd.Flags()
	f.StringVar(&serverURL, "server", "",
		"base URL to call (defaults to the first server in the spec)")
	f.BoolVar(&listOnly, "list", false,
		"print the endpoints and exit, no TUI")
	f.StringArrayVarP(&headerFlags, "header", "H", nil,
		"header sent with every request, e.g. -H \"Authorization: Bearer TOKEN\" (repeatable)")
	f.DurationVar(&timeout, "timeout", 30*time.Second,
		"per-request timeout")
	f.BoolVarP(&insecure, "insecure", "k", false,
		"skip TLS certificate verification (use only for trusted self-signed hosts)")
	f.IntVar(&maxBodyMB, "max-body", 2,
		"maximum response body to read, in megabytes")
}

func run(cmd *cobra.Command, args []string) error {
	baseHeaders, err := parseHeaders(headerFlags)
	if err != nil {
		return err
	}

	if maxBodyMB < 1 {
		return fmt.Errorf("--max-body must be at least 1")
	}

	// Apply transport + fetch configuration before any request goes out.
	spec.FetchTimeout = timeout
	spec.InsecureTLS = insecure
	httpx.Configure(httpx.Config{
		Timeout:      timeout,
		MaxBodyBytes: int64(maxBodyMB) << 20,
		InsecureTLS:  insecure,
	})

	api, err := spec.Load(args[0])
	if err != nil {
		return err
	}
	if len(api.Endpoints) == 0 {
		return fmt.Errorf("no endpoints found in %s (if this is a Swagger UI page, point at the raw spec URL instead)", args[0])
	}

	base := serverURL
	if base == "" && len(api.Servers) > 0 {
		base = api.Servers[0]
	}
	if base == "" {
		return fmt.Errorf("spec declares no server; pass --server https://api.example.com")
	}

	// --list is handy for scripting and for checking the parser without
	// opening the UI at all.
	if listOnly {
		fmt.Printf("%s %s (%s)\n\n", api.Title, api.Version, base)
		for _, ep := range api.Endpoints {
			fmt.Printf("%-6s %-40s %s\n", ep.Method, ep.Path, ep.Summary)
		}
		return nil
	}

	_, err = tea.NewProgram(
		ui.New(api, base, baseHeaders),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	).Run()
	return err
}

// parseHeaders turns repeated "Name: value" flags into a map. It is strict
// about the format so a typo surfaces immediately instead of silently
// dropping an auth header.
func parseHeaders(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raw))
	for _, h := range raw {
		name, value, ok := strings.Cut(h, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --header %q (want \"Name: value\")", h)
		}
		out[name] = strings.TrimSpace(value)
	}
	return out, nil
}
