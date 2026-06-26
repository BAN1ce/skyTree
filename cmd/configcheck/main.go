package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BAN1ce/skyTree/config"
)

func main() {
	pattern := flag.String("pattern", "etc/*.yaml", "glob pattern for config files")
	profile := flag.String("profile", "", "optional validation profile (for example: beta)")
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		matched, err := filepath.Glob(*pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid pattern %q: %v\n", *pattern, err)
			os.Exit(2)
		}
		files = matched
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no config files found\n")
		os.Exit(2)
	}

	sort.Strings(files)
	failed := false
	for _, file := range files {
		cfg, err := config.Load(file)
		if err != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", file, err)
			continue
		}
		if *profile != "" {
			if err := config.ValidateForProfile(cfg, config.ValidationProfile(*profile)); err != nil {
				failed = true
				fmt.Fprintf(os.Stderr, "[FAIL] %s: %v\n", file, err)
				continue
			}
		}
		fmt.Printf("[OK]   %s\n", file)
	}

	if failed {
		os.Exit(1)
	}
}
