package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kennyandries/driftwatch/pkg/config"
	"github.com/kennyandries/driftwatch/pkg/differ"
	disc "github.com/kennyandries/driftwatch/pkg/discovery"
	"github.com/kennyandries/driftwatch/pkg/extras"
	"github.com/kennyandries/driftwatch/pkg/fetcher"
	"github.com/kennyandries/driftwatch/pkg/flux"
	"github.com/kennyandries/driftwatch/pkg/pipeline"
	"github.com/kennyandries/driftwatch/pkg/renderer"
	"github.com/kennyandries/driftwatch/pkg/reporter"
	"github.com/kennyandries/driftwatch/pkg/types"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sdiscovery "k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// restMapperAdapter wraps a RESTMapper to implement fetcher.ResourceMapper.
type restMapperAdapter struct {
	mapper meta.RESTMapper
}

func (a *restMapperAdapter) ResourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return mapping.Resource, nil
}

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan manifests and compare against live cluster state",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Determine scan path
		scanPath := "."
		if len(args) > 0 {
			scanPath = args[0]
		}
		scanPath, err := filepath.Abs(scanPath)
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}

		// 2. Load config if available
		configPath, _ := cmd.Flags().GetString("config")
		var cfg *config.Config
		if _, statErr := os.Stat(configPath); statErr == nil {
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
		}

		// 3. Parse flags
		kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
		kubecontext, _ := cmd.Flags().GetString("context")
		output, _ := cmd.Flags().GetString("output")
		failOn, _ := cmd.Flags().GetString("fail-on")
		fluxMode, _ := cmd.Flags().GetString("flux")

		// Apply config overrides
		if cfg != nil {
			if kubecontext == "" && cfg.Cluster.Context != "" {
				kubecontext = cfg.Cluster.Context
			}
			if failOn == "critical" && cfg.FailOn != "" {
				failOn = cfg.FailOn
			}
			if fluxMode == "auto" && cfg.Flux.Enabled {
				fluxMode = "enabled"
			}
		}

		threshold, err := types.ParseSeverity(failOn)
		if err != nil {
			return fmt.Errorf("invalid fail-on severity: %w", err)
		}

		// 4. Create k8s client (optional — handle missing kubeconfig gracefully)
		var dynClient dynamic.Interface
		var resourceMapper fetcher.ResourceMapper

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loadingRules.ExplicitPath = kubeconfig
		}
		configOverrides := &clientcmd.ConfigOverrides{}
		if kubecontext != "" {
			configOverrides.CurrentContext = kubecontext
		}

		restConfig, restErr := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules, configOverrides).ClientConfig()

		if restErr == nil {
			dynClient, err = dynamic.NewForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create dynamic client: %w", err)
			}

			discoveryClient, err := k8sdiscovery.NewDiscoveryClientForConfig(restConfig)
			if err != nil {
				return fmt.Errorf("failed to create discovery client: %w", err)
			}

			apiResources, err := restmapper.GetAPIGroupResources(discoveryClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not discover API resources: %v\n", err)
				dynClient = nil
			} else {
				rm := restmapper.NewDiscoveryRESTMapper(apiResources)
				resourceMapper = &restMapperAdapter{mapper: rm}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: no kubeconfig available (%v). Running in local-only mode.\n", restErr)
		}

		// 5. Discover sources
		sources, err := disc.Discover(scanPath)
		if err != nil {
			return fmt.Errorf("discovery error: %w", err)
		}

		if len(sources) == 0 {
			fmt.Fprintln(os.Stderr, "No Kubernetes sources discovered.")
			return nil
		}

		// 6. Build ignore fields and severity rules
		ignoreFields := differ.DefaultIgnoreFields()
		severityRules := differ.DefaultSeverityRules()
		if cfg != nil && len(cfg.Ignore.Fields) > 0 {
			ignoreFields = cfg.Ignore.Fields
		}

		// 7. For each source, create renderer and run pipeline
		var allResults []types.DriftResult

		for _, src := range sources {
			var r pipeline.RendererInterface
			sourceInfo := types.SourceInfo{Path: src.Path}

			switch src.Type {
			case "manifest":
				sourceInfo.Type = types.SourceManifest
				r = &renderer.ManifestRenderer{SkipSecrets: true}
			case "kustomize":
				sourceInfo.Type = types.SourceKustomize
				r = &renderer.KustomizeRenderer{SkipSecrets: true}
			case "helm":
				sourceInfo.Type = types.SourceHelm
				r = &renderer.HelmRenderer{SkipSecrets: true}
			default:
				fmt.Fprintf(os.Stderr, "Warning: unknown source type %q for %s, skipping\n", src.Type, src.Path)
				continue
			}

			var f pipeline.FetcherInterface
			if dynClient != nil && resourceMapper != nil {
				f = fetcher.NewFetcher(dynClient, resourceMapper)
			} else {
				f = &noopFetcher{}
			}

			p := &pipeline.Pipeline{
				Renderer:      r,
				Fetcher:       f,
				IgnoreFields:  ignoreFields,
				SeverityRules: severityRules,
				Source:        sourceInfo,
			}

			results, runErr := p.Run(context.Background(), src.Path)
			if runErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: error scanning %s: %v\n", src.Path, runErr)
				continue
			}
			allResults = append(allResults, results...)
		}

		// 8. Flux enrichment
		if dynClient != nil && (fluxMode == "enabled" || fluxMode == "auto") {
			enricher := flux.NewEnricher(dynClient)
			ctx := context.Background()
			if fluxMode == "auto" {
				if enricher.Available(ctx) {
					enricher.Enrich(ctx, allResults)
				}
			} else {
				enricher.Enrich(ctx, allResults)
			}
		}

		// 8b. Extras detection
		detectExtras, _ := cmd.Flags().GetBool("detect-extras")
		if detectExtras && dynClient != nil {
			// Use config values if available, otherwise use defaults
			extrasExclude := config.DefaultExtrasExclude()
			ignoreNS := config.DefaultExtrasIgnoreNamespaces()
			if cfg != nil {
				extrasExclude = cfg.Extras.Exclude
				ignoreNS = cfg.Extras.IgnoreNamespaces
			}
			var excludeKinds []string
			for _, e := range extrasExclude {
				if k, ok := e["kind"]; ok {
					excludeKinds = append(excludeKinds, k)
				}
			}

			detector := &extras.Detector{
				InventoryChecker: &extras.FluxInventoryChecker{Client: dynClient, Stderr: os.Stderr},
				NamespaceScanner: &extras.NamespaceScanner{
					Client:        dynClient,
					ExcludeKinds:  excludeKinds,
					ResourceTypes: commonResourceTypes(),
				},
				NamespaceAuditor: &extras.NamespaceAuditor{
					Client:           dynClient,
					IgnoreNamespaces: ignoreNS,
				},
			}

			extrasResults, extrasErr := detector.Detect(context.Background(), allResults)
			if extrasErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: extras detection error: %v\n", extrasErr)
			} else {
				allResults = append(allResults, extrasResults...)
			}
		}

		// 9. Report
		var rep reporter.Reporter
		switch output {
		case "json":
			rep = reporter.NewJSONReporter(os.Stdout)
		default:
			rep = reporter.NewTerminalReporter(os.Stdout, true)
		}

		if err := rep.Report(allResults); err != nil {
			return fmt.Errorf("report error: %w", err)
		}

		// 10. Exit with code based on threshold
		exitCode := reporter.ExitCode(allResults, threshold)
		if exitCode != 0 {
			os.Exit(exitCode)
		}

		return nil
	},
}

// noopFetcher returns nil for all gets (local-only mode).
type noopFetcher struct{}

func (n *noopFetcher) Get(_ context.Context, _ types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	return nil, nil
}

func commonResourceTypes() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	}
}

func init() {
	scanCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	scanCmd.Flags().String("kubeconfig", "", "Kubeconfig path (defaults to ~/.kube/config)")
	scanCmd.Flags().String("context", "", "Kubernetes context (defaults to current)")
	scanCmd.Flags().StringSlice("namespace", nil, "Limit to namespace(s)")
	scanCmd.Flags().String("source-type", "auto", "Force source type: manifest, helm, kustomize")
	scanCmd.Flags().String("output", "terminal", "Output format: terminal, json")
	scanCmd.Flags().String("fail-on", "critical", "Severity threshold: critical, warning, info")
	scanCmd.Flags().String("flux", "auto", "Flux enrichment: auto, enabled, disabled")
	scanCmd.Flags().Bool("detect-extras", false, "Detect extra resources not in Git")
	rootCmd.AddCommand(scanCmd)
}
