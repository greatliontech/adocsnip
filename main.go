package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytesparadise/libasciidoc/pkg/configuration"
	"github.com/bytesparadise/libasciidoc/pkg/parser"
	"github.com/bytesparadise/libasciidoc/pkg/types"
	"github.com/spf13/cobra"
)

type pkg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Contributes struct {
		Snippets []snippetFile `json:"snippets"`
	} `json:"contributes"`
}

type snippet struct {
	Name        string   `json:"-"`
	Prefix      string   `json:"prefix"`
	Body        []string `json:"body"`
	Description string   `json:"description,omitempty"`
	Scope       string   `json:"scope,omitempty"`
}

type snippetFile struct {
	Language []string `json:"language"`
	Path     string   `json:"path"`
}

var outPath string

var cmd = &cobra.Command{
	Use:  "adocsnip",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {

		src := args[0]
		p := &pkg{}

		if err := os.MkdirAll(outPath, 0755); err != nil {
			return err
		}

		err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if path == "package.json" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(data, p); err != nil {
					return err
				}
			}
			if strings.HasSuffix(d.Name(), "adoc") {

				l, s, err := parseFile(path)
				if err != nil {
					return err
				}

				out := strings.TrimPrefix(path, filepath.Clean(src))
				out = strings.TrimSuffix(out, "adoc") + "json"
				sf := snippetFile{
					Language: l,
					Path:     "." + out,
				}
				p.Contributes.Snippets = append(p.Contributes.Snippets, sf)

				out = filepath.Join(outPath, out)
				data, err := json.MarshalIndent(s, "", "  ")
				if err != nil {
					return err
				}
				if err := ioutil.WriteFile(out, data, 0666); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(outPath, "package.json"), data, 0666); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	cmd.Flags().StringVarP(&outPath, "out", "o", "./dist", "output path")
}

func main() {
	cmd.Execute()
}

func parseFile(path string) ([]string, map[string]*snippet, error) {

	langs := []string{}
	spts := map[string]*snippet{}

	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	config := &configuration.Configuration{}

	doc, err := parser.ParseDocument(file, config)
	if err != nil {
		return nil, nil, err
	}

	for _, v := range doc.Elements {
		switch e := v.(type) {
		case *types.DocumentHeader:
			langs = strings.Split(e.Title[0].(*types.StringElement).Content, ",")
		case *types.Section:
			snpt, err := sectionToSnipet(e)
			if err != nil {
				return nil, nil, err
			}
			spts[snpt.Name] = snpt
		default:
			return nil, nil, fmt.Errorf("unexpected token in document %T\n", e)
		}
	}

	return langs, spts, nil
}

func (sf snippetFile) MarshalJSON() ([]byte, error) {
	type snippetFileInternal struct {
		Language string `json:"language"`
		Path     string `json:"path"`
	}
	type snippetFileInternalMany struct {
		Language []string `json:"language"`
		Path     string   `json:"path"`
	}
	if len(sf.Language) == 1 {
		sfi := snippetFileInternal{
			Language: sf.Language[0],
			Path:     sf.Path,
		}
		return json.Marshal(sfi)
	}
	sfim := snippetFileInternalMany{
		Language: sf.Language,
		Path:     sf.Path,
	}
	return json.Marshal(sfim)
}

func sectionToSnipet(s *types.Section) (*snippet, error) {
	if s.Level != 1 {
		return nil, fmt.Errorf("not the correct level")
	}

	title := s.Title[0].(*types.StringElement).Content

	snpt := &snippet{
		Name: title,
	}

	for _, v := range s.Elements {
		switch e := v.(type) {
		case *types.Paragraph:
			snpt.Description = e.Elements[0].(*types.StringElement).Content
		case *types.DelimitedBlock:
			snpt.Prefix = e.Attributes.GetAsStringWithDefault("prefix", "")
			if snpt.Prefix == "" {
				return nil, fmt.Errorf("prefix must be defined. snippet: %s", title)
			}
			snpt.Scope = e.Attributes.GetAsStringWithDefault("scope", "")
			snpt.Body = toStringArray(e.Elements[0].(*types.StringElement).Content)
		default:
			return nil, fmt.Errorf("unexpected token in section %T\n", e)
		}
	}
	return snpt, nil
}

func toStringArray(s string) []string {
	out := []string{}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	return out
}
