package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/mcpserver"
	runsvc "github.com/Vergil2599/startup/pes/internal/run"
	"github.com/Vergil2599/startup/pes/internal/storage/fsrepo"
	"github.com/Vergil2599/startup/pes/internal/validation"
)

func init() { registerIntegrations = addIntegrationCommands }

func addIntegrationCommands(root *cobra.Command, wsPath *string) {
	root.AddCommand(cmdMCP(wsPath), cmdLint(wsPath), cmdGolden(wsPath))
}

func cmdMCP(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Sirve la biblioteca como servidor MCP por stdio (Claude Code, Cursor, agentes)",
		Long: `Expone los prompts del workspace como herramientas MCP:
search_prompts, list_prompts, get_prompt, render_prompt y validate_prompt.

Registro en Claude Code:
  claude mcp add pes -- pes mcp -w /ruta/a/tu/workspace

Registro en Cursor u otros clientes MCP (mcp.json):
  {"mcpServers": {"pes": {"command": "pes", "args": ["mcp", "-w", "/ruta/a/tu/workspace"]}}}`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			return mcpserver.New(s, Version).Serve(cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}

func cmdLint(wsPath *string) *cobra.Command {
	var minScore int
	var format string
	c := &cobra.Command{
		Use:   "lint",
		Short: "Valida TODOS los prompts del workspace (gate para CI)",
		Long: `Recorre prompts/ completo, valida cada prompt y falla (exit 1) si alguno
puntúa por debajo de --min-score o no se puede parsear.

En CI de GitHub, usa --format github para anotaciones en el diff:
  pes lint -w . --min-score 70 --format github`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			out := cmd.OutOrStdout()
			engine := validation.NewEngine()
			vars, err := s.Vars("")
			if err != nil {
				return err
			}

			type row struct {
				path  string
				title string
				score int
			}
			var rows []row
			var broken []string
			walkErr := s.WS.Walk(fsrepo.DirPrompts, func(e *fsrepo.Entry) {
				report := engine.Validate(validation.Input{Prompt: e.Prompt, Vars: vars})
				rows = append(rows, row{e.Path, e.Prompt.Title, report.Score})
				if format == "github" && report.Score < minScore {
					for _, f := range report.Findings {
						fmt.Fprintf(out, "::%s file=%s::%s: %s\n",
							ghLevel(string(f.Severity)), e.Path, f.RuleID, f.Message)
					}
				}
			}, func(path string, perr error) {
				broken = append(broken, fmt.Sprintf("%s: %v", path, perr))
				if format == "github" {
					fmt.Fprintf(out, "::error file=%s::archivo de prompt inválido\n", path)
				}
			})
			if walkErr != nil {
				return walkErr
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].score < rows[j].score })

			failed := 0
			for _, r := range rows {
				mark := "✓"
				if r.score < minScore {
					mark = "✗"
					failed++
				}
				if format != "github" {
					fmt.Fprintf(out, "%s %3d  %-40s %s\n", mark, r.score, truncateStr(r.title, 40), r.path)
				}
			}
			for _, b := range broken {
				fmt.Fprintf(out, "✗ ERR %s\n", b)
			}
			fmt.Fprintf(out, "\n%d prompts · %d bajo el umbral %d · %d ilegibles\n",
				len(rows), failed, minScore, len(broken))
			if failed > 0 || len(broken) > 0 {
				return fmt.Errorf("lint falló")
			}
			return nil
		},
	}
	c.Flags().IntVar(&minScore, "min-score", 60, "puntuación mínima exigida")
	c.Flags().StringVar(&format, "format", "text", "salida: text|github")
	return c
}

func ghLevel(sev string) string {
	switch sev {
	case "error":
		return "error"
	case "warn":
		return "warning"
	default:
		return "notice"
	}
}

func cmdGolden(wsPath *string) *cobra.Command {
	c := &cobra.Command{
		Use:   "golden",
		Short: "Respuestas de referencia para detectar regresiones de prompts",
	}

	var provider string
	set := &cobra.Command{
		Use:   "set <id|ruta.md>",
		Short: "Ejecuta el prompt una vez y guarda la respuesta como referencia",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, svc, result, err := runOnce(cmd, *wsPath, args[0], provider)
			if err != nil {
				return err
			}
			_ = s
			if err := svc.SaveGolden(result); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "golden guardado para %s (%s, %d tokens de respuesta)\n",
				args[0], result.Provider, result.TokensOut)
			return nil
		},
	}
	set.Flags().StringVarP(&provider, "provider", "P", "", "proveedor a usar (por defecto el primero configurado)")

	var threshold float64
	var provider2 string
	check := &cobra.Command{
		Use:   "check <id|ruta.md>",
		Short: "Re-ejecuta el prompt y compara con la referencia (exit 1 si diverge)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, svc, result, err := runOnce(cmd, *wsPath, args[0], provider2)
			if err != nil {
				return err
			}
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			verdict, err := svc.CheckGolden(e.Prompt.ID, result, threshold)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "similitud con golden: %.2f (umbral %.2f)\n", verdict.Similarity, verdict.Threshold)
			if verdict.PromptChanged {
				fmt.Fprintln(out, "aviso: el prompt cambió desde que se grabó la referencia")
			}
			if !verdict.Pass {
				return fmt.Errorf("REGRESIÓN: la respuesta divergió de la referencia (%.2f < %.2f)",
					verdict.Similarity, verdict.Threshold)
			}
			fmt.Fprintln(out, "OK: sin regresión")
			return nil
		},
	}
	check.Flags().Float64Var(&threshold, "threshold", 0.6, "similitud mínima Jaccard (0-1)")
	check.Flags().StringVarP(&provider2, "provider", "P", "", "proveedor a usar")

	c.AddCommand(set, check)
	return c
}

// runOnce ejecuta el prompt una vez contra un proveedor y devuelve el resultado.
func runOnce(cmd *cobra.Command, wsPath, ref, providerName string) (studio studioT, svc *runsvc.Service, result runsvc.Result, err error) {
	s, done, err := openStudio(wsPath)
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	defer done()
	e, err := s.Get(ref)
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	rendered, err := s.Render(ref, "", false)
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	cfgs, err := ai.LoadConfigs(s.WS.Root)
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	if len(cfgs) == 0 {
		return nil, nil, runsvc.Result{}, fmt.Errorf("no hay proveedores en .pes/providers.yaml")
	}
	cfg := cfgs[0]
	if providerName != "" {
		found := false
		for _, c := range cfgs {
			if c.Name == providerName {
				cfg, found = c, true
			}
		}
		if !found {
			return nil, nil, runsvc.Result{}, fmt.Errorf("proveedor no configurado: %q", providerName)
		}
	}
	p, err := ai.Build(cfg)
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	svc = runsvc.NewService(s.WS)
	results, err := svc.Run(cmd.Context(), e.Prompt.ID, rendered,
		[]runsvc.Target{{Provider: p, Model: cfg.Model}}, runsvc.Params{})
	if err != nil {
		return nil, nil, runsvc.Result{}, err
	}
	if results[0].Error != "" {
		return nil, nil, runsvc.Result{}, fmt.Errorf("la ejecución falló: %s", results[0].Error)
	}
	return s, svc, results[0], nil
}

// studioT evita exportar el tipo del studio en la firma del helper.
type studioT interface {
	Get(ref string) (*fsrepo.Entry, error)
}
