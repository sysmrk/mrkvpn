// Command mrkvpn validates and renders gateway configuration.
// No command here starts WireGuard or modifies networking settings.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sysmrk/mrkvpn/internal/wgconfig"
)

const maxConfigBytes = 1 << 20

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mrkvpn:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) < 2 || args[0] != "gateway" || (args[1] != "check" && args[1] != "render") {
		return errors.New("usage: mrkvpn gateway check|render -config gateway.json [-out wg0.conf]")
	}

	action := args[1]
	fs := flag.NewFlagSet("gateway "+action, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "path to gateway JSON")
	outputPath := fs.String("out", "", "path to new wg-quick config")
	if err := fs.Parse(args[2:]); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	if fs.NArg() != 0 || *configPath == "" {
		return errors.New("-config required and no positional arguments allowed")
	}
	if action == "render" && *outputPath == "" {
		return errors.New("gateway render requires -out")
	}
	if action == "check" && *outputPath != "" {
		return errors.New("gateway check must not specify -out")
	}
	config, err := readConfig(*configPath)
	if err != nil {
		return err
	}

	// Rendering into memory is also validation. The key is never printed.
	content, err := wgconfig.Render(config)
	if err != nil {
		return err
	}
	if action == "check" {
		_, err = fmt.Fprintf(stdout, "valid gateway configuration for %s (%d peers); no system changes made\n", config.InterfaceName, len(config.Peers))
		return err
	}
	if err := wgconfig.WriteNew(*outputPath, content); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "created %s (0600); no VPN service started\n", *outputPath)
	return err
}

func readConfig(path string) (wgconfig.Gateway, error) {
	var result wgconfig.Gateway
	f, err := os.Open(path)
	if err != nil {
		return result, fmt.Errorf("cannot open gateway JSON: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return result, fmt.Errorf("cannot inspect gateway JSON: %w", err)
	}
	if !fi.Mode().IsRegular() || fi.Size() > maxConfigBytes {
		return result, errors.New("gateway JSON must be a regular file not exceeding 1 MiB")
	}
	dec := json.NewDecoder(io.LimitReader(f, maxConfigBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return result, fmt.Errorf("invalid gateway JSON: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return result, errors.New("gateway JSON must contain only one object")
		}
		return result, fmt.Errorf("trailing gateway JSON: %w", err)
	}
	return result, nil
}
