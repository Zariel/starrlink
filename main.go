package main

import (
	"cmp"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golift.io/starr"
	"golift.io/starr/sonarr"
)

func deref[T any](v *T) T {
	if v == nil {
		return *new(T)
	}
	return *v
}

type sonarrConf struct {
	url string
	key string
}

func require[T comparable](param, envar string, val T) {
	var z T
	if val != z {
		return
	}

	fmt.Printf("starrlink: required parameter not provided; flag %q or envar %q must be set\n", param, envar)
	os.Exit(2)
}

type cmd interface {
	Exec() error
	Flags(fs *flag.FlagSet)
}

type sonarrCmd struct {
	url string
	key string

	series int64
}

var globals struct {
	mapFrom string
	mapTo   string
	dst     string
}

func mapPath(p string) string {
	if globals.mapFrom == "" || globals.mapTo == "" {
		return p
	}

	p = strings.TrimPrefix(p, globals.mapFrom)
	return filepath.Join(globals.mapTo, p)
}

func globalFlags(fs *flag.FlagSet) {
	fs.StringVar(&globals.mapFrom, "map-from", "", "map `from` the path stored, eg when the data is mounted in a different location")
	fs.StringVar(&globals.mapTo, "map-to", "", "map `to` the path stored, eg when the data is mounted in a different location")

	pwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("starrlinker: unable to get wd: %v", err)
		os.Exit(1)
	}

	fs.StringVar(&globals.dst, "dest", pwd, "the `destination` to write the links to")
}

func (s *sonarrCmd) Flags(fs *flag.FlagSet) {
	fs.StringVar(&s.url, "sonarr-url", "", "the sonarr `url`")
	fs.StringVar(&s.key, "sonarr-key", "", "the sonarr api `key`")
	fs.Int64Var(&s.series, "series", -1, "the series `id` to link")
}

func (s *sonarrCmd) Exec() error {
	conf := &starr.Config{
		URL:    cmp.Or(s.url, os.Getenv("SONARR_URL")),
		APIKey: cmp.Or(s.key, os.Getenv("SONARR_KEY")),
	}

	require("sonarr-url", "SONARR_URL", conf.URL)
	require("sonarr-key", "SONARR_KEY", conf.APIKey)

	client := sonarr.New(conf)

	series, err := client.GetSeries(s.series)
	if err != nil {
		return fmt.Errorf("sonarr: unable to get series for tvdvb id %d: %w", s.series, err)
	}

	eps, err := client.GetSeriesEpisodeFiles(series[0].ID)
	if err != nil {
		return fmt.Errorf("sonarr: unable to get series episode files: %w", err)
	}

	title := strings.ReplaceAll(series[0].Title, " ", ".")

	for _, ep := range eps {
		dst := filepath.Join(globals.dst, fmt.Sprintf("%s.S%02d", title, ep.SeasonNumber))
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("sonarr: unable to create destination: %w", err)
		}

		src := mapPath(ep.Path)
		ext := filepath.Ext(ep.Path)
		link := filepath.Join(dst, ep.SceneName+ext)

		if err := os.Link(src, link); err != nil {
			return fmt.Errorf("sonarr: unable to create link at %s: %w", link, err)
		}
		fmt.Printf("sonarr: created link %s\n", link)
	}

	return nil
}

func getCmd(args []string) cmd {
	if len(args) == 0 {
		return nil
	}

	switch args[0] {
	case "sonarr":
		return &sonarrCmd{}
	}
	return nil
}

func main() {
	cmd := getCmd(os.Args[1:])
	if cmd == nil {
		fmt.Println("starrlink: usage: sonarr <args>")
		os.Exit(2)
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	globalFlags(fs)
	cmd.Flags(fs)

	fs.Parse(os.Args[2:])

	if err := cmd.Exec(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
