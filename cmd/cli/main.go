package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"openai_extract/internal/extract"
	"openai_extract/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	baseName := filepath.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   baseName + " -f <archive_file.zip> -p <pattern> [-p <pattern> ...] -o <output_folder> [--content-type code,code_interpreter] [--language python,go]",
		Short: "Extract full conversations from an OpenAI ChatGPT export ZIP by multiple patterns (AND), with optional content-type/language filters",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			viper.SetEnvPrefix("openai_search")
			viper.AutomaticEnv()
			if viper.GetString("file") == "" {
				return errors.New("missing required flag: -f, --file")
			}
			if len(viper.GetStringSlice("pattern")) == 0 {
				return errors.New("missing required flag: -p, --pattern (repeat -p to AND multiple patterns)")
			}
			if viper.GetString("output") == "" {
				return errors.New("missing required flag: -o, --output")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			archiveFilePath := viper.GetString("file")
			searchPatterns := viper.GetStringSlice("pattern")
			outputRoot := viper.GetString("output")
			contentTypes := viper.GetStringSlice("content-type")
			languagesRaw := viper.GetStringSlice("language")

			languages := make([]string, 0, len(languagesRaw))
			for _, raw := range languagesRaw {
				for _, piece := range strings.Split(raw, ",") {
					trimmed := strings.TrimSpace(piece)
					if trimmed != "" {
						languages = append(languages, trimmed)
					}
				}
			}
			return extract.Run(archiveFilePath, searchPatterns, outputRoot, contentTypes, languages)
		},
	}

	rootCmd.Flags().StringP("file", "f", "", "Path to the OpenAI ChatGPT ZIP archive (required)")
	rootCmd.Flags().StringP("output", "o", "", "Output folder (required)")
	rootCmd.Flags().StringSliceP("pattern", "p", nil,
		"Case-insensitive search terms or raw regexes; repeat -p to AND multiple patterns (all must match)")
	rootCmd.Flags().StringSlice("content-type", nil,
		"Require ALL of these content types to be present (comma-separated or repeated flag)")
	rootCmd.Flags().StringSliceP("language", "l", nil,
		"Require ALL of these languages to be present (comma-separated or repeated flag). Example: -l python -l go,js")

	_ = viper.BindPFlag("file", rootCmd.Flags().Lookup("file"))
	_ = viper.BindPFlag("pattern", rootCmd.Flags().Lookup("pattern"))
	_ = viper.BindPFlag("output", rootCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("content-type", rootCmd.Flags().Lookup("content-type"))
	_ = viper.BindPFlag("language", rootCmd.Flags().Lookup("language"))

	// Support -ct shorthand → --content-type
	rootCmd.SetArgs(utils.NormalizeCTShorthand(os.Args[1:]))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
