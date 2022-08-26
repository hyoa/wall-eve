package cli

import (
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var envFile string

var (
	rootCmd = &cobra.Command{}
)

func Execute() error {
	installCmd.Flags().StringVarP((&envFile), "envFile", "e", "", "env file location")
	return rootCmd.Execute()
}

func loadEnv() error {
	if envFile != "" {
		err := godotenv.Load(envFile)

		if err != nil {
			log.Errorln("Unable to read env file: %w", err)

			return err
		}
	}

	return nil
}
