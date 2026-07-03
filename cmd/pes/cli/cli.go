// Package cli define los comandos de la CLI de PES.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Vergil2599/startup/pes/internal/core"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/storage/sqlindex"
)

// Version se fija en build time con -ldflags.
var Version = "1.0.0"

// registerV1 lo instala cli_v1.go en init(); indirección para mantener los
// comandos v1 en su propio archivo.
var registerV1 func(root *cobra.Command, wsPath *string)

// Root construye el árbol de comandos.
func Root() *cobra.Command {
	var wsPath string
	root := &cobra.Command{
		Use:           "pes",
		Short:         "Prompt Engineering Studio — IDE offline-first para prompts",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&wsPath, "workspace", "w", ".", "ruta del workspace")

	root.AddCommand(
		cmdInit(&wsPath),
		cmdNew(&wsPath),
		cmdList(&wsPath),
		cmdSearch(&wsPath),
		cmdRender(&wsPath),
		cmdValidate(&wsPath),
		cmdExport(&wsPath),
		cmdImport(&wsPath),
		cmdReindex(&wsPath),
	)
	if registerV1 != nil {
		registerV1(root, &wsPath)
	}
	return root
}

// openStudio abre workspace + índice y sincroniza incrementalmente.
func openStudio(wsPath string) (*core.Studio, func(), error) {
	ws, err := fsrepo.Open(wsPath)
	if err != nil {
		return nil, nil, err
	}
	ix, err := sqlindex.Open(filepath.Join(ws.Root, fsrepo.DirMeta, "index.db"))
	if err != nil {
		return nil, nil, err
	}
	s := core.NewStudio(ws, ix)
	if _, err := s.Reindex(false); err != nil {
		ix.Close()
		return nil, nil, err
	}
	return s, func() { ix.Close() }, nil
}

func cmdInit(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Inicializa un workspace de PES en la carpeta indicada",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws, err := fsrepo.Init(*wsPath)
			if err != nil {
				return err
			}
			n, err := installStarterTemplates(ws.Root)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "workspace inicializado en %s (%d plantillas de inicio)\n", ws.Root, n)
			return nil
		},
	}
}

func cmdNew(wsPath *string) *cobra.Command {
	var category, templateID string
	var tags []string
	c := &cobra.Command{
		Use:   "new <título>",
		Short: "Crea un prompt nuevo (en blanco o desde plantilla)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			p, rel, err := s.CreatePrompt(core.CreateOpts{
				Title: strings.Join(args, " "), Category: category,
				TemplateID: domain.ID(templateID), Tags: tags,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "creado %s  id=%s\n", rel, p.ID)
			return nil
		},
	}
	c.Flags().StringVarP(&category, "category", "c", "", "categoría")
	c.Flags().StringVarP(&templateID, "template", "t", "", "ID de plantilla")
	c.Flags().StringSliceVar(&tags, "tags", nil, "etiquetas separadas por coma")
	return c
}

func cmdList(wsPath *string) *cobra.Command {
	var tag, category string
	var favorites bool
	c := &cobra.Command{
		Use:   "list",
		Short: "Lista los prompts de la biblioteca",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			hits, err := s.List(sqlindex.ListFilter{Tag: tag, Category: category, Favorites: favorites})
			if err != nil {
				return err
			}
			printHits(cmd, hits)
			return nil
		},
	}
	c.Flags().StringVar(&tag, "tag", "", "filtrar por etiqueta")
	c.Flags().StringVar(&category, "category", "", "filtrar por categoría")
	c.Flags().BoolVar(&favorites, "favorites", false, "solo favoritos")
	return c
}

func cmdSearch(wsPath *string) *cobra.Command {
	var limit int
	c := &cobra.Command{
		Use:   "search <consulta>",
		Short: "Busca prompts por texto (FTS local, con prefijos)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			hits, err := s.Search(strings.Join(args, " "), limit)
			if err != nil {
				return err
			}
			printHits(cmd, hits)
			return nil
		},
	}
	c.Flags().IntVarP(&limit, "limit", "n", 20, "máximo de resultados")
	return c
}

func cmdRender(wsPath *string) *cobra.Command {
	var project string
	var strict bool
	c := &cobra.Command{
		Use:   "render <id|ruta.md>",
		Short: "Renderiza un prompt con sus variables resueltas",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			out, err := s.Render(args[0], project, strict)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", "", "conjunto de variables de proyecto")
	c.Flags().BoolVar(&strict, "strict", false, "fallar si quedan variables sin resolver")
	return c
}

func cmdValidate(wsPath *string) *cobra.Command {
	var project string
	var minScore int
	c := &cobra.Command{
		Use:   "validate <id|ruta.md>",
		Short: "Valida un prompt y muestra su puntuación de calidad",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			report, err := s.Validate(args[0], project)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "puntuación: %d/100\n", report.Score)
			for _, f := range report.Findings {
				loc := ""
				if f.BlockType != "" {
					loc = " [" + string(f.BlockType) + "]"
				}
				fmt.Fprintf(out, "  %-5s %s%s: %s\n", f.Severity, f.RuleID, loc, f.Message)
				if f.Suggestion != "" {
					fmt.Fprintf(out, "        → %s\n", f.Suggestion)
				}
			}
			if report.Score < minScore {
				return fmt.Errorf("puntuación %d por debajo del umbral %d", report.Score, minScore)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", "", "conjunto de variables de proyecto")
	c.Flags().IntVar(&minScore, "min-score", 0, "falla (exit 1) si la puntuación es menor — útil en CI")
	return c
}

func cmdExport(wsPath *string) *cobra.Command {
	var format, outPath, project string
	c := &cobra.Command{
		Use:   "export <id|ruta.md>",
		Short: "Exporta un prompt (md, txt, json, yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			data, ext, err := s.Export(args[0], format, project)
			if err != nil {
				return err
			}
			if outPath == "" {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if !strings.Contains(filepath.Base(outPath), ".") {
				outPath += ext
			}
			if err := fsrepo.WriteFileAtomic(outPath, data); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "exportado a %s\n", outPath)
			return nil
		},
	}
	c.Flags().StringVarP(&format, "format", "f", "md", "formato: md|txt|json|yaml")
	c.Flags().StringVarP(&outPath, "out", "o", "", "archivo de salida (por defecto stdout)")
	c.Flags().StringVarP(&project, "project", "p", "", "conjunto de variables de proyecto")
	return c
}

func cmdImport(wsPath *string) *cobra.Command {
	var title string
	c := &cobra.Command{
		Use:   "import <archivo>",
		Short: "Importa un prompt desde Markdown PES, JSON, YAML o texto plano",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			if title == "" {
				title = strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0]))
			}
			p, rel, err := s.Import(data, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "importado %s  id=%s\n", rel, p.ID)
			return nil
		},
	}
	c.Flags().StringVar(&title, "title", "", "título para el prompt importado")
	return c
}

func cmdReindex(wsPath *string) *cobra.Command {
	var rebuild bool
	c := &cobra.Command{
		Use:   "reindex",
		Short: "Sincroniza el índice de búsqueda con el workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			stats, err := s.Reindex(rebuild)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "indexados=%d sin_cambios=%d purgados=%d errores=%d\n",
				stats.Indexed, stats.Skipped, stats.Removed, stats.Errors)
			return nil
		},
	}
	c.Flags().BoolVar(&rebuild, "rebuild", false, "reconstruir el índice desde cero")
	return c
}

func printHits(cmd *cobra.Command, hits []sqlindex.Hit) {
	out := cmd.OutOrStdout()
	if len(hits) == 0 {
		fmt.Fprintln(out, "(sin resultados)")
		return
	}
	for _, h := range hits {
		fav, score := " ", "  -"
		if h.Favorite {
			fav = "★"
		}
		if h.Score != nil {
			score = fmt.Sprintf("%3d", *h.Score)
		}
		tags := ""
		if len(h.Tags) > 0 {
			tags = "  #" + strings.Join(h.Tags, " #")
		}
		fmt.Fprintf(out, "%s %s  %s  %-40s %s%s\n", fav, score, h.ID, h.Title, h.Path, tags)
	}
}
