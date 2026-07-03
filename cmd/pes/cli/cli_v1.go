package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Vergil2599/startup/pes/internal/ai"
	"github.com/Vergil2599/startup/pes/internal/backup"
	"github.com/Vergil2599/startup/pes/internal/composer"
	"github.com/Vergil2599/startup/pes/internal/diff"
	"github.com/Vergil2599/startup/pes/internal/domain"
	"github.com/Vergil2599/startup/pes/internal/history"
	"github.com/Vergil2599/startup/pes/internal/optimize"
	"github.com/Vergil2599/startup/pes/internal/plugin"
	runsvc "github.com/Vergil2599/startup/pes/internal/run"
	"github.com/Vergil2599/startup/pes/internal/server"
)

func init() { registerV1 = addV1Commands }

// addV1Commands añade los comandos del hito v1 al árbol de la CLI.
func addV1Commands(root *cobra.Command, wsPath *string) {
	root.AddCommand(
		cmdUI(wsPath),
		cmdHistory(wsPath),
		cmdSnapshot(wsPath),
		cmdRestore(wsPath),
		cmdDiff(wsPath),
		cmdCompose(wsPath),
		cmdRun(wsPath),
		cmdOptimize(wsPath),
		cmdBackup(wsPath),
		cmdPlugin(wsPath),
		cmdDuplicate(wsPath),
		cmdVars(wsPath),
		cmdBench(wsPath),
	)
}

func cmdUI(wsPath *string) *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:   "ui",
		Short: "Lanza la interfaz web local de PES",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			var optim *optimize.Client
			if aiCmd, aerr := optimize.DetectCommand(); aerr == nil {
				if client, cerr := optimize.New(aiCmd); cerr == nil {
					optim = client
					defer optim.Close()
				}
			}
			srv := server.New(s, optim, Version)
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			ai := "IA no disponible (opcional)"
			if optim != nil {
				ai = "IA disponible (pes-ai)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "PES escuchando en http://%s  ·  %s\nCtrl+C para salir.\n", ln.Addr(), ai)
			httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
			return httpSrv.Serve(ln)
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:8787", "dirección de escucha (solo localhost)")
	return c
}

func cmdSnapshot(wsPath *string) *cobra.Command {
	var comment string
	c := &cobra.Command{
		Use:   "snapshot <id|ruta.md>",
		Short: "Crea una versión del estado actual del prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			v, err := history.New(s.WS).Snapshot(e.Path, "", comment)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "versión v%d creada (%s)\n", v.Seq, v.ContentHash[:12])
			return nil
		},
	}
	c.Flags().StringVarP(&comment, "message", "m", "", "comentario de la versión")
	return c
}

func cmdHistory(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "history <id|ruta.md>",
		Short: "Lista las versiones de un prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			versions, err := history.New(s.WS).List(e.Prompt.ID)
			if err != nil {
				return err
			}
			if len(versions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(sin versiones: usa `pes snapshot`)")
				return nil
			}
			for _, v := range versions {
				fmt.Fprintf(cmd.OutOrStdout(), "v%-3d %s  %s  %s\n",
					v.Seq, v.CreatedAt.Format("2006-01-02 15:04"), v.ContentHash[:12], v.Comment)
			}
			return nil
		},
	}
}

func cmdRestore(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id|ruta.md> <versión>",
		Short: "Restaura una versión anterior (la historia nunca se reescribe)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			v, err := history.New(s.WS).Restore(e.Path, strings.TrimPrefix(args[1], "v"))
			if err != nil {
				return err
			}
			if _, err := s.Reindex(false); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "restaurado; nueva versión v%d\n", v.Seq)
			return nil
		},
	}
}

func cmdDiff(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "diff <a> <b>",
		Short: "Compara dos prompts (estructural + texto)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			ea, err := s.Get(args[0])
			if err != nil {
				return err
			}
			eb, err := s.Get(args[1])
			if err != nil {
				return err
			}
			st := diff.Structural(ea.Prompt, eb.Prompt)
			out := cmd.OutOrStdout()
			if st.Equal {
				fmt.Fprintln(out, "los prompts son estructuralmente idénticos")
			}
			for _, b := range st.Blocks {
				fmt.Fprintf(out, "%-10s %s\n", b.Kind, b.Type)
			}
			fmt.Fprintf(out, "palabras: %d → %d\n\n", st.WordsA, st.WordsB)
			ta, _ := s.Render(args[0], "", false)
			tb, _ := s.Render(args[1], "", false)
			fmt.Fprint(out, diff.Unified(ta, tb))
			return nil
		},
	}
}

func cmdCompose(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "compose <composición.yaml>",
		Short: "Materializa una composición de capas en un prompt nuevo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			c, err := composer.ParseComposition(data)
			if err != nil {
				return err
			}
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			p, err := composer.Compose(c, func(ref string) (*domain.Prompt, error) {
				e, gerr := s.Get(ref)
				if gerr != nil {
					return nil, gerr
				}
				return e.Prompt, nil
			})
			if err != nil {
				return err
			}
			rel, err := s.Save(p, "")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compuesto %s  id=%s\n", rel, p.ID)
			return nil
		},
	}
}

func cmdRun(wsPath *string) *cobra.Command {
	var providers []string
	var temperature float64
	var maxTokens int
	c := &cobra.Command{
		Use:   "run <id|ruta.md>",
		Short: "Ejecuta el prompt contra proveedores LLM configurados (.pes/providers.yaml)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			rendered, err := s.Render(args[0], "", false)
			if err != nil {
				return err
			}
			cfgs, err := ai.LoadConfigs(s.WS.Root)
			if err != nil {
				return err
			}
			if len(cfgs) == 0 {
				return fmt.Errorf("no hay proveedores en .pes/providers.yaml")
			}
			byName := map[string]ai.ProviderConfig{}
			for _, c := range cfgs {
				byName[c.Name] = c
			}
			if len(providers) == 0 {
				for _, c := range cfgs {
					providers = append(providers, c.Name)
				}
			}
			var targets []runsvc.Target
			for _, name := range providers {
				cfg, ok := byName[name]
				if !ok {
					return fmt.Errorf("proveedor no configurado: %q", name)
				}
				p, berr := ai.Build(cfg)
				if berr != nil {
					return berr
				}
				targets = append(targets, runsvc.Target{Provider: p, Model: cfg.Model})
			}
			results, err := runsvc.NewService(s.WS).Run(cmd.Context(), e.Prompt.ID, rendered,
				targets, runsvc.Params{Temperature: temperature, MaxTokens: maxTokens})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, r := range results {
				fmt.Fprintf(out, "── %s (%s) · %dms · %d/%d tok ──\n", r.Provider, r.Model, r.LatencyMS, r.TokensIn, r.TokensOut)
				if r.Error != "" {
					fmt.Fprintf(out, "⚠ %s\n\n", r.Error)
				} else {
					fmt.Fprintf(out, "%s\n\n", r.Response)
				}
			}
			return nil
		},
	}
	c.Flags().StringSliceVarP(&providers, "provider", "P", nil, "proveedores a usar (por defecto todos)")
	c.Flags().Float64Var(&temperature, "temperature", 0, "temperatura")
	c.Flags().IntVar(&maxTokens, "max-tokens", 0, "máximo de tokens de salida")
	return c
}

func cmdOptimize(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "optimize <id|ruta.md>",
		Short: "Sugerencias de mejora del sidecar pes-ai (opcional; nunca modifica el prompt)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			aiCmd, err := optimize.DetectCommand()
			if err != nil {
				return err
			}
			client, err := optimize.New(aiCmd)
			if err != nil {
				return err
			}
			defer client.Close()
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			sugs, err := client.Suggest(ctx, e.Prompt)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(sugs) == 0 {
				fmt.Fprintln(out, "sin sugerencias: el prompt ya está en buena forma")
				return nil
			}
			for _, sg := range sugs {
				fmt.Fprintf(out, "[%s] (%s) %s\n", sg.Kind, sg.BlockType, sg.Message)
				if sg.Replacement != "" {
					fmt.Fprintf(out, "    propuesta: %s\n", sg.Replacement)
				}
				if sg.ReplacementHint != "" {
					fmt.Fprintf(out, "    → %s\n", sg.ReplacementHint)
				}
			}
			return nil
		},
	}
}

func cmdBackup(wsPath *string) *cobra.Command {
	var passphrase string
	var keep int
	c := &cobra.Command{
		Use:   "backup",
		Short: "Crea un backup del workspace (tar.gz, cifrado age opcional) y rota los antiguos",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			path, err := backup.Create(s.WS, passphrase)
			if err != nil {
				return err
			}
			removed, err := backup.Rotate(s.WS, keep)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "backup creado: %s (rotados %d antiguos)\n", path, removed)
			return nil
		},
	}
	c.Flags().StringVar(&passphrase, "passphrase", "", "cifra el backup con age")
	c.Flags().IntVar(&keep, "keep", 7, "backups a conservar")
	return c
}

func cmdPlugin(wsPath *string) *cobra.Command {
	c := &cobra.Command{Use: "plugin", Short: "Gestiona plugins (JSON-RPC/stdio)"}
	c.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Lista los plugins descubiertos y su estado",
		RunE: func(cmd *cobra.Command, args []string) error {
			ds, err := plugin.Discover(*wsPath)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(ds) == 0 {
				fmt.Fprintln(out, "(sin plugins: colócalos en .pes/plugins/<id>/)")
				return nil
			}
			for _, d := range ds {
				if d.Manifest == nil {
					fmt.Fprintf(out, "✗ %s: %s\n", d.Dir, d.LoadErr)
					continue
				}
				state := "deshabilitado (usa `pes plugin enable`)"
				if d.Enabled {
					state = "habilitado"
				}
				fmt.Fprintf(out, "%s %s v%s [%s] — %s\n", map[bool]string{true: "●", false: "○"}[d.Enabled],
					d.Manifest.Plugin.ID, d.Manifest.Plugin.Version,
					strings.Join(d.Manifest.Contributes.ExtensionPoints, ", "), state)
			}
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "enable <id>",
		Short: "Habilita un plugin (consentimiento explícito)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := plugin.Enable(*wsPath, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "plugin %s habilitado\n", args[0])
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "disable <id>",
		Short: "Deshabilita un plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := plugin.Disable(*wsPath, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "plugin %s deshabilitado\n", args[0])
			return nil
		},
	})
	return c
}
