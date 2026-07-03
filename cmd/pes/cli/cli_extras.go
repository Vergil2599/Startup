package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Vergil2599/startup/pes/internal/ai"
	runsvc "github.com/Vergil2599/startup/pes/internal/run"
)

func cmdDuplicate(wsPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "duplicate <id|ruta.md>",
		Short: "Crea una copia independiente de un prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			p, rel, err := s.Duplicate(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "duplicado %s  id=%s\n", rel, p.ID)
			return nil
		},
	}
}

func cmdVars(wsPath *string) *cobra.Command {
	var project string
	c := &cobra.Command{
		Use:   "vars",
		Short: "Lista las variables (globales o de un proyecto)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			name := "global"
			if project != "" {
				name = project
			}
			vars, err := s.WS.LoadVariables(name)
			if err != nil {
				return err
			}
			if len(vars) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "(sin variables en variables/%s.yaml)\n", name)
				return nil
			}
			keys := make([]string, 0, len(vars))
			for k := range vars {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s = %s\n", k, vars[k])
			}
			return nil
		},
	}
	c.PersistentFlags().StringVarP(&project, "project", "p", "", "conjunto de variables de proyecto")

	c.AddCommand(&cobra.Command{
		Use:   "set <nombre> <valor>",
		Short: "Define o actualiza una variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			name := "global"
			if project != "" {
				name = project
			}
			vars, err := s.WS.LoadVariables(name)
			if err != nil {
				return err
			}
			vars[args[0]] = args[1]
			if err := s.WS.SaveVariables(name, vars); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s (variables/%s.yaml)\n", args[0], args[1], name)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use:   "rm <nombre>",
		Short: "Elimina una variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, done, err := openStudio(*wsPath)
			if err != nil {
				return err
			}
			defer done()
			name := "global"
			if project != "" {
				name = project
			}
			vars, err := s.WS.LoadVariables(name)
			if err != nil {
				return err
			}
			if _, ok := vars[args[0]]; !ok {
				return fmt.Errorf("la variable %q no existe en variables/%s.yaml", args[0], name)
			}
			delete(vars, args[0])
			if err := s.WS.SaveVariables(name, vars); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s eliminada\n", args[0])
			return nil
		},
	})
	return c
}

func cmdBench(wsPath *string) *cobra.Command {
	var providers []string
	var reps int
	var temperature float64
	c := &cobra.Command{
		Use:   "bench <id|ruta.md>",
		Short: "Benchmark: N repeticiones contra cada proveedor, con métricas agregadas",
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
			for _, cfg := range cfgs {
				byName[cfg.Name] = cfg
			}
			if len(providers) == 0 {
				for _, cfg := range cfgs {
					providers = append(providers, cfg.Name)
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
			report, err := runsvc.NewService(s.WS).Benchmark(cmd.Context(), e.Prompt.ID,
				rendered, targets, runsvc.Params{Temperature: temperature}, reps)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "benchmark %s · %d reps × %d proveedores\n\n", report.ID, reps, len(targets))
			fmt.Fprintf(out, "%-16s %-14s %6s %6s %8s %8s %9s %12s\n",
				"proveedor", "modelo", "reps", "fallos", "p50(ms)", "p95(ms)", "tok/resp", "consistencia")
			for _, sum := range report.Summaries {
				cons := "n/a"
				if sum.Consistency >= 0 {
					cons = fmt.Sprintf("%.2f", sum.Consistency)
				}
				fmt.Fprintf(out, "%-16s %-14s %6d %6d %8d %8d %9.1f %12s\n",
					sum.Provider, truncateStr(sum.Model, 14), sum.Reps, sum.Failures,
					sum.LatencyP50MS, sum.LatencyP95MS, sum.TokensOutAvg, cons)
			}
			return nil
		},
	}
	c.Flags().StringSliceVarP(&providers, "provider", "P", nil, "proveedores (por defecto todos)")
	c.Flags().IntVarP(&reps, "reps", "n", 3, "repeticiones por proveedor")
	c.Flags().Float64Var(&temperature, "temperature", 0, "temperatura")
	return c
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

var _ = strings.TrimSpace
