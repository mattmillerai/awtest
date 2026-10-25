package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattmillerai/awtest/cmd/awtest/formatters"
	"github.com/mattmillerai/awtest/cmd/awtest/services"
	"github.com/mattmillerai/awtest/cmd/awtest/types"
	"github.com/mattmillerai/awtest/cmd/awtest/utils"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/logrusorgru/aurora"
)

var (
	Version   = "dev"
	BuildDate = "unknown"
)

const (
	MinConcurrency = 1
	MaxConcurrency = 20
)

func main() {
	if isDiffInvocation(os.Args) {
		os.Exit(runDiff(os.Args[2:], os.Stdout, os.Stderr))
	}

	awsAccessKeyID := flag.String("access-key-id", "", "AWS Access Key ID")
	awsSecretAccessKey := flag.String("secret-access-key", "", "AWS Secret Access Key")
	awsSessionToken := flag.String("session-token", "", "AWS Session Token (optional)")
	awsRegion := flag.String("region", "us-west-2", "AWS Region")

	awsAccessKeyIDAbbr := flag.String("aki", "", "Abbreviated AWS Access Key ID")
	awsSecretAccessKeyAbbr := flag.String("sak", "", "Abbreviated AWS Secret Access Key")
	awsSessionTokenAbbr := flag.String("st", "", "Abbreviated AWS Session Token")

	outputFormat := flag.String("format", "text", "Output format: text, json, yaml, csv, table")
	outputFile := flag.String("output-file", "", "Write output to file instead of stdout (short: -o)")
	quiet := flag.Bool("quiet", false, "Suppress informational messages, show only findings")

	debug := flag.Bool("debug", false, "Enable debug mode")

	var includeServices string
	var excludeServices string
	flag.StringVar(&includeServices, "services", "", "Include only specific services (comma-separated, e.g., s3,ec2,iam)")
	flag.StringVar(&excludeServices, "exclude-services", "", "Exclude specific services (comma-separated, e.g., cloudwatch,cloudtrail)")

	endpointURL := flag.String("endpoint-url", "", "Custom AWS-compatible endpoint URL (e.g. http://localhost:4566) used for ALL services; env AWS_ENDPOINT_URL is used if unset")

	allRegions := flag.Bool("all-regions", false, "Scan every enabled AWS region instead of the default four US regions (many more API calls and CloudTrail events; see README)")

	timeout := flag.Duration("timeout", 5*time.Minute, "Maximum scan timeout duration (e.g., 5m, 300s)")

	concurrency := flag.Int("concurrency", MinConcurrency, "Number of concurrent service scans (1-20)")
	speed := flag.String("speed", SpeedSafe, "Speed preset: safe, fast, insane")

	version := flag.Bool("version", false, "Print version and build date (short: -v)")
	checkUpdate := flag.Bool("check-update", false, "Check if a newer version is available")

	registerShortFlags(flag.CommandLine, outputFile, version)

	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), "Usage:\n  awtest [flags]\n  awtest diff [-format text|json] <before.json> <after.json>\n\nCommands:\n  diff    Compare two JSON scan outputs offline (exit 0: no differences, 1: differences, 2: error)\n\nUse -endpoint-url to target LocalStack or a mock instead of AWS.\nUse -all-regions to scan every enabled region (opt-in).\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// Detect if --concurrency was explicitly set on the command line
	concurrencyExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "concurrency" {
			concurrencyExplicit = true
		}
	})

	if *version {
		fmt.Printf("awtest %s (built %s)\n", Version, BuildDate)
		os.Exit(0)
	}

	if *checkUpdate {
		msg, warn := checkForUpdate(Version)
		if warn != "" {
			fmt.Fprintln(os.Stderr, warn)
		}
		if msg != "" {
			fmt.Print(msg)
		}
		os.Exit(0)
	}

	endpoint, err := utils.ResolveEndpointURL(*endpointURL, os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(2)
	}

	utils.Quiet = *quiet

	// Resolve speed preset and concurrency
	speedResult, err := resolveSpeedAndConcurrency(*speed, *concurrency, concurrencyExplicit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}

	// Start background update check (non-blocking, only for release builds)
	updateCh := make(chan updateNotice, 1)
	if !*quiet && Version != "dev" {
		go func() {
			msg, warn := checkForUpdate(Version)
			updateCh <- updateNotice{msg, warn}
		}()
	}

	if !*quiet {
		fmt.Fprintln(os.Stderr, "     /\\ \\        / /__   __|      | |")
		fmt.Fprintln(os.Stderr, "    /  \\ \\  /\\  / /   | | ___  ___| |_")
		fmt.Fprintln(os.Stderr, "   / /\\ \\ \\/  \\/ /    | |/ _ \\/ __| __|")
		fmt.Fprintln(os.Stderr, "  / ____ \\  /\\  /     | |  __/\\__ \\ |_")
		fmt.Fprintln(os.Stderr, " /_/    \\_\\/  \\/      |_|\\___||___/\\__|")
		fmt.Fprintln(os.Stderr, "----------------------------------------")
		fmt.Fprintln(os.Stderr, "Version:", Version)
		fmt.Fprintf(os.Stderr, "Speed: %s (concurrency: %d)\n", speedResult.Preset, speedResult.Concurrency)
		fmt.Fprintln(os.Stderr, "----------------------------------------")
	}

	if *awsAccessKeyIDAbbr != "" {
		awsAccessKeyID = awsAccessKeyIDAbbr
	}
	if *awsSecretAccessKeyAbbr != "" {
		awsSecretAccessKey = awsSecretAccessKeyAbbr
	}
	if *awsSessionTokenAbbr != "" {
		awsSessionToken = awsSessionTokenAbbr
	}

	if *awsAccessKeyID == "" {
		*awsAccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if *awsSecretAccessKey == "" {
		*awsSecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if *awsSessionToken == "" {
		*awsSessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}

	// Check if AWS_PROFILE is set and no parameters were provided
	awsProfile := os.Getenv("AWS_PROFILE")
	var sess *session.Session

	if *awsAccessKeyID == "" || *awsSecretAccessKey == "" {
		if awsProfile != "" {
			// Use the AWS_PROFILE if set and no access keys are provided
			sess, err = session.NewSessionWithOptions(session.Options{
				Profile:           awsProfile,
				SharedConfigState: session.SharedConfigEnable,
				Config: aws.Config{
					Region: aws.String(*awsRegion),
				},
			})
		} else {
			// Fall back to default shared config if no profile is set
			sess, err = session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
				Config: aws.Config{
					Region: aws.String(*awsRegion),
				},
			})
		}
		if err != nil {
			fmt.Println("Failed to create session with shared config: ", err)
			return
		}
	} else {
		// Check if the access key starts with 'ASIA'
		if strings.HasPrefix(*awsAccessKeyID, "ASIA") && *awsSessionToken != "" {
			// Use the session token as well
			sess, _ = session.NewSession(&aws.Config{
				Region:      aws.String(*awsRegion),
				Credentials: credentials.NewStaticCredentials(*awsAccessKeyID, *awsSecretAccessKey, *awsSessionToken),
			})
		} else {
			// If keys are provided, use them to create session without session token
			sess, _ = session.NewSession(&aws.Config{
				Region:      aws.String(*awsRegion),
				Credentials: credentials.NewStaticCredentials(*awsAccessKeyID, *awsSecretAccessKey, ""),
			})
		}
	}

	utils.ApplyEndpoint(sess, endpoint)
	endpointNotice := ""
	if endpoint != "" {
		endpointNotice = fmt.Sprintf("Using custom endpoint: %s (results do not come from real AWS)", endpoint)
		if !*quiet {
			fmt.Fprintln(os.Stderr, endpointNotice)
		}
	}

	if *debug {
		fmt.Println("Debug mode enabled")
		fmt.Println("-----------------------------")
		fmt.Println("Using the following AWS configuration:")

		// Get credentials from session if they are not provided explicitly
		creds, err := sess.Config.Credentials.Get()
		if err != nil {
			fmt.Println("Failed to retrieve credentials from session: ", err)
		} else {
			fmt.Println("Access Key ID:", creds.AccessKeyID)
			fmt.Println("Secret Access Key:", utils.MaskSecret(creds.SecretAccessKey))
			if creds.SessionToken != "" {
				fmt.Println("Session Token:", utils.MaskSecret(creds.SessionToken))
			}
		}

		if awsProfile != "" {
			fmt.Println("Profile:", awsProfile)
		}
		fmt.Println("Region:", *awsRegion)
		if endpointNotice != "" {
			fmt.Println(endpointNotice)
		}
		fmt.Println("-----------------------------")
	}

	// Validate format flag early
	formatter, err := getFormatter(*outputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	startTime := time.Now()

	allSvcs := services.AllServices()
	filteredSvcs := services.FilterServices(allSvcs, includeServices, excludeServices)
	if len(filteredSvcs) == 0 {
		fmt.Fprintln(os.Stderr, "No services matched filter criteria")
		os.Exit(1)
	}
	if includeServices != "" || excludeServices != "" {
		fmt.Fprintf(os.Stderr, "Scanning %d of %d services...\n", len(filteredSvcs), len(allSvcs))
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var regionStats *regionFailures
	if *allRegions {
		regions, source := discoverRegions(ctx, sess)
		regions = orderRegions(regions, *awsRegion)
		regionStats = &regionFailures{}
		ctx = withAllRegions(ctx, &allRegionsConfig{regions: regions, failures: regionStats})
		if !*quiet {
			fmt.Fprintf(os.Stderr, "All-regions mode: scanning %d regions (source: %s). This multiplies API calls roughly %d-fold and creates much more CloudTrail activity; consider -speed fast and a longer -timeout.\n", len(regions), source, len(regions))
		}
	}

	if *allRegions && !*debug {
		utils.SetErrorDedupe(true)
	}

	results, skippedServices := scanServices(ctx, filteredSvcs, sess, speedResult.Concurrency, *quiet, *debug)

	if regionStats != nil && !*quiet {
		printRegionFailures(os.Stderr, regionStats, *debug, utils.SuppressedErrorLines())
	}

	if len(skippedServices) > 0 {
		fmt.Fprintf(os.Stderr, "\nScan timeout reached after %s. %d services not scanned:\n", *timeout, len(skippedServices))
		for _, name := range skippedServices {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
	}

	summary := types.GenerateSummary(results, startTime)

	// For text format to stdout, results are already printed by Process() methods
	// Unless quiet mode is set — then we need to use the formatter
	if *outputFormat == "text" && *outputFile == "" && !*quiet {
		printTextSummary(summary)
		printUpdateNotice(updateCh)
		return
	}

	// For all other cases, use formatter
	// Quiet mode suppresses summary (AC6) — use Format() instead of FormatWithSummary()
	var formatted string
	if *quiet {
		formatted, err = formatter.Format(results)
	} else {
		formatted, err = formatter.FormatWithSummary(results, summary)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, []byte(formatted), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Output written to %s\n", *outputFile)
	} else {
		fmt.Print(formatted)
	}

	if !*quiet {
		printUpdateNotice(updateCh)
	}
}

// printRegionFailures prints the one-line -all-regions failure summary and,
// with debug, one line per failure.
func printRegionFailures(w io.Writer, f *regionFailures, debug bool, suppressed int) {
	total, regions := f.summary()
	note := ""
	if suppressed > 0 {
		note = fmt.Sprintf(" %d repeated per-region error line(s) were suppressed.", suppressed)
	}
	if total == 0 {
		if suppressed > 0 {
			fmt.Fprintf(w, "All-regions:%s Use -debug to see every line.\n", note)
		}
		return
	}
	shown := regions
	suffix := ""
	if len(shown) > 10 {
		shown = shown[:10]
		suffix = ", ..."
	}
	fmt.Fprintf(w, "All-regions: %d region-level call(s) failed in %d region(s) (opt-in region not enabled or service unavailable): %s%s; re-run with -debug for details.%s\n", total, len(regions), strings.Join(shown, ", "), suffix, note)
	if debug {
		for _, fl := range f.all() {
			fmt.Fprintf(w, "  %s %s: %s\n", fl.region, fl.call, fl.detail)
		}
	}
}

// registerShortFlags binds short aliases to existing flag variables: -o for
// -output-file and -v for -version.
func registerShortFlags(fs *flag.FlagSet, outputFile *string, version *bool) {
	fs.StringVar(outputFile, "o", "", "Shorthand for -output-file")
	fs.BoolVar(version, "v", false, "Shorthand for -version")
}

type updateNotice struct {
	message string
	warning string
}

// printUpdateNotice checks if a background update result is available and prints it.
// Non-blocking: if the goroutine hasn't finished yet, it silently skips.
func printUpdateNotice(ch chan updateNotice) {
	select {
	case result := <-ch:
		if result.message != "" && !strings.Contains(result.message, "up to date") {
			fmt.Fprintf(os.Stderr, "\n%s", result.message)
		}
	default:
		// Update check still in progress — don't block
	}
}

// printTextSummary prints a scan summary to stderr for the default text+stdout path.
func printTextSummary(summary types.ScanSummary) {
	au := aurora.NewAurora(isTerminal(int(os.Stderr.Fd())))
	fmt.Fprintf(os.Stderr, "========================================\n")
	fmt.Fprintf(os.Stderr, "Scan Summary\n")
	fmt.Fprintf(os.Stderr, "========================================\n")
	fmt.Fprintf(os.Stderr, "Timestamp:          %s\n", summary.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "Duration:           %s\n", summary.ScanDuration)
	fmt.Fprintf(os.Stderr, "Total Services:     %d\n", summary.TotalServices)
	fmt.Fprintf(os.Stderr, "Accessible:         %d\n", summary.AccessibleServices)
	fmt.Fprintf(os.Stderr, "Access Denied:      %d\n", summary.AccessDeniedServices)
	fmt.Fprintf(os.Stderr, "Resources Found:    %d\n", summary.TotalResources)
	methodLines := types.FormatAccessibleMethods(summary.AccessibleMethodNames, func(name string) string {
		return au.BrightGreen(name).String()
	})
	for _, line := range methodLines {
		fmt.Fprintf(os.Stderr, "%s\n", line)
	}
	fmt.Fprintf(os.Stderr, "========================================\n")
}

// getFormatter returns the appropriate OutputFormatter for the given format string.
func getFormatter(format string) (formatters.OutputFormatter, error) {
	switch strings.ToLower(format) {
	case "text":
		return formatters.NewTextFormatter(), nil
	case "json":
		return formatters.NewJSONFormatter(), nil
	case "yaml":
		return formatters.NewYAMLFormatter(), nil
	case "csv":
		return formatters.NewCSVFormatter(), nil
	case "table":
		return formatters.NewTableFormatter(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: text, json, yaml, csv, table)", format)
	}
}

// validateConcurrency checks that the concurrency value is within the allowed range.
func validateConcurrency(val int) error {
	if val < MinConcurrency {
		return fmt.Errorf("Concurrency must be >= %d", MinConcurrency)
	}
	if val > MaxConcurrency {
		return fmt.Errorf("Concurrency must be <= %d", MaxConcurrency)
	}
	return nil
}

// scanServices executes service scans and collects results.
// When concurrency is 1, services run sequentially (Phase 1 behavior).
// When concurrency > 1, services run concurrently via a worker pool.
func scanServices(ctx context.Context, svcs []types.AWSService, sess *session.Session, concurrency int, quiet, debug bool) ([]types.ScanResult, []string) {
	// Sequential mode: preserve Phase 1 behavior exactly
	if concurrency <= 1 {
		var results []types.ScanResult
		var skippedServices []string
		for _, service := range svcs {
			select {
			case <-ctx.Done():
				skippedServices = append(skippedServices, service.Name)
				continue
			default:
				if !quiet {
					fmt.Fprintf(os.Stderr, "Scanning %s...\n", service.Name)
				}
				serviceResults, _ := scanService(ctx, service, sess, debug)
				results = append(results, serviceResults...)
			}
		}
		return results, skippedServices
	}

	// Concurrent mode: buffer inline output so progress display is clean
	utils.ConcurrentMode = true
	progress := newProgressReporter(len(svcs), quiet)
	progress.Start()
	results, skipped := runWorkerPool(ctx, svcs, sess, concurrency, quiet, debug, progress)
	progress.Stop()
	utils.FlushOutput()
	return results, skipped
}
